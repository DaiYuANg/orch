package systemd

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/coreos/go-systemd/v22/unit"

	"github.com/lyonbrown4d/orch/internal/config"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	"github.com/lyonbrown4d/orch/internal/runtime/runconfig"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
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

// DefaultUnitName returns the systemd unit name for an orch workload.
func DefaultUnitName(meta deployv1.Metadata, workloadName string) string {
	return NormalizeUnitName(fmt.Sprintf("orch-%s-%s-%s",
		workloadmeta.SanitizeName(workloadmeta.NamespaceOrDefault(meta.Namespace)),
		workloadmeta.SanitizeName(meta.Name),
		workloadmeta.SanitizeName(workloadName),
	))
}

// NormalizeUnitName normalizes a systemd unit name and appends .service when needed.
func NormalizeUnitName(name string) string {
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

// RenderUnit renders a systemd unit for an orch workload.
func RenderUnit(meta deployv1.Metadata, w deployv1.Workload, unitName string) (string, error) {
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
	case deployv1.WorkloadKindJob, deployv1.WorkloadKindCron:
		return ""
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
