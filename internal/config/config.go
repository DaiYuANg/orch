package config

type Config struct {
	Gossip GossipConfig
	Http   Http
	Raft   Raft
}

func defaultConfig() Config {

	return Config{
		Http: Http{Port: 7443},
	}
}
