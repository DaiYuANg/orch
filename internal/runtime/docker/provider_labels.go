package docker

import (
	"strings"

	"github.com/arcgolabs/collectionx/mapping"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
)

// WorkloadLabelsMatch reports whether Docker labels match the orch workload identity.
func WorkloadLabelsMatch(labels map[string]string, meta deployv1.Metadata, w deployv1.Workload) bool {
	expected := workloadmeta.LabelMap(meta, w)
	matches := true
	expected.Range(func(key, want string) bool {
		if labels[key] != want {
			matches = false
			return false
		}
		return true
	})
	return matches
}

// ContainerLabels returns Docker labels merged with orch workload identity labels.
func ContainerLabels(meta deployv1.Metadata, w deployv1.Workload) map[string]string {
	labels := mapping.NewMapWithCapacity[string, string](4)
	if w.Run.Options.Docker != nil {
		w.Run.Options.Docker.LabelMap().Range(func(k, v string) bool {
			if key := strings.TrimSpace(k); key != "" {
				labels.Set(key, v)
			}
			return true
		})
	}
	workloadmeta.LabelMap(meta, w).Range(func(k, v string) bool {
		labels.Set(k, v)
		return true
	})
	return labels.All()
}
