package orchvpn

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
)

// orch-vpn/encap-v0 wire layout (big-endian):
//
//	[0:4]  magic "ORC0"
//	[4]    version (0)
//	[5]    msg type
//	[6:8]  reserved (0)
//	[8:12] payload length (bytes)
//	[12:]  payload
const encapV0HeaderLen = 12

var encapV0Magic = [4]byte{'O', 'R', 'C', '0'}

// Encap V0 message types (orch-vpn/encap-v0).
const (
	EncapV0MsgHeartbeat    byte = 0
	EncapV0MsgIPv4Payload  byte = 1 // reserved for TUN IPv4-in-UDP
	EncapV0MsgHeartbeatACK byte = 2
)

// EncodeEncapV0 builds a v0 frame with the given message type and payload.
func EncodeEncapV0(msgType byte, payload []byte) []byte {
	out := make([]byte, encapV0HeaderLen+len(payload))
	copy(out[0:4], encapV0Magic[:])
	out[4] = 0
	out[5] = msgType
	out[6], out[7] = 0, 0
	binary.BigEndian.PutUint32(out[8:12], payloadLenUint32(len(payload)))
	copy(out[encapV0HeaderLen:], payload)
	return out
}

func payloadLenUint32(n int) uint32 {
	if n < 0 || uint64(n) > math.MaxUint32 {
		panic("encap-v0: payload too large")
	}
	v, err := strconv.ParseUint(strconv.Itoa(n), 10, 32)
	if err != nil {
		panic(fmt.Sprintf("encap-v0: payload length invalid: %v", err))
	}
	return uint32(v)
}

// DecodeEncapV0 validates magic/version and returns msg type and payload.
func DecodeEncapV0(b []byte) (msgType byte, payload []byte, err error) {
	if len(b) < encapV0HeaderLen {
		return 0, nil, errors.New("encap-v0: packet too short")
	}
	if !bytes.Equal(b[0:4], encapV0Magic[:]) {
		return 0, nil, errors.New("encap-v0: bad magic")
	}
	if b[4] != 0 {
		return 0, nil, fmt.Errorf("encap-v0: unsupported version %d", b[4])
	}
	msgType = b[5]
	plen := binary.BigEndian.Uint32(b[8:12])
	if uint64(encapV0HeaderLen)+uint64(plen) > uint64(len(b)) {
		return 0, nil, errors.New("encap-v0: payload length out of range")
	}
	payload = b[encapV0HeaderLen : encapV0HeaderLen+int(plen)]
	return msgType, payload, nil
}
