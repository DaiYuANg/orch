//go:build windows

package hostdns

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type windowsManager struct{}

func DefaultManager() Manager {
	return &windowsManager{}
}

func (m *windowsManager) Install(ctx context.Context, cfg Config) error {
	if err := requirePort53(cfg, "Windows NRPT"); err != nil {
		return err
	}
	script, err := renderWindowsScript("windows-install.ps1", cfg)
	if err != nil {
		return err
	}
	return runPowerShell(ctx, script)
}

func (m *windowsManager) Uninstall(ctx context.Context, cfg Config) error {
	script, err := renderWindowsScript("windows-uninstall.ps1", cfg)
	if err != nil {
		return err
	}
	return runPowerShell(ctx, script)
}

func (m *windowsManager) Status(ctx context.Context, cfg Config) (Status, error) {
	st := Status{Supported: true, Config: cfg}
	if err := requirePort53(cfg, "Windows NRPT"); err != nil {
		st.Supported = false
		st.Detail = err.Error()
		return st, nil
	}
	script, err := renderWindowsScript("windows-status.ps1", cfg)
	if err != nil {
		return st, err
	}
	out, err := runPowerShellOutput(ctx, script)
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

func renderWindowsScript(name string, cfg Config) (string, error) {
	return renderHostDNSTemplate(name, hostDNSTemplateData{
		Namespace:  "." + escapePowerShell(cfg.Zone),
		Nameserver: escapePowerShell(cfg.Nameserver),
	})
}

func runPowerShell(ctx context.Context, script string) error {
	_, err := runPowerShellOutput(ctx, script)
	return err
}

func runPowerShellOutput(ctx context.Context, script string) (string, error) {
	name := "powershell.exe"
	if _, err := exec.LookPath(name); err != nil {
		if _, pwshErr := exec.LookPath("pwsh.exe"); pwshErr == nil {
			name = "pwsh.exe"
		}
	}
	cmd := exec.CommandContext(ctx, name, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run PowerShell host DNS command: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func escapePowerShell(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
