# 08 — Implementation Roadmap

**Document:** Phased Implementation Roadmap
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## Overview

pg-flux is delivered in four phases over approximately 12 weeks. Each phase ends with a working, testable artifact — never broken intermediate states.

```
Week 1─3           Week 4─6           Week 7─9           Week 10─12
┌────────────┐    ┌────────────┐    ┌────────────┐    ┌────────────┐
│  Phase 1   │───►│  Phase 2   │───►│  Phase 3   │───►│  Phase 4   │
│ Foundation │    │   Differ   │    │  Complex   │    │  Polish &  │
│ & Inspector│    │   Engine   │    │  Objects & │    │  Release   │
│            │    │            │    │  Safety    │    │            │
└────────────┘    └────────────┘    └────────────┘    └────────────┘
     ↓                 ↓                 ↓                 ↓
 Parse + Inspect  Table/Column     Functions/Triggers  CLI + Tests
                  Diff + Rename    RLS + Hazards       Binary Release
```

---

## Phase 1: Foundation (Weeks 1–3)

**Goal:** A working foundation: Go module set up, PostgreSQL 18 inspector building a `SchemaState`, and basic table parsing from SQL files.

### Milestones

| ID | Task | Acceptance Criteria |
|----|------|---------------------|
| P1-01 | Initialize Go module | `go.mod` created; `pg_query_go/v6`, `pgx/v5`, `cobra`, `viper`, `errgroup` added as dependencies |
| P1-02 | CGO build pipeline | `go build ./...` succeeds with CGO on all Tier 1 platforms; CI matrix passes |
| P1-03 | CLI skeleton | `pg-flux --help` shows all commands; `cobra` root command + subcommands stubbed |
| P1-04 | Config loader | `.pg-flux.yml` loaded via `viper`; env var overrides working |
| P1-05 | PostgreSQL 18 connection | `pkg/db/` package; connects using pgx/v5; connection string from `--db` or `DATABASE_URL` |
| P1-06 | Schema inspector (tables only) | `inspector` queries `pg_class`, `pg_attribute`, `pg_constraint` for tables only; returns `SchemaState.Tables` map |
| P1-07 | PG18 catalog compatibility | All inspector queries tested against PG18 Docker image; `attcacheoff` not referenced |
| P1-08 | SQL source parser (tables only) | `src` package parses `CREATE TABLE` statements using `pg_query_go`; returns `SchemaState.Tables` map |
| P1-09 | Integration test harness | `testcontainers-go` starts a PG18 container; inspector and parser tests pass against real PG18 |

### Technical Notes

**P1-06 — Catalog Queries:**
```sql
-- Tables query (PG18 compatible)
SELECT
    c.oid,
    c.relname,
    n.nspname AS schema_name,
    c.relrowsecurity,
    c.relforcerowsecurity,
    c.relpersistence
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'r'
  AND n.nspname = ANY($1)
ORDER BY n.nspname, c.relname;
```

Note: No reference to `attcacheoff` (removed in PG18).

**P1-08 — Parse Strategy:**
```go
// Walk pg_query AST for CreateStmt nodes
result, err := pg_query.Parse(sql)
for _, stmt := range result.Stmts {
    if create, ok := stmt.Stmt.Node.(*pg_query.Node_CreateStmt); ok {
        table := parseCreateStmt(create.CreateStmt, hints)
        state.Tables[table.FullName()] = table
    }
}
```

### Deliverable
`pg-flux inspect --db postgres://... --out ./schema` produces `.sql` files for all tables.

---

## Phase 2: Differ Engine (Weeks 4–6)

**Goal:** Full column-level diffing, rename resolution, DAG-sorted execution plan, and a working `engine plan` command.

### Milestones

| ID | Task | Acceptance Criteria |
|----|------|---------------------|
| P2-01 | Column differ | Detects added/removed/modified columns; produces `ALTER TABLE ADD/DROP/ALTER COLUMN` statements |
| P2-02 | Rename resolution | `-- @renamed from=X` hints are extracted by parser; differ resolves to `RENAME COLUMN` instead of DROP+ADD |
| P2-03 | Table-level differ | Detects added/removed tables; produces `CREATE TABLE` / `DROP TABLE` |
| P2-04 | Constraint differ | Diffs PRIMARY KEY, UNIQUE, CHECK, FK constraints; PK changes handled as DROP+ADD |
| P2-05 | Index differ | Diffs index definitions; new indexes generate `CREATE INDEX CONCURRENTLY` |
| P2-06 | DAG sorter | Topological sort of all statements; creation and deletion ordering enforced |
| P2-07 | Basic hazard engine | `DATA_LOSS` (drop table/column) and `TABLE_LOCK` (non-concurrent index) hazards detected |
| P2-08 | `engine plan` command | Generates and displays sorted execution plan with hazard annotations |
| P2-09 | `engine drift` command | Exits `0` (no diff) or `1` (diff), with human/JSON output |
| P2-10 | Named NOT NULL (PG18) | Inspector and differ handle `pg_constraint` type `'n'` for named NOT NULL constraints |
| P2-11 | Basic execution engine | `engine apply` runs transactional DDL statements; rolls back on failure |

