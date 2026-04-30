package orchvpn

import "net"

// SummarizeEncapIPv4Payload returns inner IPv4 src/dst if b looks like an IPv4 packet (minimal header check).
func SummarizeEncapIPv4Payload(b []byte) (src, dst string, ok bool) {
	if len(b) < 20 {
		return "", "", false
	}
	if b[0]>>4 != 4 {
		return "", "", false
	}
	ihl := int(b[0]&0x0f) * 4
	if ihl < 20 || ihl > len(b) {
		return "", "", false
	}
	return net.IP(b[12:16]).String(), net.IP(b[16:20]).String(), true
}
