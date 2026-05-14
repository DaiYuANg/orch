package raftsvc

import (
	"fmt"
	"strings"

	"github.com/arcgolabs/collectionx/list"
)

func (s *Service) rememberMember(id string, replicaID uint64, address string) {
	id = strings.TrimSpace(id)
	address = strings.TrimSpace(address)
	if id == "" || replicaID == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mu.replicaToNode[replicaID] = id
	if address != "" {
		s.mu.addressToNode[address] = id
	}
}

func (s *Service) forgetMember(replicaID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mu.replicaToNode, replicaID)
}

func (s *Service) nodeIDForMember(replicaID uint64, address string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id, ok := s.mu.replicaToNode[replicaID]; ok && strings.TrimSpace(id) != "" {
		return id
	}
	if id, ok := s.mu.addressToNode[strings.TrimSpace(address)]; ok && strings.TrimSpace(id) != "" {
		return id
	}
	return fmt.Sprintf("replica-%d", replicaID)
}

// BootstrapServerList returns the configured static peers used to bootstrap Dragonboat's initial members.
func (s *Service) BootstrapServerList(localID, localAddr string) (*list.List[Member], error) {
	replicaID, err := replicaIDForNodeID(localID)
	if err != nil {
		return nil, err
	}
	targets, err := s.bootstrapReplicaTargets(replicaID, localAddr)
	if err != nil {
		return nil, err
	}
	members := list.NewListWithCapacity[Member](len(targets))
	for rid, target := range targets {
		members.Add(Member{
			ID:       s.nodeIDForMember(rid, target),
			Address:  target,
			Suffrage: "Voter",
		})
	}
	members.Sort(func(a, b Member) int {
		return strings.Compare(a.ID, b.ID)
	})
	return members, nil
}
