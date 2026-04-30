package ingress

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

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
func CompileIngressRoutesFromDeploy(apps []deployv1.App, dns workloadIPv4Lookup, log *slog.Logger) []config.IngressRoute {
	if dns == nil || len(apps) == 0 {
		return nil
	}
	keys := make([]string, 0, len(apps))
	byKey := make(map[string]deployv1.App, len(apps))
	for _, app := range apps {
		if strings.TrimSpace(app.Metadata.Name) == "" {
			continue
		}
		k := deployAppSortKey(app.Metadata)
		if _, have := byKey[k]; !have {
			keys = append(keys, k)
		}
		byKey[k] = app
	}
	sort.Strings(keys)

	var out []config.IngressRoute
	for _, k := range keys {
		app := byKey[k]
		ns := strings.TrimSpace(app.Metadata.Namespace)
		wlByName := make(map[string]*deployv1.Workload)
		for i := range app.Workloads {
			w := &app.Workloads[i]
			wlByName[strings.TrimSpace(w.Name)] = w
		}
		for ingIdx := range app.Ingresses {
			ing := &app.Ingresses[ingIdx]
			for _, r := range ing.Routes {
				path := strings.TrimSpace(r.Path)
				if path == "" {
					continue
				}
				if !strings.HasPrefix(path, "/") {
					path = "/" + path
				}
				bw := strings.TrimSpace(r.Backend.Workload)
				be := strings.TrimSpace(r.Backend.Endpoint)
				w := wlByName[bw]
				if w == nil {
					if log != nil {
						log.Warn("ingress route skipped: unknown workload",
							"app", app.Metadata.Name, "namespace", app.Metadata.Namespace, "workload", bw)
					}
					continue
				}
				port, ok := endpointHTTPPort(w, be)
				if !ok {
					if log != nil {
						log.Warn("ingress route skipped: missing or non-http endpoint",
							"app", app.Metadata.Name, "workload", bw, "endpoint", be)
					}
					continue
				}
				ip, ok := dns.LookupWorkloadIPv4(ns, bw)
				if !ok {
					if log != nil {
						log.Debug("ingress route deferred: workload not in dns yet",
							"app", app.Metadata.Name, "workload", bw)
					}
					continue
				}
				out = append(out, config.IngressRoute{
					PathPrefix: path,
					Upstream:   fmt.Sprintf("http://%s:%d", ip, port),
				})
			}
		}
	}
	return out
}

func endpointHTTPPort(w *deployv1.Workload, endpointName string) (int, bool) {
	for i := range w.Endpoints {
		ep := &w.Endpoints[i]
		if strings.TrimSpace(ep.Name) != endpointName {
			continue
		}
		if ep.Protocol != "" && ep.Protocol != deployv1.ProtoHTTP {
			return 0, false
		}
		if ep.Port <= 0 {
			return 0, false
		}
		return ep.Port, true
	}
	return 0, false
}
