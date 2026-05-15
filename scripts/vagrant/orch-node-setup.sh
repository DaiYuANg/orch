#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[orch-vagrant] $*"
}

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    log "orch-node-setup.sh must run as root"
    exit 1
  fi
}

require_args() {
  local missing=0
  if [ "${1:-}" = "" ]; then
    missing=1
  fi
  for i in "$@"; do
    if [ "${i:-}" = "" ]; then
      missing=1
      break
    fi
  done
  if [ "$missing" -ne 0 ]; then
    cat <<EOF
Usage: orch-node-setup.sh NODE_ID HTTP_ADDR RAFT_NODE_ID RAFT_BIND RAFT_ADVERTISE RAFT_PEERS CLUSTER_NODES
EOF
    exit 1
  fi
}

main() {
  require_root
  require_args "$@"

  local node_id="$1"
  local http_addr="$2"
  local raft_node_id="$3"
  local raft_bind="$4"
  local raft_advertise="$5"
  local raft_peers="$6"
  local cluster_nodes="$7"

  local node_id_for_logs="node_id=$node_id"
  local install_dir="${ORCH_INSTALL_DIR:-/usr/local/bin}"
  local data_dir="${ORCH_DATA_DIR:-/var/lib/orch}"
  local raft_data_dir="${ORCH_RAFT_DATA_DIR:-$data_dir/dragonboat}"
  local work_dir="${ORCH_WORK_DIR:-.orch-vagrant}"
  local source_dir="${ORCH_INSTALL_SOURCE:-/vagrant/$work_dir/dist/bin}"
  local dns_enabled="${ORCH_DNS_ENABLED:-false}"
  local ingress_enabled="${ORCH_INGRESS_ENABLED:-false}"
  local log_level="${ORCH_LOG_LEVEL:-info}"
  local server_bin_src="$source_dir/orch-server"
  local cli_bin_src="$source_dir/orch"

  local server_bin_dst="$install_dir/orch-server"
  local cli_bin_dst="$install_dir/orch"
  local systemd_unit="/etc/systemd/system/orch-server.service"

  local cluster_env="${ORCH_CLUSTER_ENV_FILE:-/etc/orch/env}"
  local service_user="${ORCH_SERVICE_USER:-root}"

  log "Preparing node $node_id_for_logs"

  mkdir -p "$(dirname "$server_bin_dst")" "$data_dir" "$raft_data_dir" "$(dirname "$cluster_env")"
  if [ ! -f "$server_bin_src" ]; then
    log "server binary not found at $server_bin_src"
    ls -la "$(dirname "$server_bin_src")" || true
    exit 1
  fi
  if [ ! -f "$cli_bin_src" ]; then
    log "cli binary not found at $cli_bin_src"
    ls -la "$(dirname "$cli_bin_src")" || true
    exit 1
  fi

  install -m 0755 "$server_bin_src" "$server_bin_dst"
  install -m 0755 "$cli_bin_src" "$cli_bin_dst"

  mkdir -p /etc/orch
  cat >/etc/orch/config.env <<EOF
ORCH_HTTP_ADDR=$http_addr
ORCH_RAFT_NODE_ID=$raft_node_id
ORCH_RAFT_BIND=$raft_bind
ORCH_RAFT_ADVERTISE=$raft_advertise
ORCH_RAFT_PEERS=$raft_peers
ORCH_RAFT_DATA_DIR=$raft_data_dir
ORCH_CLUSTER_NODES=$cluster_nodes
ORCH_DNS_ENABLED=$dns_enabled
ORCH_INGRESS_ENABLED=$ingress_enabled
ORCH_DATA_DIR=$data_dir
EOF

  cat >/etc/orch/env <<EOF
ORCH_NODE_ID=$node_id
ORCH_SERVER_LOG_LEVEL=$log_level
ORCH_LOG_LEVEL=$log_level
EOF

  cat >"$systemd_unit" <<EOF
[Unit]
Description=Orch server on $node_id_for_logs
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
Type=simple
User=$service_user
EnvironmentFile=-/etc/orch/env
ExecStart=$server_bin_dst \
  --http-addr="$http_addr" \
  --raft-node-id="$raft_node_id" \
  --raft-bind="$raft_bind" \
  --raft-advertise="$raft_advertise" \
  --raft-peers="$raft_peers" \
  --raft-data-dir="$raft_data_dir" \
  --cluster-nodes="$cluster_nodes" \
  --dns-enabled="$dns_enabled" \
  --ingress-enabled="$ingress_enabled" \
  --observability-prometheus-enabled=false \
  --observability-otlp-enabled=false \
  --log-level="$log_level"
Restart=always
RestartSec=2
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable orch-server.service
  systemctl restart orch-server.service
  systemctl is-active --quiet orch-server.service
  if [ "$?" -ne 0 ]; then
    log "orch-server service failed to start on $node_id_for_logs"
    systemctl status --no-pager orch-server.service || true
    exit 1
  fi

  log "Node $node_id_for_logs setup complete"
}

main "$@"
