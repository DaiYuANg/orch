package ingress

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/DaiYuANg/warden/internal/config"
	"github.com/DaiYuANg/warden/internal/registry"
	"github.com/samber/lo"
	"github.com/valyala/fasthttp"
	"go.uber.org/fx"
)

type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
)

type Route struct {
	Protocol   Protocol
	Host       string
	PathPrefix string
	ListenPort int
	Backend    string
}

type Ingress struct {
	httpAddr string
	logger   *slog.Logger
	registry *registry.Service

	mu sync.RWMutex

	tcpListeners map[int]*tcpListener
	udpListeners map[int]*udpListener
	httpCache    map[string]httpBackendCacheItem
	httpCacheTTL time.Duration

	httpServer *fasthttp.Server
	stopCh     chan struct{}
}

type httpBackendCacheItem struct {
	backend   string
	expiresAt time.Time
}

var Module = fx.Module("ingress", fx.Provide(newIngress), fx.Invoke(lifecycle))

type newIngressDependency struct {
	fx.In
	Config   *config.Config
	Logger   *slog.Logger
	Registry *registry.Service
}

func newIngress(dep newIngressDependency) *Ingress {
	httpAddr := dep.Config.Network.IngressHTTPListen
	if strings.TrimSpace(httpAddr) == "" {
		httpAddr = ":8088"
	}
	return &Ingress{
		httpAddr:     httpAddr,
		logger:       dep.Logger,
		registry:     dep.Registry,
		tcpListeners: make(map[int]*tcpListener),
		udpListeners: make(map[int]*udpListener),
		httpCache:    make(map[string]httpBackendCacheItem),
		httpCacheTTL: 2 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

func lifecycle(lc fx.Lifecycle, ingress *Ingress) {
	lc.Append(fx.StartStopHook(
		func() error {
			return ingress.Start()
		},
		func() error {
			return ingress.Stop()
		},
	))
}

func (i *Ingress) Start() error {
	i.httpServer = &fasthttp.Server{
		Handler:            i.handleHTTP,
		ReadTimeout:        10 * time.Second,
		WriteTimeout:       10 * time.Second,
		MaxRequestBodySize: 10 * 1024 * 1024,
	}

	go func() {
		if err := i.httpServer.ListenAndServe(i.httpAddr); err != nil {
			i.logger.Error("ingress http listen failed", "error", err, "addr", i.httpAddr)
		}
	}()

	go i.syncStreamRoutesLoop()
	return nil
}

func (i *Ingress) Stop() error {
	select {
	case <-i.stopCh:
	default:
		close(i.stopCh)
	}

	i.mu.Lock()
	lo.ForEach(lo.Keys(i.tcpListeners), func(port int, _ int) {
		_ = i.unregisterTCP(Route{ListenPort: port})
	})
	lo.ForEach(lo.Keys(i.udpListeners), func(port int, _ int) {
		_ = i.unregisterUDP(Route{ListenPort: port})
	})
	i.httpCache = make(map[string]httpBackendCacheItem)
	i.mu.Unlock()

	if i.httpServer != nil {
		return i.httpServer.Shutdown()
	}
	return nil
}

func (i *Ingress) handleHTTP(ctx *fasthttp.RequestCtx) {
	host := normalizeHostWithPort(string(ctx.Host()))
	path := string(ctx.Path())

	if backend, ok := i.getCachedHTTPBackend(host, path); ok {
		i.reverseProxy(ctx, backend)
		return
	}

	_, _, backend, err := i.registry.ResolveHTTPBackend(host, path)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		ctx.SetBody([]byte("route not found"))
		return
	}

	i.storeHTTPBackend(host, path, backend)
	i.reverseProxy(ctx, backend)
}

func (i *Ingress) reverseProxy(ctx *fasthttp.RequestCtx, backend string) {
	targetAddr := backend
	if strings.HasPrefix(targetAddr, "http://") {
		targetAddr = strings.TrimPrefix(targetAddr, "http://")
	} else if strings.HasPrefix(targetAddr, "https://") {
		ctx.SetStatusCode(fasthttp.StatusBadGateway)
		ctx.SetBody([]byte("https backend is not supported yet"))
		return
	}

	client := &fasthttp.HostClient{
		Addr:                targetAddr,
		ReadTimeout:         10 * time.Second,
		WriteTimeout:        10 * time.Second,
		MaxIdleConnDuration: 60 * time.Second,
	}
	if err := client.DoTimeout(&ctx.Request, &ctx.Response, 10*time.Second); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadGateway)
		ctx.SetBody([]byte(fmt.Sprintf("proxy error: %v", err)))
	}
}

