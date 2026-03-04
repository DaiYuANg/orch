package dns

import (
	"container/list"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DaiYuANg/warden/internal/registry"
	"github.com/adrg/xdg"
	"github.com/miekg/dns"
	"go.etcd.io/bbolt"
)

const (
	defaultDomainSuffix = ".warden.local."
	dnsRecordBucket     = "dns_records"

	defaultCacheCapacity = 4096
	defaultServiceTTL    = 3 * time.Second
)

type cachedAnswer struct {
	answers   []dns.RR
	expiresAt time.Time
}

type DNSServer struct {
	registry *registry.Service
	logger   *slog.Logger
	db       *bbolt.DB

	udpServer *dns.Server
	tcpServer *dns.Server

	cacheMu         sync.RWMutex
	answerCache     map[string]cachedAnswer
	answerCacheList *list.List
	answerCachePos  map[string]*list.Element
	cacheCapacity   int
	serviceCacheTTL time.Duration
}

func NewDNSServer(logger *slog.Logger, registryService *registry.Service) (*DNSServer, error) {
	db, err := openDNSStore()
	if err != nil {
		return nil, err
	}
	return &DNSServer{
		registry:        registryService,
		logger:          logger,
		db:              db,
		answerCache:     make(map[string]cachedAnswer),
		answerCacheList: list.New(),
		answerCachePos:  make(map[string]*list.Element),
		cacheCapacity:   defaultCacheCapacity,
		serviceCacheTTL: defaultServiceTTL,
	}, nil
}

func openDNSStore() (*bbolt.DB, error) {
	dataDir := filepath.Join(xdg.DataHome, "warden")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir dns data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "dns.db")
	db, err := bbolt.Open(dbPath, 0o700, nil)
	if err != nil {
		return nil, fmt.Errorf("open dns db: %w", err)
	}
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, createErr := tx.CreateBucketIfNotExists([]byte(dnsRecordBucket))
		return createErr
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create dns bucket: %w", err)
	}
	return db, nil
}

func (d *DNSServer) SetRecord(domain, ip string) {
	normalizedDomain := dns.Fqdn(strings.TrimSpace(domain))
	normalizedIP := strings.TrimSpace(ip)
	if normalizedDomain == "." || normalizedIP == "" {
		return
	}

	now := time.Now()
	record, found, err := d.loadRecord(normalizedDomain)
	if err != nil {
		d.logger.Error("load dns record before set failed", "domain", normalizedDomain, "error", err)
		return
	}
	created := now
	if found && !record.CreatedAt.IsZero() {
		created = record.CreatedAt
	}
	next := Record{
		Domain:     normalizedDomain,
		Type:       "A",
		Value:      normalizedIP,
		TTLSeconds: 10,
		CreatedAt:  created,
		UpdatedAt:  now,
	}
	if err := d.persistRecord(next); err != nil {
		d.logger.Error("persist dns record failed", "domain", normalizedDomain, "error", err)
		return
	}
	d.invalidateCache(normalizedDomain, dns.TypeA)
}

func (d *DNSServer) DeleteRecord(domain string) {
	normalizedDomain := dns.Fqdn(strings.TrimSpace(domain))
	if normalizedDomain == "." {
		return
	}
	if err := d.removeRecord(normalizedDomain); err != nil {
		d.logger.Error("delete dns record failed", "domain", normalizedDomain, "error", err)
		return
	}
	d.invalidateCache(normalizedDomain, dns.TypeA)
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
		msg.Answer = append(msg.Answer, d.resolveA(domain)...)
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
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DNSServer) resolveA(domain string) []dns.RR {
	cacheKey := queryCacheKey(domain, dns.TypeA)
	if cached, ok := d.lookupCache(cacheKey); ok {
		return cached
	}

	if staticAnswers := d.lookupStaticA(domain); len(staticAnswers) > 0 {
		d.storeCache(cacheKey, staticAnswers, minTTL(staticAnswers))
		return staticAnswers
	}

	service := extractServiceName(domain)
	if service == "" {
		return nil
	}
	serviceAnswers := d.lookupServiceA(domain, service)
	if len(serviceAnswers) == 0 {
		return nil
	}
	d.storeCache(cacheKey, serviceAnswers, d.serviceCacheTTL)
	return serviceAnswers
}

