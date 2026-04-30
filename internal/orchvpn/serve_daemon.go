package orchvpn

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// ServerDaemonService is the standalone `orch-vpn serve` process: UDP encap-v0 listener for local dev
// (same frame handling as orch-server [GatewayService], without the control plane).
type ServerDaemonService struct {
	cfg ServerConfig
	log *slog.Logger
}

func NewServerDaemonService(cfg ServerConfig, log *slog.Logger) *ServerDaemonService {
	if log == nil {
		log = slog.Default()
	}
	return &ServerDaemonService{cfg: cfg, log: log.With(slog.String("component", "orchvpn-serve"))}
}

type serveEncapObserver struct {
	log *slog.Logger
}

func (o *serveEncapObserver) InvalidFrame(remote net.Addr, err error, n int) {
	o.log.Debug("udp discard", "from", remote.String(), "bytes", n, "error", err)
}

func (o *serveEncapObserver) Heartbeat(remote net.Addr) []byte {
	o.log.Info("encap-v0 heartbeat", "from", remote.String())
	return EncodeEncapV0(EncapV0MsgHeartbeatACK, nil)
}

func (o *serveEncapObserver) HeartbeatACK(remote net.Addr) {
	o.log.Debug("encap-v0 heartbeat ack", "from", remote.String())
}

func (o *serveEncapObserver) IPv4Inner(remote net.Addr, src, dst string, inner []byte) {
	o.log.Info("encap-v0 ipv4 observe (no kernel forward)",
		"src", src, "dst", dst, "inner_bytes", len(inner), "from", remote.String())
}

func (o *serveEncapObserver) UnknownMessage(remote net.Addr, typ byte, payload []byte) {
	o.log.Debug("encap-v0 packet", "msg_type", typ, "payload_bytes", len(payload), "from", remote.String())
}

func (o *serveEncapObserver) AckWriteFailed(remote net.Addr, err error) {
	o.log.Debug("heartbeat ack write failed", "to", remote.String(), "error", err)
}

// Run listens on UDP until ctx is cancelled.
func (s *ServerDaemonService) Run(ctx context.Context) error {
	addr := s.cfg.ListenUDPOrDefault()
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return oopsx.B("orchvpn").Wrapf(err, "listen udp %s", addr)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = pc.Close() }()
		obs := &serveEncapObserver{log: s.log}
		buf := make([]byte, 65535)
		for {
			if ctx.Err() != nil {
				return
			}
			_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, remote, rerr := pc.ReadFrom(buf)
			if rerr != nil {
				if ne, ok := rerr.(net.Error); ok && ne.Timeout() {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				s.log.Warn("serve udp read error", "error", rerr)
				continue
			}
			if n > 0 {
				HandleEncapUDP(pc, remote, buf[:n], obs)
			}
		}
	}()

	s.log.Info("orch-vpn serve listening", "udp", pc.LocalAddr().String(), "encap", "orch-vpn/encap-v0")
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}
