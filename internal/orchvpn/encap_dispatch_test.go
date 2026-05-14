package orchvpn_test

import (
	"net"
	"testing"

	"github.com/lyonbrown4d/orch/internal/orchvpn"
)

type testEncapObs struct {
	heartbeat    int
	heartbeatACK int
	invalid      int
	ipv4         int
	unknown      int
	ackFail      int
}

func (o *testEncapObs) InvalidFrame(net.Addr, error, int) { o.invalid++ }
func (o *testEncapObs) Heartbeat(net.Addr) []byte {
	o.heartbeat++
	return orchvpn.EncodeEncapV0(orchvpn.EncapV0MsgHeartbeatACK, nil)
}
func (o *testEncapObs) HeartbeatACK(net.Addr)                      { o.heartbeatACK++ }
func (o *testEncapObs) IPv4Inner(net.Addr, string, string, []byte) { o.ipv4++ }
func (o *testEncapObs) UnknownMessage(net.Addr, byte, []byte)      { o.unknown++ }
func (o *testEncapObs) AckWriteFailed(net.Addr, error)             { o.ackFail++ }

func TestHandleEncapUDPHeartbeat(t *testing.T) {
	var o testEncapObs
	h := orchvpn.EncodeEncapV0(orchvpn.EncapV0MsgHeartbeat, nil)
	orchvpn.HandleEncapUDP(nil, &net.UDPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 9}, h, &o)
	if o.heartbeat != 1 || o.invalid != 0 || o.heartbeatACK != 0 {
		t.Fatalf("obs counts heartbeat=%d invalid=%d ack=%d", o.heartbeat, o.invalid, o.heartbeatACK)
	}
}

func TestHandleEncapUDPACK(t *testing.T) {
	var o testEncapObs
	h := orchvpn.EncodeEncapV0(orchvpn.EncapV0MsgHeartbeatACK, nil)
	orchvpn.HandleEncapUDP(nil, &net.UDPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 9}, h, &o)
	if o.heartbeatACK != 1 || o.heartbeat != 0 {
		t.Fatalf("obs counts ack=%d heartbeat=%d", o.heartbeatACK, o.heartbeat)
	}
}

func TestHandleEncapUDPInvalidMagic(t *testing.T) {
	var o testEncapObs
	orchvpn.HandleEncapUDP(nil, &net.UDPAddr{}, []byte("XXXX"), &o)
	if o.invalid != 1 || o.heartbeat != 0 {
		t.Fatalf("expected invalid frame, got invalid=%d heartbeat=%d", o.invalid, o.heartbeat)
	}
}
