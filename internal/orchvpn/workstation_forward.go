package orchvpn

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/daiyuang/orch/internal/api"
	orchtun "github.com/daiyuang/orch/internal/orchvpn/tun"
)

func forwardSignature(peer string, boot *api.OrchVPNBootstrapOutput, tunName string) string {
	mtu := 0
	if boot != nil {
		mtu = boot.Body.MTU
	}
	return peer + "|" + strconv.Itoa(mtu) + "|" + tunName
}

func (d *WorkstationDaemon) stopTunnelDataPlane() {
	d.fwdMu.Lock()
	cancel := d.tunCancel
	d.tunCancel = nil
	d.fwdKey = ""
	d.tunEpoch.Add(1)
	d.fwdMu.Unlock()
	if cancel != nil {
		cancel()
		d.tunWG.Wait()
	}
}

func (d *WorkstationDaemon) maybeRestartTunnelForward(parent context.Context, peer string, boot *api.OrchVPNBootstrapOutput) {
	if !d.cfg.EnableTUN || boot == nil {
		d.stopTunnelDataPlane()
		return
	}
	key := forwardSignature(peer, boot, d.cfg.TUNName)
	d.fwdMu.Lock()
	if d.fwdKey == key && d.tunCancel != nil {
		d.fwdMu.Unlock()
		return
	}
	if d.tunCancel != nil {
		d.tunEpoch.Add(1)
		old := d.tunCancel
		d.tunCancel = nil
		d.fwdKey = ""
		d.fwdMu.Unlock()
		old()
		d.tunWG.Wait()
		d.fwdMu.Lock()
	}
	fctx, cancel := context.WithCancel(parent)
	d.tunCancel = cancel
	d.fwdKey = key
	epoch := d.tunEpoch.Add(1)
	d.tunWG.Add(1)
	d.fwdMu.Unlock()

	go func(e uint64) {
		defer d.tunWG.Done()
		defer func() {
			if d.tunEpoch.Load() == e {
				d.fwdMu.Lock()
				d.tunCancel = nil
				d.fwdKey = ""
				d.fwdMu.Unlock()
			}
		}()
		d.runTunnelForward(fctx, peer, boot)
	}(epoch)
}

func (d *WorkstationDaemon) runTunnelForward(ctx context.Context, peer string, boot *api.OrchVPNBootstrapOutput) {
	log := d.log.With(slog.String("subcomponent", "tun-forward"))
	mtu := boot.Body.MTU
	dev, err := orchtun.New(orchtun.Config{Name: d.cfg.TUNName, MTU: mtu})
	if err != nil {
		if errors.Is(err, orchtun.ErrUnsupported) {
			log.Warn("TUN not available on this GOOS")
		} else {
			log.Warn("orch-vpn TUN error", "error", err)
		}
		return
	}

	conn, err := net.Dial("udp", peer)
	if err != nil {
		_ = dev.Close()
		log.Warn("tunnel UDP dial failed", "peer", peer, "error", err)
		return
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
		_ = dev.Close()
	}()

	log.Info("TUN up", "if_name", dev.InterfaceName(), "mtu", dev.MTU())
	printRouteHints(log, dev.InterfaceName(), boot.Body.ContainerRoutes)

	buf := make([]byte, 65535)
	packet := make([]byte, 65535)
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			n, err := dev.ReadPacket(packet)
			if err != nil {
				return
			}
			if n == 0 {
				continue
			}
			if n >= 1 && (packet[0]>>4) != 4 {
				continue
			}
			frame := EncodeEncapV0(EncapV0MsgIPv4Payload, packet[:n])
			if _, err := conn.Write(frame); err != nil {
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			typ, payload, decErr := DecodeEncapV0(buf[:n])
			if decErr != nil || typ != EncapV0MsgIPv4Payload {
				continue
			}
			if len(payload) > 0 && (payload[0]>>4) == 4 {
				if _, werr := dev.WritePacket(payload); werr != nil {
					log.Debug("tunnel tun write", "error", werr)
				}
			}
		}
	}()

	wg.Wait()
}
