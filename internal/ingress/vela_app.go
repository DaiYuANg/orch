package ingress

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	velaproxy "github.com/arcgolabs/vela/proxy"
	velaruntime "github.com/arcgolabs/vela/runtime"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const ingressEntrypoint = "ingress"

func newIngressHTTPServer(log *slog.Logger, gateway *velaruntime.Gateway, routeCount func() int) *http.Server {
	return &http.Server{
		Handler:           newIngressHTTPHandler(gateway, routeCount),
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
}

func newIngressHTTPHandler(gateway *velaruntime.Gateway, routeCount func() int) http.Handler {
	gatewayHandler := gateway.Handler(ingressEntrypoint)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if routeCount != nil && routeCount() == 0 {
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("orch ingress is running"))
			return
		}
		gatewayHandler.ServeHTTP(w, r)
	})
}

func buildVelaSnapshot(routes []config.IngressRoute) (*velaruntime.CompiledSnapshot, int, error) {
	snapshot := velaruntime.NewSnapshot().
		AddEntrypoint(ingressEntrypoint, "", velaruntime.EntrypointRuntime{Name: ingressEntrypoint})
	if len(routes) == 0 {
		return snapshot.BuildMatchers(), 0, nil
	}

	routeCount := 0
	for i := range routes {
		raw := &routes[i]
		meta, err := newRouteMeta(raw)
		if err != nil {
			return nil, 0, oopsx.B("ingress").Wrapf(err, "route %d (prefix %q)", i, raw.PathPrefix)
		}
		eps := raw.UpstreamEndpoints()
		if len(eps) > 1 {
			if pol := raw.LBPolicy(); pol != "round_robin" {
				return nil, 0, oopsx.B("ingress").Errorf("route %d: unsupported ingress.lb %q (supported: round_robin)", i, raw.LB)
			}
		}
		endpoints, err := buildVelaEndpoints(eps, meta)
		if err != nil {
			return nil, 0, oopsx.B("ingress").Wrapf(err, "route %d", i)
		}
		service := velaruntime.NewService(routeServiceName(i), "round_robin", endpoints...)
		route := velaruntime.NewRoute(routeName(i), ingressEntrypoint, service).
			WithPathPrefix(normalizePathPrefix(raw.PathPrefix))
		snapshot.AddService(service).AddRoute(route)
		routeCount++
	}
	return snapshot.BuildMatchers(), routeCount, nil
}

func buildVelaEndpoints(eps []string, meta routeMeta) ([]*velaruntime.EndpointRuntime, error) {
	servers := normalizeProxyServers(eps)
	if len(servers) == 0 {
		return nil, oopsx.B("ingress").Errorf("no valid upstream URLs")
	}
	out := make([]*velaruntime.EndpointRuntime, 0, len(servers))
	for _, raw := range servers {
		target, err := url.Parse(raw)
		if err != nil {
			return nil, oopsx.B("ingress").Wrapf(err, "parse upstream %q", raw)
		}
		proxyHandler := velaRewriteHandler(target, meta)
		endpoint, err := velaruntime.NewEndpoint(raw, 1, proxyHandler)
		if err != nil {
			return nil, oopsx.B("ingress").Wrapf(err, "upstream %q", raw)
		}
		out = append(out, endpoint)
	}
	return out, nil
}

func velaRewriteHandler(target *url.URL, meta routeMeta) http.Handler {
	proxyHandler := velaproxy.Build(target)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := normalizedPath(r.URL.Path)
		if !meta.matches(p) {
			http.NotFound(w, r)
			return
		}
		rel, ok := meta.pathRel(p)
		if !ok {
			http.NotFound(w, r)
			return
		}
		clone := r.Clone(r.Context())
		u := *r.URL
		u.Path = rel
		u.RawPath = ""
		clone.URL = &u
		proxyHandler.ServeHTTP(w, clone)
	})
}

func normalizeProxyServers(eps []string) []string {
	out := make([]string, 0, len(eps))
	for _, s := range eps {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
			s = "http://" + s
		}
		out = append(out, s)
	}
	return out
}

func routeName(index int) string {
	return "orch-ingress-route-" + strconv.Itoa(index)
}

func routeServiceName(index int) string {
	return "orch-ingress-service-" + strconv.Itoa(index)
}
