# pg-flux benchmark methodology

This document describes exactly how the benchmark numbers in the README were produced so they can be independently reproduced and updated.

## Environment

| Item | Value |
|---|---|
| Hardware | Apple M-series (darwin-arm64) |
| OS | macOS 15 |
| PostgreSQL | 17, running in Docker (`postgres:17`) on localhost:5440 |
| pg-flux | v0.1.2 (stripped binary: `go build -ldflags="-s -w"`) |
| Atlas | v1.2.1-canary |
| golang-migrate | latest, built with `-tags postgres` |
| goose | latest |

## Schema used

The drift and codegen benchmarks use the `rust-hrm` example schema (`examples/rust-hrm/`), which contains 73 catalog objects:

- 9 tables (including partitioned, self-referential)
- 4 enums, 1 composite type, 2 domains
- 2 views (1 security-invoker, 1 materialized with window function)
- 2 functions + 1 stored procedure
- 5 triggers, 4 RLS policies, 2 grants
- Extensions: `btree_gist`, `pg_trgm`
- Indexes: GIN, BRIN, GiST EXCLUDE, covering (INCLUDE), NULLS NOT DISTINCT, trigram

The apply benchmark uses the `fastapi-todo` migrations (simpler, avoids dollar-quoted functions that some tools can't run inside a transaction).

## Measurement approach

All timings use Python's `time.perf_counter()` — wall-clock time from `subprocess.run()` call to return. Each tool runs 10–12 times; the lowest and highest values are dropped and the median of the remaining runs is reported. The database is on localhost to eliminate network jitter.

```python
import subprocess, time, statistics

times = []
for _ in range(12):
    t0 = time.perf_counter()
    subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    t1 = time.perf_counter()
    times.append((t1 - t0) * 1000)

times_sorted = sorted(times)[1:-1]   # drop 2 outliers
median = statistics.median(times_sorted)
```

## Cold-start latency

Command run for each tool:

| Tool | Command |
|---|---|
| pg-flux | `pg-flux --help` |
| atlas | `atlas version` |
| goose | `goose --version` |
| golang-migrate | `migrate -version` |

Flyway, Liquibase, Alembic, and Prisma numbers are estimates based on well-documented JVM/Node/Python interpreter startup overhead, not direct measurements (those tools are not installed in this environment).

## Drift / diff speed

- **pg-flux**: `pg-flux drift` in `examples/rust-hrm/` directory, `DATABASE_URL` pointing at the applied rust-hrm database. This runs the full 3-layer check (source↔live + verify + baseline hash).
- **atlas**: `atlas schema diff --from <dsn> --to <dsn> --dev-url <dev-dsn>` comparing the live database against itself (zero-diff path — exercises the same inspect+diff code path without network asymmetry).

## Migration apply speed

Each run creates a fresh database, applies migrations, then drops the database. The timer wraps the CLI invocation only.

- **pg-flux**: `pg-flux migrate apply` — includes source parse, baseline-hash verify, hazard scan, advisory lock acquisition, and transactional apply.
- **atlas**: `atlas migrate apply` — same SQL content, `CONCURRENTLY` keyword stripped (Atlas does not support it inside transactions).
- **goose**: `goose postgres <dsn> up` — plain SQL execution, no pre-flight checks.
- **golang-migrate**: `migrate -path <dir> -database <dsn> up` — plain SQL execution, no pre-flight checks.

## Codegen / inspect speed

- **pg-flux**: `pg-flux gen --lang go --out <tempdir>` against the rust-hrm database. Produces Go structs, enums, view types, function stubs.
- **atlas**: `atlas schema inspect --url <dsn>` — nearest equivalent operation (schema introspection); Atlas has no app codegen.

## Binary size

Release tarballs for linux-amd64 taken from each project's GitHub Releases page. The pg-flux number is from the v0.1.2 release assets. Atlas canary binary size is from the downloaded binary rather than a tagged release (no linux-amd64 tarball was listed in the v1.2.0 release at time of writing).

## Reproducing

```bash
# Install tools
curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh | sh
curl -sSLo ~/.local/bin/atlas "https://release.ariga.io/atlas/atlas-darwin-arm64-latest" && chmod +x ~/.local/bin/atlas
go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install github.com/pressly/goose/v3/cmd/goose@latest

# Start Postgres
docker run -d --name pg-bench -p 5440:5432 \
  -e POSTGRES_USER=pgflux -e POSTGRES_PASSWORD=pgflux -e POSTGRES_DB=pgflux \
  postgres:17

# Apply rust-hrm schema
export DATABASE_URL=postgres://pgflux:pgflux@localhost:5440/pgflux_rust_hrm?sslmode=disable
psql $DATABASE_URL -c "CREATE SCHEMA IF NOT EXISTS audit;"
cd examples/rust-hrm && pg-flux migrate apply

# Run cold-start benchmark
python3 - <<'EOF'
import subprocess, time, statistics
tools = [
    ("pg-flux",         ["pg-flux", "--help"]),
    ("atlas",           ["atlas", "version"]),
    ("goose",           ["goose", "--version"]),
    ("golang-migrate",  ["migrate", "-version"]),
]
for name, cmd in tools:
    times = [((lambda t0, t1: t1-t0)(time.perf_counter(), (subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL), time.perf_counter())[1])) * 1000 for _ in range(12)]
    med = statistics.median(sorted(times)[1:-1])
    print(f"{name:20s}  {med:5.0f}ms")
EOF
```
