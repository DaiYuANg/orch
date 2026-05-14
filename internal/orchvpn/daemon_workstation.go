package orchvpn

import (
	"context"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// WorkstationDaemon is the long-running workstation side (health + bootstrap; optional TUN data plane).
// Constructed via dix: [ClientConfig], [*apiclient.Client], [*slog.Logger]. Logs with the injected logger.
type WorkstationDaemon struct {
	cfg    ClientConfig
	client *apiclient.Client
	log    *slog.Logger

	lastTunnelOK   bool
	lastTunnelPeer string

	fwdMu     sync.Mutex
	tunCancel context.CancelFunc
	tunWG     sync.WaitGroup
	fwdKey    string
	tunEpoch  atomic.Uint64
}

func NewWorkstationDaemon(cfg ClientConfig, hc *apiclient.Client, log *slog.Logger) *WorkstationDaemon {
	if log == nil {
		log = slog.Default()
	}
	return &WorkstationDaemon{cfg: cfg, client: hc, log: log.With(slog.String("component", "orchvpn-workstation"))}
}

// Run blocks until ctx is canceled.
func (d *WorkstationDaemon) Run(ctx context.Context) error {
	tick := time.NewTicker(time.Duration(d.cfg.HealthPeriodSec) * time.Second)
	defer tick.Stop()

	d.log.Info("orch-vpn client running", "control_plane", d.cfg.ControlPlaneURL)
	d.log.Debug("poll loop: UDP encap-v0 heartbeats when server has orch_vpn.enabled and tunnel peer is known")

	for {
		if err := d.pollControlPlane(ctx); err != nil {
			return err
		}
		if !waitWorkstationTick(ctx, tick) {
			return oopsx.B("orchvpn").Wrapf(ctx.Err(), "workstation context")
		}
	}
}

func (d *WorkstationDaemon) pollControlPlane(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return oopsx.B("orchvpn").Wrapf(err, "workstation context")
	}
	if err := d.checkControlPlaneHealth(ctx); err != nil {
		if ctx.Err() != nil {
			return oopsx.B("orchvpn").Wrapf(ctx.Err(), "workstation context")
		}
		d.log.Warn("control plane health check failed", "error", err)
		d.stopTunnelDataPlane()
		return nil
	}
	return d.refreshTunnelBootstrap(ctx)
}

func (d *WorkstationDaemon) checkControlPlaneHealth(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err := d.client.Health(hctx)
	if err != nil {
		return oopsx.B("orchvpn").Wrapf(err, "check control plane health")
	}
	return nil
}

func (d *WorkstationDaemon) refreshTunnelBootstrap(ctx context.Context) error {
	bctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	boot, err := d.client.OrchVPNBootstrap(bctx)
	if err != nil {
		if ctx.Err() != nil {
			return oopsx.B("orchvpn").Wrapf(ctx.Err(), "workstation context")
		}
		d.handleBootstrapError(err)
		return nil
	}
	d.handleBootstrap(ctx, boot)
	return nil
}

func (d *WorkstationDaemon) handleBootstrapError(err error) {
	d.stopTunnelDataPlane()
	if d.lastTunnelOK {
		d.log.Warn("orch-vpn bootstrap failed (tunnel down)", "error", err)
	} else {
		d.log.Warn("orch-vpn bootstrap failed", "error", err)
	}
	d.lastTunnelOK = false
	d.lastTunnelPeer = ""
}

func (d *WorkstationDaemon) handleBootstrap(ctx context.Context, boot *api.OrchVPNBootstrapOutput) {
	d.logBootstrap(boot)
	peer, ok := tunnelPeerFromBootstrap(d.cfg.ControlPlaneURL, boot)
	if ok {
		d.pulseTunnel(ctx, peer, boot)
		return
	}
	d.stopTunnelDataPlane()
	if d.lastTunnelOK {
		d.log.Info("tunnel stopped: orch-vpn disabled or no peer in bootstrap")
	}
	d.lastTunnelOK = false
	d.lastTunnelPeer = ""
}

