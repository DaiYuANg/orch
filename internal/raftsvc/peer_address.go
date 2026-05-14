package raftsvc

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"strings"

	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func replicaIDForNodeID(id string) (uint64, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, oopsx.B("raft").Errorf("raft node id is required")
	}
	h := fnv.New64a()
	if _, err := io.WriteString(h, id); err != nil {
		return 0, oopsx.B("raft").Wrapf(err, "hash raft node id")
	}
	replicaID := h.Sum64()
	if replicaID == 0 {
		return 1, nil
	}
	return replicaID, nil
}

func validateRaftPeerAddress(label, raw string) (string, error) {
	addr := strings.TrimSpace(raw)
	if addr == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("%s must be host:port: %w", label, err)
	}
	if strings.TrimSpace(host) == "" {
		return "", fmt.Errorf("%s must include a host", label)
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("%s must include a port", label)
	}
	return addr, nil
}

func validateConcreteRaftAddress(label, raw string) (string, error) {
	addr, err := validateRaftPeerAddress(label, raw)
	if err != nil {
		return "", oopsx.B("raft").Wrapf(err, "validate raft peer address")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", oopsx.B("raft").Wrapf(err, "parse raft address")
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip != nil && ip.IsUnspecified() {
		return "", fmt.Errorf("%s must be a concrete host:port, got %q", label, raw)
	}
	return addr, nil
}

func resolveEphemeralTCPAddr(ctx context.Context, raw string) (string, error) {
	port := ""
	if _, splitPort, splitErr := net.SplitHostPort(strings.TrimSpace(raw)); splitErr == nil {
		port = splitPort
	}
	if port != "0" {
		return raw, nil
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", raw)
	if err != nil {
		return "", oopsx.B("raft").Wrapf(err, "listen ephemeral raft address")
	}
	addr := ln.Addr().String()
	if closeErr := ln.Close(); closeErr != nil {
		return "", oopsx.B("raft").Wrapf(closeErr, "close ephemeral raft listener")
	}
	return addr, nil
}
