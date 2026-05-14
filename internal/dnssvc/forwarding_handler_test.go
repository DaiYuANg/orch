package dnssvc_test

import (
	"context"
	"log/slog"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/arcgolabs/dnsx/dnsserver"
	"github.com/miekg/dns"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/dnssvc"
)

func TestServiceForwardsNonOrchQueriesToWorkloadUpstream(t *testing.T) {
	t.Parallel()

	var upstreamQueries atomic.Int32
	upstream := startTestDNSServer(t, func(writer dns.ResponseWriter, request *dns.Msg) {
		upstreamQueries.Add(1)
		reply := new(dns.Msg)
		reply.SetReply(request)
		reply.RecursionAvailable = true
		reply.Answer = []dns.RR{&dns.A{
			Hdr: dns.RR_Header{
				Name:   request.Question[0].Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    30,
			},
			A: net.ParseIP("203.0.113.10"),
		}}
		if err := writer.WriteMsg(reply); err != nil {
			t.Errorf("write upstream dns reply: %v", err)
		}
	})

	dnsCfg := config.DNSConfig{
		Enabled: true,
		Listen:  "127.0.0.1:0",
		Zone:    "orch.local",
	}
	dnsCfg.Data.Path = filepath.Join(t.TempDir(), "dns.db")
	dnsCfg.Workload.Upstream = []string{upstream}
	svc := dnssvc.New(config.Config{DNS: dnsCfg}, testDNSLogger())

	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("start service: %v", err)
	}
	t.Cleanup(func() {
		if err := svc.Stop(ctx); err != nil {
			t.Fatalf("stop service: %v", err)
		}
	})

	response := queryTestDNS(t, svc.UDPAddr(), "www.example.net.", dns.TypeA)
	if response.Rcode != dns.RcodeSuccess || len(response.Answer) != 1 {
		t.Fatalf("external response rcode=%d answer=%#v", response.Rcode, response.Answer)
	}
	if upstreamQueries.Load() != 1 {
		t.Fatalf("upstream queries = %d, want 1", upstreamQueries.Load())
	}

	response = queryTestDNS(t, svc.UDPAddr(), "missing.orch.local.", dns.TypeA)
	if response.Rcode != dns.RcodeNameError {
		t.Fatalf("orch-zone miss rcode = %d, want NXDOMAIN", response.Rcode)
	}
	if upstreamQueries.Load() != 1 {
		t.Fatalf("orch-zone query was forwarded, upstream queries = %d", upstreamQueries.Load())
	}
}

func startTestDNSServer(t *testing.T, handler dns.HandlerFunc) string {
	t.Helper()

	conn, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	server := &dns.Server{
		PacketConn: conn,
		Handler:    handler,
	}
	go func() {
		if err := server.ActivateAndServe(); err != nil {
			t.Logf("dns test server stopped: %v", err)
		}
	}()
	t.Cleanup(func() {
		if err := server.Shutdown(); err != nil {
			t.Fatalf("shutdown dns test server: %v", err)
		}
	})
	return conn.LocalAddr().String()
}

func queryTestDNS(t *testing.T, addr, name string, qtype uint16) *dns.Msg {
	t.Helper()

	msg := new(dns.Msg)
	msg.SetQuestion(name, qtype)
	response, _, err := (&dns.Client{Net: "udp"}).Exchange(msg, addr)
	if err != nil {
		t.Fatalf("dns query %s @ %s: %v", name, addr, err)
	}
	if response == nil {
		t.Fatalf("dns query %s @ %s returned nil response", name, addr)
	}
	return response
}

func TestForwardingHandlerWithoutUpstreamRefusesExternalQueries(t *testing.T) {
	t.Parallel()

	store, err := dnsserver.OpenBboltStore(filepath.Join(t.TempDir(), "dns.db"), testDNSLogger())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close dns store: %v", err)
		}
	})
	if err := store.SaveRecord(context.Background(), dnsserver.Record{
		Zone: "orch.local",
		Name: "orch.local",
		TTL:  60,
		Type: dns.TypeA,
		Data: "127.0.0.1",
	}); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	resolver := dnsserver.NewResolver(store, dnsserver.WithResolverLogger(testDNSLogger()))
	server := startTestDNSServer(t, dnssvc.NewForwardingHandler(resolver, nil, nil).ServeDNS)
	response := queryTestDNS(t, server, "www.example.net.", dns.TypeA)
	if response.Rcode != dns.RcodeRefused {
		t.Fatalf("external rcode = %d, want refused", response.Rcode)
	}
}

func testDNSLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
