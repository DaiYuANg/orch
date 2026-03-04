package dns

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDNSDataHome(t *testing.T) {
	t.Helper()
	old := xdg.DataHome
	xdg.DataHome = t.TempDir()
	t.Cleanup(func() {
		xdg.DataHome = old
	})
}

func newTestDNSServer(t *testing.T) *DNSServer {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server, err := NewDNSServer(logger, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = server.Shutdown()
	})
	return server
}

func ipv4FromAnswer(t *testing.T, answer dns.RR) string {
	t.Helper()
	record, ok := answer.(*dns.A)
	require.True(t, ok)
	return record.A.String()
}

func TestRecordPersistsAcrossServerRestart(t *testing.T) {
	setupDNSDataHome(t)
	first := newTestDNSServer(t)

	domain := "api.warden.local."
	first.SetRecord(domain, "127.0.0.1")

	answers := first.resolveA(domain)
	require.Len(t, answers, 1)
	assert.Equal(t, "127.0.0.1", ipv4FromAnswer(t, answers[0]))

	require.NoError(t, first.Shutdown())

	second := newTestDNSServer(t)
	recovered := second.resolveA(domain)
	require.Len(t, recovered, 1)
	assert.Equal(t, "127.0.0.1", ipv4FromAnswer(t, recovered[0]))
}

func TestResolveAUsesHotCacheBeforeTTLExpiry(t *testing.T) {
	setupDNSDataHome(t)
	server := newTestDNSServer(t)

	domain := "cache.warden.local."
	now := time.Now()
	require.NoError(t, server.persistRecord(Record{
		Domain:     domain,
		Type:       "A",
		Value:      "127.0.0.10",
		TTLSeconds: 1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))
	server.invalidateCache(domain, dns.TypeA)

	firstLookup := server.resolveA(domain)
	require.Len(t, firstLookup, 1)
	assert.Equal(t, "127.0.0.10", ipv4FromAnswer(t, firstLookup[0]))

	require.NoError(t, server.persistRecord(Record{
		Domain:     domain,
		Type:       "A",
		Value:      "127.0.0.20",
		TTLSeconds: 1,
		CreatedAt:  now,
		UpdatedAt:  now.Add(time.Second),
	}))

	cachedLookup := server.resolveA(domain)
	require.Len(t, cachedLookup, 1)
	assert.Equal(t, "127.0.0.10", ipv4FromAnswer(t, cachedLookup[0]))

	time.Sleep(1200 * time.Millisecond)

	refreshedLookup := server.resolveA(domain)
	require.Len(t, refreshedLookup, 1)
	assert.Equal(t, "127.0.0.20", ipv4FromAnswer(t, refreshedLookup[0]))
}
