package ingress

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func deployAppSortKey(m deployv1.Metadata) string {
	return strings.TrimSpace(m.Namespace) + "/" + strings.TrimSpace(m.Name)
}

// workloadIPv4Lookup is implemented by *dnssvc.Service.
type workloadIPv4Lookup interface {
	LookupWorkloadIPv4(namespace, workloadName string) (string, bool)
}

// CompileIngressRoutesFromDeploy flattens app.ingresses into config routes pointing at workload
// container IPs from dnssvc (HTTP endpoints only). Apps are ordered by namespace/name; first match wins.
func CompileIngressRoutesFromDeploy(apps *list.List[deployv1.App], dns workloadIPv4Lookup, log *slog.Logger) *list.List[config.IngressRoute] {
	if dns == nil || apps.Len() == 0 {
		return list.NewList[config.IngressRoute]()
	}
	out := list.NewList[config.IngressRoute]()
	orderedApps(apps).Range(func(_ int, app deployv1.App) bool {
		compileAppIngressRoutes(out, app, dns, log)
		return true
	})
	return out
}

func orderedApps(apps *list.List[deployv1.App]) *list.List[deployv1.App] {
	keys, byKey := deployAppsBySortKey(apps)
	out := list.NewListWithCapacity[deployv1.App](keys.Len())
	keys.Range(func(_ int, k string) bool {
		app, _ := byKey.Get(k)
		out.Add(app)
		return true
	})
	return out
}

func deployAppsBySortKey(apps *list.List[deployv1.App]) (*list.List[string], *mapping.Map[string, deployv1.App]) {
	keys := list.NewListWithCapacity[string](apps.Len())
	byKey := mapping.NewMapWithCapacity[string, deployv1.App](apps.Len())
	apps.Range(func(_ int, app deployv1.App) bool {
		if strings.TrimSpace(app.Metadata.Name) != "" {
			addDeployAppSortKey(keys, byKey, app)
		}
		return true
	})
	keys.Sort(strings.Compare)
	return keys, byKey
}

func addDeployAppSortKey(keys *list.List[string], byKey *mapping.Map[string, deployv1.App], app deployv1.App) {
	key := deployAppSortKey(app.Metadata)
	if _, have := byKey.Get(key); !have {
		keys.Add(key)
	}
	byKey.Set(key, app)
}

func workloadMap(app deployv1.App) *mapping.Map[string, deployv1.Workload] {
	workloads := app.WorkloadList()
	out := mapping.NewMapWithCapacity[string, deployv1.Workload](workloads.Len())
	workloads.Range(func(_ int, w deployv1.Workload) bool {
		out.Set(strings.TrimSpace(w.Name), w)
		return true
	})
	return out
}

func compileAppIngressRoutes(out *list.List[config.IngressRoute], app deployv1.App, dns workloadIPv4Lookup, log *slog.Logger) {
	ctx := ingressCompileContext{
		app:       app,
		namespace: strings.TrimSpace(app.Metadata.Namespace),
		workloads: workloadMap(app),
		dns:       dns,
		log:       log,
	}
	app.IngressList().Range(func(_ int, ing deployv1.Ingress) bool {
		ing.RouteList().Range(func(_ int, route deployv1.IngressRoute) bool {
			if compiled, ok := ctx.compileRoute(route); ok {
				out.Add(compiled)
			}
			return true
		})
		return true
	})
}

type ingressCompileContext struct {
	app       deployv1.App
	namespace string
	workloads *mapping.Map[string, deployv1.Workload]
	dns       workloadIPv4Lookup
	log       *slog.Logger
}

func (c ingressCompileContext) compileRoute(route deployv1.IngressRoute) (config.IngressRoute, bool) {
	path, ok := ingressPath(route.Path)
	if !ok {
		return config.IngressRoute{}, false
	}
	workloadName := strings.TrimSpace(route.Backend.Workload)
	endpointName := strings.TrimSpace(route.Backend.Endpoint)
	workload, ok := c.workload(workloadName)
	if !ok {
		return config.IngressRoute{}, false
	}
	port, ok := c.endpointPort(workload, workloadName, endpointName)
	if !ok {
		return config.IngressRoute{}, false
	}
	ip, ok := c.workloadIP(workloadName)
	if !ok {
		return config.IngressRoute{}, false
	}
	return config.IngressRoute{PathPrefix: path, Upstream: fmt.Sprintf("http://%s:%d", ip, port)}, true
}

func ingressPath(raw string) (string, bool) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return "", false
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path, true
}

func (c ingressCompileContext) workload(name string) (deployv1.Workload, bool) {
	workload, ok := c.workloads.Get(name)
	if ok {
		return workload, true
	}
	if c.log != nil {
		c.log.Warn("ingress route skipped: unknown workload",
			"app", c.app.Metadata.Name, "namespace", c.app.Metadata.Namespace, "workload", name)
	}
	return deployv1.Workload{}, false
}

func (c ingressCompileContext) endpointPort(workload deployv1.Workload, workloadName, endpointName string) (int, bool) {
	port, ok := endpointHTTPPort(workload, endpointName)
	if ok {
		return port, true
	}
	if c.log != nil {
		c.log.Warn("ingress route skipped: missing or non-http endpoint",
			"app", c.app.Metadata.Name, "workload", workloadName, "endpoint", endpointName)
	}
	return 0, false
}

func (c ingressCompileContext) workloadIP(workloadName string) (string, bool) {
	ip, ok := c.dns.LookupWorkloadIPv4(c.namespace, workloadName)
	if ok {
		return ip, true
	}
	if c.log != nil {
		c.log.Debug("ingress route deferred: workload not in dns yet",
			"app", c.app.Metadata.Name, "workload", workloadName)
	}
	return "", false
}

func endpointHTTPPort(w deployv1.Workload, endpointName string) (int, bool) {
	port := 0
	found := false
	w.EndpointList().Range(func(_ int, ep deployv1.Endpoint) bool {
		if strings.TrimSpace(ep.Name) != endpointName {
			return true
		}
		if ep.Protocol != "" && ep.Protocol != deployv1.ProtoHTTP {
			return false
		}
		if ep.Port <= 0 {
			return false
		}
		port = ep.Port
		found = true
		return false
	})
	return port, found
}
