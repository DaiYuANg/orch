package orchvpn_test

import (
	"bytes"
	"testing"

	"github.com/daiyuang/orch/internal/orchvpn"
)

func TestEncapV0_heartbeatRoundTrip(t *testing.T) {
	t.Parallel()
	p := orchvpn.EncodeEncapV0(orchvpn.EncapV0MsgHeartbeat, nil)
	typ, payload, err := orchvpn.DecodeEncapV0(p)
	if err != nil {
		t.Fatal(err)
	}
	if typ != orchvpn.EncapV0MsgHeartbeat {
		t.Fatalf("type %d", typ)
	}
	if len(payload) != 0 {
		t.Fatalf("payload %#v", payload)
	}
}

func TestEncapV0_payload(t *testing.T) {
	t.Parallel()
	in := []byte{1, 2, 3}
	p := orchvpn.EncodeEncapV0(orchvpn.EncapV0MsgIPv4Payload, in)
	typ, payload, err := orchvpn.DecodeEncapV0(p)
	if err != nil {
		t.Fatal(err)
	}
	if typ != orchvpn.EncapV0MsgIPv4Payload {
		t.Fatal(typ)
	}
	if !bytes.Equal(payload, in) {
		t.Fatalf("got %#v", payload)
	}
}

func TestEncapV0_rejectBadMagic(t *testing.T) {
	t.Parallel()
	p := orchvpn.EncodeEncapV0(orchvpn.EncapV0MsgHeartbeat, nil)
	p[0] = 'x'
	_, _, err := orchvpn.DecodeEncapV0(p)
	if err == nil {
		t.Fatal("expected error")
	}
}
