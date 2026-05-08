//go:build linux

package systemd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/runtime/runconfig"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func (p *Provider) Deploy(ctx context.Context, meta deployv1.Metadata, w deployv1.Workload) error {
	unitName := p.workloadUnitName(meta, w)
	unitPath := systemdUnitPath(unitName)
	content, err := renderUnit(meta, w, unitName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return oopsx.B("runtime", "systemd").Wrapf(err, "create systemd unit dir")
	}
	if err := os.WriteFile(unitPath, []byte(content), 0o644); err != nil {
		return oopsx.B("runtime", "systemd").Wrapf(err, "write unit %s", unitName)
	}
	if err := p.systemctl(ctx, "daemon-reload"); err != nil {
		_ = os.Remove(unitPath)
		return err
	}
	if err := p.systemctl(ctx, "enable", "--now", unitName); err != nil {
		p.cleanupUnit(ctx, unitName, unitPath)
		return err
	}

	st := state{
		UnitName:  unitName,
		UnitPath:  unitPath,
		Metadata:  meta,
		Workload:  w.Name,
		Runtime:   w.Runtime,
		Artifact:  runconfig.ArtifactSummary(w.Run),
		StartedAt: time.Now().UTC(),
	}
	if err := p.writeState(meta, w.Name, st); err != nil {
		p.cleanupUnit(ctx, unitName, unitPath)
		return err
	}
	if p.dns != nil {
		if err := p.dns.UpsertWorkloadA(ctx, meta.Namespace, w.Name, p.dns.WorkloadAdvertiseAddress("127.0.0.1")); err != nil {
			p.cleanupUnit(ctx, unitName, unitPath)
			_ = p.removeState(meta, w.Name)
			return oopsx.B("runtime", "dns").Wrapf(err, "upsert workload DNS")
		}
	}
	p.logger.Info("systemd workload running", "workload", w.Name, "unit", unitName)
	return nil
}

func (p *Provider) Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error {
	unitName := defaultUnitName(meta, workloadName)
	unitPath := systemdUnitPath(unitName)
	if st, err := p.readState(meta, workloadName); err == nil {
		if strings.TrimSpace(st.UnitName) != "" {
			unitName = st.UnitName
		}
		if strings.TrimSpace(st.UnitPath) != "" {
			unitPath = st.UnitPath
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := p.systemctl(ctx, "disable", "--now", unitName); err != nil {
		p.logger.Warn("systemd disable unit", "unit", unitName, "error", err)
	}
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return oopsx.B("runtime", "systemd").Wrapf(err, "remove unit %s", unitName)
	}
	if err := p.systemctl(ctx, "daemon-reload"); err != nil {
		p.logger.Warn("systemd daemon-reload", "error", err)
	}
	if p.dns != nil {
		if err := p.dns.RemoveWorkloadA(ctx, meta.Namespace, workloadName); err != nil {
			return oopsx.B("runtime", "dns").Wrapf(err, "remove workload DNS")
		}
	}
	if err := p.removeState(meta, workloadName); err != nil {
		return err
	}
	p.logger.Info("systemd workload stopped", "workload", workloadName, "unit", unitName)
	return nil
}

func (p *Provider) cleanupUnit(ctx context.Context, unitName, unitPath string) {
	if err := p.systemctl(ctx, "disable", "--now", unitName); err != nil {
		p.logger.Warn("systemd cleanup disable unit", "unit", unitName, "error", err)
	}
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		p.logger.Warn("systemd cleanup remove unit", "unit", unitName, "error", err)
	}
	if err := p.systemctl(ctx, "daemon-reload"); err != nil {
		p.logger.Warn("systemd cleanup daemon-reload", "error", err)
	}
}

func (p *Provider) systemctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return oopsx.B("runtime", "systemd").Wrapf(err, "systemctl %s: %s", strings.Join(args, " "), msg)
		}
		return oopsx.B("runtime", "systemd").Wrapf(err, "systemctl %s", strings.Join(args, " "))
	}
	return nil
}
