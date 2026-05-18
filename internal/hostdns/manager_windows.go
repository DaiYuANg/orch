//go:build windows

package hostdns

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type powershellRunner func(context.Context, string) (string, error)

type windowsManager struct {
	runner powershellRunner
}

func DefaultManager() Manager {
	return newWindowsManager(runPowerShellOutput)
}

func newWindowsManager(runner powershellRunner) *windowsManager {
	if runner == nil {
		runner = runPowerShellOutput
	}
	return &windowsManager{runner: runner}
}

func (m *windowsManager) Install(ctx context.Context, cfg Config) error {
	if err := requirePort53(cfg, "Windows NRPT"); err != nil {
		return err
	}
	script, err := renderWindowsScript("windows-install.ps1", cfg)
	if err != nil {
		return err
	}
	return m.runPowerShell(ctx, script)
}

func (m *windowsManager) Uninstall(ctx context.Context, cfg Config) error {
	script, err := renderWindowsScript("windows-uninstall.ps1", cfg)
	if err != nil {
		return err
	}
	return m.runPowerShell(ctx, script)
}

func (m *windowsManager) Status(ctx context.Context, cfg Config) (Status, error) {
	st := Status{Supported: true, Config: cfg}
	if detail, ok := windowsNRPTSupported(cfg); !ok {
		st.Supported = false
		st.Detail = detail
		return st, nil
	}
	script, err := renderWindowsScript("windows-status.ps1", cfg)
	if err != nil {
		return st, err
	}
	out, err := m.runPowerShellOutput(ctx, script)
	if err != nil {
		return st, err
	}
	st.Installed = strings.Contains(strings.ToLower(out), "installed")
	if st.Installed {
		st.Detail = "Windows NRPT rule is installed"
	} else {
		st.Detail = "Windows NRPT rule is not installed"
	}
	return st, nil
}

func windowsNRPTSupported(cfg Config) (string, bool) {
	if err := requirePort53(cfg, "Windows NRPT"); err != nil {
		return err.Error(), false
	}
	return "", true
}

func renderWindowsScript(name string, cfg Config) (string, error) {
	return RenderTemplate(name, TemplateData{
		Namespace:  "." + escapePowerShell(cfg.Zone),
		Nameserver: escapePowerShell(cfg.Nameserver),
	})
}

func (m *windowsManager) runPowerShell(ctx context.Context, script string) error {
	_, err := m.runPowerShellOutput(ctx, script)
	return err
}

func (m *windowsManager) runPowerShellOutput(ctx context.Context, script string) (string, error) {
	if m.runner == nil {
		m.runner = runPowerShellOutput
	}
	return m.runner(ctx, script)
}

func runPowerShellOutput(ctx context.Context, script string) (string, error) {
	name := "powershell.exe"
	if _, err := exec.LookPath(name); err != nil {
		if _, pwshErr := exec.LookPath("pwsh.exe"); pwshErr == nil {
			name = "pwsh.exe"
		}
	}
	cmd := &exec.Cmd{
		Path: name,
		Args: []string{name, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script},
	}
	out, err := combinedOutputContext(ctx, cmd)
	if err != nil {
		return "", fmt.Errorf("run PowerShell host DNS command: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func combinedOutputContext(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return out.Bytes(), err
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return out.Bytes(), err
	case <-ctx.Done():
		var killErr error
		if cmd.Process != nil {
			killErr = cmd.Process.Kill()
		}
		waitErr := <-done
		return out.Bytes(), errors.Join(ctx.Err(), killErr, waitErr)
	}
}

func escapePowerShell(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
