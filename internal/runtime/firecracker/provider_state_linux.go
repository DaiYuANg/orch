//go:build linux

package firecracker

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/arcgolabs/collectionx/mapping"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

type runningState struct {
	items *mapping.ConcurrentMap[string, runningVMM]
}

func newRunningState() runningState {
	return runningState{items: mapping.NewConcurrentMap[string, runningVMM]()}
}

type runningVMM struct {
	pid  int
	stop func() error
}

type state struct {
	PID       int                  `json:"pid"`
	APISocket string               `json:"apiSocket"`
	Network   *NetworkConfig       `json:"network,omitempty"`
	Metadata  deployv1.Metadata    `json:"metadata"`
	Workload  string               `json:"workload"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Artifact  string               `json:"artifact,omitempty"`
	StartedAt time.Time            `json:"startedAt"`
}

func (p *Provider) openLogs(cfg VMConfig) (*os.File, *os.File, func(), error) {
	stdout, err := openAppend(cfg.StdoutPath)
	if err != nil {
		return nil, nil, func() {}, err
	}
	stderr, err := openAppend(cfg.StderrPath)
	if err != nil {
		_ = stdout.Close()
		return nil, nil, func() {}, err
	}
	closeLogs := func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}
	return stdout, stderr, closeLogs, nil
}

func openAppend(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, oopsx.B("runtime", "firecracker").Wrapf(err, "create log dir")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, oopsx.B("runtime", "firecracker").Wrapf(err, "open log %s", filepath.Base(path))
	}
	return f, nil
}

func (p *Provider) readState(meta deployv1.Metadata, workloadName string) (state, error) {
	var st state
	b, err := os.ReadFile(p.statePath(meta, workloadName))
	if err != nil {
		return st, oopsx.B("runtime", "firecracker").Wrapf(err, "read firecracker state")
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, oopsx.B("runtime", "firecracker").Wrapf(err, "decode firecracker state")
	}
	return st, nil
}

func (p *Provider) writeState(meta deployv1.Metadata, workloadName string, st state) error {
	path := p.statePath(meta, workloadName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "create state dir")
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "encode firecracker state")
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "write firecracker state")
	}
	return nil
}

func (p *Provider) removeState(meta deployv1.Metadata, workloadName string) error {
	err := os.Remove(p.statePath(meta, workloadName))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return oopsx.B("runtime", "firecracker").Wrapf(err, "remove firecracker state")
}

func (p *Provider) removeStateIfPID(meta deployv1.Metadata, workloadName string, pid int) error {
	st, err := p.readState(meta, workloadName)
	if err != nil || st.PID != pid {
		return nil
	}
	return p.removeState(meta, workloadName)
}

func (p *Provider) trackRunningVMM(meta deployv1.Metadata, workloadName string, vm runningVMM) {
	if p.running.items == nil {
		p.running.items = mapping.NewConcurrentMap[string, runningVMM]()
	}
	p.running.items.Set(p.runningKey(meta, workloadName), vm)
}

func (p *Provider) untrackRunningVMM(meta deployv1.Metadata, workloadName string, pid int) {
	if p.running.items == nil {
		return
	}
	key := p.runningKey(meta, workloadName)
	if vm, ok := p.running.items.Get(key); ok && vm.pid == pid {
		p.running.items.Delete(key)
	}
}

func (p *Provider) runningVMM(meta deployv1.Metadata, workloadName string) (runningVMM, bool) {
	if p.running.items == nil {
		return runningVMM{}, false
	}
	return p.running.items.Get(p.runningKey(meta, workloadName))
}

func (p *Provider) runningKey(meta deployv1.Metadata, workloadName string) string {
	return p.nameBase(meta, workloadName)
}

func (p *Provider) statePath(meta deployv1.Metadata, workloadName string) string {
	return filepath.Join(p.rootOrDefault(), "state", p.nameBase(meta, workloadName)+".json")
}
