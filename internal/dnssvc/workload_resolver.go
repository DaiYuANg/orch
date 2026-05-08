package dnssvc

import (
	"strings"

	"github.com/arcgolabs/collectionx/list"
)

// WorkloadNameserver returns the nameserver address to inject into runtimes that
// support per-workload DNS. It intentionally has no port; resolvers query :53.
func (s *Service) WorkloadNameserver() (string, bool) {
	if s == nil || !s.cfg.Enabled {
		return "", false
	}
	return s.cfg.WorkloadNameserver()
}

// WorkloadSearchDomains returns the default search list for a workload namespace.
func (s *Service) WorkloadSearchDomains(namespace string) *list.List[string] {
	if s == nil || !s.cfg.Enabled {
		return list.NewList[string]()
	}
	return s.cfg.WorkloadSearchDomainList(namespace)
}

// WorkloadAdvertiseAddress returns a configured node-level address for host-style
// runtimes such as process/systemd/windows-service, or fallback when unset.
func (s *Service) WorkloadAdvertiseAddress(fallback string) string {
	if s == nil || !s.cfg.Enabled {
		return strings.TrimSpace(fallback)
	}
	if addr := s.cfg.WorkloadAdvertiseAddress(); addr != "" {
		return addr
	}
	return strings.TrimSpace(fallback)
}
