package composeimport

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/collectionx/set"
	composetypes "github.com/compose-spec/compose-go/v2/types"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

// MapProject converts a compose-spec [composetypes.Project] into the canonical orch
// [deployv1.App]. Runtime scheduling uses only the App; this function is the compatibility edge.
func MapProject(proj *composetypes.Project) (*Result, error) {
	if proj == nil {
		return nil, fmt.Errorf("compose project is nil")
	}
	var rep Report
	app := &deployv1.App{
		APIVersion: "warden.arcgolabs.io/v1alpha1",
		Kind:       "App",
		Metadata: deployv1.Metadata{
			Name:      sanitizeProjectName(proj.Name),
			Namespace: "default",
			Annotations: map[string]string{
				"compose.arcgolabs.io/import": "true",
			},
		},
	}

	if proj.Networks != nil && len(proj.Networks) > 0 {
		rep.warnf("compose networks are not mapped into the canonical model yet (%d defined)", len(proj.Networks))
	}

	app.Volumes = mapComposeVolumes(proj)
	nameOrder := proj.ServiceNames()

	wloads := list.NewListWithCapacity[deployv1.Workload](len(nameOrder))
	for _, svcName := range nameOrder {
		svc := proj.Services[svcName]
		wl, extraVol, ok := mapService(svcName, svc, &rep)
		if !ok {
			continue
		}
		wloads.Add(wl)
		app.Volumes = mergeVolumesDedup(app.Volumes, extraVol)
	}
	app.Workloads = wloads.Values()

	if len(app.Workloads) == 0 {
		return nil, fmt.Errorf("compose import produced no workloads (need image per service)")
	}

	return &Result{App: app, Report: rep}, nil
}

func sanitizeProjectName(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return "compose-import"
	}
	// Match deploy name validation loosely: letters/digits/separator.
	var b strings.Builder
	for _, r := range n {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	if out == "" || out[0] < 'A' || (out[0] > 'Z' && out[0] < 'a') {
		return "compose-" + out
	}
	return out
}

func mapComposeVolumes(proj *composetypes.Project) []deployv1.Volume {
	if proj.Volumes == nil {
		return nil
	}
	names := mapping.NewMapFrom(proj.Volumes).Keys()
	sort.Strings(names)
	return list.MapList(list.NewList(names...), func(_ int, name string) deployv1.Volume {
		return deployv1.Volume{Name: name, Persistent: true}
	}).Values()
}

func mergeVolumesDedup(existing []deployv1.Volume, add []deployv1.Volume) []deployv1.Volume {
	seen := set.NewSetWithCapacity[string](len(existing) + len(add))
	for _, v := range existing {
		seen.Add(v.Name)
	}
	out := existing
	for _, v := range add {
		if seen.Contains(v.Name) {
			continue
		}
		seen.Add(v.Name)
		out = append(out, v)
	}
	return out
}

func mapService(name string, s composetypes.ServiceConfig, rep *Report) (deployv1.Workload, []deployv1.Volume, bool) {
	image := strings.TrimSpace(s.Image)
	if image == "" {
		rep.warnf("service %q: skipped (no image; compose build-only services are not mapped yet)", name)
		return deployv1.Workload{}, nil, false
	}

	w := deployv1.Workload{
		Name:    name,
		Kind:    deployv1.WorkloadKindService,
		Runtime: deployv1.RuntimeDocker,
		Run: deployv1.RunSpec{
			Artifact: deployv1.ArtifactSpec{Image: image},
			Exec: deployv1.ExecSpec{
				Command: []string(s.Entrypoint),
				Args:    []string(s.Command),
			},
			Env: envFromCompose(s.Environment),
			Cwd: strings.TrimSpace(s.WorkingDir),
		},
		Replicas: replicasFromCompose(&s),
	}

	w.Run.Options.Docker = dockerOptionsFromCompose(name, &s, rep)

	w.Endpoints = endpointsFromCompose(name, s.Ports, rep)
	w.DependsOn = dependsFromCompose(s.DependsOn, rep)

	if res := resourcesFromCompose(s.Deploy); res != nil {
		w.Resources = res
	}

	if s.HealthCheck != nil && !s.HealthCheck.Disable {
		rep.warnf("service %q: healthcheck is not mapped (only HTTP probes exist in canonical v1)", name)
	}

	var extraVol []deployv1.Volume
	if s.Volumes != nil {
		w.Mounts, extraVol = mountsFromCompose(name, s.Volumes, rep)
	}

	return w, extraVol, true
}

func envFromCompose(m composetypes.MappingWithEquals) []deployv1.EnvVar {
	if len(m) == 0 {
		return nil
	}
	flat := m.ToMapping()
	keys := mapping.NewMapFrom(flat).Keys()
	sort.Strings(keys)
	return list.MapList(list.NewList(keys...), func(_ int, k string) deployv1.EnvVar {
		return deployv1.EnvVar{Name: k, Value: flat[k]}
	}).Values()
}

func replicasFromCompose(s *composetypes.ServiceConfig) int {
	if s.Scale != nil && *s.Scale > 0 {
		return *s.Scale
	}
	if s.Deploy != nil && s.Deploy.Replicas != nil && *s.Deploy.Replicas > 0 {
		return *s.Deploy.Replicas
	}
	return 1
}

