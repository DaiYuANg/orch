package hostdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/lyonbrown4d/orch/internal/config"
)

type Config struct {
	Zone       string `json:"zone"`
	Nameserver string `json:"nameserver"`
	Port       int    `json:"port"`
	Listen     string `json:"listen"`
}

type Status struct {
	Supported bool   `json:"supported"`
	Installed bool   `json:"installed"`
	Detail    string `json:"detail,omitempty"`
	Config    Config `json:"config"`
}

type Manager interface {
	Install(ctx context.Context, cfg Config) error
	Uninstall(ctx context.Context, cfg Config) error
	Status(ctx context.Context, cfg Config) (Status, error)
}

func ConfigFromOrch(cfg config.Config) (Config, error) {
	zone := strings.Trim(strings.ToLower(strings.TrimSpace(cfg.DNS.ZoneName())), ".")
	if zone == "" {
		return Config{}, errors.New("dns zone is required")
	}
	listen := strings.TrimSpace(cfg.DNS.Listen)
	if listen == "" {
		return Config{}, errors.New("dns.listen is required")
	}
	host, portRaw, err := net.SplitHostPort(listen)
	if err != nil {
		return Config{}, fmt.Errorf("parse dns.listen %q: %w", listen, err)
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port <= 0 || port > 65535 {
		return Config{}, fmt.Errorf("dns.listen port must be 1-65535: %q", portRaw)
	}
	nameserver := hostDNSNameserver(host)
	if nameserver == "" {
		return Config{}, fmt.Errorf("dns.listen host must be an IP address: %q", host)
	}
	return Config{
		Zone:       zone,
		Nameserver: nameserver,
		Port:       port,
		Listen:     listen,
	}, nil
}

func hostDNSNameserver(host string) string {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}
	if ip.IsUnspecified() {
		return "127.0.0.1"
	}
	return ip.String()
}

func requirePort53(cfg Config, platform string) error {
	if cfg.Port != 53 {
		return fmt.Errorf("%s host DNS integration requires dns.listen on port 53, got %s", platform, cfg.Listen)
	}
	return nil
}

// DNSServerEndpoint returns the server endpoint string used by host DNS installers.
func DNSServerEndpoint(cfg Config) string {
	if cfg.Port == 53 {
		return cfg.Nameserver
	}
	return net.JoinHostPort(cfg.Nameserver, strconv.Itoa(cfg.Port))
}
