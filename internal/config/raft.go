package config

type Raft struct {
	Enable            bool     `koanf:"enable"`
	NodeID            string   `koanf:"node_id"`
	BindAddr          string   `koanf:"bind_addr"`
	APIAddr           string   `koanf:"api_addr"`
	NodeAPI           []string `koanf:"node_api"`
	DataDir           string   `koanf:"data_dir"`
	Bootstrap         bool     `koanf:"bootstrap"`
	Join              []string `koanf:"join"`
	ApplyTimeout      string   `koanf:"apply_timeout"`
	LeaderWaitTimeout string   `koanf:"leader_wait_timeout"`
}
