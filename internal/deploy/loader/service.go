package loader

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/daiyuang/orch/internal/deploy/orch"
	v1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

// Loader selects the manifest path (.orch vs YAML) and returns a canonical [v1.App].
type Loader struct {
	orch *orch.Orch
}

// NewLoader wires [.orch] loading via [orch.Orch]. orch must be non-nil when loading .orch sources.
func NewLoader(o *orch.Orch) (*Loader, error) {
	if o == nil {
		return nil, fmt.Errorf("deploy loader: nil orch.Orch")
	}
	return &Loader{orch: o}, nil
}

// LoadApp loads a deploy document from path. ".orch" uses [orch.Orch]; other extensions use [v1.LoadAppFile].
func (l *Loader) LoadApp(ctx context.Context, path string) (*v1.App, error) {
	if strings.EqualFold(filepath.Ext(path), ".orch") {
		if l == nil || l.orch == nil {
			return nil, fmt.Errorf("deploy loader: orch.Orch is required for .orch files")
		}
		return l.orch.LoadAppFile(ctx, path)
	}
	return v1.LoadAppFile(path)
}

// LoadAppBytes loads from memory. virtualPath ending in ".orch" uses [orch.Orch]; otherwise [v1.ParseAppYAML].
func (l *Loader) LoadAppBytes(ctx context.Context, virtualPath string, src []byte) (*v1.App, error) {
	if strings.EqualFold(filepath.Ext(virtualPath), ".orch") {
		if l == nil || l.orch == nil {
			return nil, fmt.Errorf("deploy loader: orch.Orch is required for .orch sources")
		}
		return l.orch.LoadAppBytes(ctx, virtualPath, src)
	}
	return v1.ParseAppYAML(src)
}

// LoadAppString is [Loader.LoadAppBytes] with string source.
func (l *Loader) LoadAppString(ctx context.Context, virtualPath, src string) (*v1.App, error) {
	return l.LoadAppBytes(ctx, virtualPath, []byte(src))
}
