package v1alpha1

import (
	"regexp"
	"strings"

	"github.com/arcgolabs/collectionx/set"
	"gopkg.in/yaml.v3"

	"github.com/daiyuang/orch/internal/oopsx"
)

// App is the YAML-friendly canonical deploy model for the first Go rewrite
// iteration. It intentionally mirrors the canonical model described in
// docs/src/dsl.md and docs/src/dsl.zh.md.
type App struct {
	APIVersion string   `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	Kind       string   `yaml:"kind,omitempty" json:"kind,omitempty"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`

	Workloads []Workload `yaml:"workloads,omitempty" json:"workloads,omitempty"`
	Configs   []Config   `yaml:"configs,omitempty" json:"configs,omitempty"`
	Secrets   []Secret   `yaml:"secrets,omitempty" json:"secrets,omitempty"`
	Volumes   []Volume   `yaml:"volumes,omitempty" json:"volumes,omitempty"`
	Ingresses []Ingress  `yaml:"ingresses,omitempty" json:"ingresses,omitempty"`
}

type Metadata struct {
	Name        string            `yaml:"name" json:"name"`
	Namespace   string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

type Workload struct {
	Name string       `yaml:"name" json:"name"`
	Kind WorkloadKind `yaml:"kind" json:"kind"` // service|worker|job|cron|stateful
	Run  RunSpec      `yaml:"run" json:"run"`   // image/command/args/env/cwd/runtimeOptions
	// Runtime selects the backend adapter. This stays separate from Run.RuntimeOptions
	// because the canonical intent model needs a stable first-class field.
	Runtime RuntimeKind `yaml:"runtime" json:"runtime"` // docker|containerd|firecracker|process

	Replicas  int           `yaml:"replicas,omitempty" json:"replicas,omitempty"`
	DependsOn []WorkloadRef `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`

	Endpoints []Endpoint `yaml:"endpoints,omitempty" json:"endpoints,omitempty"`
	Mounts    []Mount    `yaml:"mounts,omitempty" json:"mounts,omitempty"`
	Resources *Resources `yaml:"resources,omitempty" json:"resources,omitempty"`
	Health    *Health    `yaml:"health,omitempty" json:"health,omitempty"`

	Scheduling *Scheduling `yaml:"scheduling,omitempty" json:"scheduling,omitempty"`
	Rollout    *Rollout    `yaml:"rollout,omitempty" json:"rollout,omitempty"`
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
	Image   string     `yaml:"image" json:"image"`
	Command []string   `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string   `yaml:"args,omitempty" json:"args,omitempty"`
	Env     []EnvVar   `yaml:"env,omitempty" json:"env,omitempty"`
	Cwd     string     `yaml:"cwd,omitempty" json:"cwd,omitempty"`
	Options RunOptions `yaml:"runtimeOptions,omitempty" json:"runtimeOptions,omitempty"`
}

type EnvVar struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

// RunOptions captures backend-specific knobs. Only docker/containerd are in-scope
// for the first Go version; other fields can be added later.
type RunOptions struct {
	Docker     *DockerOptions     `yaml:"docker,omitempty" json:"docker,omitempty"`
	Containerd *ContainerdOptions `yaml:"containerd,omitempty" json:"containerd,omitempty"`
}

type DockerOptions struct {
	NetworkMode string            `yaml:"networkMode,omitempty" json:"networkMode,omitempty"`
	Privileged  bool              `yaml:"privileged,omitempty" json:"privileged,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

type ContainerdOptions struct {
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	// Snapshotter or runtime handler can be added later when we wire containerd.
}

type Endpoint struct {
	Name     string        `yaml:"name" json:"name"`
	Port     int           `yaml:"port" json:"port"`
	Protocol EndpointProto `yaml:"protocol" json:"protocol"` // tcp|udp|http
}

type EndpointProto string

const (
	ProtoTCP  EndpointProto = "tcp"
	ProtoUDP  EndpointProto = "udp"
	ProtoHTTP EndpointProto = "http"
)

type Mount struct {
	Volume   VolumeRef `yaml:"volume" json:"volume"`
	Target   string    `yaml:"target" json:"target"`
	ReadOnly bool      `yaml:"readOnly,omitempty" json:"readOnly,omitempty"`
}

type Resources struct {
	CPUMillis   int64 `yaml:"cpuMillis,omitempty" json:"cpuMillis,omitempty"`
	MemoryBytes int64 `yaml:"memoryBytes,omitempty" json:"memoryBytes,omitempty"`
}

type Health struct {
	Readiness *Probe `yaml:"readiness,omitempty" json:"readiness,omitempty"`
	Liveness  *Probe `yaml:"liveness,omitempty" json:"liveness,omitempty"`
	Startup   *Probe `yaml:"startup,omitempty" json:"startup,omitempty"`
}

type Probe struct {
	HTTP *HTTPProbe `yaml:"http,omitempty" json:"http,omitempty"`
	// Future: tcp, exec
}

type HTTPProbe struct {
	Path     string      `yaml:"path" json:"path"`
	Endpoint EndpointRef `yaml:"endpoint" json:"endpoint"`
}

type Scheduling struct {
	Stateful       bool     `yaml:"stateful,omitempty" json:"stateful,omitempty"`
	AllowLeader    bool     `yaml:"allowLeader,omitempty" json:"allowLeader,omitempty"`
	PreferredNodes []string `yaml:"preferredNodes,omitempty" json:"preferredNodes,omitempty"`
}

type Rollout struct {
	Strategy       string `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	MaxUnavailable int    `yaml:"maxUnavailable,omitempty" json:"maxUnavailable,omitempty"`
	MaxSurge       int    `yaml:"maxSurge,omitempty" json:"maxSurge,omitempty"`
}

type Config struct {
	Name string            `yaml:"name" json:"name"`
	Data map[string]string `yaml:"data,omitempty" json:"data,omitempty"`
}

type Secret struct {
	Name string            `yaml:"name" json:"name"`
	Data map[string]string `yaml:"data,omitempty" json:"data,omitempty"`
}

type Volume struct {
	Name       string `yaml:"name" json:"name"`
	Persistent bool   `yaml:"persistent,omitempty" json:"persistent,omitempty"`
	// SizeBytes is optional; keep numeric for canonical normalization.
	SizeBytes int64 `yaml:"sizeBytes,omitempty" json:"sizeBytes,omitempty"`
}

type Ingress struct {
	Name   string         `yaml:"name" json:"name"`
	Host   string         `yaml:"host,omitempty" json:"host,omitempty"`
	Routes []IngressRoute `yaml:"routes,omitempty" json:"routes,omitempty"`
}

type IngressRoute struct {
	Path    string      `yaml:"path" json:"path"`
	Backend EndpointRef `yaml:"backend" json:"backend"`
}

// ---- Typed references (YAML-friendly) ----

// WorkloadRef refers to a workload by name. YAML form:
// - "redis" (string)  OR  { name: "redis" }
type WorkloadRef struct {
	Name string `yaml:"name" json:"name"`
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
	Name string `yaml:"name" json:"name"`
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
	Workload string `yaml:"workload" json:"workload"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
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
