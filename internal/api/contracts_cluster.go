package api

import "github.com/arcgolabs/collectionx/list"

type RaftMemberItem struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Suffrage string `json:"suffrage"`
}

type RaftStatusOutput struct {
	Body struct {
		Ready         bool                       `json:"ready"`
		NodeID        string                     `json:"nodeId"`
		State         string                     `json:"state"`
		IsLeader      bool                       `json:"isLeader"`
		LeaderID      string                     `json:"leaderId,omitempty"`
		LeaderAddress string                     `json:"leaderAddress,omitempty"`
		LeaderAPIURL  string                     `json:"leaderApiUrl,omitempty"`
		LocalAddress  string                     `json:"localAddress,omitempty"`
		Message       string                     `json:"message,omitempty"`
		Members       *list.List[RaftMemberItem] `json:"members"`
	} `json:"body"`
}

type ListRaftMembersOutput struct {
	Body struct {
		Items *list.List[RaftMemberItem] `json:"items"`
	} `json:"body"`
}

type AddRaftMemberInput struct {
	Body struct {
		ID      string `json:"id"`
		Address string `json:"address"`
	} `json:"body"`
}

type AddRaftMemberOutput struct {
	Body struct {
		Accepted bool           `json:"accepted"`
		Member   RaftMemberItem `json:"member"`
	} `json:"body"`
}

type RemoveRaftMemberInput struct {
	ID string `path:"id"`
}

type RemoveRaftMemberOutput struct {
	Body struct {
		Accepted bool   `json:"accepted"`
		ID       string `json:"id"`
	} `json:"body"`
}

// OrchVPNBootstrapOutput is the response body for GET PathV1OrchVPNBootstrap.
type OrchVPNBootstrapOutput struct {
	Body struct {
		Enabled         bool               `json:"enabled"`
		APIVersion      string             `json:"api_version"`
		Encap           string             `json:"encap"`
		MTU             int                `json:"mtu"`
		TunnelUDPPort   int                `json:"tunnel_udp_port"`
		DNSZone         string             `json:"dns_zone"`
		ContainerRoutes *list.List[string] `json:"container_routes,omitempty"`
	} `json:"body"`
}
