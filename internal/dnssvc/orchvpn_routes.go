package dnssvc

import (
	"net"
	"slices"
	"strings"

	"github.com/miekg/dns"
)

// ListWorkloadIPv4HostRoutes returns unique /32 routes for each in-memory workload A record (orch-vpn bootstrap).
// Non-IPv4 or invalid data are skipped. Sorted for stable API output.
func (s *Service) ListWorkloadIPv4HostRoutes() []string {
	if s == nil || s.workloadRecords == nil {
		return nil
	}
	recs := s.workloadRecords.Values()
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, rec := range recs {
		if rec.Type != dns.TypeA {
			continue
		}
		raw := strings.TrimSpace(rec.Data)
		if raw == "" {
			continue
		}
		pip := net.ParseIP(raw)
		if pip == nil || pip.To4() == nil {
			continue
		}
		cidr := pip.String() + "/32"
		if _, dup := seen[cidr]; dup {
			continue
		}
		seen[cidr] = struct{}{}
		out = append(out, cidr)
	}
	slices.Sort(out)
	return out
}
