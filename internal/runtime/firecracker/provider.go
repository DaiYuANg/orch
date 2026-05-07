package firecracker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const (
	defaultBinaryPath = "firecracker"
	defaultBootArgs   = "console=ttyS0 reboot=k panic=1 pci=off root=/dev/vda"
	defaultVCPUCount  = 1
	defaultMemSizeMiB = 128
)

type Provider struct {
	logger *slog.Logger
	dns    *dnssvc.Service
	root   string
}

type state struct {
	PID       int                  `json:"pid"`
	APISocket string               `json:"apiSocket"`
	Network   *networkConfig       `json:"network,omitempty"`
	Metadata  deployv1.Metadata    `json:"metadata"`
	Workload  string               `json:"workload"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Artifact  string               `json:"artifact,omitempty"`
	StartedAt time.Time            `json:"startedAt"`
}

type vmConfig struct {
	ID             string
	BinaryPath     string
	APISocket      string
	KernelImage    string
	RootfsPath     string
	RootfsReadOnly bool
	BootArgs       string
	VCPUCount      int
	MemSizeMiB     int
	Network        *networkConfig
	StdoutPath     string
	StderrPath     string
}

type networkConfig struct {
	InterfaceID       string `json:"interfaceID"`
	TapDeviceName     string `json:"tapDeviceName"`
	GuestMAC          string `json:"guestMAC,omitempty"`
	AllowMMDSRequests bool   `json:"allowMMDSRequests,omitempty"`
}

type machineConfigRequest struct {
	VCPUCount  int `json:"vcpu_count"`
	MemSizeMiB int `json:"mem_size_mib"`
}

type bootSourceRequest struct {
	KernelImagePath string `json:"kernel_image_path"`
	BootArgs        string `json:"boot_args,omitempty"`
}

type driveRequest struct {
	DriveID      string `json:"drive_id"`
	PathOnHost   string `json:"path_on_host"`
	IsRootDevice bool   `json:"is_root_device"`
	IsReadOnly   bool   `json:"is_read_only"`
}

type networkInterfaceRequest struct {
	IfaceID           string `json:"iface_id"`
	HostDevName       string `json:"host_dev_name"`
	GuestMAC          string `json:"guest_mac,omitempty"`
	AllowMMDSRequests bool   `json:"allow_mmds_requests,omitempty"`
}

type actionRequest struct {
	ActionType string `json:"action_type"`
}

func NewProvider(logger *slog.Logger, dns *dnssvc.Service) *Provider {
	return &Provider{
		logger: logger,
		dns:    dns,
		root:   filepath.Join(config.DefaultDataRoot(), "runtime", "firecracker"),
	}
}

func (p *Provider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeFirecracker
}

func (p *Provider) buildConfig(meta deployv1.Metadata, w deployv1.Workload) (vmConfig, error) {
	if w.Run.Options.Firecracker == nil {
		return vmConfig{}, oopsx.B("runtime", "firecracker").Errorf("workload %q: run.runtimeOptions.firecracker is required", w.Name)
	}
	opts := w.Run.Options.Firecracker
	id := p.nameBase(meta, w.Name)
	cfg := vmConfig{
		ID:             id,
		BinaryPath:     strings.TrimSpace(opts.BinaryPath),
		APISocket:      strings.TrimSpace(opts.SocketPath),
		KernelImage:    strings.TrimSpace(opts.KernelImagePath),
		RootfsPath:     strings.TrimSpace(opts.RootfsPath),
		RootfsReadOnly: opts.RootfsReadOnly,
		BootArgs:       strings.TrimSpace(opts.BootArgs),
		VCPUCount:      opts.VCPUCount,
		MemSizeMiB:     opts.MemSizeMiB,
		Network:        firecrackerNetworkConfig(opts),
		StdoutPath:     filepath.Join(p.rootOrDefault(), "logs", id+".stdout.log"),
		StderrPath:     filepath.Join(p.rootOrDefault(), "logs", id+".stderr.log"),
	}
	if cfg.BinaryPath == "" {
		cfg.BinaryPath = defaultBinaryPath
	}
	if cfg.APISocket == "" {
		cfg.APISocket = filepath.Join(p.rootOrDefault(), "sockets", id+".sock")
	}
	if cfg.KernelImage == "" {
		return vmConfig{}, oopsx.B("runtime", "firecracker").Errorf("workload %q: kernel_image_path is required", w.Name)
	}
	if cfg.RootfsPath == "" {
		return vmConfig{}, oopsx.B("runtime", "firecracker").Errorf("workload %q: rootfs_path is required", w.Name)
	}
	if cfg.Network != nil && cfg.Network.TapDeviceName == "" {
		return vmConfig{}, oopsx.B("runtime", "firecracker").Errorf("workload %q: tap_device_name is required when firecracker network is configured", w.Name)
	}
	if cfg.BootArgs == "" {
		cfg.BootArgs = defaultBootArgsForRootfs(cfg.RootfsReadOnly)
	}
	if cfg.VCPUCount <= 0 {
		cfg.VCPUCount = defaultVCPUCount
	}
	if cfg.MemSizeMiB <= 0 {
		cfg.MemSizeMiB = defaultMemSizeMiB
	}
	return cfg, nil
}

func firecrackerNetworkConfig(opts *deployv1.FirecrackerOptions) *networkConfig {
	if opts == nil {
		return nil
	}
	tap := strings.TrimSpace(opts.TapDeviceName)
	if tap == "" && strings.TrimSpace(opts.NetworkInterfaceID) == "" && strings.TrimSpace(opts.GuestMAC) == "" && !opts.AllowMMDSRequests {
		return nil
	}
	id := strings.TrimSpace(opts.NetworkInterfaceID)
	if id == "" {
		id = "eth0"
	}
	return &networkConfig{
		InterfaceID:       id,
		TapDeviceName:     tap,
		GuestMAC:          strings.TrimSpace(opts.GuestMAC),
		AllowMMDSRequests: opts.AllowMMDSRequests,
	}
}

func defaultBootArgsForRootfs(readOnly bool) string {
	if readOnly {
		return defaultBootArgs + " ro"
	}
	return defaultBootArgs + " rw"
}

func (p *Provider) configureMachine(ctx context.Context, cfg vmConfig) error {
	if err := firecrackerPut(ctx, cfg.APISocket, "/machine-config", machineConfigRequest{
		VCPUCount:  cfg.VCPUCount,
		MemSizeMiB: cfg.MemSizeMiB,
	}); err != nil {
		return err
	}
	if err := firecrackerPut(ctx, cfg.APISocket, "/boot-source", bootSourceRequest{
		KernelImagePath: cfg.KernelImage,
		BootArgs:        cfg.BootArgs,
	}); err != nil {
		return err
	}
	if err := firecrackerPut(ctx, cfg.APISocket, "/drives/rootfs", driveRequest{
		DriveID:      "rootfs",
		PathOnHost:   cfg.RootfsPath,
		IsRootDevice: true,
		IsReadOnly:   cfg.RootfsReadOnly,
	}); err != nil {
		return err
	}
	if cfg.Network != nil {
		if err := firecrackerPut(ctx, cfg.APISocket, "/network-interfaces/"+url.PathEscape(cfg.Network.InterfaceID), networkInterfaceRequest{
			IfaceID:           cfg.Network.InterfaceID,
			HostDevName:       cfg.Network.TapDeviceName,
			GuestMAC:          cfg.Network.GuestMAC,
			AllowMMDSRequests: cfg.Network.AllowMMDSRequests,
		}); err != nil {
			return err
		}
	}
	return firecrackerPut(ctx, cfg.APISocket, "/actions", actionRequest{ActionType: "InstanceStart"})
}

func firecrackerPut(ctx context.Context, socketPath, apiPath string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "encode api request %s", apiPath)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "http://firecracker"+apiPath, bytes.NewReader(b))
	if err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "build api request %s", apiPath)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := firecrackerHTTPClient(socketPath).Do(req)
	if err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "firecracker api %s", apiPath)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return oopsx.B("runtime", "firecracker").Errorf("firecracker api %s: status %d: %s", apiPath, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}

func firecrackerHTTPClient(socketPath string) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
	}
}

func waitForSocket(ctx context.Context, socketPath string, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		if st, err := os.Stat(socketPath); err == nil && st.Mode()&os.ModeSocket != 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return oopsx.B("runtime", "firecracker").Errorf("firecracker api socket did not appear: %s", socketPath)
		case <-ticker.C:
		}
	}
}

func (p *Provider) openLogs(cfg vmConfig) (*os.File, *os.File, func(), error) {
	stdout, err := openAppend(cfg.StdoutPath)
	if err != nil {
		return nil, nil, func() {}, err
	}
	stderr, err := openAppend(cfg.StderrPath)
	if err != nil {
		_ = stdout.Close()
		return nil, nil, func() {}, err
	}
	closeLogs := func() {
		_ = stdout.Close()
		_ = stderr.Close()
	}
	return stdout, stderr, closeLogs, nil
}

func openAppend(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, oopsx.B("runtime", "firecracker").Wrapf(err, "create log dir")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, oopsx.B("runtime", "firecracker").Wrapf(err, "open log %s", filepath.Base(path))
	}
	return f, nil
}

func (p *Provider) readState(meta deployv1.Metadata, workloadName string) (state, error) {
	var st state
	b, err := os.ReadFile(p.statePath(meta, workloadName))
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, oopsx.B("runtime", "firecracker").Wrapf(err, "decode firecracker state")
	}
	return st, nil
}

func (p *Provider) writeState(meta deployv1.Metadata, workloadName string, st state) error {
	path := p.statePath(meta, workloadName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "create state dir")
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "encode firecracker state")
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return oopsx.B("runtime", "firecracker").Wrapf(err, "write firecracker state")
	}
	return nil
}

func (p *Provider) removeState(meta deployv1.Metadata, workloadName string) error {
	err := os.Remove(p.statePath(meta, workloadName))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return oopsx.B("runtime", "firecracker").Wrapf(err, "remove firecracker state")
}

func (p *Provider) removeStateIfPID(meta deployv1.Metadata, workloadName string, pid int) error {
	st, err := p.readState(meta, workloadName)
	if err != nil || st.PID != pid {
		return nil
	}
	return p.removeState(meta, workloadName)
}

func (p *Provider) statePath(meta deployv1.Metadata, workloadName string) string {
	return filepath.Join(p.rootOrDefault(), "state", p.nameBase(meta, workloadName)+".json")
}

func (p *Provider) nameBase(meta deployv1.Metadata, workloadName string) string {
	return fmt.Sprintf("%s-%s-%s",
		workloadmeta.SanitizeName(workloadmeta.NamespaceOrDefault(meta.Namespace)),
		workloadmeta.SanitizeName(meta.Name),
		workloadmeta.SanitizeName(workloadName),
	)
}

func (p *Provider) rootOrDefault() string {
	if strings.TrimSpace(p.root) != "" {
		return filepath.Clean(p.root)
	}
	return filepath.Join(config.DefaultDataRoot(), "runtime", "firecracker")
}

func firecrackerArtifactSummary(run deployv1.RunSpec) string {
	if summary := runconfig.ArtifactSummary(run); summary != "" {
		return summary
	}
	if run.Options.Firecracker != nil {
		return strings.TrimSpace(run.Options.Firecracker.RootfsPath)
	}
	return ""
}
