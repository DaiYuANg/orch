package v1alpha1

import (
	"regexp"
	"strings"

	"github.com/arcgolabs/collectionx/set"

	"github.com/daiyuang/orch/pkg/oopsx"
)

var (
	nameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]{0,127}$`)
)

func (a *App) Validate() error {
	if err := a.validateMetadata(); err != nil {
		return err
	}

	seenWorkloads := set.NewSet[string]()
	for i := range a.Workloads {
		w := &a.Workloads[i]
		if err := w.validate(seenWorkloads); err != nil {
			return oopsx.B("deploy").Wrapf(err, "workloads[%d]", i)
		}
	}

	if err := a.validateWorkloadCrossRefs(seenWorkloads); err != nil {
		return err
	}
	if err := a.validateIngresses(seenWorkloads); err != nil {
		return err
	}
	return nil
}

func (a *App) validateMetadata() error {
	if strings.TrimSpace(a.Metadata.Name) == "" {
		return oopsx.B("deploy").Errorf("metadata.name is required")
	}
	if !nameRe.MatchString(a.Metadata.Name) {
		return oopsx.B("deploy").Errorf("metadata.name is invalid: %q", a.Metadata.Name)
	}
	if a.Metadata.Namespace != "" && !nameRe.MatchString(a.Metadata.Namespace) {
		return oopsx.B("deploy").Errorf("metadata.namespace is invalid: %q", a.Metadata.Namespace)
	}
	return nil
}

func (a *App) validateWorkloadCrossRefs(seenWorkloads *set.Set[string]) error {
	if err := a.validateWorkloadDepends(seenWorkloads); err != nil {
		return err
	}
	if err := a.validateWorkloadMounts(); err != nil {
		return err
	}
	return a.validateWorkloadEndpointsAll()
}

func (a *App) validateWorkloadDepends(seenWorkloads *set.Set[string]) error {
	for i := range a.Workloads {
		w := &a.Workloads[i]
		for j := range w.DependsOn {
			if !seenWorkloads.Contains(w.DependsOn[j].Name) {
				return oopsx.B("deploy").Errorf("workloads[%d].dependsOn[%d]: unknown workload %q", i, j, w.DependsOn[j].Name)
			}
		}
	}
	return nil
}

func (a *App) validateWorkloadMounts() error {
	for i := range a.Workloads {
		w := &a.Workloads[i]
		for j := range w.Mounts {
			if strings.TrimSpace(w.Mounts[j].Volume.Name) == "" {
				return oopsx.B("deploy").Errorf("workloads[%d].mounts[%d].volume: name is required", i, j)
			}
		}
	}
	return nil
}

func (a *App) validateWorkloadEndpointsAll() error {
	for i := range a.Workloads {
		w := &a.Workloads[i]
		for j := range w.Endpoints {
			if err := w.Endpoints[j].validate(); err != nil {
				return oopsx.B("deploy").Wrapf(err, "workloads[%d].endpoints[%d]", i, j)
			}
		}
	}
	return nil
}

func (a *App) validateIngresses(seenWorkloads *set.Set[string]) error {
	for i := range a.Ingresses {
		if err := a.validateIngressOne(i, seenWorkloads); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) validateIngressOne(ingIndex int, seenWorkloads *set.Set[string]) error {
	ing := &a.Ingresses[ingIndex]
	if strings.TrimSpace(ing.Name) == "" {
		return oopsx.B("deploy").Errorf("ingresses[%d].name is required", ingIndex)
	}
	for j := range ing.Routes {
		r := &ing.Routes[j]
		if strings.TrimSpace(r.Path) == "" {
			return oopsx.B("deploy").Errorf("ingresses[%d].routes[%d].path is required", ingIndex, j)
		}
		if strings.TrimSpace(r.Backend.Workload) == "" || strings.TrimSpace(r.Backend.Endpoint) == "" {
			return oopsx.B("deploy").Errorf("ingresses[%d].routes[%d].backend must specify workload + endpoint", ingIndex, j)
		}
		if !seenWorkloads.Contains(r.Backend.Workload) {
			return oopsx.B("deploy").Errorf("ingresses[%d].routes[%d].backend: unknown workload %q", ingIndex, j, r.Backend.Workload)
		}
	}
	return nil
}

func (w *Workload) validate(seen *set.Set[string]) error {
	if strings.TrimSpace(w.Name) == "" {
		return oopsx.B("deploy").Errorf("name is required")
	}
	if !nameRe.MatchString(w.Name) {
		return oopsx.B("deploy").Errorf("name is invalid: %q", w.Name)
	}
	if seen.Contains(w.Name) {
		return oopsx.B("deploy").Errorf("duplicate workload name %q", w.Name)
	}
	seen.Add(w.Name)

	switch w.Kind {
	case WorkloadKindService, WorkloadKindWorker, WorkloadKindJob, WorkloadKindCron, WorkloadKindStateful:
	default:
		return oopsx.B("deploy").Errorf("invalid kind %q", w.Kind)
	}
	switch w.Runtime {
	case RuntimeDocker, RuntimeContainerd, RuntimeFirecracker, RuntimeProcess, RuntimeSystemd, RuntimeWindowsService:
	default:
		return oopsx.B("deploy").Errorf("invalid runtime %q", w.Runtime)
	}
	if err := w.validateRunForRuntime(); err != nil {
		return err
	}
	for i := range w.Run.Env {
		if strings.TrimSpace(w.Run.Env[i].Name) == "" {
			return oopsx.B("deploy").Errorf("run.env[%d].name is required", i)
		}
	}
	if w.Replicas < 0 {
		return oopsx.B("deploy").Errorf("replicas must be >= 0")
	}
	if w.Resources != nil {
		if w.Resources.CPUMillis < 0 {
			return oopsx.B("deploy").Errorf("resources.cpuMillis must be >= 0")
		}
		if w.Resources.MemoryBytes < 0 {
			return oopsx.B("deploy").Errorf("resources.memoryBytes must be >= 0")
		}
	}
	return nil
}

func (w *Workload) validateRunForRuntime() error {
	switch w.Runtime {
	case RuntimeDocker, RuntimeContainerd:
		if strings.TrimSpace(w.Run.Artifact.Image) == "" {
			return oopsx.B("deploy").Errorf("run.artifact.image is required for runtime %q", w.Runtime)
		}
	case RuntimeFirecracker:
		if w.Run.Options.Firecracker == nil {
			return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker is required for runtime %q", w.Runtime)
		}
		if strings.TrimSpace(w.Run.Options.Firecracker.KernelImagePath) == "" {
			return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker.kernelImagePath is required")
		}
		if strings.TrimSpace(w.Run.Options.Firecracker.RootfsPath) == "" {
			return oopsx.B("deploy").Errorf("run.runtimeOptions.firecracker.rootfsPath is required")
		}
	case RuntimeProcess, RuntimeSystemd, RuntimeWindowsService:
		if len(w.Run.Exec.Command) == 0 && strings.TrimSpace(w.Run.Artifact.Path) == "" {
			return oopsx.B("deploy").Errorf("run.exec.command or run.artifact.path is required for runtime %q", w.Runtime)
		}
	}
	return nil
}

func (e *Endpoint) validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return oopsx.B("deploy").Errorf("name is required")
	}
	if !nameRe.MatchString(e.Name) {
		return oopsx.B("deploy").Errorf("name is invalid: %q", e.Name)
	}
	if e.Port <= 0 || e.Port > 65535 {
		return oopsx.B("deploy").Errorf("port must be 1..65535 (got %d)", e.Port)
	}
	switch e.Protocol {
	case ProtoTCP, ProtoUDP, ProtoHTTP:
	default:
		return oopsx.B("deploy").Errorf("invalid protocol %q", e.Protocol)
	}
	return nil
}
