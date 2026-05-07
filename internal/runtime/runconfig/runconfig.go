package runconfig

import (
	"math"
	"strings"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

const (
	nanoCPUsPerMilli = int64(1_000_000)
	cfsPeriodMicros  = uint64(100_000)
)

// Env returns Docker/OCI-style environment entries.
func Env(vars []deployv1.EnvVar) []string {
	out := make([]string, 0, len(vars))
	for _, v := range vars {
		name := strings.TrimSpace(v.Name)
		if name == "" {
			continue
		}
		out = append(out, name+"="+v.Value)
	}
	return out
}

// CommandArgs returns an explicit OCI process argv when run.exec.command is set.
func CommandArgs(run deployv1.RunSpec) []string {
	if len(run.Exec.Command) == 0 {
		return nil
	}
	out := make([]string, 0, len(run.Exec.Command)+len(run.Exec.Args))
	out = append(out, run.Exec.Command...)
	out = append(out, run.Exec.Args...)
	return out
}

// ProcessCommand returns the executable path and argv for local process-style runtimes.
func ProcessCommand(run deployv1.RunSpec) (string, []string, bool) {
	if len(run.Exec.Command) > 0 {
		exe := strings.TrimSpace(run.Exec.Command[0])
		if exe == "" {
			return "", nil, false
		}
		args := make([]string, 0, len(run.Exec.Command)-1+len(run.Exec.Args))
		args = append(args, run.Exec.Command[1:]...)
		args = append(args, run.Exec.Args...)
		return exe, args, true
	}
	exe := strings.TrimSpace(run.Artifact.Path)
	if exe == "" {
		return "", nil, false
	}
	return exe, append([]string(nil), run.Exec.Args...), true
}

// ArtifactSummary returns a compact human-facing identifier for a workload artifact.
func ArtifactSummary(run deployv1.RunSpec) string {
	switch {
	case strings.TrimSpace(run.Artifact.Image) != "":
		return strings.TrimSpace(run.Artifact.Image)
	case strings.TrimSpace(run.Artifact.Path) != "":
		return strings.TrimSpace(run.Artifact.Path)
	case strings.TrimSpace(run.Artifact.URL) != "":
		return strings.TrimSpace(run.Artifact.URL)
	case len(run.Exec.Command) > 0:
		return strings.TrimSpace(run.Exec.Command[0])
	default:
		return ""
	}
}

// NanoCPUs converts millicores to Docker NanoCPUs.
func NanoCPUs(cpuMillis int64) int64 {
	if cpuMillis <= 0 {
		return 0
	}
	if cpuMillis > math.MaxInt64/nanoCPUsPerMilli {
		return math.MaxInt64
	}
	return cpuMillis * nanoCPUsPerMilli
}

// CFSQuota converts millicores to a Linux CFS quota using a stable 100ms period.
func CFSQuota(cpuMillis int64) (quota int64, period uint64) {
	if cpuMillis <= 0 {
		return 0, cfsPeriodMicros
	}
	quota = int64(cfsPeriodMicros) * cpuMillis / 1000
	if quota <= 0 {
		quota = 1
	}
	return quota, cfsPeriodMicros
}
