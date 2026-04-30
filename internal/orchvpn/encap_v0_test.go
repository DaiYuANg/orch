package orchvpn

import "testing"

func TestEncapV0_heartbeatRoundTrip(t *testing.T) {
	t.Parallel()
	p := EncodeEncapV0(EncapV0MsgHeartbeat, nil)
	typ, payload, err := DecodeEncapV0(p)
	if err != nil {
		t.Fatal(err)
	}
	if typ != EncapV0MsgHeartbeat {
		t.Fatalf("type %d", typ)
	}
	if len(payload) != 0 {
		t.Fatalf("payload %#v", payload)
	}
}

func TestEncapV0_payload(t *testing.T) {
	t.Parallel()
	in := []byte{1, 2, 3}
	p := EncodeEncapV0(EncapV0MsgIPv4Payload, in)
	typ, payload, err := DecodeEncapV0(p)
	if err != nil {
		t.Fatal(err)
	}
	if typ != EncapV0MsgIPv4Payload {
		t.Fatal(typ)
	}
	if string(payload) != string(in) {
		t.Fatalf("got %#v", payload)
	}
}

func TestEncapV0_rejectBadMagic(t *testing.T) {
	t.Parallel()
	p := EncodeEncapV0(EncapV0MsgHeartbeat, nil)
	p[0] = 'x'
	_, _, err := DecodeEncapV0(p)
	if err == nil {
		t.Fatal("expected error")
	}
}
