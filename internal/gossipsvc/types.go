package gossipsvc

import "time"

type Node struct {
	ID            string    `json:"id"`
	GossipAddress string    `json:"gossipAddress"`
	RaftAddress   string    `json:"raftAddress,omitempty"`
	APIURL        string    `json:"apiUrl,omitempty"`
	Version       string    `json:"version,omitempty"`
	State         string    `json:"state"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type nodeMetadata struct {
	NodeID      string `json:"node_id"`
	RaftAddress string `json:"raft_addr,omitempty"`
	APIURL      string `json:"api_url,omitempty"`
	Version     string `json:"version,omitempty"`
}
