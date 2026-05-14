package dnssvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/arcgolabs/dnsx/dnsserver"
	"github.com/miekg/dns"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func dnsZoneName(cfg config.DNSConfig) string {
	return cfg.ZoneName()
}

func workloadRecordKey(namespace, workloadName string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = "default"
	}
	return strings.ToLower(ns) + "/" + strings.ToLower(strings.TrimSpace(workloadName))
}

// workloadServiceFQDN returns the relative owner name segments before normalization (zone is separate).
func workloadServiceFQDN(namespace, workloadName, zone string) string {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = "default"
	}
	base := strings.Trim(strings.ToLower(zone), ".")
	return fmt.Sprintf("%s.%s.svc.%s",
		strings.ToLower(strings.TrimSpace(workloadName)),
		strings.ToLower(ns),
		base,
	)
}

// UpsertWorkloadA publishes or replaces an A record for workload.<namespace>.svc.<zone>.
func (s *Service) UpsertWorkloadA(ctx context.Context, namespace, workloadName, ipv4 string) error {
	if !s.cfg.Enabled || s.store == nil {
		return nil
	}
	if strings.TrimSpace(ipv4) == "" {
		return oopsx.B("dns").Errorf("workload %q: empty ipv4", workloadName)
	}

	zone := dnsZoneName(s.cfg)
	rec := dnsserver.Record{
		Zone: zone,
		Name: workloadServiceFQDN(namespace, workloadName, zone),
		TTL:  60,
		Type: dns.TypeA,
		Data: strings.TrimSpace(ipv4),
	}
	norm, err := dnsserver.NormalizeRecord(rec)
	if err != nil {
		return oopsx.B("dns").Wrapf(err, "dns workload record")
	}

	key := workloadRecordKey(namespace, workloadName)
	prev, hadPrev := s.workloadRecords.Get(key)
	if err := s.store.SaveRecord(ctx, norm); err != nil {
		return oopsx.B("dns").Wrapf(err, "save workload record")
	}
	if hadPrev && prev.Key() != norm.Key() {
		if delErr := s.store.DeleteRecord(ctx, prev); delErr != nil {
			s.logger.Warn("delete stale dns workload record", "error", delErr)
		}
	}
	s.workloadRecords.Set(key, norm)
	s.logger.Debug("dns workload registered", "fqdn", norm.Name, "ip", ipv4)
	return nil
}

// RemoveWorkloadA deletes the A record previously registered for this workload (if any).
func (s *Service) RemoveWorkloadA(ctx context.Context, namespace, workloadName string) error {
	if !s.cfg.Enabled || s.store == nil {
		return nil
	}
	key := workloadRecordKey(namespace, workloadName)
	prev, ok := s.workloadRecords.Get(key)
	if !ok {
		return nil
	}
	if err := s.store.DeleteRecord(ctx, prev); err != nil {
		return oopsx.B("dns").Wrapf(err, "delete workload record")
	}
	s.workloadRecords.Delete(key)
	s.logger.Debug("dns workload deregistered", "workload", workloadName, "namespace", namespace)
	return nil
}

// LookupWorkloadIPv4 returns the last registered A record data for workload DNS (in-memory + store),
// using the same keying as UpsertWorkloadA.
func (s *Service) LookupWorkloadIPv4(namespace, workloadName string) (string, bool) {
	key := workloadRecordKey(namespace, workloadName)
	rec, ok := s.workloadRecords.Get(key)
	if !ok {
		return "", false
	}
	ip := strings.TrimSpace(rec.Data)
	if ip == "" {
		return "", false
	}
	return ip, true
}
