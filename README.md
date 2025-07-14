# Warden

> **Warden** is a lightweight runtime and control layer designed for long-lived, stateful services such as databases,
> message queues, and object storage — outside of the traditional container orchestration systems.

---

## 🚧 Problem

Modern infrastructure is dominated by Kubernetes and container orchestration. However, many workloads — especially *
*stateful services** — do not fit well into the container-native model:

- Ephemeral containers conflict with long-running services
- Volume mounts and state management are complex and fragile
- High availability often relies on intricate custom operators
- DNS and service discovery across mixed environments is inconsistent

Running databases in Kubernetes is possible, but rarely ideal.

---

## 🎯 Warden's Goal

Warden aims to provide a better home for **stateful services** by combining:

- 🧠 **Declarative service definition** (via YAML)
- 🛠️ **Runtime management** (binary, systemd, container, or package-based)
- 🌐 **Service discovery & DNS registration** (integrates with Kubernetes or container DNS)
- 📦 **Service packaging and distribution** (WIP)
- 🔁 **Health checks and failover** (automatic respawn and migration)
- 🔒 **Secrets and environment injection** (optional integration with external systems)

It also supports **stateless services** where required — especially when deployed outside of containerized environments.

---

## 🧩 Architecture

Warden is designed as a **peer-to-peer runtime**, without reliance on centralized controllers.

Each node runs a self-contained **agent**, capable of:

- Managing services and their lifecycle
- Communicating with other peers to coordinate service migration or registration
- Interfacing with local executors: Docker, containerd, package managers, or raw binaries

Optionally, a Web UI and CLI tool can provide inspection and control.

---

## 🔧 Features Overview

| Feature                 | Description                                                               |
|-------------------------|---------------------------------------------------------------------------|
| **Flexible installers** | Binary runner, Docker/Podman, systemd, or package manager-based execution |
| **DNS integration**     | Supports service registration to Kubernetes (via Service + Endpoints)     |
| **Secrets manager**     | Optional key/value secret registration and injection                      |
| **Remote storage**      | Supports NFS, block devices, or local mounts                              |
| **Process supervision** | Handles respawn, restart, exit handling, and health probes                |
| **Minimal dependency**  | No Kubernetes or container runtime required                               |

---

## 👥 Who is this for?

Warden is built for:

- DevOps teams running **stateful services on bare-metal or VMs**
- Developers needing a **simple declarative runtime** without full Kubernetes
- Infrastructure teams managing **distributed long-living workloads**
- Edge deployments where **containers are not the ideal abstraction**

---

## 📦 Roadmap

- [x] Core runtime and installer system
- [x] Peer-based node communication
- [x] Kubernetes DNS integration
- [ ] Secret injection support
- [ ] Service migration & HA with rsync-based state movement
- [ ] Package format for sharing and deploying services
- [ ] Plugin system for custom installers or probes
- [ ] Web UI for inspection and control

---

## 📝 License

MIT © 2025 Warden Authors
