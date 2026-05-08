#!/bin/sh
set -eu

if command -v orch-server >/dev/null 2>&1; then
  if ! orch-server host-dns uninstall --non-interactive >/dev/null 2>&1; then
    echo "orch: host DNS uninstall skipped or failed; remove orch resolver config manually if needed" >&2
  fi
fi
