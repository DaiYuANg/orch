//go:build windows
// +build windows

package task

import (
	"context"
	"fmt"
	"strings"
	"sync"

	winservice "github.com/DaiYuANg/warden/internal/runtime_engine/windows_service"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"golang.org/x/sys/windows/svc"
)

type windowsServiceRuntimeExecutor struct {
	manager *winservice.WindowsServiceManager

	mu       sync.RWMutex
	labelsBy map[string]map[string]string
	imageBy  map[string]string
}

func newWindowsServiceRuntimeExecutor() (RuntimeExecutor, error) {
	manager, err := winservice.NewWindowsServiceManager()
	if err != nil {
		return nil, err
	}
	return &windowsServiceRuntimeExecutor{
		manager:  manager,
		labelsBy: make(map[string]map[string]string),
		imageBy:  make(map[string]string),
	}, nil
}

func (w *windowsServiceRuntimeExecutor) Driver() string {
	return driverWindowsService
}

func (w *windowsServiceRuntimeExecutor) Ping(context.Context) error {
	return nil
}

func (w *windowsServiceRuntimeExecutor) Run(_ context.Context, spec RuntimeRunSpec) (string, error) {
	exePath, args, err := resolveWindowsServiceCommand(spec)
	if err != nil {
		return "", err
	}

	serviceName := sanitizeContainerName(strings.TrimSpace(spec.Name))
	if serviceName == "" || serviceName == "warden-task" {
		serviceName = "warden-" + uuid.NewString()
	}

	if err := w.manager.CreateService(serviceName, serviceName, exePath, args...); err != nil {
		_ = w.manager.DeleteService(serviceName)
		if retryErr := w.manager.CreateService(serviceName, serviceName, exePath, args...); retryErr != nil {
			return "", fmt.Errorf("create windows service %s failed: %w", serviceName, retryErr)
		}
	}
	if err := w.manager.StartService(serviceName); err != nil {
		_ = w.manager.DeleteService(serviceName)
		return "", fmt.Errorf("start windows service %s failed: %w", serviceName, err)
	}

	w.mu.Lock()
	w.labelsBy[serviceName] = cloneStringMap(spec.Labels)
	w.imageBy[serviceName] = exePath
	w.mu.Unlock()
	return serviceName, nil
}

func (w *windowsServiceRuntimeExecutor) Stop(_ context.Context, containerID string) error {
	id := strings.TrimSpace(containerID)
	if id == "" {
		return fmt.Errorf("container id is empty")
	}
	if err := w.manager.StopService(id); err != nil && !isWindowsServiceMissing(err) {
		return err
	}
	if err := w.manager.DeleteService(id); err != nil && !isWindowsServiceMissing(err) {
		return err
	}

	w.mu.Lock()
	delete(w.labelsBy, id)
	delete(w.imageBy, id)
	w.mu.Unlock()
	return nil
}

func (w *windowsServiceRuntimeExecutor) Status(_ context.Context, containerID string) (RuntimeStatus, error) {
	id := strings.TrimSpace(containerID)
	if id == "" {
		return RuntimeStatus{}, fmt.Errorf("container id is empty")
	}
	status, err := w.manager.QueryService(id)
	if err != nil {
		if isWindowsServiceMissing(err) {
			return RuntimeStatus{
				ContainerID: id,
				Name:        id,
				Running:     false,
				State:       "not_found",
			}, nil
		}
		return RuntimeStatus{}, err
	}
	state := windowsSvcState(status.State)
	return RuntimeStatus{
		ContainerID: id,
		Name:        id,
		Running:     status.State == svc.Running,
		State:       state,
		ExitCode:    int(status.Win32ExitCode),
		Error:       lo.Ternary(status.Win32ExitCode != 0, fmt.Sprintf("win32_exit_code=%d", status.Win32ExitCode), ""),
	}, nil
}

func (w *windowsServiceRuntimeExecutor) Logs(context.Context, string, int) (string, error) {
	return "", fmt.Errorf("windows-service logs is not implemented yet")
}

func (w *windowsServiceRuntimeExecutor) List(ctx context.Context, _ bool, filters map[string][]string) ([]RuntimeContainer, error) {
	w.mu.RLock()
	known := lo.Keys(w.labelsBy)
	w.mu.RUnlock()

	items := make([]RuntimeContainer, 0, len(known))
	for _, id := range known {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		status, err := w.Status(ctx, id)
		if err != nil || status.State == "not_found" {
			continue
		}
		labels := w.labelsFor(id)
		if !matchLabelFilters(labels, filters) {
			continue
		}
		items = append(items, RuntimeContainer{
			ID:     id,
			Names:  []string{id},
			Image:  w.imageBy[id],
			Labels: labels,
		})
	}
	return items, nil
}

func (w *windowsServiceRuntimeExecutor) labelsFor(id string) map[string]string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return cloneStringMap(w.labelsBy[id])
}

func resolveWindowsServiceCommand(spec RuntimeRunSpec) (string, []string, error) {
	image := strings.TrimSpace(spec.Image)
	if image != "" {
		return image, spec.Cmd, nil
	}
	if len(spec.Cmd) == 0 {
		return "", nil, fmt.Errorf("windows-service run requires image or command")
	}
	return strings.TrimSpace(spec.Cmd[0]), spec.Cmd[1:], nil
}

func windowsSvcState(state svc.State) string {
	switch state {
	case svc.Stopped:
		return "stopped"
	case svc.StartPending:
		return "start_pending"
	case svc.StopPending:
		return "stop_pending"
	case svc.Running:
		return "running"
	case svc.ContinuePending:
		return "continue_pending"
	case svc.PausePending:
		return "pause_pending"
	case svc.Paused:
		return "paused"
	default:
		return fmt.Sprintf("state_%d", state)
	}
}

func isWindowsServiceMissing(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "service does not exist") ||
		strings.Contains(text, "cannot find") ||
		strings.Contains(text, "1060")
}
