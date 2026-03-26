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

docs-build:
    mdbook build docs

docs-serve:
    mdbook serve docs --open

run *args:
    cargo run -p warden-server -- {{args}}

run-conf conf:
    cargo run -p warden-server -- --conf {{conf}}

workloads api="auto":
    cargo run -p warden-cli -- --api {{api}} workloads

endpoints api="auto":
    cargo run -p warden-cli -- --api {{api}} endpoints

routes api="auto":
    cargo run -p warden-cli -- --api {{api}} routes

dns api="auto":
    cargo run -p warden-cli -- --api {{api}} dns

deploy name runtime="docker" host="" port="80" ingress_port="8088" api="auto":
    cargo run -p warden-cli -- --api {{api}} deploy --name {{name}} --runtime {{runtime}} --host {{host}} --port {{port}} --ingress-port {{ingress_port}}

stop id api="auto":
    cargo run -p warden-cli -- --api {{api}} stop {{id}}

tasks api="auto":
    cargo run -p warden-cli -- --api {{api}} task list

task-logs id api="auto" tail="200":
    cargo run -p warden-cli -- --api {{api}} task logs {{id}} --tail {{tail}}

dsl-plan file api="auto":
    cargo run -p warden-cli -- --api {{api}} dsl plan --file {{file}}

dsl-plan-json file api="auto":
    cargo run -p warden-cli -- --api {{api}} dsl plan --file {{file}} --json

dsl-plan-strict file api="auto":
    cargo run -p warden-cli -- --api {{api}} dsl plan --file {{file}} --strict

dsl-planner file:
    cargo run -p warden-cli -- dsl planner --file {{file}}

dsl-render file:
    cargo run -p warden-cli -- dsl render --file {{file}}

dsl-apply file api="auto":
    cargo run -p warden-cli -- --api {{api}} dsl apply --file {{file}}

dsl-apply-strict file api="auto":
    cargo run -p warden-cli -- --api {{api}} dsl apply --file {{file}} --strict

dsl-apply-prune file api="auto":
    cargo run -p warden-cli -- --api {{api}} dsl apply --file {{file}} --prune

dsl-delete file api="auto":
    cargo run -p warden-cli -- --api {{api}} dsl delete --file {{file}}

cluster-up nodes="3" start_port="7443":
    cargo xtask cluster run --nodes {{nodes}} --start-port {{start_port}}

cluster-status:
    cargo xtask cluster status

cluster-down:
    cargo xtask cluster stop

package:
    cargo xtask package

e2e api="http://127.0.0.1:7443" runtime="containerd" image="" port="18080" ingress_port="18088":
    cargo xtask e2e --api {{api}} --runtime {{runtime}} --port {{port}} --ingress-port {{ingress_port}} --image {{image}}
