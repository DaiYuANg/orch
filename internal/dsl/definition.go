package dsl

type Workload struct {
	Name        string    `yaml:"name" hcl:"name"`
	Description string    `yaml:"description,omitempty" hcl:"description,optional"`
	Include     []string  `yaml:"include,omitempty" hcl:"include,optional"`
	Includes    []string  `yaml:"includes,omitempty" hcl:"includes,optional"`
	Datacenters []string  `yaml:"datacenters,omitempty" hcl:"datacenters,optional"`
	Units       []Unit    `yaml:"units" hcl:"units,block"`
	Resources   Resources `yaml:"resources,omitempty" hcl:"resources,block"`
}

type Unit struct {
	Name      string    `yaml:"name" hcl:"name"`
	Tasks     []Task    `yaml:"tasks" hcl:"tasks,block"`
	Resources Resources `yaml:"resources,omitempty" hcl:"resources,block"`
}

type Task struct {
	Name      string            `yaml:"name" hcl:"name"`
	Type      string            `yaml:"type" hcl:"type"`
	Driver    string            `yaml:"driver" hcl:"driver"`
	Command   []string          `yaml:"command,omitempty" hcl:"command,optional"`
	Image     string            `yaml:"image,omitempty" hcl:"image,optional"`
	Schedule  string            `yaml:"schedule,omitempty" hcl:"schedule,optional"`
	Replicas  int               `yaml:"replicas,omitempty" hcl:"replicas,optional"`
	Env       map[string]string `yaml:"env,omitempty" hcl:"env,optional"`
	Tags      []string          `yaml:"tags,omitempty" hcl:"tags,optional"`
	Labels    map[string]string `yaml:"labels,omitempty" hcl:"labels,optional"`
	Check     *HealthCheck      `yaml:"check,omitempty" hcl:"check,block"`
	Resources Resources         `yaml:"resources,omitempty" hcl:"resources,block"`
	Network   *NetworkConfig    `yaml:"network,omitempty" hcl:"network,block"`
	DNS       *DNSConfig        `yaml:"dns,omitempty" hcl:"dns,block"`
}

type HealthCheck struct {
	Type     string `yaml:"type" hcl:"type"`
	Path     string `yaml:"path,omitempty" hcl:"path,optional"`
	Command  string `yaml:"command,omitempty" hcl:"command,optional"`
	Interval string `yaml:"interval" hcl:"interval"`
	Retries  int    `yaml:"retries,omitempty" hcl:"retries,optional"`
	Timeout  string `yaml:"timeout,omitempty" hcl:"timeout,optional"`
}

type Resources struct {
	CPU     int     `yaml:"cpu" hcl:"cpu"`
	Memory  int     `yaml:"memory" hcl:"memory"`
	Network Network `yaml:"network" hcl:"network,block"`
}

type Network struct {
	Mbits int `yaml:"mbits" hcl:"mbits"`
}

type NetworkConfig struct {
	Name string         `yaml:"name" hcl:"name"`
	Port map[string]int `yaml:"port" hcl:"port"`
}

type DNSConfig struct {
	Resolver string   `yaml:"resolver" hcl:"resolver"`
	Domains  []string `yaml:"domains" hcl:"domains"`
}

func (w *Workload) normalize() {
	if w == nil {
		return
	}
	if len(w.Include) > 0 {
		w.Includes = append(w.Includes, w.Include...)
	}
}
