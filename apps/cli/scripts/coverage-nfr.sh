#!/usr/bin/env bash
# Report total statement coverage (all packages). Optional gate with ENFORCE_COVERAGE=1.
# Usage: ./scripts/coverage-nfr.sh [min_percent]
set -euo pipefail
cd "$(dirname "$0")/.."
MIN="${1:-80}"
go test ./... -count=1 -coverprofile=/tmp/pgflux-cov.out -coverpkg=./... >/dev/null
PCT="$(go tool cover -func=/tmp/pgflux-cov.out | tail -1 | awk '{print $3}' | tr -d '%')"
echo "Total statement coverage: ${PCT}% (NFR reference target: ${MIN}%)"
if [[ "${ENFORCE_COVERAGE:-}" == "1" ]]; then
  awk -v p="$PCT" -v m="$MIN" 'BEGIN { exit (p+0 >= m+0) ? 0 : 1 }' || {
    echo "ENFORCE_COVERAGE=1: coverage below ${MIN}%."
    exit 1
  }
fi
