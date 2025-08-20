package system

import "github.com/danielgtaylor/huma/v2"

func (s *Endpoint) Register(openapi huma.API) {
	tag := huma.OperationTags("System")
	huma.Get(openapi, "/system/cpu", s.CPUHandler, tag)
	huma.Get(openapi, "/system/mem", s.MemHandler, tag)
	huma.Get(openapi, "/system/disk", s.DiskHandler, tag)
	huma.Get(openapi, "/system/net", s.NetHandler, tag)
	huma.Get(openapi, "/system/info", s.InfoHandler, tag)
}

func NewSystemEndpoint() *Endpoint {
	return &Endpoint{}
}
