//go:build linux

package firecracker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const defaultStopTimeout = 5 * time.Second

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	cfg, err := p.buildConfig(meta, w)
	if err != nil {
		return err
	}
	if err := p.ensureNoLiveState(meta, w.Name); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.APISocket), 0o755); err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "create api socket dir")
	}
	_ = os.Remove(cfg.APISocket)

	stdout, stderr, closeLogs, err := p.openLogs(cfg)
	if err != nil {
		return err
	}

	cmd := exec.Command(cfg.BinaryPath, "--api-sock", cfg.APISocket)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		closeLogs()
		return oopsx.B("runtime", "firecracker").Wrapf(err, "start firecracker")
	}

	cleanupStarted := func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		_ = terminateCommand(cmd, defaultStopTimeout)
		closeLogs()
		_ = os.Remove(cfg.APISocket)
	}

	if err := waitForSocket(ctx, cfg.APISocket, 5*time.Second); err != nil {
		cleanupStarted()
		return err
	}
	if err := p.configureMachine(ctx, cfg); err != nil {
		cleanupStarted()
		return err
	}

	st := state{
		PID:       cmd.Process.Pid,
		APISocket: cfg.APISocket,
		Network:   cfg.Network,
		Metadata:  meta,
		Workload:  w.Name,
		Runtime:   w.Runtime,
		Artifact:  firecrackerArtifactSummary(w.Run),
		StartedAt: time.Now().UTC(),
	}
	if err := p.writeState(meta, w.Name, st); err != nil {
		cleanupStarted()
		return err
	}

	go p.waitVMM(meta, w.Name, cmd, closeLogs, cfg.APISocket)
	p.logger.Info("firecracker workload running", "workload", w.Name, "pid", cmd.Process.Pid, "socket", cfg.APISocket)
	return nil
}

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	st, err := p.readState(meta, workloadName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	proc, err := os.FindProcess(st.PID)
	if err == nil && proc != nil {
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			_ = proc.Kill()
		} else if !waitExit(st.PID, defaultStopTimeout) {
			_ = proc.Kill()
		}
	}
	if st.APISocket != "" {
		_ = os.Remove(st.APISocket)
	}
	if err := p.removeState(meta, workloadName); err != nil {
		return err
	}
	p.logger.Info("firecracker workload stopped", "workload", workloadName, "pid", st.PID)
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
		return oopsx.B("runtime", "firecracker").Errorf("firecracker workload %q already has live pid %d", workloadName, st.PID)
	}
	if st.APISocket != "" {
		_ = os.Remove(st.APISocket)
	}
	return p.removeState(meta, workloadName)
}

func (p *Provider) waitVMM(meta deployv1.Metadata, workloadName string, cmd *exec.Cmd, closeLogs func(), apiSocket string) {
	err := cmd.Wait()
	closeLogs()
	if err != nil {
		p.logger.Warn("firecracker workload exited", "workload", workloadName, "pid", cmd.ProcessState.Pid(), "error", err)
	} else {
		p.logger.Info("firecracker workload exited", "workload", workloadName, "pid", cmd.ProcessState.Pid())
	}
	_ = os.Remove(apiSocket)
	_ = p.removeStateIfPID(meta, workloadName, cmd.ProcessState.Pid())
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
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

func terminateCommand(cmd *exec.Cmd, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		select {
		case <-done:
			return true
		case <-time.After(timeout):
			return false
		}
	}
}
