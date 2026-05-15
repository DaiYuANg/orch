#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[orch-vagrant] $*"
}

require_root() {
  if [ "$(id -u)" -ne 0 ]; then
    log "bootstrap-node.sh must run as root"
    exit 1
  fi
}

ensure_systemd_enabled() {
  if ! command -v systemctl >/dev/null 2>&1; then
    log "systemctl unavailable; cannot configure systemd-managed services"
    return 1
  fi
  if [ ! -d /run/systemd/system ]; then
    log "systemd init system not detected"
    return 1
  fi
  return 0
}

install_debian_like() {
  local docker_channel="${ORCH_VAGRANT_DOCKER_CHANNEL:-stable}"
  local docker_arch="${ORCH_VAGRANT_DOCKER_ARCH:-$(dpkg --print-architecture)}"
  local distro_id=""
  local distro_codename=""

  log "Installing Debian/Ubuntu base dependencies"
  apt-get update -y
  apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    apt-transport-https \
    gnupg \
    lsb-release \
    jq \
    iproute2 \
    iptables \
    netcat-openbsd \
    systemd \
    dbus \
    procps \
    coreutils

  mkdir -p /etc/apt/keyrings
  distro_id="$(. /etc/os-release && echo "$ID")"
  distro_codename="$(. /etc/os-release && echo "${VERSION_CODENAME:-$(lsb_release -sc)}")"
  curl -fsSL "https://download.docker.com/linux/${distro_id}/gpg" \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg

  cat >/etc/apt/sources.list.d/docker.list <<EOF
deb [arch=$docker_arch signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/${distro_id} \
  ${distro_codename} ${docker_channel}
EOF

  apt-get update -y
  apt-get install -y --no-install-recommends \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin

  usermod -aG docker vagrant || true
}

install_arch_like() {
  log "Installing Arch Linux base dependencies"
  pacman -Syu --noconfirm
  pacman -S --noconfirm --needed \
    docker \
    docker-compose \
    jq \
    iproute2 \
    iptables \
    openssl \
    systemd \
    procps-ng \
    coreutils \
    dbus \
    ca-certificates \
    curl

  usermod -aG docker vagrant || true
}

main() {
  require_root

  source /etc/os-release

  case "$ID" in
    ubuntu|debian)
      install_debian_like
      ;;
    arch)
      install_arch_like
      ;;
    *)
      log "Unsupported OS $ID; attempting Debian-style fallback"
      install_debian_like
      ;;
  esac

  if ensure_systemd_enabled; then
    systemctl enable docker
    systemctl restart docker
    log "Docker service enabled and started"
  else
    log "Skipping docker service enable/start because systemd is unavailable"
  fi

  log "Node bootstrap complete"
}

main "$@"
