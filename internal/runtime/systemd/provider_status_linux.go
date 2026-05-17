//go:build linux

package systemd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) Status(ctx context.Context, meta deployv1.Metadata, workloadName string) (runtimeinfo.Status, error) {
	unitName, startedAt, err := p.statusUnitName(meta, workloadName)
	if err != nil {
		return runtimeinfo.Status{}, err
	}
	out := runtimeinfo.Status{
		Name:      strings.TrimSpace(workloadName),
		Runtime:   deployv1.RuntimeSystemd,
		Status:    "stopped",
		NativeID:  unitName,
		StartedAt: startedAt,
		UpdatedAt: time.Now().UTC(),
	}

	conn, err := p.connect(ctx)
	if err != nil {
		return runtimeinfo.Status{}, oopsx.B("runtime", "systemd").Wrapf(err, "connect systemd dbus")
	}
	defer conn.Close()

	units, err := conn.ListUnitsByNamesContext(ctx, []string{unitName})
	if err != nil {
		return runtimeinfo.Status{}, oopsx.B("runtime", "systemd").Wrapf(err, "list unit %s", unitName)
	}
	if len(units) == 0 || systemdUnitMissing(units[0]) {
		return out, nil
	}
	unit := units[0]
	out.Status = RuntimeStatusFromActiveState(unit.ActiveState)
	out.Message = systemdStatusMessage(unit.LoadState, unit.ActiveState, unit.SubState)
	return out, nil
}

func (p *Provider) Logs(ctx context.Context, meta deployv1.Metadata, workloadName string, opts runtimeinfo.LogOptions) (runtimeinfo.LogResult, error) {
	unitName, _, err := p.statusUnitName(meta, workloadName)
	if err != nil {
		return runtimeinfo.LogResult{}, err
	}
	lines := strconv.Itoa(runtimeinfo.NormalizeTailLines(opts.Tail))
	cmd := exec.CommandContext(ctx, "journalctl", "--unit", unitName, "--lines", lines, "--no-pager", "--output", "cat")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "systemd").Wrapf(err, "journalctl unit %s: %s", unitName, strings.TrimSpace(string(out)))
	}
	return runtimeinfo.LogResult{
		Name:    strings.TrimSpace(workloadName),
		Runtime: deployv1.RuntimeSystemd,
		Source:  "journalctl:" + unitName,
		Content: strings.ReplaceAll(string(out), "\r\n", "\n"),
	}, nil
}

func (p *Provider) statusUnitName(meta deployv1.Metadata, workloadName string) (string, time.Time, error) {
	st, err := p.readState(meta, workloadName)
	if err == nil {
		if unitName := strings.TrimSpace(st.UnitName); unitName != "" {
			return unitName, st.StartedAt, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", time.Time{}, err
	}
	return DefaultUnitName(meta, workloadName), time.Time{}, nil
}

func systemdUnitMissing(unit dbus.UnitStatus) bool {
	return strings.TrimSpace(unit.Name) == "" || strings.TrimSpace(unit.LoadState) == "not-found"
}

func systemdStatusMessage(loadState, activeState, subState string) string {
	parts := []string{}
	if load := strings.TrimSpace(loadState); load != "" {
		parts = append(parts, "load="+load)
	}
	if active := strings.TrimSpace(activeState); active != "" {
		parts = append(parts, "active="+active)
	}
	if sub := strings.TrimSpace(subState); sub != "" {
		parts = append(parts, "sub="+sub)
	}
	return strings.Join(parts, " ")
}
