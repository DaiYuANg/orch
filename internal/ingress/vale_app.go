package ingress

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
	valeproxy "github.com/arcgolabs/vale/proxy"
	valeruntime "github.com/arcgolabs/vale/runtime"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const ingressEntrypoint = "ingress"

// NewHTTPHandler builds an ingress HTTP handler for the provided static routes.
func NewHTTPHandler(routes *list.List[config.IngressRoute], log *slog.Logger) (http.Handler, error) {
	if routes == nil {
		routes = list.NewList[config.IngressRoute]()
	}
	if log == nil {
		log = slog.Default()
	}
	snapshot, routeCount, err := buildValeSnapshot(routes)
	if err != nil {
		return nil, err
	}
	gateway := valeruntime.NewGateway(snapshot, log, false, valeruntime.NewNoopMetrics())
	return newIngressHTTPHandler(gateway, func() int { return routeCount }), nil
}

func newIngressHTTPServer(log *slog.Logger, gateway *valeruntime.Gateway, routeCount func() int) *http.Server {
	return &http.Server{
		Handler:           newIngressHTTPHandler(gateway, routeCount),
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
}

func newIngressHTTPHandler(gateway *valeruntime.Gateway, routeCount func() int) http.Handler {
	gatewayHandler := gateway.Handler(ingressEntrypoint)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if routeCount != nil && routeCount() == 0 {
			w.Header().Set("content-type", "text/plain; charset=utf-8")
			if _, err := w.Write([]byte("orch ingress is running")); err != nil {
				return
			}
			return
		}
		gatewayHandler.ServeHTTP(w, r)
	})
}

func buildValeSnapshot(routes *list.List[config.IngressRoute]) (*valeruntime.CompiledSnapshot, int, error) {
	if routes == nil {
		routes = list.NewList[config.IngressRoute]()
	}
	snapshot := valeruntime.NewSnapshot().
		AddEntrypoint(ingressEntrypoint, "", valeruntime.EntrypointRuntime{Name: ingressEntrypoint})
	if routes.Len() == 0 {
		return snapshot.BuildMatchers(), 0, nil
	}

	routeCount := 0
	var buildErr error
	routes.Range(func(i int, routeCfg config.IngressRoute) bool {
		buildErr = addValeRoute(snapshot, i, routeCfg)
		if buildErr != nil {
			return false
		}
		routeCount++
		return true
	})
	if buildErr != nil {
		return nil, 0, buildErr
	}
	return snapshot.BuildMatchers(), routeCount, nil
}

func addValeRoute(snapshot *valeruntime.CompiledSnapshot, index int, routeCfg config.IngressRoute) error {
	raw := &routeCfg
	meta, err := newRouteMeta(raw)
	if err != nil {
		return oopsx.B("ingress").Wrapf(err, "route %d (prefix %q)", index, raw.PathPrefix)
	}
	if policyErr := validateValeLBPolicy(index, raw); policyErr != nil {
		return policyErr
	}
	endpoints, err := buildValeEndpoints(raw.UpstreamEndpoints(), meta)
	if err != nil {
		return oopsx.B("ingress").Wrapf(err, "route %d", index)
	}
	service := valeruntime.NewService(routeServiceName(index), "round_robin", endpoints.Values()...)
	compiledRoute := valeruntime.NewRoute(routeName(index), ingressEntrypoint, service).
		WithPathPrefix(normalizePathPrefix(raw.PathPrefix))
	snapshot.AddService(service).AddRoute(compiledRoute)
	return nil
}

func validateValeLBPolicy(index int, route *config.IngressRoute) error {
	if route.UpstreamEndpoints().Len() <= 1 || route.LBPolicy() == "round_robin" {
		return nil
	}
	return oopsx.B("ingress").Errorf("route %d: unsupported ingress.lb %q (supported: round_robin)", index, route.LB)
}

func buildValeEndpoints(eps *list.List[string], meta routeMeta) (*list.List[*valeruntime.EndpointRuntime], error) {
	servers := normalizeProxyServers(eps)
	if servers.Len() == 0 {
		return nil, oopsx.B("ingress").Errorf("no valid upstream URLs")
	}
	out := list.NewListWithCapacity[*valeruntime.EndpointRuntime](servers.Len())
	var buildErr error
	servers.Range(func(_ int, raw string) bool {
		target, err := url.Parse(raw)
		if err != nil {
			buildErr = oopsx.B("ingress").Wrapf(err, "parse upstream %q", raw)
			return false
		}
		proxyHandler := valeRewriteHandler(target, meta)
		endpoint, err := valeruntime.NewEndpoint(raw, 1, proxyHandler)
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

func valeRewriteHandler(target *url.URL, meta routeMeta) http.Handler {
	proxyHandler := valeproxy.Build(target)
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
