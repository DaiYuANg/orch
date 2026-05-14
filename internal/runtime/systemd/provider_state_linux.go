//go:build linux

package systemd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lyonbrown4d/orch/internal/config"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

type state struct {
	UnitName  string               `json:"unitName"`
	UnitPath  string               `json:"unitPath"`
	Metadata  deployv1.Metadata    `json:"metadata"`
	Workload  string               `json:"workload"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Artifact  string               `json:"artifact,omitempty"`
	StartedAt time.Time            `json:"startedAt"`
}

func (p *Provider) workloadUnitName(meta deployv1.Metadata, w deployv1.Workload) string {
	if w.Run.Options.Systemd != nil {
		if name := NormalizeUnitName(w.Run.Options.Systemd.UnitName); name != "" {
			return name
		}
	}
	return DefaultUnitName(meta, w.Name)
}

func systemdUnitPath(unitName string) string {
	return filepath.Join(systemdSystemUnitDir, unitName)
}

func (p *Provider) readState(meta deployv1.Metadata, workloadName string) (state, error) {
	var st state
	b, err := os.ReadFile(p.statePath(meta, workloadName))
	if err != nil {
		return st, oopsx.B("runtime", "systemd").Wrapf(err, "read systemd state")
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, oopsx.B("runtime", "systemd").Wrapf(err, "decode systemd state")
	}
	return st, nil
}

func (p *Provider) writeState(meta deployv1.Metadata, workloadName string, st state) error {
	path := p.statePath(meta, workloadName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return oopsx.B("runtime", "systemd").Wrapf(err, "create state dir")
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return oopsx.B("runtime", "systemd").Wrapf(err, "encode systemd state")
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return oopsx.B("runtime", "systemd").Wrapf(err, "write systemd state")
	}
	return nil
}

func (p *Provider) removeState(meta deployv1.Metadata, workloadName string) error {
	err := os.Remove(p.statePath(meta, workloadName))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return oopsx.B("runtime", "systemd").Wrapf(err, "remove systemd state")
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
	return filepath.Join(config.DefaultDataRoot(), "runtime", "systemd")
}
