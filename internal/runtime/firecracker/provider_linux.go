//go:build linux

package firecracker

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v5"
	fc "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

const defaultStopTimeout = 5 * time.Second

var errProcessStillRunning = errors.New("process still running")

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	cfg, err := p.BuildConfig(meta, w)
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

	machineCtx := context.Background()
	cmd := firecrackerCommand(machineCtx, cfg, stdout, stderr)
	machine, err := fc.NewMachine(machineCtx, firecrackerMachineConfig(cfg), fc.WithProcessRunner(cmd))
	if err != nil {
		closeLogs()
		return oopsx.B("runtime", "firecracker").Wrapf(err, "create firecracker machine")
	}

	if err := machine.Start(machineCtx); err != nil {
		closeLogs()
		_ = os.Remove(cfg.APISocket)
		return oopsx.B("runtime", "firecracker").Wrapf(err, "start firecracker")
	}
	pid, err := machine.PID()
	if err != nil {
		_ = machine.StopVMM()
		closeLogs()
		_ = os.Remove(cfg.APISocket)
		return oopsx.B("runtime", "firecracker").Wrapf(err, "read firecracker pid")
	}

	cleanupStarted := func() {
		_ = machine.StopVMM()
		waitCtx, cancel := context.WithTimeout(context.Background(), defaultStopTimeout)
		defer cancel()
		_ = machine.Wait(waitCtx)
		closeLogs()
		_ = os.Remove(cfg.APISocket)
	}

	st := state{
		PID:       pid,
		APISocket: cfg.APISocket,
		Network:   cfg.Network,
		Metadata:  meta,
		Workload:  w.Name,
		Runtime:   w.Runtime,
		Artifact:  ArtifactSummary(w.Run),
		StartedAt: time.Now().UTC(),
	}
	if err := p.writeState(meta, w.Name, st); err != nil {
		cleanupStarted()
		return err
	}

	p.trackRunningVMM(meta, w.Name, runningVMM{pid: pid, stop: machine.StopVMM})
	go p.waitVMM(meta, w.Name, machine, pid, closeLogs, cfg.APISocket)
	p.logger.Info("firecracker workload running", "workload", w.Name, "pid", pid, "socket", cfg.APISocket)
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
	stoppedViaSDK := false
	if vm, ok := p.runningVMM(meta, workloadName); ok && vm.pid == st.PID && vm.stop != nil {
		if err := vm.stop(); err != nil {
			p.logger.Warn("firecracker sdk stop vmm", "workload", workloadName, "pid", st.PID, "error", err)
		} else {
			stoppedViaSDK = true
		}
	}
	proc, err := os.FindProcess(st.PID)
	if err == nil && proc != nil {
		if !stoppedViaSDK {
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				_ = proc.Kill()
			}
		}
		if !waitExit(ctx, st.PID, defaultStopTimeout) {
			_ = proc.Kill()
		}
	}
	p.untrackRunningVMM(meta, workloadName, st.PID)
	if st.APISocket != "" {
		_ = os.Remove(st.APISocket)
	}
	if err := p.removeState(meta, workloadName); err != nil {
		return err
	}
	p.logger.Info("firecracker workload stopped", "workload", workloadName, "pid", st.PID)
	return nil
}

func (p *Provider) Status(_ context.Context, meta deployv1.Metadata, workloadName string) (runtimeinfo.Status, error) {
	st, err := p.readState(meta, workloadName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return runtimeinfo.Status{Name: strings.TrimSpace(workloadName), Runtime: deployv1.RuntimeFirecracker, Status: "stopped"}, nil
		}
		return runtimeinfo.Status{}, err
	}
	status := "stopped"
	message := "firecracker state exists but pid is not running"
	if st.PID > 0 && processAlive(st.PID) {
		status = "running"
		message = ""
	}
	return runtimeinfo.Status{
		Name:      strings.TrimSpace(workloadName),
		Runtime:   deployv1.RuntimeFirecracker,
		Status:    status,
		NativeID:  strconv.Itoa(st.PID),
		StartedAt: st.StartedAt,
		UpdatedAt: time.Now().UTC(),
		Message:   message,
	}, nil
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

func firecrackerMachineConfig(cfg VMConfig) fc.Config {
	return fc.Config{
		SocketPath:      cfg.APISocket,
		VMID:            cfg.ID,
		KernelImagePath: cfg.KernelImage,
		KernelArgs:      cfg.BootArgs,
		Drives: []models.Drive{
			{
				DriveID:      fc.String("rootfs"),
				PathOnHost:   fc.String(cfg.RootfsPath),
				IsRootDevice: fc.Bool(true),
				IsReadOnly:   fc.Bool(cfg.RootfsReadOnly),
			},
		},
		NetworkInterfaces: firecrackerNetworkInterfaces(cfg.Network),
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  fc.Int64(int64(cfg.VCPUCount)),
			MemSizeMib: fc.Int64(int64(cfg.MemSizeMiB)),
		},
	}
}

func firecrackerCommand(ctx context.Context, cfg VMConfig, stdout, stderr *os.File) *exec.Cmd {
	return fc.VMCommandBuilder{}.
		WithBin(cfg.BinaryPath).
		WithSocketPath(cfg.APISocket).
		WithStdout(stdout).
		WithStderr(stderr).
		Build(ctx)
}

func firecrackerNetworkInterfaces(netCfg *NetworkConfig) fc.NetworkInterfaces {
	if netCfg == nil {
		return nil
	}
	return fc.NetworkInterfaces{
		{
			StaticConfiguration: &fc.StaticNetworkConfiguration{
				HostDevName: netCfg.TapDeviceName,
				MacAddress:  netCfg.GuestMAC,
			},
			AllowMMDS: netCfg.AllowMMDSRequests,
		},
	}
}

func (p *Provider) waitVMM(meta deployv1.Metadata, workloadName string, machine *fc.Machine, pid int, closeLogs func(), apiSocket string) {
	err := machine.Wait(context.Background())
	closeLogs()
	if err != nil {
		p.logger.Warn("firecracker workload exited", "workload", workloadName, "pid", pid, "error", err)
	} else {
		p.logger.Info("firecracker workload exited", "workload", workloadName, "pid", pid)
	}
	_ = os.Remove(apiSocket)
	p.untrackRunningVMM(meta, workloadName, pid)
	_ = p.removeStateIfPID(meta, workloadName, pid)
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

func waitExit(ctx context.Context, pid int, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, err := backoff.Retry(ctx, func() (struct{}, error) {
		if !processAlive(pid) {
			return struct{}{}, nil
		}
		return struct{}{}, errProcessStillRunning
	}, backoff.WithBackOff(backoff.NewConstantBackOff(100*time.Millisecond)))
	return err == nil || !processAlive(pid)
}
