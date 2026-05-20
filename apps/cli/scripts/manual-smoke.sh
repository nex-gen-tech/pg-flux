#!/usr/bin/env bash
# Manual smoke: same flow a user runs (plan → apply → drift).
# Usage:
#   export DATABASE_URL='postgres://...'
#   ./scripts/manual-smoke.sh
# Or let the script start a throwaway Postgres 18 on port 54333 (requires Docker):
#   ./scripts/manual-smoke.sh --docker
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCHEMA="${ROOT}/testdata/integration-smoke"
run_flux() { (cd "$ROOT" && go run ./cmd/pg-flux "$@"); }

start_docker() {
  docker rm -f pg-flux-smoke 2>/dev/null || true
  docker run --rm -d --name pg-flux-smoke \
    -e POSTGRES_USER=app -e POSTGRES_PASSWORD=app -e POSTGRES_DB=appdb \
    -p 54333:5432 postgres:18-alpine
  for i in $(seq 1 40); do
    if docker exec pg-flux-smoke pg_isready -U app -d appdb &>/dev/null; then
      break
    fi
    sleep 0.5
  done
  export DATABASE_URL='postgres://app:app@127.0.0.1:54333/appdb?sslmode=disable'
  echo "Using DATABASE_URL=$DATABASE_URL"
}

if [[ "${1:-}" == "--docker" ]]; then
  if ! command -v docker &>/dev/null; then
    echo "docker not found" >&2
    exit 1
  fi
  start_docker
  cleanup() { docker stop pg-flux-smoke 2>/dev/null || true; }
  trap cleanup EXIT
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "Set DATABASE_URL or run: $0 --docker" >&2
  exit 1
fi

echo "== plan (expect non-empty) =="
run_flux plan --db "$DATABASE_URL" --schema "$SCHEMA" --format human || true
echo
echo "== apply (allow index rebuild / table lock hazards) =="
run_flux apply --db "$DATABASE_URL" --schema "$SCHEMA" --allow-hazards TABLE_LOCK,INDEX_REBUILD,CONSTRAINT_SCAN 2>&1
echo
echo "== drift (expect success / no drift) =="
if run_flux drift --db "$DATABASE_URL" --schema "$SCHEMA" 2>&1; then
  echo "drift: OK"
else
  echo "drift: failed (exit $?)" >&2
  exit 1
fi

echo "== done =="
