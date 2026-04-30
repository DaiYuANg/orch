package orchvpn

import (
	"errors"
	"net"
)

// ErrEncapInvalidIPv4Inner is returned when an IPv4 inner frame fails minimal header checks.
var ErrEncapInvalidIPv4Inner = errors.New("encap-v0: invalid ipv4 inner")

// EncapObserver receives decoded encap-v0 frames from a UDP datagram.
type EncapObserver interface {
	InvalidFrame(remote net.Addr, err error, packetBytes int)
	Heartbeat(remote net.Addr) (ack []byte)
	HeartbeatACK(remote net.Addr)
	IPv4Inner(remote net.Addr, src, dst string, inner []byte)
	UnknownMessage(remote net.Addr, msgType byte, payload []byte)
	AckWriteFailed(remote net.Addr, err error)
}

// HandleEncapUDP decodes one datagram and dispatches it. When Heartbeat returns a non-empty ack,
// it is written via pc (best effort); write errors are reported with AckWriteFailed.
func HandleEncapUDP(pc net.PacketConn, remote net.Addr, packet []byte, obs EncapObserver) {
	typ, payload, err := DecodeEncapV0(packet)
	if err != nil {
		obs.InvalidFrame(remote, err, len(packet))
		return
	}
	switch typ {
	case EncapV0MsgHeartbeat:
		if ack := obs.Heartbeat(remote); len(ack) > 0 && pc != nil {
			if _, werr := pc.WriteTo(ack, remote); werr != nil {
				obs.AckWriteFailed(remote, werr)
			}
		}
	case EncapV0MsgHeartbeatACK:
		obs.HeartbeatACK(remote)
	case EncapV0MsgIPv4Payload:
		src, dst, ok := SummarizeEncapIPv4Payload(payload)
		if !ok {
			obs.InvalidFrame(remote, ErrEncapInvalidIPv4Inner, len(packet))
			return
		}
		obs.IPv4Inner(remote, src, dst, payload)
	default:
		obs.UnknownMessage(remote, typ, payload)
	}
}
