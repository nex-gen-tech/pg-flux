# 01 — Executive Summary

**Document:** Executive Summary & Product Vision
**Project:** pg-flux — Declarative PostgreSQL 18 Schema Migration Engine
**Version:** 1.0 | **Status:** Active Draft

---

## 1.1 Product Vision

`pg-flux` is a state-of-the-art, high-performance, and mathematically safe database schema migration engine purpose-built for PostgreSQL 18. It abandons the brittle, imperative "up/down" script methodology in favor of a **declarative, Database-as-Code paradigm**. Acting as a deterministic schema diffing engine, it allows developers to manage their database state as easily as their application code — define what your schema *should* look like, and let the engine figure out the safest path to get there.

**North Star:** Zero schema drift. Zero accidental data loss. Zero production downtime.

---

## 1.2 Problem Statement

### The State of Database Migration Tooling

Traditional migration tools — Flyway, Liquibase, and standard ORM migration frameworks — all share a fundamental design flaw: they are **imperative and sequential**. Developers write numbered scripts that run in order (`001_create_users.sql`, `002_add_index.sql`, etc.). This approach collapses under the weight of real production systems.

### Core Failure Modes

#### 1. Schema Drift
The live production database diverges from the repository. This happens constantly:
- A DBA applies an emergency `ALTER TABLE` hotfix at 3am.
- A developer manually fixes a broken index in staging but forgets to commit the change.
- A third-party tool creates an auxiliary table or index outside the migration system.

**Impact:** The next migration script assumes a state that no longer exists, causing failures at the worst possible time — during a deployment.

#### 2. The Rename Ambiguity Problem
When a developer renames a column (e.g., `name` → `full_name`), a declarative diff tool that only inspects names will see:
- A column `full_name` that doesn't exist in the live DB → **CREATE** it.
- A column `name` that doesn't exist in the desired schema → **DROP** it.

The tool cannot distinguish this from a genuine "drop old column, add new column" intent. The result is **catastrophic data loss** — all existing data in that column is silently deleted.

Most existing declarative tools either refuse to handle this case or simply generate the destructive pair, leaving it to the developer to manually intervene.

#### 3. Complex Object Mismanagement
Standard tooling handles `CREATE TABLE` adequately but frequently breaks on:
- **PL/pgSQL Functions:** Body comparison is treated as a raw string diff. Whitespace changes trigger needless `CREATE OR REPLACE`. Worse, body changes that alter function signatures are not caught.
- **Triggers:** Dependencies between trigger events and their underlying functions are not modeled, leading to invalid drop/create ordering (attempting to drop a function before the trigger that depends on it).
- **Row-Level Security (RLS) Policies:** The `USING` and `WITH CHECK` expression trees are stored internally by PostgreSQL in a normalized form that differs from how they appear in SQL source files. Naïve string comparison generates constant false-positive diffs, causing repeated no-op `ALTER POLICY` statements on every migration run.

#### 4. Accidental Production Downtime
PostgreSQL's locking model is aggressive for schema changes. The following operations acquire an **Access Exclusive lock**, which blocks *all* reads and writes on the target table for the duration:
- `ALTER TABLE ... ADD COLUMN` with a volatile default (pre-PG 11 behavior, still relevant for non-constant defaults).
- `CREATE INDEX` (non-concurrent).
- `ALTER TABLE ... ADD CONSTRAINT CHECK ...` on a populated table (validates the entire table).
- `ALTER TABLE ... ALTER COLUMN ... SET NOT NULL` on a populated table (requires full scan).

No existing tool outside specialized platforms (like Planetscale or specialized Postgres tooling like pg-schema-diff) provides automated detection and rewriting of these patterns into their zero-downtime equivalents.

#### 5. CI/CD Pipeline Integration Gaps
Without a reliable drift-detection command that returns a non-zero exit code on schema mismatch, teams cannot:
- Automatically gate deployments when production has drifted.
- Enforce "schema reviewed by a second engineer" policies.
- Generate migration previews in pull request comments.

---

## 1.3 The Solution: pg-flux

`pg-flux` is a Go-based CLI tool that addresses every failure mode above through a clean three-phase pipeline:

