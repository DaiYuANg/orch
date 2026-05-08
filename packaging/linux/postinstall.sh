#!/bin/sh
set -eu

if command -v orch-server >/dev/null 2>&1; then
  if ! orch-server host-dns install --non-interactive >/dev/null 2>&1; then
    echo "orch: host DNS install skipped or failed; run 'orch-server host-dns status' for details" >&2
  fi
fi
