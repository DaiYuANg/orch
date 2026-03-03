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

	httpServer *fasthttp.Server
	stopCh     chan struct{}
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
	for port := range i.tcpListeners {
		_ = i.unregisterTCP(Route{ListenPort: port})
	}
	for port := range i.udpListeners {
		_ = i.unregisterUDP(Route{ListenPort: port})
	}
	i.mu.Unlock()

	if i.httpServer != nil {
		return i.httpServer.Shutdown()
	}
	return nil
}

func (i *Ingress) handleHTTP(ctx *fasthttp.RequestCtx) {
	host := normalizeHostWithPort(string(ctx.Host()))
	path := string(ctx.Path())

	_, _, backend, err := i.registry.ResolveHTTPBackend(host, path)
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusNotFound)
		ctx.SetBody([]byte("route not found"))
		return
	}

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
	for _, item := range routes {
		if !item.Enabled || item.ListenPort <= 0 {
			continue
		}

		_, _, backend, resolveErr := i.registry.ResolveStreamBackend(protocol, item.ListenPort)
		if resolveErr != nil {
			continue
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
	}

	i.mu.Lock()
	if protocol == registry.RouteProtocolTCP {
		for port := range i.tcpListeners {
			if _, ok := active[port]; !ok {
				_ = i.unregisterTCP(Route{ListenPort: port})
			}
		}
	} else {
		for port := range i.udpListeners {
			if _, ok := active[port]; !ok {
				_ = i.unregisterUDP(Route{ListenPort: port})
			}
		}
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
