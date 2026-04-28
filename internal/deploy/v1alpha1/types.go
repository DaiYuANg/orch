package v1alpha1

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
	Image   string     `json:"image"             yaml:"image"`
	Command []string   `json:"command,omitempty" yaml:"command,omitempty"`
	Args    []string   `json:"args,omitempty"    yaml:"args,omitempty"`
	Env     []EnvVar   `json:"env,omitempty"     yaml:"env,omitempty"`
	Cwd     string     `json:"cwd,omitempty"     yaml:"cwd,omitempty"`
	Options RunOptions `json:"runtimeOptions"    yaml:"runtimeOptions"`
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

// VolumeRef refers to a volume by name. YAML form:
// - "redisData"  OR  { name: "redisData" }
type VolumeRef struct {
	Name string `json:"name" yaml:"name"`
}

// EndpointRef refers to a workload endpoint. YAML form:
// - "gateway:http" OR { workload: "gateway", endpoint: "http" }
type EndpointRef struct {
	Workload string `json:"workload" yaml:"workload"`
	Endpoint string `json:"endpoint" yaml:"endpoint"`
}