func waitWorkstationTick(ctx context.Context, tick *time.Ticker) bool {
	select {
	case <-ctx.Done():
		return false
	case <-tick.C:
		return true
	}
}

func (d *WorkstationDaemon) pulseTunnel(ctx context.Context, peer string, boot *api.OrchVPNBootstrapOutput) {
	ok := d.sendEncapHeartbeat(ctx, peer)
	if ok {
		if !d.lastTunnelOK || d.lastTunnelPeer != peer {
			d.log.Info("tunnel heartbeat ok", "peer", peer)
		}
		d.lastTunnelOK = true
		d.lastTunnelPeer = peer
		d.maybeRestartTunnelForward(ctx, peer, boot)
		return
	}
	d.stopTunnelDataPlane()
	if d.lastTunnelOK {
		d.log.Warn("tunnel heartbeat lost", "peer", peer)
	}
	d.lastTunnelOK = false
	d.lastTunnelPeer = peer
}

func tunnelPeerFromBootstrap(controlPlaneURL string, boot *api.OrchVPNBootstrapOutput) (string, bool) {
	if boot == nil || !boot.Body.Enabled || boot.Body.TunnelUDPPort <= 0 {
		return "", false
	}
	u, err := url.Parse(controlPlaneURL)
	if err != nil {
		return "", false
	}
	host := u.Hostname()
	if host == "" {
		return "", false
	}
	return net.JoinHostPort(host, strconv.Itoa(boot.Body.TunnelUDPPort)), true
}

func (d *WorkstationDaemon) sendEncapHeartbeat(parent context.Context, peer string) bool {
	dctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	var dialer net.Dialer
	c, err := dialer.DialContext(dctx, "udp", peer)
	if err != nil {
		d.log.Debug("tunnel udp dial", "peer", peer, "error", err)
		return false
	}
	defer func() {
		if closeErr := c.Close(); closeErr != nil {
			d.log.Debug("tunnel udp close", "peer", peer, "error", closeErr)
		}
	}()

	pkt := EncodeEncapV0(EncapV0MsgHeartbeat, nil)
	if _, writeErr := c.Write(pkt); writeErr != nil {
		d.log.Debug("tunnel heartbeat write", "error", writeErr)
		return false
	}
	if deadlineErr := c.SetReadDeadline(time.Now().Add(2 * time.Second)); deadlineErr != nil {
		d.log.Debug("tunnel heartbeat deadline", "error", deadlineErr)
		return false
	}
	ackBuf := make([]byte, 2048)
	n, err := c.Read(ackBuf)
	if err != nil {
		d.log.Debug("tunnel heartbeat read", "error", err)
		return false
	}
	typ, _, decErr := DecodeEncapV0(ackBuf[:n])
	if decErr != nil || typ != EncapV0MsgHeartbeatACK {
		d.log.Debug("tunnel heartbeat ack invalid", "type", typ, "error", decErr)
		return false
	}
	return true
}

func (d *WorkstationDaemon) logBootstrap(boot *api.OrchVPNBootstrapOutput) {
	if boot == nil {
		return
	}
	peer := ""
	u, err := url.Parse(d.cfg.ControlPlaneURL)
	if err == nil {
		host := u.Hostname()
		if host != "" && boot.Body.TunnelUDPPort > 0 {
			peer = net.JoinHostPort(host, strconv.Itoa(boot.Body.TunnelUDPPort))
		}
	}
	d.log.Info("orch-vpn bootstrap",
		"enabled", boot.Body.Enabled,
		"api_version", boot.Body.APIVersion,
		"encap", boot.Body.Encap,
		"mtu", boot.Body.MTU,
		"tunnel_udp_port", boot.Body.TunnelUDPPort,
		"tunnel_udp_peer", peer,
		"dns_zone", boot.Body.DNSZone,
		"container_routes", boot.Body.ContainerRoutes.Len(),
	)
}
