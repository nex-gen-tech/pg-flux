# 07 — Non-Functional Requirements

**Document:** Non-Functional Requirements
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## 7.1 Performance Requirements

### Parsing
| Metric | Requirement | Rationale |
|--------|-------------|-----------|
| Parse 1,000 table definitions | < 500ms | Developer feedback loop during `plan` must feel instant |
| Parse 10,000 table definitions | < 3s | Large enterprise schemas must remain practical |
| Parse a single 10,000-line SQL file | < 200ms | Single-file schemas common in smaller projects |

**pg_query_go benchmark context:** pg_query_go benchmarks show ~1.7ms per average `SELECT` parse and ~250ns for fingerprinting. DDL statements are heavier, but 1,000 `CREATE TABLE` statements should parse well under 500ms.

### Schema Inspection
| Metric | Requirement | Rationale |
|--------|-------------|-----------|
| Inspect 500-table database | < 2s | Full inspection run during CI must be fast |
| Inspect 5,000-table database | < 10s | Large-scale schemas must still be manageable |
| Time to first meaningful output | < 500ms | Users need to know the tool is working |

Concurrent catalog queries using Go's `errgroup` should allow inspecting 8 object types in parallel, hitting this target.

### Diffing & Plan Generation
| Metric | Requirement | Rationale |
|--------|-------------|-----------|
| Diff 5,000 objects | < 100ms | Diffing must not be a bottleneck |
| Generate 1,000-statement execution plan | < 50ms | DAG sort is O(V+E); should be trivial |

### Migration Execution
| Metric | Requirement | Rationale |
|--------|-------------|-----------|
| Statement overhead (excluding PG execution time) | < 5ms per statement | Tool overhead must not be measurable |
| Advisory lock acquisition | < 1s or fail fast | Do not stall migration on lock acquisition |

---

## 7.2 Binary Distribution

### Size
| Target | Requirement |
|--------|-------------|
| `linux/amd64` binary (CGO static) | < 35MB |
| `darwin/arm64` binary | < 35MB |

**Note:** Because pg-flux uses CGO (for `pg_query_go`), binaries include the compiled libpg_query C library. Static linking is required for distribution. `goreleaser` with `CGO_ENABLED=1` is used for release builds.

### Platforms
| Platform | Architecture | Support Level |
|----------|-------------|---------------|
| `linux/amd64` | x86_64 | Tier 1 (fully tested) |
| `linux/arm64` | ARM64 | Tier 1 (fully tested) |
| `darwin/arm64` | Apple Silicon | Tier 1 (fully tested) |
| `darwin/amd64` | Intel Mac | Tier 2 (tested opportunistically) |
| `windows/amd64` | x86_64 | Not supported (CGO limitations) |

Windows is not supported in v1.0 due to CGO cross-compilation complexity. Users on Windows are directed to use WSL2 or Docker.

### Distribution Channels
- **GitHub Releases:** Pre-built binaries via `goreleaser`
- **Homebrew tap:** `brew install pg-flux/tap/pg-flux`
- **Docker image:** `ghcr.io/pg-flux/pg-flux:latest` (Alpine-based, includes pg_query C library)
- **Go install:** `go install github.com/your-org/pg-flux@latest` (requires CGO toolchain)

---

## 7.3 Memory

| Scenario | Requirement |
|----------|-------------|
| Inspect 500-table database | < 50MB RSS |
| Inspect 5,000-table database | < 200MB RSS |
| Parse 10,000-table schema file | < 100MB RSS |
| Idle (no active operation) | < 20MB RSS |

The pg_query_go AST objects are large in memory. After parsing, pg-flux must convert the protobuf AST to its own lightweight Go structs and release the protobuf objects to GC before proceeding to the differ.

---

## 7.4 Concurrency and Safety

### Serial Execution Guarantee
Only one `engine apply` can execute on a target database at a time. This is enforced by a PostgreSQL advisory lock (`pg_advisory_lock(hash)` where `hash` is derived from the target schema name).

### Connection Pooling
pg-flux is a CLI tool, not a server. It opens a single PostgreSQL connection, performs its operation, and closes it. No connection pool is needed.

### Goroutine Safety
- [ ] All catalog queries run as goroutines in `errgroup.WithContext` — any error cancels the group.
- [ ] The `SchemaState` struct is write-once (assembled during inspection) and read-only during diffing.
- [ ] No global mutable state is used.

---

## 7.5 Reliability

### Atomicity
- All transactional DDL is wrapped in a single `BEGIN`/`COMMIT` block.
- If any transactional DDL fails, the transaction is rolled back completely.
- The live database is never left in a partially migrated state for transactional operations.

### Concurrent Operation Tracking
Concurrent operations (`CREATE INDEX CONCURRENTLY`) run outside transactions and cannot be rolled back. The tool must:
- Report the state of each concurrent operation upon failure.
- Provide manual remediation instructions in the error output.
- Log the exact statement that failed for manual re-run.

### Idempotency
- `engine plan` is always safe to run multiple times.
- `engine apply` on an already-applied migration produces a "no changes" result.
- `engine inspect` on the same database always produces the same output (deterministic).

---

## 7.6 Observability

### Exit Codes
All exit codes are documented in [06-cli-interface.md](06-cli-interface.md) and are stable within a major version.

### Structured Logging
- `--log-level=debug` enables verbose per-query logging (useful for debugging catalog queries).
- All log output is to stderr; user-facing output is to stdout.
- In `--format=json` mode, errors are also emitted as JSON to stdout.

### Metrics (v2 roadmap)
Metrics integration (Prometheus, OpenTelemetry) is deferred to v2.

---

## 7.7 PostgreSQL Version Support

| PostgreSQL Version | Support Level |
|-------------------|---------------|
| PostgreSQL 18 | Tier 1 (primary target, fully tested) |
| PostgreSQL 17 | Tier 2 (supported, tested in CI) |
| PostgreSQL 16 | Tier 2 (supported, tested opportunistically) |
| PostgreSQL 15 | Tier 3 (best effort, no CI guarantee) |
| PostgreSQL 14 and below | Not supported |

**Why PG18 is Tier 1:** pg-flux's value proposition is PG18-specific features (uuidv7, named NOT NULL, temporal constraints). Older versions are supported for teams that plan to upgrade.

---

## 7.8 Backward Compatibility

### CLI
- Flag names and semantics are stable within a major version (v1.x).
- Flags may be deprecated but not removed without a major version bump.
- New flags are always optional with defaults that preserve existing behavior.

### JSON Output
- The JSON output schema is versioned (`"version": "1.0"`).
- New fields may be added to JSON output in minor versions.
- Existing fields are never removed or renamed in minor versions.
- JSON consumers should use schema validation against the published JSON Schema.

### Configuration File
- `.pg-flux.yml` keys are stable within a major version.
- Unknown keys are ignored (forward compatibility for newer configs on older tools).

---

## 7.9 Build Requirements

| Requirement | Specification |
|-------------|--------------|
| Go version | 1.22 or later |
| CGO | Required (`CGO_ENABLED=1`) |
| C compiler | `gcc` or `clang` (for libpg_query) |
| libpg_query | Bundled as a Git submodule (or via pg_query_go's vendor) |
| Build time (clean) | < 3 minutes on standard CI hardware |

**Cross-compilation:** Because pg_query_go uses CGO, cross-compilation requires a cross-compilation toolchain (e.g., `zig cc` for cross-compiling from macOS to Linux). `goreleaser` manages this automatically.
