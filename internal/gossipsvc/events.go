package gossipsvc

import (
	"context"
	"strings"
	"time"

	"github.com/hashicorp/memberlist"
)

func (s *Service) eventLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-s.events:
			s.applyNodeEvent(event)
		}
	}
}

func (s *Service) applyNodeEvent(event memberlist.NodeEvent) {
	if event.Node == nil {
		return
	}
	node, ok := nodeFromMemberlist(event.Node)
	if !ok {
		return
	}
	switch event.Event {
	case memberlist.NodeJoin, memberlist.NodeUpdate:
		s.members.Set(node.ID, node)
	case memberlist.NodeLeave:
		node.State = "left"
		node.UpdatedAt = time.Now().UTC()
		s.members.Set(node.ID, node)
	}
}

func (s *Service) refreshMembers() {
	if s == nil || s.ml == nil {
		return
	}
	for _, member := range s.ml.Members() {
		node, ok := nodeFromMemberlist(member)
		if ok {
			s.members.Set(node.ID, node)
		}
	}
}

func nodeFromMemberlist(member *memberlist.Node) (Node, bool) {
	if member == nil {
		return Node{}, false
	}
	meta, ok := decodeNodeMetadata(member.Meta)
	if !ok {
		return Node{}, false
	}
	return Node{
		ID:            meta.NodeID,
		GossipAddress: strings.TrimSpace(member.Address()),
		RaftAddress:   meta.RaftAddress,
		APIURL:        meta.APIURL,
		Version:       meta.Version,
		State:         nodeStateName(member.State),
		UpdatedAt:     time.Now().UTC(),
	}, true
}