func (i *Ingress) syncStreamRoutesLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-i.stopCh:
			return
		case <-ticker.C:
			i.syncStreamRoutes(registry.RouteProtocolTCP)
			i.syncStreamRoutes(registry.RouteProtocolUDP)
		}
	}
}

func (i *Ingress) syncStreamRoutes(protocol registry.RouteProtocol) {
	routes, err := i.registry.ListRoutes(protocol)
	if err != nil {
		i.logger.Error("list stream routes failed", "protocol", protocol, "error", err)
		return
	}

	active := make(map[int]struct{})
	lo.ForEach(routes, func(item registry.Route, _ int) {
		if !item.Enabled || item.ListenPort <= 0 {
			return
		}

		_, _, backend, resolveErr := i.registry.ResolveStreamBackend(protocol, item.ListenPort)
		if resolveErr != nil {
			return
		}
		active[item.ListenPort] = struct{}{}

		localRoute := Route{
			Protocol:   Protocol(protocol),
			ListenPort: item.ListenPort,
			Backend:    backend,
		}
		i.mu.Lock()
		if protocol == registry.RouteProtocolTCP {
			_ = i.registerTCP(localRoute)
		} else {
			_ = i.registerUDP(localRoute)
		}
		i.mu.Unlock()
	})

	i.mu.Lock()
	if protocol == registry.RouteProtocolTCP {
		lo.ForEach(lo.Keys(i.tcpListeners), func(port int, _ int) {
			if _, ok := active[port]; !ok {
				_ = i.unregisterTCP(Route{ListenPort: port})
			}
		})
	} else {
		lo.ForEach(lo.Keys(i.udpListeners), func(port int, _ int) {
			if _, ok := active[port]; !ok {
				_ = i.unregisterUDP(Route{ListenPort: port})
			}
		})
	}
	i.mu.Unlock()
}

func normalizeHostWithPort(host string) string {
	raw := strings.TrimSpace(strings.ToLower(host))
	if raw == "" {
		return raw
	}
	if parsed, _, err := net.SplitHostPort(raw); err == nil {
		return parsed
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		return strings.Trim(raw, "[]")
	}
	if idx := strings.Index(raw, ":"); idx >= 0 {
		return raw[:idx]
	}
	return raw
}

func (i *Ingress) getCachedHTTPBackend(host, path string) (string, bool) {
	key := httpCacheKey(host, path)

	i.mu.RLock()
	item, ok := i.httpCache[key]
	i.mu.RUnlock()
	if !ok {
		return "", false
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		i.mu.Lock()
		delete(i.httpCache, key)
		i.mu.Unlock()
		return "", false
	}

	return item.backend, item.backend != ""
}

func (i *Ingress) storeHTTPBackend(host, path, backend string) {
	if strings.TrimSpace(backend) == "" {
		return
	}

	key := httpCacheKey(host, path)
	entry := httpBackendCacheItem{
		backend:   backend,
		expiresAt: time.Now().Add(i.httpCacheTTL),
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	if len(i.httpCache) >= 2048 {
		for cacheKey, cacheItem := range i.httpCache {
			if !cacheItem.expiresAt.IsZero() && time.Now().After(cacheItem.expiresAt) {
				delete(i.httpCache, cacheKey)
			}
		}
	}
	if len(i.httpCache) >= 2048 {
		for cacheKey := range i.httpCache {
			delete(i.httpCache, cacheKey)
			break
		}
	}
	i.httpCache[key] = entry
}

func httpCacheKey(host, path string) string {
	return strings.ToLower(strings.TrimSpace(host)) + "|" + strings.TrimSpace(path)
}
