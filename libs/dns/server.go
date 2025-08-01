package dns

import (
	"github.com/dgraph-io/ristretto"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	ristretto_store "github.com/eko/gocache/store/ristretto/v4"
	"github.com/miekg/dns"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"sync"
)

// DNS服务器结构体，存储域名和IP的映射
type DNSServer struct {
	records map[string]string
	cm      *cache.Cache[string]
	mu      sync.RWMutex
	db      *bbolt.DB
	logger  *zap.SugaredLogger
}

func NewDNSServer(logger *zap.SugaredLogger) *DNSServer {
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000,
		MaxCost:     100,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	ristrettoStore := ristretto_store.NewRistretto(ristrettoCache)

	cacheManager := cache.New[string](ristrettoStore)
	err = cacheManager.Set(context.Background(), "my-key", "my-value", store.WithCost(2))
	if err != nil {
		panic(err)
	}
	return &DNSServer{
		records: make(map[string]string),
		logger:  logger,
		cm:      cacheManager,
	}
}

// 添加或修改解析记录
func (d *DNSServer) SetRecord(domain, ip string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.records[domain] = ip
}

// 删除解析记录
func (d *DNSServer) DeleteRecord(domain string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.records, domain)
}

// DNS请求处理函数
func (d *DNSServer) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
		switch q.Qtype {
		case dns.TypeA:
			domain := q.Name
			d.mu.RLock()
			ip, ok := d.records[domain]
			d.mu.RUnlock()
			if ok {
				rr, err := dns.NewRR(domain + " A " + ip)
				if err == nil {
					msg.Answer = append(msg.Answer, rr)
				}
			}
		}
	}

	w.WriteMsg(&msg)
}

// 封装 UDP 和 TCP 监听，统一启动服务
func (d *DNSServer) Serve(addr string) error {
	// 注册处理函数
	dns.HandleFunc(".", d.ServeDNS)

	udpServer := &dns.Server{Addr: addr, Net: "udp"}
	tcpServer := &dns.Server{Addr: addr, Net: "tcp"}

	// 并发启动 UDP 和 TCP 服务器
	go func() {
		if err := udpServer.ListenAndServe(); err != nil {
			d.logger.Fatalf("Failed to start UDP server: %v", err)
		}
	}()

	go func() {
		if err := tcpServer.ListenAndServe(); err != nil {
			d.logger.Fatalf("Failed to start TCP server: %v", err)
		}
	}()

	select {}
}
