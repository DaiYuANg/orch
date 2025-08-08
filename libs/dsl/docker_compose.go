package dsl

type ComposeFile struct {
	Version  string                                `yaml:"version"`
	Services map[string]DockerComposeServiceConfig `yaml:"services"`
	Volumes  map[string]DockerComposeVolumeConfig  `yaml:"volumes,omitempty"`
	Networks map[string]DockerComposeNetworkConfig `yaml:"networks,omitempty"`
}

type DockerComposeServiceConfig struct {
	Image         string                    `yaml:"image,omitempty"`
	Build         *DockerComposeBuildConfig `yaml:"build,omitempty"`
	Ports         []string                  `yaml:"ports,omitempty"`
	Environment   map[string]string         `yaml:"environment,omitempty"`
	Volumes       []string                  `yaml:"volumes,omitempty"`
	DependsOn     []string                  `yaml:"depends_on,omitempty"`
	Command       []string                  `yaml:"command,omitempty"`
	Networks      []string                  `yaml:"networks,omitempty"`
	Restart       string                    `yaml:"restart,omitempty"`
	ContainerName string                    `yaml:"container_name,omitempty"`
	ExtraHosts    []string                  `yaml:"extra_hosts,omitempty"`
}

type DockerComposeBuildConfig struct {
	Context    string            `yaml:"context,omitempty"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Args       map[string]string `yaml:"args,omitempty"`
}

type DockerComposeVolumeConfig struct {
	Driver     string            `yaml:"driver,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
}

type DockerComposeNetworkConfig struct {
	Driver     string            `yaml:"driver,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
	External   bool              `yaml:"external,omitempty"`
}
