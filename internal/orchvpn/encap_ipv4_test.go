package orchvpn_test

import (
	"testing"

	"github.com/daiyuang/orch/internal/orchvpn"
)

func TestSummarizeEncapIPv4Payload(t *testing.T) {
	t.Parallel()
	// minimal IPv4 header 20 bytes, total length 20, protocol 1 (ICMP), src 10.1.0.1 dst 10.2.0.2
	pkt := []byte{
		0x45, 0x00, 0x00, 0x14,
		0x00, 0x00, 0x40, 0x00,
		0x40, 0x01, 0x00, 0x00,
		10, 1, 0, 1,
		10, 2, 0, 2,
	}
	src, dst, ok := orchvpn.SummarizeEncapIPv4Payload(pkt)
	if !ok || src != "10.1.0.1" || dst != "10.2.0.2" {
		t.Fatalf("got %s %s %v", src, dst, ok)
	}
}
