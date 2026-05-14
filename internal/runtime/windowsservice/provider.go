package windowsservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lyonbrown4d/orch/internal/config"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

type Provider struct {
	logger *slog.Logger
	dns    *dnssvc.Service
	root   string
}

type state struct {
	ServiceName string               `json:"serviceName"`
	Metadata    deployv1.Metadata    `json:"metadata"`
	Workload    string               `json:"workload"`
	Runtime     deployv1.RuntimeKind `json:"runtime"`
	Artifact    string               `json:"artifact,omitempty"`
	StartedAt   time.Time            `json:"startedAt"`
}

func NewProvider(logger *slog.Logger, dns *dnssvc.Service) *Provider {
	return &Provider{
		logger: logger,
		dns:    dns,
		root:   filepath.Join(config.DefaultDataRoot(), "runtime", "windows-service"),
	}
}

func (p *Provider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeWindowsService
}

func (p *Provider) workloadServiceName(meta deployv1.Metadata, w deployv1.Workload) string {
	if w.Run.Options.WindowsService != nil {
		if name := normalizeServiceName(w.Run.Options.WindowsService.ServiceName); name != "" {
			return name
		}
	}
	return defaultServiceName(meta, w.Name)
}

func defaultServiceName(meta deployv1.Metadata, workloadName string) string {
	return normalizeServiceName(fmt.Sprintf("orch-%s-%s-%s",
		workloadmeta.SanitizeName(workloadmeta.NamespaceOrDefault(meta.Namespace)),
		workloadmeta.SanitizeName(meta.Name),
		workloadmeta.SanitizeName(workloadName),
	))
}

// DefaultServiceName returns the stable SCM service name orch uses for a workload.
func DefaultServiceName(meta deployv1.Metadata, workloadName string) string {
	return defaultServiceName(meta, workloadName)
}

func normalizeServiceName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "/", "-")
	return strings.Join(strings.Fields(name), "-")
}

// NormalizeServiceName returns a service-control-manager-safe service name.
func NormalizeServiceName(name string) string {
	return normalizeServiceName(name)
}

func (p *Provider) workloadDisplayName(meta deployv1.Metadata, w deployv1.Workload) string {
	if w.Run.Options.WindowsService != nil {
		if displayName := strings.TrimSpace(w.Run.Options.WindowsService.DisplayName); displayName != "" {
			return displayName
		}
	}
	return fmt.Sprintf("orch %s/%s/%s", workloadmeta.NamespaceOrDefault(meta.Namespace), meta.Name, w.Name)
}

// WorkloadDisplayName returns the configured or default Windows service display name.
func (p *Provider) WorkloadDisplayName(meta deployv1.Metadata, w deployv1.Workload) string {
	return p.workloadDisplayName(meta, w)
}

func (p *Provider) readState(meta deployv1.Metadata, workloadName string) (state, error) {
	var st state
	b, err := os.ReadFile(p.statePath(meta, workloadName))
	if err != nil {
		return st, oopsx.B("runtime", "windows-service").Wrapf(err, "read windows service state")
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, oopsx.B("runtime", "windows-service").Wrapf(err, "decode windows service state")
	}
	return st, nil
}

func (p *Provider) writeState(meta deployv1.Metadata, workloadName string, st state) error {
	path := p.statePath(meta, workloadName)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return oopsx.B("runtime", "windows-service").Wrapf(err, "create state dir")
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return oopsx.B("runtime", "windows-service").Wrapf(err, "encode windows service state")
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return oopsx.B("runtime", "windows-service").Wrapf(err, "write windows service state")
	}
	return nil
}

func (p *Provider) removeState(meta deployv1.Metadata, workloadName string) error {
	err := os.Remove(p.statePath(meta, workloadName))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return oopsx.B("runtime", "windows-service").Wrapf(err, "remove windows service state")
}

func (p *Provider) statePath(meta deployv1.Metadata, workloadName string) string {
	return filepath.Join(p.rootOrDefault(), "state", p.nameBase(meta, workloadName)+".json")
}

func (p *Provider) nameBase(meta deployv1.Metadata, workloadName string) string {
	return fmt.Sprintf("%s-%s-%s",
		workloadmeta.SanitizeName(workloadmeta.NamespaceOrDefault(meta.Namespace)),
		workloadmeta.SanitizeName(meta.Name),
		workloadmeta.SanitizeName(workloadName),
	)
}

func (p *Provider) rootOrDefault() string {
	if strings.TrimSpace(p.root) != "" {
		return filepath.Clean(p.root)
	}
	return filepath.Join(config.DefaultDataRoot(), "runtime", "windows-service")
}