func (d *DNSServer) lookupStaticA(domain string) []dns.RR {
	record, found, err := d.loadRecord(domain)
	if err != nil {
		d.logger.Error("load dns record failed", "domain", domain, "error", err)
		return nil
	}
	if !found || !strings.EqualFold(record.Type, "A") || strings.TrimSpace(record.Value) == "" {
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
				Ttl:    uint32(d.serviceCacheTTL / time.Second),
			},
			A: ip.To4(),
		})
	}
	return answers
}

func (d *DNSServer) persistRecord(record Record) error {
	if d.db == nil {
		return fmt.Errorf("dns store is not initialized")
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return d.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(dnsRecordBucket))
		if bucket == nil {
			return fmt.Errorf("dns bucket not found")
		}
		return bucket.Put([]byte(record.Domain), payload)
	})
}

func (d *DNSServer) loadRecord(domain string) (Record, bool, error) {
	if d.db == nil {
		return Record{}, false, fmt.Errorf("dns store is not initialized")
	}
	var record Record
	err := d.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(dnsRecordBucket))
		if bucket == nil {
			return fmt.Errorf("dns bucket not found")
		}
		raw := bucket.Get([]byte(domain))
		if len(raw) == 0 {
			return nil
		}
		return json.Unmarshal(raw, &record)
	})
	if err != nil {
		return Record{}, false, err
	}
	if record.Domain == "" {
		return Record{}, false, nil
	}
	return record, true, nil
}

func (d *DNSServer) removeRecord(domain string) error {
	if d.db == nil {
		return fmt.Errorf("dns store is not initialized")
	}
	return d.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(dnsRecordBucket))
		if bucket == nil {
			return fmt.Errorf("dns bucket not found")
		}
		return bucket.Delete([]byte(domain))
	})
}

func queryCacheKey(domain string, qtype uint16) string {
	return fmt.Sprintf("%d:%s", qtype, strings.ToLower(strings.TrimSpace(domain)))
}

func (d *DNSServer) lookupCache(key string) ([]dns.RR, bool) {
	d.cacheMu.RLock()
	entry, ok := d.answerCache[key]
	d.cacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		d.cacheMu.Lock()
		d.removeCacheEntryLocked(key)
		d.cacheMu.Unlock()
		return nil, false
	}

	d.cacheMu.Lock()
	if node, exists := d.answerCachePos[key]; exists {
		d.answerCacheList.MoveToFront(node)
	}
	d.cacheMu.Unlock()
	return cloneAnswers(entry.answers), true
}

func (d *DNSServer) storeCache(key string, answers []dns.RR, ttl time.Duration) {
	if len(answers) == 0 || ttl <= 0 {
		return
	}
	if d.cacheCapacity <= 0 {
		return
	}

	expiresAt := time.Now().Add(ttl)
	cloned := cloneAnswers(answers)

	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()

	d.answerCache[key] = cachedAnswer{
		answers:   cloned,
		expiresAt: expiresAt,
	}
	if node, ok := d.answerCachePos[key]; ok {
		d.answerCacheList.MoveToFront(node)
	} else {
		d.answerCachePos[key] = d.answerCacheList.PushFront(key)
	}

	for len(d.answerCache) > d.cacheCapacity {
		node := d.answerCacheList.Back()
		if node == nil {
			break
		}
		cacheKey, _ := node.Value.(string)
		d.removeCacheEntryLocked(cacheKey)
	}
}

func (d *DNSServer) invalidateCache(domain string, qtype uint16) {
	key := queryCacheKey(domain, qtype)
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	d.removeCacheEntryLocked(key)
}

func (d *DNSServer) removeCacheEntryLocked(key string) {
	delete(d.answerCache, key)
	if node, ok := d.answerCachePos[key]; ok {
		d.answerCacheList.Remove(node)
		delete(d.answerCachePos, key)
	}
}

func minTTL(answers []dns.RR) time.Duration {
	minTTLSeconds := uint32(0)
	for _, answer := range answers {
		if answer == nil || answer.Header() == nil {
			continue
		}
		ttl := answer.Header().Ttl
		if ttl == 0 {
			continue
		}
		if minTTLSeconds == 0 || ttl < minTTLSeconds {
			minTTLSeconds = ttl
		}
	}
	if minTTLSeconds == 0 {
		return 10 * time.Second
	}
	return time.Duration(minTTLSeconds) * time.Second
}

func cloneAnswers(answers []dns.RR) []dns.RR {
	cloned := make([]dns.RR, 0, len(answers))
	for _, answer := range answers {
		if answer == nil {
			continue
		}
		cloned = append(cloned, dns.Copy(answer))
	}
	return cloned
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
