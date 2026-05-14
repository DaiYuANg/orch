//go:build linux

package hostdns

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
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
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return fmt.Errorf("create resolved drop-in dir: %w", err)
	}
	if err := os.WriteFile(m.path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", m.path, err)
	}
	if err := restartSystemdResolved(ctx); err != nil {
		return fmt.Errorf("restart systemd-resolved: %w", err)
	}
	return nil
}

func (m *linuxManager) Uninstall(ctx context.Context, _ Config) error {
	if err := os.Remove(m.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", m.path, err)
	}
	if err := restartSystemdResolved(ctx); err != nil {
		return fmt.Errorf("restart systemd-resolved: %w", err)
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
	return RenderTemplate("linux-resolved.conf.tmpl", TemplateData{
		Zone:       cfg.Zone,
		Nameserver: cfg.Nameserver,
		DNSServer:  DNSServerEndpoint(cfg),
	})
}

func restartSystemdResolved(ctx context.Context) error {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return fmt.Errorf("connect systemd dbus: %w", err)
	}
	defer conn.Close()

	ch := make(chan string, 1)
	if _, err := conn.RestartUnitContext(ctx, "systemd-resolved.service", "replace", ch); err != nil {
		return fmt.Errorf("restart unit: %w", err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-ch:
		if strings.TrimSpace(result) != "done" {
			return fmt.Errorf("restart unit: %s", result)
		}
	}
	return nil
}
