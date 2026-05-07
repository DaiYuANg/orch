//go:build windows

package windowsservice

import (
	"context"
	"errors"
	"strings"
	"syscall"
	"time"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/pkg/oopsx"
	"golang.org/x/sys/windows"
	winsvc "golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	exe, args, ok := runconfig.ProcessCommand(w.Run)
	if !ok {
		return oopsx.B("runtime", "windows-service").Errorf("workload %q: run.exec.command or run.artifact.path is required", w.Name)
	}
	startType, err := serviceStartType(w)
	if err != nil {
		return err
	}

	manager, err := mgr.Connect()
	if err != nil {
		return oopsx.B("runtime", "windows-service").Wrapf(err, "connect service manager")
	}
	defer manager.Disconnect()

	serviceName := p.workloadServiceName(meta, w)
	if existing, err := manager.OpenService(serviceName); err == nil {
		_ = existing.Close()
		return oopsx.B("runtime", "windows-service").Errorf("windows service %q already exists", serviceName)
	}

	service, err := manager.CreateService(serviceName, exe, mgr.Config{
		DisplayName:  p.workloadDisplayName(meta, w),
		StartType:    startType,
		ErrorControl: mgr.ErrorNormal,
		Description:  "orch managed workload",
	}, args.Values()...)
	if err != nil {
		return oopsx.B("runtime", "windows-service").Wrapf(err, "create service %s", serviceName)
	}
	defer service.Close()

	if err := service.Start(); err != nil {
		_ = service.Delete()
		return oopsx.B("runtime", "windows-service").Wrapf(err, "start service %s", serviceName)
	}

	st := state{
		ServiceName: serviceName,
		Metadata:    meta,
		Workload:    w.Name,
		Runtime:     w.Runtime,
		Artifact:    runconfig.ArtifactSummary(w.Run),
		StartedAt:   time.Now().UTC(),
	}
	if err := p.writeState(meta, w.Name, st); err != nil {
		_ = service.Delete()
		return err
	}
	if p.dns != nil {
		if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, "127.0.0.1"); err != nil {
			_ = service.Delete()
			_ = p.removeState(meta, w.Name)
			return oopsx.B("runtime", "dns").Wrapf(err, "upsert workload DNS")
		}
	}
	p.logger.Info("windows service workload running", "workload", w.Name, "service", serviceName)
	return nil
}

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	serviceName := defaultServiceName(meta, workloadName)
	if st, err := p.readState(meta, workloadName); err == nil {
		if strings.TrimSpace(st.ServiceName) != "" {
			serviceName = st.ServiceName
		}
	} else if !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return err
	}

	manager, err := mgr.Connect()
	if err != nil {
		return oopsx.B("runtime", "windows-service").Wrapf(err, "connect service manager")
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(serviceName)
	if err != nil {
		if p.dns != nil {
			_ = p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName)
		}
		return p.removeState(meta, workloadName)
	}
	defer service.Close()

	if status, err := service.Query(); err == nil && status.State != winsvc.Stopped {
		if _, ctrlErr := service.Control(winsvc.Stop); ctrlErr != nil {
			p.logger.Warn("windows service stop control", "service", serviceName, "error", ctrlErr)
		}
		waitServiceStopped(service, 10*time.Second)
	}
	if err := service.Delete(); err != nil {
		return oopsx.B("runtime", "windows-service").Wrapf(err, "delete service %s", serviceName)
	}
	if p.dns != nil {
		if err := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); err != nil {
			return oopsx.B("runtime", "dns").Wrapf(err, "remove workload DNS")
		}
	}
	if err := p.removeState(meta, workloadName); err != nil {
		return err
	}
	p.logger.Info("windows service workload stopped", "workload", workloadName, "service", serviceName)
	return nil
}

func serviceStartType(w deployv1.Workload) (uint32, error) {
	startType := "manual"
	if w.Run.Options.WindowsService != nil {
		if v := strings.TrimSpace(strings.ToLower(w.Run.Options.WindowsService.StartType)); v != "" {
			startType = v
		}
	}
	switch startType {
	case "manual", "demand":
		return mgr.StartManual, nil
	case "automatic", "auto":
		return mgr.StartAutomatic, nil
	case "disabled":
		return mgr.StartDisabled, nil
	default:
		return 0, oopsx.B("runtime", "windows-service").Errorf("invalid windowsService.startType %q", startType)
	}
}

func waitServiceStopped(service *mgr.Service, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := service.Query()
		if err != nil || status.State == winsvc.Stopped {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}
