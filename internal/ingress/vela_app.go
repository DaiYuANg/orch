package ingress

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
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

func buildVelaSnapshot(routes *list.List[config.IngressRoute]) (*velaruntime.CompiledSnapshot, int, error) {
	snapshot := velaruntime.NewSnapshot().
		AddEntrypoint(ingressEntrypoint, "", velaruntime.EntrypointRuntime{Name: ingressEntrypoint})
	if routes.Len() == 0 {
		return snapshot.BuildMatchers(), 0, nil
	}

	routeCount := 0
	var buildErr error
	routes.Range(func(i int, routeCfg config.IngressRoute) bool {
		raw := &routeCfg
		meta, err := newRouteMeta(raw)
		if err != nil {
			buildErr = oopsx.B("ingress").Wrapf(err, "route %d (prefix %q)", i, raw.PathPrefix)
			return false
		}
		eps := raw.UpstreamEndpoints()
		if eps.Len() > 1 {
			if pol := raw.LBPolicy(); pol != "round_robin" {
				buildErr = oopsx.B("ingress").Errorf("route %d: unsupported ingress.lb %q (supported: round_robin)", i, raw.LB)
				return false
			}
		}
		endpoints, err := buildVelaEndpoints(eps, meta)
		if err != nil {
			buildErr = oopsx.B("ingress").Wrapf(err, "route %d", i)
			return false
		}
		service := velaruntime.NewService(routeServiceName(i), "round_robin", endpoints.Values()...)
		compiledRoute := velaruntime.NewRoute(routeName(i), ingressEntrypoint, service).
			WithPathPrefix(normalizePathPrefix(raw.PathPrefix))
		snapshot.AddService(service).AddRoute(compiledRoute)
		routeCount++
		return true
	})
	if buildErr != nil {
		return nil, 0, buildErr
	}
	return snapshot.BuildMatchers(), routeCount, nil
}

func buildVelaEndpoints(eps *list.List[string], meta routeMeta) (*list.List[*velaruntime.EndpointRuntime], error) {
	servers := normalizeProxyServers(eps)
	if servers.Len() == 0 {
		return nil, oopsx.B("ingress").Errorf("no valid upstream URLs")
	}
	out := list.NewListWithCapacity[*velaruntime.EndpointRuntime](servers.Len())
	var buildErr error
	servers.Range(func(_ int, raw string) bool {
		target, err := url.Parse(raw)
		if err != nil {
			buildErr = oopsx.B("ingress").Wrapf(err, "parse upstream %q", raw)
			return false
		}
		proxyHandler := velaRewriteHandler(target, meta)
		endpoint, err := velaruntime.NewEndpoint(raw, 1, proxyHandler)
		if err != nil {
			buildErr = oopsx.B("ingress").Wrapf(err, "upstream %q", raw)
			return false
		}
		out.Add(endpoint)
		return true
	})
	if buildErr != nil {
		return nil, buildErr
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

func normalizeProxyServers(eps *list.List[string]) *list.List[string] {
	return list.FilterMapList(eps, func(_ int, s string) (string, bool) {
		s = strings.TrimSpace(s)
		if s == "" {
			return "", false
		}
		if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
			s = "http://" + s
		}
		return s, true
	})
}

func routeName(index int) string {
	return "orch-ingress-route-" + strconv.Itoa(index)
}

func routeServiceName(index int) string {
	return "orch-ingress-service-" + strconv.Itoa(index)
}
