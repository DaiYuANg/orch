package dnssvc

import (
	"net"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	"github.com/arcgolabs/dnsx/dnsserver"
	"github.com/miekg/dns"
)

// ListWorkloadIPv4HostRoutes returns unique /32 routes for each in-memory workload A record (orch-vpn bootstrap).
// Non-IPv4 or invalid data are skipped. Sorted for stable API output.
func (s *Service) ListWorkloadIPv4HostRoutes() []string {
	return s.WorkloadIPv4HostRouteList().Values()
}

// WorkloadIPv4HostRouteList returns unique /32 routes for each in-memory workload A record.
func (s *Service) WorkloadIPv4HostRouteList() *list.List[string] {
	if s == nil || s.workloadRecords == nil {
		return list.NewList[string]()
	}
	routes := set.NewSet[string]()
	s.workloadRecords.Range(func(_ string, rec dnsserver.Record) bool {
		if rec.Type != dns.TypeA {
			return true
		}
		raw := strings.TrimSpace(rec.Data)
		if raw == "" {
			return true
		}
		pip := net.ParseIP(raw)
		if pip == nil || pip.To4() == nil {
			return true
		}
		routes.Add(pip.String() + "/32")
		return true
	})
	out := list.NewListWithCapacity[string](routes.Len())
	routes.Range(func(route string) bool {
		out.Add(route)
		return true
	})
	out.Sort(strings.Compare)
	return out
}
