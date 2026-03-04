set windows-shell := ["pwsh.exe", "-NoLogo", "-NoProfile", "-Command"]

default:
    @just --list

check:
    cargo check --workspace

fmt:
    cargo fmt --all

fmt-check:
    cargo fmt --all -- --check

lint:
    cargo clippy --workspace --all-targets -- -D warnings

test:
    cargo test --workspace

run *args:
    cargo run -p warden-server-rs -- {{args}}

run-conf conf:
    cargo run -p warden-server-rs -- --conf {{conf}}

workloads api="auto":
    cargo run -p warden-cli-rs -- --api {{api}} workloads

endpoints api="auto":
    cargo run -p warden-cli-rs -- --api {{api}} endpoints

routes api="auto":
    cargo run -p warden-cli-rs -- --api {{api}} routes

dns api="auto":
    cargo run -p warden-cli-rs -- --api {{api}} dns

deploy name runtime="docker" host="" port="80" ingress_port="8088" api="auto":
    cargo run -p warden-cli-rs -- --api {{api}} deploy --name {{name}} --runtime {{runtime}} --host {{host}} --port {{port}} --ingress-port {{ingress_port}}

stop id api="auto":
    cargo run -p warden-cli-rs -- --api {{api}} stop {{id}}

tasks api="auto":
    cargo run -p warden-cli-rs -- --api {{api}} task list

cluster-up nodes="3" start_port="7443":
    cargo xtask cluster run --nodes {{nodes}} --start-port {{start_port}}

cluster-status:
    cargo xtask cluster status

cluster-down:
    cargo xtask cluster stop

package:
    cargo xtask package

e2e api="http://127.0.0.1:7443":
    cargo xtask e2e --api {{api}}
