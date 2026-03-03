package dns

import (
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/DaiYuANg/warden/internal/registry"
	"github.com/miekg/dns"
)

const defaultDomainSuffix = ".warden.local."

type DNSServer struct {
	mu      sync.RWMutex
	records map[string]Record

	registry *registry.Service
	logger   *slog.Logger

	udpServer *dns.Server
	tcpServer *dns.Server
}

func NewDNSServer(logger *slog.Logger, registryService *registry.Service) *DNSServer {
	return &DNSServer{
		records:  map[string]Record{},
		registry: registryService,
		logger:   logger,
	}
}

func (d *DNSServer) SetRecord(domain, ip string) {
	domain = dns.Fqdn(strings.TrimSpace(domain))
	ip = strings.TrimSpace(ip)
	if domain == "." || ip == "" {
		return
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	created := now
	if old, ok := d.records[domain]; ok && !old.CreatedAt.IsZero() {
		created = old.CreatedAt
	}
	d.records[domain] = Record{
		Domain:     domain,
		Type:       "A",
		Value:      ip,
		TTLSeconds: 10,
		CreatedAt:  created,
		UpdatedAt:  now,
	}
}

func (d *DNSServer) DeleteRecord(domain string) {
	domain = dns.Fqdn(strings.TrimSpace(domain))
	if domain == "." {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.records, domain)
}

func (d *DNSServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
		if q.Qtype != dns.TypeA {
			continue
		}
		domain := dns.Fqdn(q.Name)

		// 1) static records
		if answers := d.lookupStaticA(domain); len(answers) > 0 {
			msg.Answer = append(msg.Answer, answers...)
			continue
		}

		// 2) service registry records
		service := extractServiceName(domain)
		if service == "" {
			continue
		}
		answers := d.lookupServiceA(domain, service)
		msg.Answer = append(msg.Answer, answers...)
	}

	_ = w.WriteMsg(msg)
}

func (d *DNSServer) Serve(addr string) error {
	dns.HandleFunc(".", d.ServeDNS)

	d.udpServer = &dns.Server{Addr: addr, Net: "udp"}
	d.tcpServer = &dns.Server{Addr: addr, Net: "tcp"}

	go func() {
		if err := d.udpServer.ListenAndServe(); err != nil {
			d.logger.Error("failed to start dns udp server", "error", err, "addr", addr)
		}
	}()
	go func() {
		if err := d.tcpServer.ListenAndServe(); err != nil {
			d.logger.Error("failed to start dns tcp server", "error", err, "addr", addr)
		}
	}()
	return nil
}

func (d *DNSServer) Shutdown() error {
	if d.udpServer != nil {
		_ = d.udpServer.Shutdown()
	}
	if d.tcpServer != nil {
		_ = d.tcpServer.Shutdown()
	}
	return nil
}

func (d *DNSServer) lookupStaticA(domain string) []dns.RR {
	d.mu.RLock()
	record, ok := d.records[domain]
	d.mu.RUnlock()
	if !ok || strings.TrimSpace(record.Value) == "" {
		return nil
	}
	ip := net.ParseIP(record.Value)
	if ip == nil || ip.To4() == nil {
		return nil
	}
	ttl := uint32(10)
	if record.TTLSeconds > 0 {
		ttl = uint32(record.TTLSeconds)
	}
	return []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			A: ip.To4(),
		},
	}
}

func (d *DNSServer) lookupServiceA(domain, service string) []dns.RR {
	if d.registry == nil {
		return nil
	}
	ips, err := d.registry.ResolveServiceIPs(service)
	if err != nil {
		return nil
	}
	answers := make([]dns.RR, 0, len(ips))
	for _, rawIP := range ips {
		ip := net.ParseIP(rawIP)
		if ip == nil || ip.To4() == nil {
			continue
		}
		answers = append(answers, &dns.A{
			Hdr: dns.RR_Header{
				Name:   domain,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    10,
			},
			A: ip.To4(),
		})
	}
	return answers
}

func extractServiceName(domain string) string {
	name := strings.ToLower(strings.TrimSpace(domain))
	if !strings.HasSuffix(name, defaultDomainSuffix) {
		return ""
	}
	service := strings.TrimSuffix(name, defaultDomainSuffix)
	service = strings.TrimSuffix(service, ".")
	return service
}
