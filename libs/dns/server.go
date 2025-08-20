package dns

import (
	"sync"

	"github.com/DaiYuANg/warden/libs/metadata_db"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/miekg/dns"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// DNS服务器结构体，存储域名和IP的映射
type DNSServer struct {
	mu      sync.RWMutex
	records map[string]Record
	repo    *metadata_db.Repository[Record]
	cm      *cache.Cache[string]
	logger  *zap.SugaredLogger
}

func NewDNSServer(logger *zap.SugaredLogger) (*DNSServer, error) {
	db, err := bbolt.Open("dns.db", 0700, nil)
	if err != nil {
		return nil, err
	}
	repo := metadata_db.NewRepository[Record](db, "dns_records")
	svr := &DNSServer{
		records: map[string]Record{},
		repo:    repo,
		logger:  logger,
	}

	// 启动时加载已存储记录
	_ = repo.ForEach(func(key string, rec Record) error {
		svr.records[rec.Domain] = rec
		return nil
	})

	return svr, nil
}

// 添加或修改解析记录
func (d *DNSServer) SetRecord(domain, ip string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.records[domain] = Record{}
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
			d.mu.RLock()
			ip, ok := d.records[q.Name]
			d.mu.RUnlock()
			if ok {
				rr, err := dns.NewRR(ip.Domain + " A " + ip.Value)
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
