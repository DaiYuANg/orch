// Package orchvpn implements the orch-vpn data plane: a TUN-style virtual interface on the workstation
// connected to the network namespace where the orchestrator runs containers, via an encrypted tunnel
// negotiated with the control plane. Wire protocol is orch-owned (not the WireGuard product).
//
// Composition: orch-server registers [GatewayModule] (dix: OrchVPNConfig → GatewayService + lifecycle hooks).
// The orch-vpn binary uses [NewWorkstationApp] / [NewServeApp] with [WorkstationConn] or [ServerConfig]
// plus [config.Config] from [config.Load]. DIX-injected services ([WorkstationDaemon], [ServerDaemonService],
// [GatewayService]) log with the app [slog.Logger] from [logging.Module] (ORCH_*). The orch-vpn cmd entrypoint
// uses pterm only for Cobra-level output (e.g. Execute errors, runtime Stop defer), not inside internal providers.
//
// Phases: control-plane bootstrap + container /32 routes from DNS (done), encap-v0 heartbeat/ack (done),
// encap-v0 IPv4 observation on gateway and `orch-vpn serve` (done), workstation TUN + IPv4 encap forward (done),
// automatic OS routing / gateway return path (next).
package orchvpn
