package api

import (
	"strings"

	"github.com/arcgolabs/collectionx/list"

	orchruntime "github.com/lyonbrown4d/orch/internal/runtime"
)

func runtimeProviderItems(runtimes *orchruntime.Manager) *list.List[RuntimeProviderItem] {
	items := list.NewList[RuntimeProviderItem]()
	if runtimes == nil {
		return items
	}
	runtimes.ProviderStatuses().Range(func(_ int, status orchruntime.ProviderStatus) bool {
		items.Add(RuntimeProviderItem{
			Kind:       status.Kind,
			Policy:     status.Policy,
			Available:  status.Available,
			Registered: status.Registered,
			Status:     status.Status,
			Reason:     status.Reason,
		})
		return true
	})
	return items
}

func runtimeDiagnostics(runtimes *orchruntime.Manager) RuntimeDiagnostics {
	providers := runtimeProviderItems(runtimes)
	out := RuntimeDiagnostics{Providers: providers}
	providers.Range(func(_ int, provider RuntimeProviderItem) bool {
		if provider.Available {
			out.Available++
		}
		if provider.Registered {
			out.Registered++
		}
		return true
	})
	return out
}

func runtimeReadiness(runtimes *orchruntime.Manager) ReadyCheckItem {
	if runtimes == nil {
		return ReadyCheckItem{Name: "runtime", Ready: false, Status: "not_ready", Detail: "runtime manager unavailable"}
	}
	providers := runtimeProviderItems(runtimes)
	registered := list.NewList[string]()
	missingRequired := list.NewList[string]()
	providers.Range(func(_ int, provider RuntimeProviderItem) bool {
		if provider.Registered {
			registered.Add(string(provider.Kind))
		}
		if provider.Policy == orchruntime.ProviderPolicyRequired && !provider.Registered {
			missingRequired.Add(runtimeProviderMissingDetail(provider))
		}
		return true
	})
	if !missingRequired.IsEmpty() {
		return ReadyCheckItem{Name: "runtime", Ready: false, Status: "missing", Detail: missingRequired.Join("; ")}
	}
	if registered.IsEmpty() {
		return ReadyCheckItem{Name: "runtime", Ready: false, Status: "not_ready", Detail: "no runtime providers registered"}
	}
	return ReadyCheckItem{Name: "runtime", Ready: true, Status: "ready", Detail: "registered: " + registered.Join(", ")}
}

func runtimeProviderMissingDetail(provider RuntimeProviderItem) string {
	parts := []string{strings.TrimSpace(string(provider.Kind))}
	if provider.Reason != "" {
		parts = append(parts, provider.Reason)
	}
	return strings.Join(parts, ": ")
}
