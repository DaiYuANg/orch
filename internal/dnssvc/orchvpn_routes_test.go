package dnssvc

import (
	"testing"

	"github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/dnsx/dnsserver"
	"github.com/miekg/dns"
)

func TestListWorkloadIPv4HostRoutes(t *testing.T) {
	t.Parallel()
	s := &Service{
		workloadRecords: mapping.NewShardedConcurrentMap[string, dnsserver.Record](0, mapping.HashString),
	}
	s.workloadRecords.Set("a", dnsserver.Record{Type: dns.TypeA, Data: "10.0.0.2"})
	s.workloadRecords.Set("b", dnsserver.Record{Type: dns.TypeA, Data: "10.0.0.3"})
	s.workloadRecords.Set("c", dnsserver.Record{Type: dns.TypeA, Data: "10.0.0.2"})
	got := s.ListWorkloadIPv4HostRoutes()
	if len(got) != 2 || got[0] != "10.0.0.2/32" || got[1] != "10.0.0.3/32" {
		t.Fatalf("got %#v", got)
	}
}
