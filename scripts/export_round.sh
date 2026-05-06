#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../backend"

if command -v mise >/dev/null 2>&1; then
  mise x go@1.26.2 -- go run ./cmd/arenactl export-round
elif command -v go >/dev/null 2>&1; then
  go run ./cmd/arenactl export-round
else
  echo "Go is required to run export_round.sh" >&2
  exit 1
fi
