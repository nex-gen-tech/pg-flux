#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
if [ -n "$(gofmt -l . 2>/dev/null)" ]; then
  echo "gofmt needed on:" && gofmt -l .
  exit 1
fi
go test ./... -count=1
# Optional: Docker E2E (uncomment in CI)
# export PGFLUX_E2E=1
# go test -tags=integration -count=1 -timeout=20m -run TestE2E ./test/integration/...