### Technical Notes

**P2-02 — Rename Resolution Algorithm:**
```go
func ResolveRenames(desired, live *SchemaState) {
    for tableName, desiredTable := range desired.Tables {
        liveTable := live.Tables[tableName]
        if liveTable == nil { continue }

        for _, desiredCol := range desiredTable.Columns {
            if hint, ok := desiredCol.Hints["renamed"]; ok {
                oldName := hint["from"]
                if _, exists := liveTable.ColumnsByName[oldName]; !exists {
                    return fmt.Errorf("rename source %q not found", oldName)
                }
                desiredCol.RenameFrom = oldName
            }
        }
    }
}
```

**P2-06 — DAG Sorter:**
Uses Kahn's algorithm. Nodes are `Statement` objects; edges are dependency relationships (FK depends on PK, policy depends on table, etc.). If a cycle is detected (theoretically impossible for valid PostgreSQL schemas), an error is returned.

### Deliverable
`pg-flux plan` and `pg-flux apply` work for tables, columns, constraints, and indexes.

---

## Phase 3: Complex Objects & Safety (Weeks 7–9)

**Goal:** Full support for functions, triggers, RLS policies, sequences, and the complete hazard detection engine.

### Milestones

| ID | Task | Acceptance Criteria |
|----|------|---------------------|
| P3-01 | Function inspector | Inspects function signatures + bodies from `pg_proc`; handles PL/pgSQL and SQL functions |
| P3-02 | Function parser | Parses `CREATE [OR REPLACE] FUNCTION` from source files |
| P3-03 | Function differ | Diffs function bodies using `pg_query.Fingerprint()`; generates `CREATE OR REPLACE FUNCTION` |
| P3-04 | Trigger inspector | Inspects all triggers from `pg_trigger` + `pg_proc` |
| P3-05 | Trigger parser | Parses `CREATE TRIGGER` from source files |
| P3-06 | Trigger differ | Diffs trigger definitions; uses DROP+CREATE (no ALTER TRIGGER in PG) |
| P3-07 | RLS inspector | Inspects all policies from `pg_policy`; uses `pg_get_expr()` for expressions |
| P3-08 | RLS parser | Parses `CREATE POLICY`, `ALTER TABLE ENABLE ROW LEVEL SECURITY` etc. |
| P3-09 | RLS differ | Diffs policy expressions using fingerprinting; generates CREATE/DROP/ALTER POLICY |
| P3-10 | Sequence inspector | Inspects sequences and owning column assignments |
| P3-11 | Sequence parser | Parses `CREATE SEQUENCE` and `ALTER SEQUENCE OWNED BY` |
| P3-12 | Sequence differ | Diffs sequence parameters; generates ALTER SEQUENCE |
| P3-13 | Full hazard engine | All hazards in the registry implemented: `CONSTRAINT_SCAN`, `COLUMN_TYPE_CHANGE`, `ENUM_VALUE_DROP`, `FUNCTION_SIGNATURE_CHANGE`, `NOT_REPLICA_SAFE` |
| P3-14 | NOT VALID + VALIDATE pattern | `CONSTRAINT_SCAN` auto-rewrite: FK and CHECK constraints added as NOT VALID, then VALIDATE CONSTRAINT |
| P3-15 | 4-step NOT NULL pattern | `CONSTRAINT_SCAN` auto-rewrite: CHECK IS NOT NULL → SET NOT NULL → DROP CHECK |
| P3-16 | PG18 temporal constraints | Parser + inspector + differ support for `WITHOUT OVERLAPS` and `PERIOD` FK |
| P3-17 | PG18 virtual generated columns | Parser + inspector + differ for `GENERATED ALWAYS AS ... VIRTUAL` |
| P3-18 | PG18 non-enforced constraints | `pg_constraint.conenforced` flag in inspector; `NOT ENFORCED` in parser |
| P3-19 | Shadow DB validation | `engine plan --validate` creates a shadow PG18 database, applies the plan, verifies state matches |

### Technical Notes

**P3-14 — NOT VALID FK Pattern:**
```sql
-- Step 1 (in transaction): Add FK without validation
ALTER TABLE orders
    ADD CONSTRAINT orders_user_fk
    FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;

-- Step 2 (outside transaction): Validate with no lock
ALTER TABLE orders
    VALIDATE CONSTRAINT orders_user_fk;
```

