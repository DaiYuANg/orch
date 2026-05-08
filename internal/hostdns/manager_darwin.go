//go:build darwin

package hostdns

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const darwinResolverDir = "/etc/resolver"

type darwinManager struct{}

func DefaultManager() Manager {
	return &darwinManager{}
}

func (m *darwinManager) Install(_ context.Context, cfg Config) error {
	if err := os.MkdirAll(darwinResolverDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", darwinResolverDir, err)
	}
	content, err := darwinResolverFile(cfg)
	if err != nil {
		return err
	}
	path := darwinResolverPath(cfg)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (m *darwinManager) Uninstall(_ context.Context, cfg Config) error {
	path := darwinResolverPath(cfg)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func (m *darwinManager) Status(_ context.Context, cfg Config) (Status, error) {
	st := Status{Supported: true, Config: cfg}
	path := darwinResolverPath(cfg)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			st.Detail = "macOS resolver file is not installed"
			return st, nil
		}
		return st, fmt.Errorf("read %s: %w", path, err)
	}
	content, err := darwinResolverFile(cfg)
	if err != nil {
		return st, err
	}
	st.Installed = strings.TrimSpace(string(data)) == strings.TrimSpace(content)
	if st.Installed {
		st.Detail = "macOS resolver file is installed"
	} else {
		st.Detail = "macOS resolver file exists but differs from desired orch config"
	}
	return st, nil
}

func darwinResolverPath(cfg Config) string {
	return filepath.Join(darwinResolverDir, cfg.Zone)
}

func darwinResolverFile(cfg Config) (string, error) {
	return renderHostDNSTemplate("darwin-resolver.tmpl", hostDNSTemplateData{
		Nameserver: cfg.Nameserver,
		Port:       cfg.Port,
	})
}
