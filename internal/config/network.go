package config

type Network struct {
	DNSListen         string `koanf:"dns_listen"`
	IngressHTTPListen string `koanf:"ingress_http_listen"`
}
