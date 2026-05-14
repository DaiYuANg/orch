package orchvpn

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"sync"

	"github.com/lyonbrown4d/orch/internal/api"
	orchtun "github.com/lyonbrown4d/orch/internal/orchvpn/tun"
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
	dev, ok := openTunnelDevice(log, d.cfg.TUNName, boot.Body.MTU)
	if !ok {
		return
	}

	conn, ok := dialTunnelPeer(ctx, log, peer)
	if !ok {
		closeTUNDevice(log, dev)
		return
	}
	startTunnelCleanup(ctx, log, dev, conn)

	log.Info("TUN up", "if_name", dev.InterfaceName(), "mtu", dev.MTU())
	printRouteHints(log, dev.InterfaceName(), boot.Body.ContainerRoutes)
	runTunnelPumps(log, dev, conn)
}

func openTunnelDevice(log *slog.Logger, name string, mtu int) (orchtun.Device, bool) {
	dev, err := orchtun.New(orchtun.Config{Name: name, MTU: mtu})
	if err == nil {
		return dev, true
	}
	if errors.Is(err, orchtun.ErrUnsupported) {
		log.Warn("TUN not available on this GOOS")
	} else {
		log.Warn("orch-vpn TUN error", "error", err)
	}
	return nil, false
}

func dialTunnelPeer(ctx context.Context, log *slog.Logger, peer string) (net.Conn, bool) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp", peer)
	if err != nil {
		log.Warn("tunnel UDP dial failed", "peer", peer, "error", err)
		return nil, false
	}
	return conn, true
}

func startTunnelCleanup(ctx context.Context, log *slog.Logger, dev orchtun.Device, conn net.Conn) {
	go func() {
		<-ctx.Done()
		if err := conn.Close(); err != nil {
			log.Debug("tunnel UDP close", "error", err)
		}
		closeTUNDevice(log, dev)
	}()
}

func runTunnelPumps(log *slog.Logger, dev orchtun.Device, conn net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go pumpTUNToUDP(dev, conn, &wg)
	go pumpUDPToTUN(log, dev, conn, &wg)
	wg.Wait()
}

func pumpTUNToUDP(dev orchtun.Device, conn net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	packet := make([]byte, 65535)
	for {
		n, err := dev.ReadPacket(packet)
		if err != nil {
			return
		}
		if !isIPv4Packet(packet[:n]) {
			continue
		}
		frame := EncodeEncapV0(EncapV0MsgIPv4Payload, packet[:n])
		if _, err := conn.Write(frame); err != nil {
			return
		}
	}
}

func pumpUDPToTUN(log *slog.Logger, dev orchtun.Device, conn net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		payload, ok := decodeIPv4TunnelPayload(buf[:n])
		if !ok {
			continue
		}
		if _, err := dev.WritePacket(payload); err != nil {
			log.Debug("tunnel tun write", "error", err)
		}
	}
}

func decodeIPv4TunnelPayload(frame []byte) ([]byte, bool) {
	typ, payload, err := DecodeEncapV0(frame)
	if err != nil || typ != EncapV0MsgIPv4Payload {
		return nil, false
	}
	return payload, isIPv4Packet(payload)
}

func isIPv4Packet(packet []byte) bool {
	return len(packet) > 0 && (packet[0]>>4) == 4
}

func closeTUNDevice(log *slog.Logger, dev orchtun.Device) {
	if err := dev.Close(); err != nil {
		log.Debug("tunnel TUN close", "error", err)
	}
}
