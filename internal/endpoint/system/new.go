package system

import "github.com/danielgtaylor/huma/v2"
import internalraft "github.com/DaiYuANg/warden/internal/raft"

func (s *Endpoint) Register(openapi huma.API) {
	tag := huma.OperationTags("System")
	huma.Get(openapi, "/system/cpu", s.CPUHandler, tag)
	huma.Get(openapi, "/system/mem", s.MemHandler, tag)
	huma.Get(openapi, "/system/disk", s.DiskHandler, tag)
	huma.Get(openapi, "/system/net", s.NetHandler, tag)
	huma.Get(openapi, "/system/info", s.InfoHandler, tag)
	huma.Get(openapi, "/system/cluster", s.ClusterHandler, tag)
	huma.Post(openapi, "/system/cluster/join", s.JoinClusterHandler, tag)
	huma.Post(openapi, "/system/cluster/remove", s.RemoveClusterHandler, tag)
}

func NewSystemEndpoint(raftService *internalraft.Service) *Endpoint {
	return &Endpoint{raft: raftService}
}
