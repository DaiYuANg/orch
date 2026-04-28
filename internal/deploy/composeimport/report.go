package composeimport

import (
	"fmt"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

// Report captures compatibility diagnostics for Compose → canonical App conversion.
// Importers must not pretend lossless imports; callers should surface these notes.
type Report struct {
	Warnings []string `json:"warnings,omitempty"`
}

// Result is the outcome of importing Compose into the canonical deploy model.
type Result struct {
	App    *deployv1.App `json:"app,omitempty"`
	Report Report        `json:"report,omitempty"`
}

func (r *Report) warnf(format string, args ...any) {
	r.Warnings = append(r.Warnings, fmt.Sprintf(format, args...))
}
