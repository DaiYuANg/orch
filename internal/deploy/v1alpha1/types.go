package v1alpha1

import (
	"regexp"
	"strings"

	"github.com/arcgolabs/collectionx/set"
	"gopkg.in/yaml.v3"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// App is the YAML-friendly canonical deploy model for the first Go rewrite
// iteration. It intentionally mirrors the canonical model described in
// docs/src/dsl.md and docs/src/dsl.zh.md.
type App struct {
	APIVersion string   `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string   `json:"kind,omitempty"       yaml:"kind,omitempty"`
	Metadata   Metadata `json:"metadata"             yaml:"metadata"`

	Workloads []Workload `json:"workloads,omitempty" yaml:"workloads,omitempty"`
	Configs   []Config   `json:"configs,omitempty"   yaml:"configs,omitempty"`
	Secrets   []Secret   `json:"secrets,omitempty"   yaml:"secrets,omitempty"`
	Volumes   []Volume   `json:"volumes,omitempty"   yaml:"volumes,omitempty"`
	Ingresses []Ingress  `json:"ingresses,omitempty" yaml:"ingresses,omitempty"`
}

type Metadata struct {
	Name        string            `json:"name"                  yaml:"name"`
	Namespace   string            `json:"namespace,omitempty"   yaml:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"      yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

type Workload struct {
	Name string       `json:"name" yaml:"name"`
	Kind WorkloadKind `json:"kind" yaml:"kind"` // service|worker|job|cron|stateful
	Run  RunSpec      `json:"run"  yaml:"run"`  // image/command/args/env/cwd/runtimeOptions
	// Runtime selects the backend adapter. This stays separate from Run.RuntimeOptions
	// because the canonical intent model needs a stable first-class field.
	Runtime RuntimeKind `json:"runtime" yaml:"runtime"` // docker|containerd|firecracker|process

	Replicas  int           `json:"replicas,omitempty"  yaml:"replicas,omitempty"`
	DependsOn []WorkloadRef `json:"dependsOn,omitempty" yaml:"dependsOn,omitempty"`

	Endpoints []Endpoint `json:"endpoints,omitempty" yaml:"endpoints,omitempty"`
	Mounts    []Mount    `json:"mounts,omitempty"    yaml:"mounts,omitempty"`
	Resources *Resources `json:"resources,omitempty" yaml:"resources,omitempty"`
	Health    *Health    `json:"health,omitempty"    yaml:"health,omitempty"`

	Scheduling *Scheduling `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	Rollout    *Rollout    `json:"rollout,omitempty"    yaml:"rollout,omitempty"`
}

type WorkloadKind string

const (
	WorkloadKindService  WorkloadKind = "service"
	WorkloadKindWorker   WorkloadKind = "worker"
	WorkloadKindJob      WorkloadKind = "job"
	WorkloadKindCron     WorkloadKind = "cron"
	WorkloadKindStateful WorkloadKind = "stateful"
)

type RuntimeKind string

const (
	RuntimeDocker      RuntimeKind = "docker"
	RuntimeContainerd  RuntimeKind = "containerd"
	RuntimeFirecracker RuntimeKind = "firecracker"
	RuntimeProcess     RuntimeKind = "process"
)

type RunSpec struct {
	Image   string     `json:"image"                    yaml:"image"`
	Command []string   `json:"command,omitempty"        yaml:"command,omitempty"`
	Args    []string   `json:"args,omitempty"           yaml:"args,omitempty"`
	Env     []EnvVar   `json:"env,omitempty"            yaml:"env,omitempty"`
	Cwd     string     `json:"cwd,omitempty"            yaml:"cwd,omitempty"`
	Options RunOptions `json:"runtimeOptions,omitempty" yaml:"runtimeOptions,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name"  yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

// RunOptions captures backend-specific knobs. Only docker/containerd are in-scope
// for the first Go version; other fields can be added later.
type RunOptions struct {
	Docker     *DockerOptions     `json:"docker,omitempty"     yaml:"docker,omitempty"`
	Containerd *ContainerdOptions `json:"containerd,omitempty" yaml:"containerd,omitempty"`
}

type DockerOptions struct {
	NetworkMode string            `json:"networkMode,omitempty" yaml:"networkMode,omitempty"`
	Privileged  bool              `json:"privileged,omitempty"  yaml:"privileged,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"      yaml:"labels,omitempty"`
}

type ContainerdOptions struct {
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Snapshotter or runtime handler can be added later when we wire containerd.
}

type Endpoint struct {
	Name     string        `json:"name"     yaml:"name"`
	Port     int           `json:"port"     yaml:"port"`
	Protocol EndpointProto `json:"protocol" yaml:"protocol"` // tcp|udp|http
}

type EndpointProto string

const (
	ProtoTCP  EndpointProto = "tcp"
	ProtoUDP  EndpointProto = "udp"
	ProtoHTTP EndpointProto = "http"
)

type Mount struct {
	Volume   VolumeRef `json:"volume"             yaml:"volume"`
	Target   string    `json:"target"             yaml:"target"`
	ReadOnly bool      `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}

type Resources struct {
	CPUMillis   int64 `json:"cpuMillis,omitempty"   yaml:"cpuMillis,omitempty"`
	MemoryBytes int64 `json:"memoryBytes,omitempty" yaml:"memoryBytes,omitempty"`
}

type Health struct {
	Readiness *Probe `json:"readiness,omitempty" yaml:"readiness,omitempty"`
	Liveness  *Probe `json:"liveness,omitempty"  yaml:"liveness,omitempty"`
	Startup   *Probe `json:"startup,omitempty"   yaml:"startup,omitempty"`
}

type Probe struct {
	HTTP *HTTPProbe `json:"http,omitempty" yaml:"http,omitempty"`
	// Future: tcp, exec
}

type HTTPProbe struct {
	Path     string      `json:"path"     yaml:"path"`
	Endpoint EndpointRef `json:"endpoint" yaml:"endpoint"`
}

type Scheduling struct {
	Stateful       bool     `json:"stateful,omitempty"       yaml:"stateful,omitempty"`
	AllowLeader    bool     `json:"allowLeader,omitempty"    yaml:"allowLeader,omitempty"`
	PreferredNodes []string `json:"preferredNodes,omitempty" yaml:"preferredNodes,omitempty"`
}

type Rollout struct {
	Strategy       string `json:"strategy,omitempty"       yaml:"strategy,omitempty"`
	MaxUnavailable int    `json:"maxUnavailable,omitempty" yaml:"maxUnavailable,omitempty"`
	MaxSurge       int    `json:"maxSurge,omitempty"       yaml:"maxSurge,omitempty"`
}

type Config struct {
	Name string            `json:"name"           yaml:"name"`
	Data map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
}

type Secret struct {
	Name string            `json:"name"           yaml:"name"`
	Data map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
}

type Volume struct {
	Name       string `json:"name"                 yaml:"name"`
	Persistent bool   `json:"persistent,omitempty" yaml:"persistent,omitempty"`
	// SizeBytes is optional; keep numeric for canonical normalization.
	SizeBytes int64 `json:"sizeBytes,omitempty" yaml:"sizeBytes,omitempty"`
}

type Ingress struct {
	Name   string         `json:"name"             yaml:"name"`
	Host   string         `json:"host,omitempty"   yaml:"host,omitempty"`
	Routes []IngressRoute `json:"routes,omitempty" yaml:"routes,omitempty"`
}

type IngressRoute struct {
	Path    string      `json:"path"    yaml:"path"`
	Backend EndpointRef `json:"backend" yaml:"backend"`
}

// ---- Typed references (YAML-friendly) ----

// WorkloadRef refers to a workload by name. YAML form:
// - "redis" (string)  OR  { name: "redis" }
type WorkloadRef struct {
	Name string `json:"name" yaml:"name"`
}

func (r *WorkloadRef) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		r.Name = strings.TrimSpace(value.Value)
		return nil
	case yaml.MappingNode:
		type alias WorkloadRef
		var a alias
		if err := value.Decode(&a); err != nil {
			return err
		}
		*r = WorkloadRef(a)
		return nil
	default:
		return oopsx.B("deploy").Errorf("invalid workload ref")
	}
}

// VolumeRef refers to a volume by name. YAML form:
// - "redisData"  OR  { name: "redisData" }
type VolumeRef struct {
	Name string `json:"name" yaml:"name"`
}

func (r *VolumeRef) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		r.Name = strings.TrimSpace(value.Value)
		return nil
	case yaml.MappingNode:
		type alias VolumeRef
		var a alias
		if err := value.Decode(&a); err != nil {
			return err
		}
		*r = VolumeRef(a)
		return nil
	default:
		return oopsx.B("deploy").Errorf("invalid volume ref")
	}
}

// EndpointRef refers to a workload endpoint. YAML form:
// - "gateway:http" OR { workload: "gateway", endpoint: "http" }
type EndpointRef struct {
	Workload string `json:"workload" yaml:"workload"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
}

func (r *EndpointRef) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		s := strings.TrimSpace(value.Value)
		if s == "" {
			return nil
		}
		parts := strings.SplitN(s, ":", 2)
		if len(parts) != 2 {
			return oopsx.B("deploy").Errorf("invalid endpoint ref %q (expected workload:endpoint)", s)
		}
		r.Workload = strings.TrimSpace(parts[0])
		r.Endpoint = strings.TrimSpace(parts[1])
		return nil
	case yaml.MappingNode:
		type alias EndpointRef
		var a alias
		if err := value.Decode(&a); err != nil {
			return err
		}
		*r = EndpointRef(a)
		return nil
	default:
		return oopsx.B("deploy").Errorf("invalid endpoint ref")
	}
}

var (
	nameRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]{0,127}$`)
)

func (a *App) Validate() error {
	if strings.TrimSpace(a.Metadata.Name) == "" {
		return oopsx.B("deploy").Errorf("metadata.name is required")
	}
	if !nameRe.MatchString(a.Metadata.Name) {
		return oopsx.B("deploy").Errorf("metadata.name is invalid: %q", a.Metadata.Name)
	}
	if a.Metadata.Namespace != "" && !nameRe.MatchString(a.Metadata.Namespace) {
		return oopsx.B("deploy").Errorf("metadata.namespace is invalid: %q", a.Metadata.Namespace)
	}

	seenWorkloads := set.NewSet[string]()
	for i := range a.Workloads {
		w := &a.Workloads[i]
		if err := w.validate(seenWorkloads); err != nil {
			return oopsx.B("deploy").Wrapf(err, "workloads[%d]", i)
		}
	}

	// Basic cross-ref validation (best-effort in v0.1)
	for i := range a.Workloads {
		w := &a.Workloads[i]
		for j := range w.DependsOn {
			if !seenWorkloads.Contains(w.DependsOn[j].Name) {
				return oopsx.B("deploy").Errorf("workloads[%d].dependsOn[%d]: unknown workload %q", i, j, w.DependsOn[j].Name)
			}
		}
		for j := range w.Mounts {
			if strings.TrimSpace(w.Mounts[j].Volume.Name) == "" {
				return oopsx.B("deploy").Errorf("workloads[%d].mounts[%d].volume: name is required", i, j)
			}
		}
		for j := range w.Endpoints {
			if err := w.Endpoints[j].validate(); err != nil {
				return oopsx.B("deploy").Wrapf(err, "workloads[%d].endpoints[%d]", i, j)
			}
		}
	}

	for i := range a.Ingresses {
		ing := &a.Ingresses[i]
		if strings.TrimSpace(ing.Name) == "" {
			return oopsx.B("deploy").Errorf("ingresses[%d].name is required", i)
		}
		for j := range ing.Routes {
			r := &ing.Routes[j]
			if strings.TrimSpace(r.Path) == "" {
				return oopsx.B("deploy").Errorf("ingresses[%d].routes[%d].path is required", i, j)
			}
			if strings.TrimSpace(r.Backend.Workload) == "" || strings.TrimSpace(r.Backend.Endpoint) == "" {
				return oopsx.B("deploy").Errorf("ingresses[%d].routes[%d].backend must specify workload + endpoint", i, j)
			}
			if !seenWorkloads.Contains(r.Backend.Workload) {
				return oopsx.B("deploy").Errorf("ingresses[%d].routes[%d].backend: unknown workload %q", i, j, r.Backend.Workload)
			}
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
	case RuntimeDocker, RuntimeContainerd, RuntimeFirecracker, RuntimeProcess:
	default:
		return oopsx.B("deploy").Errorf("invalid runtime %q", w.Runtime)
	}
	if strings.TrimSpace(w.Run.Image) == "" {
		return oopsx.B("deploy").Errorf("run.image is required")
	}
	if w.Replicas < 0 {
		return oopsx.B("deploy").Errorf("replicas must be >= 0")
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
