package workloadmeta

import (
	"fmt"
	"strings"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

// NamespaceOrDefault returns a non-empty namespace for DNS and naming.
func NamespaceOrDefault(ns string) string {
	if strings.TrimSpace(ns) == "" {
		return "default"
	}
	return strings.TrimSpace(ns)
}

// SanitizeName maps arbitrary strings to container-friendly identifiers.
func SanitizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "x"
	}
	return out
}

// OrchContainerName is a stable container name derived from app metadata + workload.
func OrchContainerName(meta deployv1.Metadata, workload string) string {
	ns := NamespaceOrDefault(meta.Namespace)
	w := strings.TrimSpace(workload)
	return fmt.Sprintf("orch-%s-%s", SanitizeName(ns), SanitizeName(w))
}

// Labels are applied to containers for lookup on stop/diagnostics.
func Labels(meta deployv1.Metadata, w deployv1.Workload) map[string]string {
	return map[string]string{
		"orch.io/app":       meta.Name,
		"orch.io/namespace": NamespaceOrDefault(meta.Namespace),
		"orch.io/workload":  w.Name,
		"orch.io/runtime":   string(w.Runtime),
	}
}

// NormalizeImageRef expands short docker.io/library names.
func NormalizeImageRef(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return image
	}
	if strings.Contains(image, "/") {
		return image
	}
	return "docker.io/library/" + image
}
