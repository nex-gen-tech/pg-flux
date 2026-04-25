# 09 — Risk Analysis

**Document:** Technical & Operational Risk Analysis
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## Risk Registry

Each risk is rated by:
- **Likelihood:** High / Medium / Low
- **Impact:** Critical / High / Medium / Low
- **Priority:** Likelihood × Impact

---

## R-01: `pg_query_go` Does Not Yet Support PG18 Syntax

| Field | Value |
|-------|-------|
| **Likelihood** | High |
| **Impact** | High |
| **Priority** | P0 — Block on release |
| **Status** | Open |

**Description:**
`pg_query_go` v6 is based on PostgreSQL 17's C parser library (`libpg_query`). PostgreSQL 18 introduces new syntax (`WITHOUT OVERLAPS`, `PERIOD` in FK, named NOT NULL, `RETURNING OLD/NEW`) that the PG17 parser does not understand.

If pg-flux tries to parse PG18-specific DDL syntax using pg_query_go v6, `pg_query.Parse()` returns an error, and the source parser fails.

**Impact:**
- Source files using PG18-specific syntax fail to parse.
- Users can't use PG18 syntax in their schema definitions, defeating the PG18 purpose of the tool.

**Mitigation Plan:**

1. **Track upstream:** Monitor `pganalyze/pg_query_go` GitHub for a v7 release (based on PG18's `libpg_query`).
2. **Fallback mode:** In v1.0, if pg_query_go/v6 is the only available version, pg-flux documents the limitations and supports all PG17-compatible syntax.
3. **PG18 catalog queries still work:** Even with a PG17 parser, the state inspector queries PG18 system catalogs directly. All PG18 live-DB features can still be detected and diffed. Only the *source file parsing* of PG18-specific syntax is limited.
4. **Custom libpg_query build:** If upstream doesn't release in time, pg-flux can vendor a PG18 `libpg_query` build and create an internal CGO binding. This is a fallback of last resort.

---

## R-02: PostgreSQL 18 Catalog Breaking Changes

| Field | Value |
|-------|-------|
| **Likelihood** | High |
| **Impact** | Critical |
| **Priority** | P0 — Must fix before release |
| **Status** | Mitigated by design |

**Description:**
PostgreSQL 18 removes `pg_attribute.attcacheoff` and adds new columns to `pg_constraint`. Any inspector code that references removed columns will fail with `"column attcacheoff does not exist"`.

**Impact:**
- Inspector crashes at runtime on PG18 databases.
- Cannot inspect any schema.

**Mitigation Plan:**

1. **Zero tolerance for `attcacheoff`:** All catalog queries are reviewed to ensure `attcacheoff` is not referenced. This is enforced by CI integration tests against a real PG18 Docker container.
2. **Catalog compatibility matrix:** A test file `inspector/pg18_compat_test.go` documents all known PG18 catalog changes and has integration tests for each.
3. **Schema-first approach:** Inspector queries are written using only stable catalog columns. When a new PG18 column is needed (e.g., `conenforced`), a `pg_catalog.pg_constraint_oid_and_conenforced_exists()` compatibility function checks for the column's existence before using it.

---

## R-03: CGO Cross-Compilation Complexity

| Field | Value |
|-------|-------|
| **Likelihood** | Medium |
| **Impact** | High |
| **Priority** | P1 — Resolve in Phase 4 |
| **Status** | Open |

**Description:**
`pg_query_go` requires CGO because it wraps a C library. CGO cross-compilation (e.g., building `linux/amd64` on `darwin/arm64`) requires special toolchains (`zig cc`, `cross` Docker images) and is significantly more complex than pure-Go cross-compilation.

**Impact:**
- CI/CD binary release pipeline is more complex.
- Developer `go install` requires a C toolchain.
- Some CI environments may not have CGO toolchains.

**Mitigation Plan:**

1. **goreleaser with `zig cc`:** Use `goreleaser` with `zig cc` as the cross-compiler, which provides a single statically-linked binary without requiring `gcc` on the build machine.
2. **Docker build image:** Provide a `Dockerfile.build` that includes all necessary CGO dependencies.
3. **Pre-built libpg_query:** Vendor the pre-built C library object files in the repository to avoid requiring users to build it from source.
4. **Documentation:** Document the CGO requirement prominently in the README so developers know they need a C compiler for `go install`.

---

## R-04: Rename Detection False Positives

| Field | Value |
|-------|-------|
| **Likelihood** | Medium |
| **Impact** | Critical |
| **Priority** | P0 — Must have strong safeguards |
| **Status** | Mitigated by design |

**Description:**
If the rename resolution algorithm has a bug, it could incorrectly interpret a drop/add as a rename, preventing legitimate column drops. Worse, if it incorrectly interprets a rename as a drop, it would generate `DROP COLUMN` on data-bearing columns.

**Impact:**
- Data loss (false DROP).
- Migration blocked (false rename preventing legitimate column removal).

**Mitigation Plan:**

1. **Require explicit hints:** Rename resolution ONLY triggers when a `-- @renamed from=X` hint is present in the source file. Without a hint, any column not in the live schema is treated as a new column. Drops require explicit absence from the desired schema.
2. **Source validation:** The hint validator checks that the `from=` identifier actually exists in the live schema. If it does not, the operation fails fast with an error.
3. **Property-based fuzz testing:** `pkg/differ/rename_test.go` uses `gopter` or `rapid` to fuzz-test rename resolution with hundreds of randomly generated schema states.
4. **Shadow DB confirmation:** All renames are executed against a shadow database and verified before the live migration plan is finalized.

---

## R-05: Long-Running Concurrent Index Builds Fail Midway

| Field | Value |
|-------|-------|
| **Likelihood** | Medium |
| **Impact** | Medium |
| **Priority** | P1 — Handling required |
| **Status** | Open |

**Description:**
`CREATE INDEX CONCURRENTLY` can run for minutes or hours on large tables. If the connection is lost or the build is cancelled, PostgreSQL leaves the index in an INVALID state. The tool cannot automatically roll it back.

**Impact:**
- Migration partially completed.
- User must manually clean up INVALID index.
- Subsequent runs of `pg-flux plan` show a new diff (the INVALID index vs. desired index).

**Mitigation Plan:**

1. **Post-execution validation:** After each `CREATE INDEX CONCURRENTLY`, query `pg_index.indisvalid` to verify the index is valid.
2. **INVALID index detection in inspector:** The inspector checks `pg_index.indisvalid` for all indexes. INVALID indexes are flagged in the diff output with a remediation suggestion: `DROP INDEX CONCURRENTLY invalid_index_name;`.
3. **Timeout configuration:** `--concurrent-statement-timeout=20m` allows operators to set appropriate timeouts for large index builds.
4. **Advisory output:** When a concurrent operation is included in the plan, pg-flux outputs: "Note: Concurrent operations cannot be rolled back. If interrupted, check `pg_index.indisvalid` and drop any INVALID indexes manually."

---

## R-06: Temporal Constraint Parsing Incompatibility

| Field | Value |
|-------|-------|
| **Likelihood** | Low |
| **Impact** | Medium |
| **Priority** | P2 — Monitor |
| **Status** | Tracking |

**Description:**
PG18's `WITHOUT OVERLAPS` and `PERIOD` FK syntax is new. If pg_query_go v7 is not available in time, these constructs cannot be parsed from source files.

**Mitigation Plan:**

1. **Feature flag:** `temporal_constraints` is a feature flag. If pg_query_go does not support the syntax, pg-flux emits a clear error when a temporal constraint is detected in source files.
2. **Inspector-only mode:** Temporal constraints can still be inspected from the live database and compared via catalog queries. The limitation is only on source file parsing.

---

## R-07: Shadow Database Side-Effects

| Field | Value |
|-------|-------|
| **Likelihood** | Low |
| **Impact** | Medium |
| **Priority** | P2 |
| **Status** | Mitigated by design |

**Description:**
The shadow database validation creates a new database, clones the schema, applies the migration plan, and verifies the result. If this process leaks (fails to drop the shadow DB), it accumulates test databases on the server.

**Mitigation Plan:**

1. **`defer` cleanup:** The shadow DB is created with a UUID name and dropped with a deferred function call, even on errors.
2. **Naming convention:** Shadow DBs are named `pgflux_shadow_{timestamp}_{uuid}`. A cleanup query `SELECT datname FROM pg_database WHERE datname LIKE 'pgflux_shadow_%'` can identify and clean up leaked databases.
3. **Limited permissions:** The shadow DB operation requires `CREATEDB` privilege. pg-flux documents the minimal required permissions clearly.

---

## R-08: Information Leakage via Connection Strings in Logs

| Field | Value |
|-------|-------|
| **Likelihood** | Medium |
| **Impact** | High |
| **Priority** | P0 — Must fix before release |
| **Status** | Mitigated by design |

**Description:**
Connection strings may contain database passwords. If pg-flux logs the full connection string, passwords are exposed in CI logs.

**Impact:**
- Database credentials exposed in CI/CD logs.
- Security breach risk.

**Mitigation Plan:**

1. **Redact password before logging:** Any connection string logged by pg-flux has the password component replaced with `[REDACTED]`.
2. **URL parser:** Use `url.Parse()` + `url.User.Username()` to safely extract and redact passwords.
3. **Never log `os.Environ()`:** pg-flux never logs environment variables.
4. **pgx secure logging:** pgx/v5 has built-in mechanisms to avoid logging query parameters; this is the default configuration.

---

## R-09: Multi-Schema FK Dependencies

| Field | Value |
|-------|-------|
| **Likelihood** | Low |
| **Impact** | Medium |
| **Priority** | P2 |
| **Status** | Deferred to v1.1 |

**Description:**
Foreign keys that span schemas (table in `public` references table in `billing`) require both schemas to be inspected and diffed together. The DAG sorter must handle cross-schema dependencies.

**Current scope:** v1.0 supports multi-schema inspection (`--schemas=public,billing`) and creates a unified dependency graph. Cross-schema FKs are correctly sorted.

---

## R-10: Enum Type Modification Limitations

| Field | Value |
|-------|-------|
| **Likelihood** | Medium |
| **Impact** | Medium |
| **Priority** | P1 |
| **Status** | Open |

**Description:**
PostgreSQL allows adding values to enum types (`ALTER TYPE ... ADD VALUE`) but does NOT support removing enum values without recreating the type. This means:
- Adding enum values: safe, always supported.
- Removing enum values: requires DROP TYPE + CREATE TYPE + ALTER COLUMN to use new type. This is inherently disruptive.

**Mitigation Plan:**

1. **`ENUM_VALUE_DROP` hazard:** Removing an enum value triggers the `ENUM_VALUE_DROP` hazard, blocking migration without explicit acknowledgment.
2. **Remediation guidance:** The hazard error message includes specific remediation steps: create a new enum type, migrate columns, drop the old type.
3. **Detect usage:** The hazard engine checks all columns using the enum type before flagging the drop, to quantify the blast radius.
