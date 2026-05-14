package gossipsvc

import (
	"encoding/json"
	"strings"

	"github.com/hashicorp/memberlist"

	"github.com/lyonbrown4d/orch/internal/buildmeta"
	"github.com/lyonbrown4d/orch/internal/config"
)

func (s *Service) nodeMeta(limit int) []byte {
	meta := nodeMetadata{
		NodeID:      strings.TrimSpace(s.local.String()),
		RaftAddress: s.localRaftAddress(),
		APIURL:      s.apiURL(),
		Version:     buildmeta.Version(),
	}
	return encodeNodeMetadata(meta, limit)
}

func (s *Service) localRaftAddress() string {
	if s == nil || s.raft == nil {
		return ""
	}
	return strings.TrimSpace(s.raft.LocalAddress())
}

func (s *Service) apiURL() string {
	if url := strings.TrimRight(strings.TrimSpace(s.cfg.Gossip.APIURL), "/"); url != "" {
		return url
	}
	if url, ok := s.cfg.Cluster.NodeURL(s.local.String()); ok {
		return url
	}
	if addr := config.FixLoopbackHost(s.cfg.HTTP.Addr); addr != "" {
		return "http://" + addr
	}
	return ""
}

func encodeNodeMetadata(meta nodeMetadata, limit int) []byte {
	meta.NodeID = strings.TrimSpace(meta.NodeID)
	meta.RaftAddress = strings.TrimSpace(meta.RaftAddress)
	meta.APIURL = strings.TrimRight(strings.TrimSpace(meta.APIURL), "/")
	meta.Version = strings.TrimSpace(meta.Version)
	b, err := json.Marshal(meta)
	if err != nil {
		return nil
	}
	if limit <= 0 || len(b) <= limit {
		return b
	}
	meta.APIURL = ""
	b, err = json.Marshal(meta)
	if err != nil || len(b) > limit {
		return nil
	}
	return b
}

func decodeNodeMetadata(raw []byte) (nodeMetadata, bool) {
	var meta nodeMetadata
	if len(raw) == 0 {
		return meta, false
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return meta, false
	}
	meta.NodeID = strings.TrimSpace(meta.NodeID)
	meta.RaftAddress = strings.TrimSpace(meta.RaftAddress)
	meta.APIURL = strings.TrimRight(strings.TrimSpace(meta.APIURL), "/")
	meta.Version = strings.TrimSpace(meta.Version)
	return meta, meta.NodeID != ""
}

func nodeStateName(state memberlist.NodeStateType) string {
	switch state {
	case memberlist.StateAlive:
		return "alive"
	case memberlist.StateSuspect:
		return "suspect"
	case memberlist.StateDead:
		return "dead"
	case memberlist.StateLeft:
		return "left"
	default:
		return "unknown"
	}
}
