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

// CommandArgs returns an explicit process argv when run.command is set.
func CommandArgs(run deployv1.RunSpec) []string {
	if len(run.Command) == 0 {
		return nil
	}
	out := make([]string, 0, len(run.Command)+len(run.Args))
	out = append(out, run.Command...)
	out = append(out, run.Args...)
	return out
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
