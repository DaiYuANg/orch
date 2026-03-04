//go:build linux
// +build linux

package task

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

type systemdRuntimeExecutor struct {
	mu       sync.RWMutex
	labelsBy map[string]map[string]string
}

func newSystemdRuntimeExecutor() (RuntimeExecutor, error) {
	return &systemdRuntimeExecutor{
		labelsBy: make(map[string]map[string]string),
	}, nil
}

func (s *systemdRuntimeExecutor) Driver() string {
	return driverSystemd
}

func (s *systemdRuntimeExecutor) Ping(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "systemctl", "--version")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemd unavailable: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *systemdRuntimeExecutor) Run(ctx context.Context, spec RuntimeRunSpec) (string, error) {
	if len(spec.Cmd) == 0 {
		return "", fmt.Errorf("systemd run requires command")
	}
	unitName := sanitizeContainerName(strings.TrimSpace(spec.Name))
	if unitName == "" || unitName == "warden-task" {
		unitName = "warden-" + uuid.NewString()
	}
	unitName = ensureSystemdServiceName(unitName)

	args := []string{
		"run",
		"--collect",
		"--unit", unitName,
	}
	for _, envEntry := range mapToEnv(spec.Env) {
		args = append(args, "--setenv", envEntry)
	}
	args = append(args, spec.Cmd...)

	cmd := exec.CommandContext(ctx, "systemd-run", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("systemd-run %s failed: %w (%s)", unitName, err, strings.TrimSpace(string(out)))
	}

	s.mu.Lock()
	s.labelsBy[unitName] = cloneStringMap(spec.Labels)
	s.mu.Unlock()
	return unitName, nil
}

func (s *systemdRuntimeExecutor) Stop(ctx context.Context, containerID string) error {
	unitName := ensureSystemdServiceName(strings.TrimSpace(containerID))
	if unitName == "" {
		return fmt.Errorf("container id is empty")
	}
	stop := exec.CommandContext(ctx, "systemctl", "stop", unitName)
	if out, err := stop.CombinedOutput(); err != nil {
		output := strings.TrimSpace(string(out))
		// "not loaded" is equivalent to already removed.
		if !strings.Contains(strings.ToLower(output), "not loaded") {
			return fmt.Errorf("systemctl stop %s failed: %w (%s)", unitName, err, output)
		}
	}
	_, _ = exec.CommandContext(ctx, "systemctl", "reset-failed", unitName).CombinedOutput()

	s.mu.Lock()
	delete(s.labelsBy, unitName)
	s.mu.Unlock()
	return nil
}

func (s *systemdRuntimeExecutor) Status(ctx context.Context, containerID string) (RuntimeStatus, error) {
	unitName := ensureSystemdServiceName(strings.TrimSpace(containerID))
	if unitName == "" {
		return RuntimeStatus{}, fmt.Errorf("container id is empty")
	}

	cmd := exec.CommandContext(ctx, "systemctl", "show", unitName, "--property=ActiveState,SubState,ExecMainStatus,Result", "--value")
	out, err := cmd.CombinedOutput()
	if err != nil {
		output := strings.TrimSpace(strings.ToLower(string(out)))
		if strings.Contains(output, "not-found") || strings.Contains(output, "could not be found") || strings.Contains(output, "not loaded") {
			return RuntimeStatus{
				ContainerID: unitName,
				Name:        unitName,
				Running:     false,
				State:       "not_found",
			}, nil
		}
		return RuntimeStatus{}, fmt.Errorf("systemctl show %s failed: %w (%s)", unitName, err, strings.TrimSpace(string(out)))
	}
	lines := lo.Map(strings.Split(strings.TrimSpace(string(out)), "\n"), func(item string, _ int) string {
		return strings.TrimSpace(item)
	})
	activeState := valueAt(lines, 0)
	subState := valueAt(lines, 1)
	exitCode := mustAtoi(valueAt(lines, 2), 0)
	result := strings.ToLower(valueAt(lines, 3))
	state := activeState
	if subState != "" {
		state = activeState + "/" + subState
	}
	running := lo.Contains([]string{"active", "activating", "reloading"}, strings.ToLower(activeState))
	errText := ""
	if result != "" && result != "success" {
		errText = result
	}

	return RuntimeStatus{
		ContainerID: unitName,
		Name:        unitName,
		Running:     running,
		State:       state,
		ExitCode:    exitCode,
		Error:       errText,
	}, nil
}

func (s *systemdRuntimeExecutor) Logs(ctx context.Context, containerID string, tail int) (string, error) {
	unitName := ensureSystemdServiceName(strings.TrimSpace(containerID))
	if unitName == "" {
		return "", fmt.Errorf("container id is empty")
	}
	if tail <= 0 {
		tail = 200
	}
	cmd := exec.CommandContext(ctx, "journalctl", "-u", unitName, "-n", strconv.Itoa(tail), "--no-pager")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("journalctl %s failed: %w (%s)", unitName, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (s *systemdRuntimeExecutor) List(ctx context.Context, _ bool, filters map[string][]string) ([]RuntimeContainer, error) {
	s.mu.RLock()
	knownUnits := lo.Keys(s.labelsBy)
	s.mu.RUnlock()

	items := make([]RuntimeContainer, 0, len(knownUnits))
	for _, unit := range knownUnits {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		statusCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		status, err := s.Status(statusCtx, unit)
		cancel()
		if err != nil || status.State == "not_found" {
			continue
		}
		labels := s.labelsFor(unit)
		if !matchLabelFilters(labels, filters) {
			continue
		}
		items = append(items, RuntimeContainer{
			ID:     unit,
			Names:  []string{unit},
			Labels: labels,
		})
	}
	return items, nil
}

func (s *systemdRuntimeExecutor) labelsFor(unit string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneStringMap(s.labelsBy[unit])
}

func ensureSystemdServiceName(unit string) string {
	name := strings.TrimSpace(unit)
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, ".service") {
		return name
	}
	return name + ".service"
}
