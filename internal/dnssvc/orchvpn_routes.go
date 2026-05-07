package dnssvc

import (
	"net"
	"slices"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	"github.com/arcgolabs/dnsx/dnsserver"
	"github.com/miekg/dns"
)

// ListWorkloadIPv4HostRoutes returns unique /32 routes for each in-memory workload A record (orch-vpn bootstrap).
// Non-IPv4 or invalid data are skipped. Sorted for stable API output.
func (s *Service) ListWorkloadIPv4HostRoutes() []string {
	if s == nil || s.workloadRecords == nil {
		return nil
	}
	recs := s.workloadRecords.Values()
	routes := list.FilterMapList(list.NewList(recs...), func(_ int, rec dnsserver.Record) (string, bool) {
		if rec.Type != dns.TypeA {
			return "", false
		}
		raw := strings.TrimSpace(rec.Data)
		if raw == "" {
			return "", false
		}
		pip := net.ParseIP(raw)
		if pip == nil || pip.To4() == nil {
			return "", false
		}
		return pip.String() + "/32", true
	}).Values()
	out := set.NewSet(routes...).Values()
	slices.Sort(out)
	return out
}
