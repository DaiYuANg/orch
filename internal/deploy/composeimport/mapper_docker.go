package composeimport

import (
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"
	composetypes "github.com/compose-spec/compose-go/v2/types"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func dockerOptionsFromCompose(svcName string, s *composetypes.ServiceConfig, rep *Report) *deployv1.DockerOptions {
	opts := baseDockerOptions(s)
	mergeDeployLabels(opts, s)
	opts.Labels = nilIfEmpty(opts.Labels)
	fillNetworkModeFromCompose(svcName, opts, s, rep)
	if dockerOptionsEmpty(opts) {
		return nil
	}
	return opts
}

func baseDockerOptions(s *composetypes.ServiceConfig) *deployv1.DockerOptions {
	labels := mapping.NewMapWithCapacity[string, string](len(s.Labels))
	labels.SetAll(s.Labels)
	return &deployv1.DockerOptions{
		NetworkMode: strings.TrimSpace(s.NetworkMode),
		Privileged:  s.Privileged,
		Labels:      labels.All(),
	}
}

func mergeDeployLabels(opts *deployv1.DockerOptions, s *composetypes.ServiceConfig) {
	if s.Deploy == nil {
		return
	}
	for k, v := range s.Deploy.Labels {
		if opts.Labels[k] == "" {
			opts.Labels[k] = v
		}
	}
}

func nilIfEmpty(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}

func fillNetworkModeFromCompose(svcName string, opts *deployv1.DockerOptions, s *composetypes.ServiceConfig, rep *Report) {
	if opts.NetworkMode != "" || len(s.Networks) == 0 {
		return
	}
	names := networkNamesSorted(s.Networks)
	if first, ok := names.GetFirstOption().Get(); ok {
		opts.NetworkMode = first
	}
	if names.Len() > 1 {
		rep.warnf("service %q: multiple compose networks; using first only: %v", svcName, names.Values())
	}
}

func networkNamesSorted(nets map[string]*composetypes.ServiceNetworkConfig) *list.List[string] {
	return sortedMapKeys(nets)
}

func dockerOptionsEmpty(opts *deployv1.DockerOptions) bool {
	return opts.NetworkMode == "" && opts.Labels == nil && !opts.Privileged
}
