package task

// 定义 Task 结构体
type Task struct {
	Name      string            `yaml:"name"`
	Driver    string            `yaml:"driver"`
	Config    map[string]string `yaml:"config"`
	Env       map[string]string `yaml:"env"`
	Runtime   string            `yaml:"runtime"`
	Service   Service           `yaml:"service"`
	Resources Resources         `yaml:"resources"`
}

// 定义 Service 结构体
type Service struct {
	Name  string      `yaml:"name"`
	Tags  []string    `yaml:"tags"`
	Port  string      `yaml:"port"`
	Check HealthCheck `yaml:"check"`
}

// 定义 HealthCheck 结构体
type HealthCheck struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
	Retries  int    `yaml:"retries"`
}

// 定义 Resources 结构体
type Resources struct {
	CPU     int     `yaml:"cpu"`
	Memory  int     `yaml:"memory"`
	Network Network `yaml:"network"`
}

// 定义 Network 结构体
type Network struct {
	Mbits int `yaml:"mbits"`
}

// 定义 Group 结构体
type Group struct {
	Name  string `yaml:"name"`
	Count int    `yaml:"count"`
	Task  []Task `yaml:"task"`
}

// 定义 Job 结构体
type Job struct {
	Name        string   `yaml:"name"`
	Datacenters []string `yaml:"datacenters"`
	Type        string   `yaml:"type"`
	Groups      []Group  `yaml:"group"`
}

// 定义 Network 配置
type NetworkConfig struct {
	Name string         `yaml:"name"`
	Port map[string]int `yaml:"port"`
}

// 定义 DNS 配置
type DNSConfig struct {
	Resolver string   `yaml:"resolver"`
	Domains  []string `yaml:"domains"`
}

// 定义整个配置结构体
type Config struct {
	Job      Job             `yaml:"task"`
	Networks []NetworkConfig `yaml:"network"`
	DNS      DNSConfig       `yaml:"dns"`
}
