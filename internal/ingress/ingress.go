package ingress

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

type Protocol string

const (
	ProtocolHTTP Protocol = "http"
	ProtocolTCP  Protocol = "tcp"
	ProtocolUDP  Protocol = "udp"
)

type Route struct {
	Protocol   Protocol
	Host       string // 仅 HTTP 用
	PathPrefix string // 仅 HTTP 用
	ListenPort int    // TCP/UDP 监听端口
	Backend    string // 后端地址，例如 http://127.0.0.1:8080 或 127.0.0.1:3306
}

type Ingress struct {
	httpAddr string

	mu sync.RWMutex
	// HTTP 路由映射: host -> []Route
	httpRoutes map[string][]Route

	// TCP 监听 port -> 监听实例
	tcpListeners map[int]*tcpListener

	// UDP 监听 port -> 监听实例
	udpListeners map[int]*udpListener

	httpServer *fasthttp.Server
}

func NewIngress(httpAddr string) *Ingress {
	return &Ingress{
		httpAddr:     httpAddr,
		httpRoutes:   make(map[string][]Route),
		tcpListeners: make(map[int]*tcpListener),
		udpListeners: make(map[int]*udpListener),
	}
}

// StartHTTP 使用 fasthttp 启动 HTTP 服务
func (i *Ingress) StartHTTP() error {
	handler := func(ctx *fasthttp.RequestCtx) {
		i.handleHTTP(ctx)
	}
	i.httpServer = &fasthttp.Server{
		Handler:            handler,
		ReadTimeout:        10 * time.Second,
		WriteTimeout:       10 * time.Second,
		MaxRequestBodySize: 10 * 1024 * 1024,
	}

	go func() {
		if err := i.httpServer.ListenAndServe(i.httpAddr); err != nil {
			log.Printf("fasthttp server error: %v\n", err)
		}
	}()
	return nil
}

func (i *Ingress) handleHTTP(ctx *fasthttp.RequestCtx) {
	host := string(ctx.Host())
	path := string(ctx.Path())

	i.mu.RLock()
	defer i.mu.RUnlock()

	routes := i.httpRoutes[host]
	for _, route := range routes {
		if strings.HasPrefix(path, route.PathPrefix) {
			i.reverseProxy(ctx, route.Backend)
			return
		}
	}
	ctx.SetStatusCode(fasthttp.StatusNotFound)
	ctx.SetBody([]byte("404 Not Found"))
}

func (i *Ingress) reverseProxy(ctx *fasthttp.RequestCtx, backend string) {
	// 创建 HostClient，注意要去掉协议前缀 http://
	targetAddr := backend
	if strings.HasPrefix(targetAddr, "http://") {
		targetAddr = targetAddr[len("http://"):]
	} else if strings.HasPrefix(targetAddr, "https://") {
		// HTTPS 需要额外支持，目前只做 http
		ctx.SetStatusCode(fasthttp.StatusBadGateway)
		ctx.SetBody([]byte("HTTPS backend not supported"))
		return
	}

	client := &fasthttp.HostClient{
		Addr:                targetAddr,
		ReadTimeout:         10 * time.Second,
		WriteTimeout:        10 * time.Second,
		MaxIdleConnDuration: 60 * time.Second,
	}

	// 代理请求到后端
	if err := client.DoTimeout(&ctx.Request, &ctx.Response, 10*time.Second); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadGateway)
		ctx.SetBody([]byte(fmt.Sprintf("proxy error: %v", err)))
	}
}
