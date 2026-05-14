package composeimport

import (
	composetypes "github.com/compose-spec/compose-go/v2/types"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
)

func resourcesFromCompose(d *composetypes.DeployConfig) *deployv1.Resources {
	if d == nil {
		return nil
	}
	if resources, ok := resourcesFromComposeResource(d.Resources.Limits); ok {
		return &resources
	}
	if resources, ok := resourcesFromComposeResource(d.Resources.Reservations); ok {
		return &resources
	}
	return nil
}

func resourcesFromComposeResource(src *composetypes.Resource) (deployv1.Resources, bool) {
	if src == nil {
		return deployv1.Resources{}, false
	}
	resources := deployv1.Resources{
		CPUMillis:   composeCPUMillis(src),
		MemoryBytes: composeMemoryBytes(src),
	}
	return resources, resources.CPUMillis != 0 || resources.MemoryBytes != 0
}

func composeCPUMillis(src *composetypes.Resource) int64 {
	if src.NanoCPUs <= 0 {
		return 0
	}
	return int64(src.NanoCPUs.Value() * 1000)
}

func composeMemoryBytes(src *composetypes.Resource) int64 {
	if src.MemoryBytes <= 0 {
		return 0
	}
	return int64(src.MemoryBytes)
}
