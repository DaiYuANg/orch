//go:build linux

package hostdns

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const linuxResolvedDropInPath = "/etc/systemd/resolved.conf.d/orch.conf"

type linuxManager struct {
	path string
}

func DefaultManager() Manager {
	return &linuxManager{path: linuxResolvedDropInPath}
}

func (m *linuxManager) Install(ctx context.Context, cfg Config) error {
	content, err := linuxResolvedDropIn(cfg)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl is required for systemd-resolved host DNS install: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create resolved drop-in dir: %w", err)
	}
	if err := os.WriteFile(m.path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", m.path, err)
	}
	if err := runCommand(ctx, "systemctl", "restart", "systemd-resolved"); err != nil {
		return fmt.Errorf("restart systemd-resolved: %w", err)
	}
	return nil
}

func (m *linuxManager) Uninstall(ctx context.Context, _ Config) error {
	if err := os.Remove(m.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", m.path, err)
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		if err := runCommand(ctx, "systemctl", "restart", "systemd-resolved"); err != nil {
			return fmt.Errorf("restart systemd-resolved: %w", err)
		}
	}
	return nil
}

func (m *linuxManager) Status(_ context.Context, cfg Config) (Status, error) {
	st := Status{Supported: true, Config: cfg}
	content, err := linuxResolvedDropIn(cfg)
	if err != nil {
		return st, err
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			st.Detail = "systemd-resolved orch drop-in is not installed"
			return st, nil
		}
		return st, fmt.Errorf("read %s: %w", m.path, err)
	}
	want := strings.TrimSpace(content)
	got := strings.TrimSpace(string(data))
	st.Installed = got == want
	if st.Installed {
		st.Detail = "systemd-resolved drop-in is installed"
	} else {
		st.Detail = "systemd-resolved drop-in exists but differs from desired orch config"
	}
	return st, nil
}

func linuxResolvedDropIn(cfg Config) (string, error) {
	return renderHostDNSTemplate("linux-resolved.conf.tmpl", hostDNSTemplateData{
		Zone:       cfg.Zone,
		Nameserver: cfg.Nameserver,
		DNSServer:  dnsServerEndpoint(cfg),
	})
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
