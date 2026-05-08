package process

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const defaultStopTimeout = 5 * time.Second

type Provider struct {
	logger *slog.Logger
	dns    *dnssvc.Service
	root   string
}

type state struct {
	PID       int                  `json:"pid"`
	Metadata  deployv1.Metadata    `json:"metadata"`
	Workload  string               `json:"workload"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Artifact  string               `json:"artifact,omitempty"`
	StopAfter string               `json:"stopAfter,omitempty"`
	StartedAt time.Time            `json:"startedAt"`
}

func NewProvider(logger *slog.Logger, dns *dnssvc.Service) *Provider {
	return &Provider{
		logger: logger,
		dns:    dns,
		root:   filepath.Join(config.DefaultDataRoot(), "runtime", "process"),
	}
}

func (p *Provider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeProcess
}

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	exe, args, ok := runconfig.ProcessCommand(w.Run)
	if !ok {
		return oopsx.B("runtime", "process").Errorf("workload %q: run.exec.command or run.artifact.path is required", w.Name)
	}
	if err := p.ensureNoLiveState(meta, w.Name); err != nil {
		return err
	}

	stdout, stderr, closeLogs, err := p.openLogs(meta, w)
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, args.Values()...)
	cmd.Env = append(os.Environ(), runconfig.Env(w.EnvList()).Values()...)
	cmd.Dir = strings.TrimSpace(w.Run.Cwd)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		closeLogs()
		return oopsx.B("runtime", "process").Wrapf(err, "process start %q", exe)
	}

	st := state{
		PID:       cmd.Process.Pid,
		Metadata:  meta,
		Workload:  w.Name,
		Runtime:   w.Runtime,
		Artifact:  runconfig.ArtifactSummary(w.Run),
		StopAfter: processStopTimeout(w.Run),
		StartedAt: time.Now().UTC(),
	}
	if err := p.writeState(meta, w.Name, st); err != nil {
		_ = cmd.Process.Kill()
		closeLogs()
		return err
	}

	if p.dns != nil {
		if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, p.dns.WorkloadAdvertiseAddress("127.0.0.1")); err != nil {
			_ = cmd.Process.Kill()
			closeLogs()
			_ = p.removeState(meta, w.Name)
			return oopsx.B("runtime", "dns").Wrapf(err, "upsert workload DNS")
		}
	}

	go p.waitProcess(meta, w.Name, cmd, closeLogs)
	p.logger.Info("process workload running", "workload", w.Name, "pid", cmd.Process.Pid, "exec", exe)
	return nil
}

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	st, err := p.readState(meta, workloadName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if p.dns != nil {
				_ = p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName)
			}
			return nil
		}
		return err
	}
	proc, err := os.FindProcess(st.PID)
	if err == nil && proc != nil {
		if err := proc.Signal(os.Interrupt); err != nil {
			_ = proc.Kill()
		} else if !waitExit(st.PID, p.stopTimeout(st)) {
			_ = proc.Kill()
		}
	}
	if p.dns != nil {
		if err := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); err != nil {
			return oopsx.B("runtime", "dns").Wrapf(err, "remove workload DNS")
		}
	}
	if err := p.removeState(meta, workloadName); err != nil {
		return err
	}
	p.logger.Info("process workload stopped", "workload", workloadName, "pid", st.PID)
	return nil
}

func (p *Provider) ensureNoLiveState(meta deployv1.Metadata, workloadName string) error {
	st, err := p.readState(meta, workloadName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if st.PID > 0 && processAlive(st.PID) {
		return oopsx.B("runtime", "process").Errorf("process workload %q already has live pid %d", workloadName, st.PID)
	}
	return p.removeState(meta, workloadName)
}

func (p *Provider) waitProcess(meta deployv1.Metadata, workloadName string, cmd *exec.Cmd, closeLogs func()) {
	err := cmd.Wait()
	closeLogs()
	if err != nil {
		p.logger.Warn("process workload exited", "workload", workloadName, "pid", cmd.ProcessState.Pid(), "error", err)
	} else {
		p.logger.Info("process workload exited", "workload", workloadName, "pid", cmd.ProcessState.Pid())
	}
	_ = p.removeStateIfPID(meta, workloadName, cmd.ProcessState.Pid())
	if p.dns != nil {
		_ = p.dns.RemoveWorkloadA(context.Background(), meta.Namespace, workloadName)
	}
}

func (p *Provider) openLogs(meta deployv1.Metadata, w deployv1.Workload) (*os.File, *os.File, func(), error) {
	stdoutPath, stderrPath := p.logPaths(meta, w)
	stdout, err := openAppend(stdoutPath)
	if err != nil {
		return nil, nil, func() {}, err
	}
	stderr, err := openAppend(stderrPath)
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

func (p *Provider) logPaths(meta deployv1.Metadata, w deployv1.Workload) (string, string) {
	base := p.nameBase(meta, w.Name)
	stdoutPath := filepath.Join(p.rootOrDefault(), "logs", base+".stdout.log")
	stderrPath := filepath.Join(p.rootOrDefault(), "logs", base+".stderr.log")
	if w.Run.Options.Process != nil {
		if path := strings.TrimSpace(w.Run.Options.Process.StdoutPath); path != "" {
			stdoutPath = path
		}
		if path := strings.TrimSpace(w.Run.Options.Process.StderrPath); path != "" {
			stderrPath = path
		}
	}
	return stdoutPath, stderrPath
}

func openAppend(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, oopsx.B("runtime", "process").Wrapf(err, "create log dir")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, oopsx.B("runtime", "process").Wrapf(err, "open log %s", filepath.Base(path))
	}
	return f, nil
}

func (p *Provider) stopTimeout(st state) time.Duration {
	if st.StopAfter != "" {
		if d, err := time.ParseDuration(st.StopAfter); err == nil && d > 0 {
			return d
		}
	}
	return defaultStopTimeout
}

func processStopTimeout(run deployv1.RunSpec) string {
	if run.Options.Process == nil {
		return ""
	}
	return strings.TrimSpace(run.Options.Process.GracefulStopTimeout)
}

func waitExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !processAlive(pid)
}

func (p *Provider) readState(meta deployv1.Metadata, workloadName string) (state, error) {
	var st state
	b, err := os.ReadFile(p.statePath(meta, workloadName))
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, oopsx.B("runtime", "process").Wrapf(err, "decode process state")
	}
	return st, nil
}

func (p *Provider) writeState(meta deployv1.Metadata, workloadName string, st state) error {
	path := p.statePath(meta, workloadName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
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
		return nil
	}
	if st.PID != pid {
		return nil
	}
	return p.removeState(meta, workloadName)
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
	return filepath.Join(config.DefaultDataRoot(), "runtime", "process")
}