func dockerOptionsFromCompose(svcName string, s *composetypes.ServiceConfig, rep *Report) *deployv1.DockerOptions {
	labels := mapping.NewMapWithCapacity[string, string](len(s.Labels))
	labels.SetAll(s.Labels)
	opts := &deployv1.DockerOptions{
		NetworkMode: strings.TrimSpace(s.NetworkMode),
		Privileged:  s.Privileged,
		Labels:      labels.All(),
	}
	if s.Deploy != nil {
		for k, v := range s.Deploy.Labels {
			if opts.Labels[k] == "" {
				opts.Labels[k] = v
			}
		}
	}
	if len(opts.Labels) == 0 {
		opts.Labels = nil
	}
	if opts.NetworkMode == "" && len(s.Networks) > 0 {
		nn := networkNamesSorted(s.Networks)
		if len(nn) == 1 {
			opts.NetworkMode = nn[0]
		} else if len(nn) > 1 {
			opts.NetworkMode = nn[0]
			rep.warnf("service %q: multiple compose networks; using first only: %v", svcName, nn)
		}
	}
	if opts.NetworkMode == "" && opts.Labels == nil && !opts.Privileged {
		return nil
	}
	return opts
}

func networkNamesSorted(nets map[string]*composetypes.ServiceNetworkConfig) []string {
	out := mapping.NewMapFrom(nets).Keys()
	sort.Strings(out)
	return out
}

func endpointsFromCompose(service string, ports []composetypes.ServicePortConfig, rep *Report) []deployv1.Endpoint {
	if len(ports) == 0 {
		return nil
	}
	var out []deployv1.Endpoint
	for i, p := range ports {
		if p.Target == 0 {
			rep.warnf("service %q: ports[%d] has no container target port, skipped", service, i)
			continue
		}
		proto := deployv1.ProtoTCP
		switch strings.ToLower(p.Protocol) {
		case "udp":
			proto = deployv1.ProtoUDP
		case "tcp", "":
			proto = deployv1.ProtoTCP
		default:
			rep.warnf("service %q: port %d protocol %q mapped as tcp", service, p.Target, p.Protocol)
		}
		// Use deterministic names that satisfy deploy name validation (first-class port ref).
		protoStr := strings.ToLower(string(proto))
		if protoStr == "" {
			protoStr = "tcp"
		}
		ename := fmt.Sprintf("%s-%d", protoStr, p.Target)
		out = append(out, deployv1.Endpoint{
			Name:     ename,
			Port:     int(p.Target),
			Protocol: proto,
		})
	}
	return out
}

func dependsFromCompose(d composetypes.DependsOnConfig, rep *Report) []deployv1.WorkloadRef {
	if len(d) == 0 {
		return nil
	}
	names := mapping.NewMapFrom(d).Keys()
	sort.Strings(names)
	out := list.NewListWithCapacity[deployv1.WorkloadRef](len(names))
	for _, name := range names {
		dep := d[name]
		switch strings.TrimSpace(dep.Condition) {
		case "", composetypes.ServiceConditionStarted, composetypes.ServiceConditionHealthy:
			out.Add(deployv1.WorkloadRef{Name: name})
		case composetypes.ServiceConditionCompletedSuccessfully:
			rep.warnf("depends_on service %q uses condition %q (not modeled); treating as dependency edge only", name, dep.Condition)
			out.Add(deployv1.WorkloadRef{Name: name})
		default:
			rep.warnf("depends_on service %q uses condition %q; treating as ordered dependency", name, dep.Condition)
			out.Add(deployv1.WorkloadRef{Name: name})
		}
	}
	return out.Values()
}

func resourcesFromCompose(d *composetypes.DeployConfig) *deployv1.Resources {
	if d == nil {
		return nil
	}
	var r deployv1.Resources
	if d.Resources.Limits != nil {
		lim := d.Resources.Limits
		if lim.NanoCPUs > 0 {
			r.CPUMillis = int64(lim.NanoCPUs.Value() * 1000)
		}
		if lim.MemoryBytes > 0 {
			r.MemoryBytes = int64(lim.MemoryBytes)
		}
	}
	if r.CPUMillis == 0 && r.MemoryBytes == 0 && d.Resources.Reservations != nil {
		res := d.Resources.Reservations
		if res.NanoCPUs > 0 {
			r.CPUMillis = int64(res.NanoCPUs.Value() * 1000)
		}
		if res.MemoryBytes > 0 {
			r.MemoryBytes = int64(res.MemoryBytes)
		}
	}
	if r.CPUMillis == 0 && r.MemoryBytes == 0 {
		return nil
	}
	return &r
}

func mountsFromCompose(service string, vols []composetypes.ServiceVolumeConfig, rep *Report) ([]deployv1.Mount, []deployv1.Volume) {
	var mounts []deployv1.Mount
	var extraVol []deployv1.Volume

	for i, v := range vols {
		tgt := strings.TrimSpace(v.Target)
		if tgt == "" {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(v.Type))
		switch typ {
		case "", "volume":
			src := strings.TrimSpace(v.Source)
			if src == "" {
				rep.warnf("service %q: volumes[%d] anonymous volume not mapped", service, i)
				continue
			}
			mounts = append(mounts, deployv1.Mount{
				Volume:   deployv1.VolumeRef{Name: src},
				Target:   tgt,
				ReadOnly: v.ReadOnly,
			})
		case "bind":
			id := fmt.Sprintf("bind-%s-%d", service, i)
			rep.warnf("service %q: bind mount %q -> %q mapped as named volume %q (host path not in canonical volume model yet)",
				service, strings.TrimSpace(v.Source), tgt, id)
			extraVol = append(extraVol, deployv1.Volume{Name: id})
			mounts = append(mounts, deployv1.Mount{
				Volume:   deployv1.VolumeRef{Name: id},
				Target:   tgt,
				ReadOnly: v.ReadOnly,
			})
		case "tmpfs":
			rep.warnf("service %q: tmpfs mount %q skipped", service, tgt)
		default:
			rep.warnf("service %q: volume type %q not mapped", service, typ)
		}
	}
	return mounts, extraVol
}
