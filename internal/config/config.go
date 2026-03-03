package config

type Config struct {
	Gossip  GossipConfig `koanf:"gossip"`
	Http    Http         `koanf:"http"`
	Network Network      `koanf:"network"`
	Logger  Logger       `koanf:"logger"`
	Raft    Raft         `koanf:"raft"`
}

func defaultConfig() Config {
	return Config{
		Http: Http{
			Port: 7443,
			APIDoc: APIDoc{
				Enable:      true,
				Title:       "warden",
				Version:     "0.1.0",
				Description: "warden api",
				Path:        "/",
				OpenAPIPath: "/openapi.json",
			},
			DisableStartupMessage: true,
			ReduceMemoryUsage:     true,
			PrintRoutes:           false,
		},
		Logger: Logger{
			Level:      "debug",
			Console:    true,
			Caller:     true,
			MaxSizeMB:  100,
			MaxAgeDays: 7,
			MaxBackups: 5,
		},
		Network: Network{
			DNSListen:         ":1053",
			IngressHTTPListen: ":8088",
		},
	}
}
