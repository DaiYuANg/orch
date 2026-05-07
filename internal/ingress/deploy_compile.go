package ingress

import (
	"fmt"
	"log/slog"
	"sort"
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
	keys := list.NewListWithCapacity[string](apps.Len())
	byKey := mapping.NewMapWithCapacity[string, deployv1.App](apps.Len())
	apps.Range(func(_ int, app deployv1.App) bool {
		if strings.TrimSpace(app.Metadata.Name) == "" {
			return true
		}
		k := deployAppSortKey(app.Metadata)
		if _, have := byKey.Get(k); !have {
			keys.Add(k)
		}
		byKey.Set(k, app)
		return true
	})
	keyValues := keys.Values()
	sort.Strings(keyValues)

	out := list.NewList[config.IngressRoute]()
	for _, k := range keyValues {
		app, _ := byKey.Get(k)
		ns := strings.TrimSpace(app.Metadata.Namespace)
		workloads := app.WorkloadList()
		wlByName := mapping.NewMapWithCapacity[string, deployv1.Workload](workloads.Len())
		workloads.Range(func(_ int, w deployv1.Workload) bool {
			wlByName.Set(strings.TrimSpace(w.Name), w)
			return true
		})
		app.IngressList().Range(func(_ int, ing deployv1.Ingress) bool {
			ing.RouteList().Range(func(_ int, r deployv1.IngressRoute) bool {
				path := strings.TrimSpace(r.Path)
				if path == "" {
					return true
				}
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
				bw := strings.TrimSpace(r.Backend.Workload)
				be := strings.TrimSpace(r.Backend.Endpoint)
				w, ok := wlByName.Get(bw)
				if !ok {
					if log != nil {
						log.Warn("ingress route skipped: unknown workload",
							"app", app.Metadata.Name, "namespace", app.Metadata.Namespace, "workload", bw)
					}
					return true
				}
				port, ok := endpointHTTPPort(w, be)
				if !ok {
					if log != nil {
						log.Warn("ingress route skipped: missing or non-http endpoint",
							"app", app.Metadata.Name, "workload", bw, "endpoint", be)
					}
					return true
				}
				ip, ok := dns.LookupWorkloadIPv4(ns, bw)
				if !ok {
					if log != nil {
						log.Debug("ingress route deferred: workload not in dns yet",
							"app", app.Metadata.Name, "workload", bw)
					}
					return true
				}
				out.Add(config.IngressRoute{
					PathPrefix: path,
					Upstream:   fmt.Sprintf("http://%s:%d", ip, port),
				})
				return true
			})
			return true
		})
	}
	return out
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
