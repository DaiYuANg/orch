package orchvpn

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// GatewayService listens for orch-vpn UDP encapsulation on the orchestrator host. When orch_vpn.enabled
// is false, Start is a no-op.
type GatewayService struct {
	cfg    config.OrchVPNConfig
	logger *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewGatewayService is constructed via dix from [config.OrchVPNConfig] + [*slog.Logger].
func NewGatewayService(vpn config.OrchVPNConfig, logger *slog.Logger) *GatewayService {
	return &GatewayService{
		cfg:    vpn,
		logger: logger,
	}
}

func (g *GatewayService) Start(parent context.Context) error {
	if !g.cfg.Enabled {
		g.logger.Debug("orch-vpn gateway disabled (orch_vpn.enabled=false)")
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	g.cancel = cancel
	addr := g.cfg.ListenUDPOrDefault()
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		cancel()
		g.cancel = nil
		return oopsx.B("orchvpn").Wrapf(err, "listen udp %s", addr)
	}
	log := g.logger.With(slog.String("component", "orchvpn-gateway"))
	log.Info("orch-vpn gateway listening", "udp", pc.LocalAddr().String(), "encap", "orch-vpn/encap-v0")

	g.wg.Add(1)
	go g.readLoop(ctx, log, pc)
	return nil
}

func (g *GatewayService) readLoop(ctx context.Context, log *slog.Logger, pc net.PacketConn) {
	defer g.wg.Done()
	defer func() {
		if err := pc.Close(); err != nil {
			log.Debug("orch-vpn udp close", "error", err)
		}
	}()

	obs := &slogEncapObserver{log: log}
	buf := make([]byte, 65535)
	for {
		if ctx.Err() != nil {
			return
		}
		_ = pc.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, remote, err := pc.ReadFrom(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			log.Warn("orch-vpn udp read", "error", err)
			continue
		}
		if n > 0 {
			HandleEncapUDP(pc, remote, buf[:n], obs)
		}
	}
}

type slogEncapObserver struct {
	log *slog.Logger
}

func (o *slogEncapObserver) InvalidFrame(remote net.Addr, err error, n int) {
	o.log.Debug("orch-vpn udp discard", "bytes", n, "from", remote.String(), "error", err)
}

func (o *slogEncapObserver) Heartbeat(remote net.Addr) []byte {
	o.log.Info("orch-vpn encap-v0 heartbeat", "from", remote.String())
	return EncodeEncapV0(EncapV0MsgHeartbeatACK, nil)
}

func (o *slogEncapObserver) HeartbeatACK(remote net.Addr) {
	o.log.Debug("orch-vpn encap-v0 heartbeat ack", "from", remote.String())
}

func (o *slogEncapObserver) IPv4Inner(remote net.Addr, src, dst string, inner []byte) {
	o.log.Info("orch-vpn encap-v0 ipv4 (observe only; no kernel forward yet)",
		"src", src, "dst", dst, "inner_bytes", len(inner), "from", remote.String())
}

func (o *slogEncapObserver) UnknownMessage(remote net.Addr, typ byte, payload []byte) {
	o.log.Debug("orch-vpn encap-v0 packet", "msg_type", typ, "payload_bytes", len(payload), "from", remote.String())
}

func (o *slogEncapObserver) AckWriteFailed(remote net.Addr, err error) {
	o.log.Debug("orch-vpn heartbeat ack write", "error", err)
}

func (g *GatewayService) Stop(_ context.Context) error {
	g.mu.Lock()
	cancel := g.cancel
	g.cancel = nil
	g.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	g.wg.Wait()
	return nil
}
