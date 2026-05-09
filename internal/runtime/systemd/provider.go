package systemd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/coreos/go-systemd/v22/unit"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const (
	defaultWantedBy      = "multi-user.target"
	systemdSystemUnitDir = "/etc/systemd/system"
)

type Provider struct {
	logger *slog.Logger
	dns    *dnssvc.Service
	root   string
}

type state struct {
	UnitName  string               `json:"unitName"`
	UnitPath  string               `json:"unitPath"`
	Metadata  deployv1.Metadata    `json:"metadata"`
	Workload  string               `json:"workload"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Artifact  string               `json:"artifact,omitempty"`
	StartedAt time.Time            `json:"startedAt"`
}

func NewProvider(logger *slog.Logger, dns *dnssvc.Service) *Provider {
	return &Provider{
		logger: logger,
		dns:    dns,
		root:   filepath.Join(config.DefaultDataRoot(), "runtime", "systemd"),
	}
}

func (p *Provider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeSystemd
}

func (p *Provider) workloadUnitName(meta deployv1.Metadata, w deployv1.Workload) string {
	if w.Run.Options.Systemd != nil {
		if name := normalizeUnitName(w.Run.Options.Systemd.UnitName); name != "" {
			return name
		}
	}
	return defaultUnitName(meta, w.Name)
}

func defaultUnitName(meta deployv1.Metadata, workloadName string) string {
	return normalizeUnitName(fmt.Sprintf("orch-%s-%s-%s",
		workloadmeta.SanitizeName(workloadmeta.NamespaceOrDefault(meta.Namespace)),
		workloadmeta.SanitizeName(meta.Name),
		workloadmeta.SanitizeName(workloadName),
	))
}

func normalizeUnitName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.Join(strings.Fields(name), "-")
	if !strings.HasSuffix(name, ".service") {
		name += ".service"
	}
	return name
}

func renderUnit(meta deployv1.Metadata, w deployv1.Workload, unitName string) (string, error) {
	exe, args, ok := runconfig.ProcessCommand(w.Run)
	if !ok {
		return "", oopsx.B("runtime", "systemd").Errorf("workload %q: run.exec.command or run.artifact.path is required", w.Name)
	}

	opts := []*unit.UnitOption{
		unit.NewUnitOption("Unit", "Description", fmt.Sprintf("orch workload %s/%s/%s", workloadmeta.NamespaceOrDefault(meta.Namespace), meta.Name, w.Name)),
		unit.NewUnitOption("Unit", "After", "network-online.target"),
		unit.NewUnitOption("Unit", "Wants", "network-online.target"),
		unit.NewUnitOption("Service", "Type", "simple"),
	}
	if id := strings.TrimSuffix(strings.TrimSpace(unitName), ".service"); id != "" {
		opts = append(opts, unit.NewUnitOption("Service", "SyslogIdentifier", id))
	}
	if cwd := strings.TrimSpace(w.Run.Cwd); cwd != "" {
		opts = append(opts, unit.NewUnitOption("Service", "WorkingDirectory", systemdQuote(cwd)))
	}
	if user := systemdUser(w); user != "" {
		opts = append(opts, unit.NewUnitOption("Service", "User", user))
	}
	if group := systemdGroup(w); group != "" {
		opts = append(opts, unit.NewUnitOption("Service", "Group", group))
	}
	runconfig.Env(w.EnvList()).Range(func(_ int, env string) bool {
		opts = append(opts, unit.NewUnitOption("Service", "Environment", systemdQuote(env)))
		return true
	})
	opts = append(opts, unit.NewUnitOption("Service", "ExecStart", systemdCommandLine(exe, args)))
	if restart := systemdRestart(w); restart != "" {
		opts = append(opts, unit.NewUnitOption("Service", "Restart", restart))
	}
	if restartSec := systemdRestartSec(w); restartSec != "" {
		opts = append(opts, unit.NewUnitOption("Service", "RestartSec", restartSec))
	}
	opts = append(opts, unit.NewUnitOption("Install", "WantedBy", systemdWantedBy(w)))

	b, err := io.ReadAll(unit.Serialize(opts))
	if err != nil {
		return "", oopsx.B("runtime", "systemd").Wrapf(err, "serialize unit %s", unitName)
	}
	return string(b), nil
}

func systemdUser(w deployv1.Workload) string {
	if w.Run.Options.Systemd != nil {
		if user := strings.TrimSpace(w.Run.Options.Systemd.User); user != "" {
			return user
		}
	}
	return strings.TrimSpace(w.Run.User)
}

func systemdGroup(w deployv1.Workload) string {
	if w.Run.Options.Systemd == nil {
		return ""
	}
	return strings.TrimSpace(w.Run.Options.Systemd.Group)
}

func systemdRestart(w deployv1.Workload) string {
	if w.Run.Options.Systemd != nil {
		if restart := strings.TrimSpace(w.Run.Options.Systemd.Restart); restart != "" {
			return restart
		}
	}
	switch w.Kind {
	case deployv1.WorkloadKindService, deployv1.WorkloadKindStateful, deployv1.WorkloadKindWorker:
		return "on-failure"
	default:
		return ""
	}
}

func systemdRestartSec(w deployv1.Workload) string {
	if w.Run.Options.Systemd == nil {
		return ""
	}
	return strings.TrimSpace(w.Run.Options.Systemd.RestartSec)
}

func systemdWantedBy(w deployv1.Workload) string {
	if w.Run.Options.Systemd != nil {
		if wantedBy := strings.TrimSpace(w.Run.Options.Systemd.WantedBy); wantedBy != "" {
			return wantedBy
		}
	}
	return defaultWantedBy
}

func systemdCommandLine(exe string, args *list.List[string]) string {
	parts := list.NewListWithCapacity[string](args.Len() + 1)
	parts.Add(systemdQuote(exe))
	args.Range(func(_ int, arg string) bool {
		parts.Add(systemdQuote(arg))
		return true
	})
	return parts.Join(" ")
}

func systemdQuote(s string) string {
	return strconv.Quote(strings.ReplaceAll(s, "%", "%%"))
}

func systemdUnitPath(unitName string) string {
	return filepath.Join(systemdSystemUnitDir, unitName)
}

func (p *Provider) readState(meta deployv1.Metadata, workloadName string) (state, error) {
	var st state
	b, err := os.ReadFile(p.statePath(meta, workloadName))
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, oopsx.B("runtime", "systemd").Wrapf(err, "decode systemd state")
	}
	return st, nil
}

func (p *Provider) writeState(meta deployv1.Metadata, workloadName string, st state) error {
	path := p.statePath(meta, workloadName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return oopsx.B("runtime", "systemd").Wrapf(err, "create state dir")
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return oopsx.B("runtime", "systemd").Wrapf(err, "encode systemd state")
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return oopsx.B("runtime", "systemd").Wrapf(err, "write systemd state")
	}
	return nil
}

func (p *Provider) removeState(meta deployv1.Metadata, workloadName string) error {
	err := os.Remove(p.statePath(meta, workloadName))
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return oopsx.B("runtime", "systemd").Wrapf(err, "remove systemd state")
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
	return filepath.Join(config.DefaultDataRoot(), "runtime", "systemd")
}