```
[ .sql Source Files ]  ──►  [ Desired State AST ]
                                       │
[ Live PostgreSQL 18 ]  ──►  [ Current State AST ]
                                       │
                              [ Differ Engine ]
                                       │
                          ┌────────────┴────────────┐
                          │                         │
                   [ Hazard Check ]        [ DAG Sort ]
                          │                         │
                          └────────────┬────────────┘
                                       │
                          [ Execution Plan (DDL) ]
                                       │
                              [ Transactional Apply ]
```

### What Makes pg-flux Different

**1. Hint-Based Rename Resolution**
Developers annotate rename intent directly in SQL using structured comments:
```sql
-- @renamed from=name
full_name TEXT NOT NULL,
```
The differ maps this as an `ALTER TABLE ... RENAME COLUMN` operation, preserving all data. No other open-source declarative migration tool provides this.

**2. Deep PL/pgSQL & RLS Parsing**
Using the official PostgreSQL C parser (`pg_query_go`), pg-flux builds a normalized, semantic AST of every function body and policy expression. Comparison is done on the normalized form, eliminating false positives from whitespace or parenthesis differences.

**3. Hazard Detection Engine**
Every generated DDL statement is analyzed before execution. Any statement that would acquire a long-held Access Exclusive lock, cause data loss, or require a full table scan triggers a named `Hazard`. The migration is blocked unless the operator explicitly acknowledges the hazard via CLI flag. Hazards can also trigger alternative zero-downtime rewrites automatically (e.g., `CREATE INDEX` → `CREATE INDEX CONCURRENTLY`).

**4. Topological DAG Ordering**
A Directed Acyclic Graph engine models the dependency relationships between all schema objects. The execution plan is guaranteed to be in valid dependency order — no "function does not exist" errors during complex deployments.

**5. PostgreSQL 18 Nativity**
- Reads the new `pg_constraint`-based `NOT NULL` storage model.
- Understands `WITHOUT OVERLAPS` temporal constraints.
- Natively handles `uuidv7()` default expressions.
- Injects `ANALYZE` post-migration to preserve PG18's enhanced optimizer statistics.
- Considers `io_method` settings for DDL batching on large tables.

**6. AI Integration Hooks**
`--dry-run --format=json` emits the full diff and proposed execution plan as structured JSON, enabling external AI agents to review the semantic correctness of a migration before it runs.

---

## 1.4 Success Metrics

The following metrics define what "success" means for the v1.0 release:

| Metric | Target |
|--------|--------|
| Rename operations correctly detected (no data loss) | 100% of annotated renames |
| False-positive RLS diffs per migration run | 0 |
| Hazards missed (unlocked destructive operations) | 0 |
| Topological ordering failures on complex schemas | 0 |
| Parse time for 1,000-table schema | < 500ms |
| State inspection time for 5,000-relation database | < 2s |
| Binary size (all platforms) | < 35MB |
| CI exit code accuracy (drift vs. no-drift) | 100% |

---

## 1.5 Out of Scope for v1.0

The following items are explicitly **not** in scope for the initial release. They are candidates for future versions.

| Item | Rationale |
|------|-----------|
| Data migrations (transforming row data) | This tool manages schema structure only. Data seeding/transformation belongs in application code or separate tooling. |
| Multi-database support (MySQL, SQLite, etc.) | PG18 specificity is a core design constraint and competitive advantage. |
| Shadow table techniques (online table rewrites) | Adds significant complexity; pt-online-schema-change handles this use case. |
| Native GUI / web dashboard | CLI-first; integrations via JSON output. |
| Column-level encryption policy management | Future security module. |
| Streaming replication topology awareness | Post-v1.0 operational feature. |
| Automatic `pg_upgrade` integration | Separate tooling concern. |

---

## 1.6 Relationship to Existing Tools

pg-flux does not replace all database tooling. It occupies a specific, well-defined layer:

```
[ Application Code ]
       │
[ pg-flux: Schema State Management ]   ← pg-flux lives here
       │
[ pg_dump / pg_restore: Backup/Restore ]
       │
[ pgbouncer / pgpool: Connection Management ]
       │
[ PostgreSQL 18 Engine ]
```

It integrates with but does not replace:
- **pganalyze / DataDog:** Runtime query performance monitoring.
- **pg_dump:** Backup and point-in-time recovery.
- **pt-online-schema-change / pg_repack:** Online table rewriting for large-scale data migrations.
- **Vault / AWS Secrets Manager:** Credential management (pg-flux reads from env vars).
