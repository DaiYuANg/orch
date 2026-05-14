package process

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type state struct {
	PID       int                  `json:"pid"`
	Metadata  deployv1.Metadata    `json:"metadata"`
	Workload  string               `json:"workload"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Artifact  string               `json:"artifact,omitempty"`
	StopAfter string               `json:"stopAfter,omitempty"`
	StartedAt time.Time            `json:"startedAt"`
}

func (p *Provider) readState(meta deployv1.Metadata, workloadName string) (state, error) {
	var st state
	b, err := os.ReadFile(p.statePath(meta, workloadName))
	if err != nil {
		return st, oopsx.B("runtime", "process").Wrapf(err, "read process state")
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, oopsx.B("runtime", "process").Wrapf(err, "decode process state")
	}
	return st, nil
}

func (p *Provider) writeState(meta deployv1.Metadata, workloadName string, st state) error {
	path := p.statePath(meta, workloadName)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return oopsx.B("runtime", "process").Wrapf(err, "create state dir")
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return oopsx.B("runtime", "process").Wrapf(err, "encode process state")
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return oopsx.B("runtime", "process").Wrapf(err, "write process state")
	}
	return nil
}

func (p *Provider) removeState(meta deployv1.Metadata, workloadName string) error {
	err := os.Remove(p.statePath(meta, workloadName))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return oopsx.B("runtime", "process").Wrapf(err, "remove process state")
}

func (p *Provider) removeStateIfPID(meta deployv1.Metadata, workloadName string, pid int) error {
	st, err := p.readState(meta, workloadName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if st.PID != pid {
		return nil
	}
	return p.removeState(meta, workloadName)
}

func (p *Provider) statePath(meta deployv1.Metadata, workloadName string) string {
	return filepath.Join(p.rootOrDefault(), "state", p.nameBase(meta, workloadName)+".json")
}

// StatePath returns the state file path for a workload.
func (p *Provider) StatePath(meta deployv1.Metadata, workloadName string) string {
	return p.statePath(meta, workloadName)
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
	return filepath.Join(config.DefaultDataRoot(), "runtime", "process")
}