**P3-15 — Safe NOT NULL Pattern:**
```sql
-- Step 1 (in transaction): Add CHECK with no validation
ALTER TABLE users
    ADD CONSTRAINT users_email_not_null_check
    CHECK (email IS NOT NULL) NOT VALID;

-- Step 2 (outside transaction): Validate CHECK (no lock)
ALTER TABLE users
    VALIDATE CONSTRAINT users_email_not_null_check;

-- Step 3 (in transaction): Set NOT NULL using validated CHECK
ALTER TABLE users
    ALTER COLUMN email SET NOT NULL;

-- Step 4 (in transaction): Drop the now-redundant CHECK
ALTER TABLE users
    DROP CONSTRAINT users_email_not_null_check;
```

**PG18 optimization:** In PG18, named NOT NULL constraints can be added with `NOT VALID` directly, eliminating the need for steps 3–4:
```sql
ALTER TABLE users
    ADD CONSTRAINT users_email_not_null NOT NULL email NOT VALID;

ALTER TABLE users
    VALIDATE CONSTRAINT users_email_not_null;
```

### Deliverable
`pg-flux plan` and `pg-flux apply` work for all object types including functions, triggers, and RLS policies.

---

## Phase 4: Polish & Release (Weeks 10–12)

**Goal:** Production-ready CLI, comprehensive test suite, documentation, and release pipeline.

### Milestones

| ID | Task | Acceptance Criteria |
|----|------|---------------------|
| P4-01 | `engine init` command | Creates schema directory with PG18 best-practice templates |
| P4-02 | `engine inspect` command | Produces valid `.sql` files from live PG18 database |
| P4-03 | JSON output format | `--format=json` for all commands; validates against published JSON Schema |
| P4-04 | ANALYZE injection | Post-migration ANALYZE statements injected and executed correctly |
| P4-05 | Advisory lock | Session-level lock prevents concurrent `engine apply` runs |
| P4-06 | Unit test coverage | ≥ 80% line coverage across all packages; all differ and hazard logic unit-tested |
| P4-07 | Integration test corpus | 100+ schema change scenarios; each tested against real PG18 container |
| P4-08 | Shadow DB test suite | All hazard auto-rewrites validated against shadow database |
| P4-09 | goreleaser config | `linux/amd64`, `linux/arm64`, `darwin/arm64` binaries built and uploaded on tag |
| P4-10 | Docker image | `ghcr.io/pg-flux/pg-flux:latest` built and published on tag |
| P4-11 | Homebrew tap | `homebrew-tap` repository with formula; `brew install pg-flux/tap/pg-flux` works |
| P4-12 | README + Quickstart | Installation, quickstart tutorial, and link to PRD docs |
| P4-13 | Changelog | CHANGELOG.md generated from conventional commits via `goreleaser` |

---

## Risk-Adjusted Schedule

| Week | Phase | Key Risks | Mitigation |
|------|-------|-----------|------------|
| 1–3 | Foundation | CGO cross-compilation issues; PG18 Docker image availability | Test CGO setup in Week 1; use `pgvector/pgvector:pg18` image early |
| 4–6 | Differ | Rename resolution edge cases; complex constraint diff logic | Fuzz-test rename resolution with property-based tests in Week 5 |
| 7–9 | Complex objects | PG18 catalog query failures; temporal constraint parser complexity | Test against PG18 nightly builds; track pg_query_go PG18 release |
| 10–12 | Polish | Integration test flakiness; binary size exceeding 35MB | Run integration tests multiple times; use UPX if needed for binary compression |

---

## Parser and coverage notes

- See `docs/parser-limitations.md` for libpg_query / `pg_query_go` behavior vs PostgreSQL.
- Total statement coverage reporting: `scripts/coverage-nfr.sh` (optional `ENFORCE_COVERAGE=1` gate).

## Dependency Tracking

| Dependency | Current Version | Required for | Notes |
|------------|----------------|--------------|-------|
| `pg_query_go` | v6 (PG17) | Phase 1+ | v7 needed for PG18 syntax; track upstream |
| `pgx/v5` | v5.x | Phase 1+ | PG18 compatible |
| `testcontainers-go` | latest | Phase 1+ | Must support `postgres:18` image |
| PostgreSQL 18 Docker image | `postgres:18` | Phase 1+ | Confirm image availability on Docker Hub |
| `goreleaser` | latest | Phase 4 | CGO cross-compilation support needed |

**Critical dependency:** `pg_query_go` must release PG18 support (libpg_query based on PG18 source) before pg-flux can parse PG18-specific syntax in source files. The project must track this upstream. If PG18 support is delayed, Phase 1–3 can proceed with PG17 parser (all tested syntax is backward-compatible) and PG18-specific syntax is added in Phase 3.
