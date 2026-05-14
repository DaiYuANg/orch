package orch

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"

	v1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func lowerResources(f *compiler.HIRForm) *v1.Resources {
	var resources v1.Resources
	if cpu, ok := int64Field(f, "cpu_millis"); ok {
		resources.CPUMillis = cpu
	}
	if mem, ok := int64Field(f, "memory_bytes"); ok {
		resources.MemoryBytes = mem
	}
	if resources.CPUMillis == 0 && resources.MemoryBytes == 0 {
		return nil
	}
	return &resources
}

func lowerWorkloadResources(f *compiler.HIRForm) (*v1.Resources, error) {
	out, err := lowerResourceFields(f)
	if err != nil {
		return nil, err
	}
	blocks := childFormsByKind(f, "resources")
	return mergeResourceBlock(out, blocks)
}

func lowerResourceFields(f *compiler.HIRForm) (*v1.Resources, error) {
	var out *v1.Resources
	if spec, ok := stringField(f, "resources"); ok && strings.TrimSpace(spec) != "" {
		resources, err := parseResourceSpec(spec)
		if err != nil {
			return nil, err
		}
		out = resources
	}
	if cpu, ok := int64Field(f, "cpu_millis"); ok {
		out = ensureResources(out)
		out.CPUMillis = cpu
	}
	if mem, ok := int64Field(f, "memory_bytes"); ok {
		out = ensureResources(out)
		out.MemoryBytes = mem
	}
	return out, nil
}

func mergeResourceBlock(out *v1.Resources, blocks []compiler.HIRForm) (*v1.Resources, error) {
	if len(blocks) > 1 {
		return nil, errors.New("at most one resources block")
	}
	if len(blocks) == 0 {
		return out, nil
	}
	if out != nil {
		return nil, errors.New("resources are set both as fields and resources block")
	}
	return lowerResources(&blocks[0]), nil
}

func ensureResources(resources *v1.Resources) *v1.Resources {
	if resources != nil {
		return resources
	}
	return &v1.Resources{}
}

func parseResourceSpec(raw string) (*v1.Resources, error) {
	parts := strings.Split(strings.TrimSpace(raw), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf(`resources must be "cpu/memory", got %q`, raw)
	}
	cpu, err := parseCPUMillis(parts[0])
	if err != nil {
		return nil, err
	}
	mem, err := schema.ParseSize(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("parse memory resource: %w", err)
	}
	return &v1.Resources{CPUMillis: cpu, MemoryBytes: mem.Bytes}, nil
}

func parseCPUMillis(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, errors.New("cpu resource is empty")
	}
	if before, ok := strings.CutSuffix(s, "m"); ok {
		n, err := strconv.ParseInt(before, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse cpu resource %q: %w", raw, err)
		}
		return n, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse cpu resource %q: %w", raw, err)
	}
	return int64(f * 1000), nil
}

func lowerScheduling(f *compiler.HIRForm) *v1.Scheduling {
	var scheduling v1.Scheduling
	if stateful, ok := boolField(f, "stateful"); ok {
		scheduling.Stateful = stateful
	}
	if allowLeader, ok := boolField(f, "allow_leader"); ok {
		scheduling.AllowLeader = allowLeader
	}
	if preferredNodes, ok := rawField(f, "preferred_nodes"); ok {
		scheduling.PreferredNodes = stringList(preferredNodes)
	}
	return nonEmptyScheduling(scheduling)
}

func lowerSchedulingFromFields(f *compiler.HIRForm, statefulDefault bool) *v1.Scheduling {
	var scheduling v1.Scheduling
	scheduling.Stateful = statefulDefault
	if allowLeader, ok := boolField(f, "allow_leader"); ok {
		scheduling.AllowLeader = allowLeader
	}
	if preferredNodes, ok := rawField(f, "preferred_nodes"); ok {
		scheduling.PreferredNodes = stringList(preferredNodes)
	}
	return nonEmptyScheduling(scheduling)
}

func mergeScheduling(base, override *v1.Scheduling) *v1.Scheduling {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}
	out := *base
	if override.Stateful {
		out.Stateful = true
	}
	if override.AllowLeader {
		out.AllowLeader = true
	}
	if len(override.PreferredNodes) > 0 {
		out.PreferredNodes = append([]string(nil), override.PreferredNodes...)
	}
	return nonEmptyScheduling(out)
}

func nonEmptyScheduling(scheduling v1.Scheduling) *v1.Scheduling {
	if !scheduling.Stateful && !scheduling.AllowLeader && len(scheduling.PreferredNodes) == 0 {
		return nil
	}
	return &scheduling
}
