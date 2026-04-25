# 05 — PostgreSQL 18 Specific Features

**Document:** PostgreSQL 18 Feature Support & Optimizations
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## Overview

pg-flux is purpose-built for PostgreSQL 18. This document specifies how each significant PG18 feature affects the parser, state inspector, differ, and execution plan generator. It also documents PG18-specific optimizations in plan generation.

> **Release Date:** PostgreSQL 18 was released 2025-09-25.

---

## 5.1 Native `uuidv7()` Support

### What Changed in PG18
PostgreSQL 18 introduces the built-in function `uuidv7()` that generates time-ordered UUID version 7 values. This is a native replacement for `uuid_generate_v7()` (from the `uuid-ossp` extension) and the `gen_random_uuid()` function.

```sql
-- PG18 native (preferred)
id uuid DEFAULT uuidv7() PRIMARY KEY,

-- Legacy (still works but discouraged for new schemas)
id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
```

### UUIDv7 vs UUIDv4 — Why It Matters
- **UUIDv7 is timestamp-prefixed:** The first 48 bits are a millisecond-precision Unix timestamp. This means rows with `DEFAULT uuidv7()` are naturally sorted by insertion time in B-tree indexes, eliminating random write amplification.
- **Index fragmentation:** UUIDv4 causes severe B-tree fragmentation because values are random. UUIDv7 inserts are sequential, leading to significantly lower page splits and WAL write volume.

### pg-flux Requirements

- [ ] The source parser correctly recognizes `uuidv7()` and `uuidv4()` as distinct expressions. They are **not** semantically equivalent.
- [ ] The state inspector captures `DEFAULT uuidv7()` from `pg_attrdef` via `pg_get_expr()` and normalizes it identically to the source.
- [ ] The differ does not generate a false-positive diff when a column's default is `uuidv7()` on both sides.
- [ ] `engine init` generates example tables using `uuidv7()` by default.
- [ ] When inspecting a PG17 or older database schema and re-applying to PG18, the `uuid_generate_v7()` → `uuidv7()` migration path is documented (migration hint in `engine inspect` output).

---

## 5.2 Asynchronous I/O Subsystem (`io_uring`-style)

### What Changed in PG18
PostgreSQL 18 introduces a new asynchronous I/O subsystem controlled by the `io_method` server variable. With `io_method = io_uring` (on supported Linux kernels) or `io_method = worker`, PostgreSQL can queue multiple concurrent read requests, dramatically improving:
- Sequential scan throughput
- Bitmap heap scan performance
- VACUUM and ANALYZE speed
- `CREATE INDEX` build time

The `io_combine_limit` and `io_max_combine_limit` variables control how I/O requests are batched.

### Impact on pg-flux

**Batching large DDL operations:**
When executing migrations on databases with large tables (estimated via `pg_class.relpages > 10000`), pg-flux should:

1. **Inject `ANALYZE` with enhanced concurrency settings:** Before running `VACUUM`/`ANALYZE` post-migration, set `maintenance_io_concurrency` to a higher value if the server supports it.

2. **Emit advisory notes on `io_method` setting:** When the inspector detects `io_method = sync` (the legacy default), emit an advisory suggestion to switch to `io_method = io_uring` for improved migration performance.

**DDL Batching Hint:**
For migrations involving multiple `CREATE INDEX CONCURRENTLY` statements on large tables, pg-flux should serialize them rather than suggest parallel execution, since concurrent index builds already saturate I/O. The DAG sorter adds sequential ordering hints for concurrent builds on the same table.

### pg-flux Requirements

- [ ] The executor queries `SHOW io_method` at connection time and records it in the execution context.
- [ ] If `io_method = sync` and large tables (>1GB estimated) are involved, emit an advisory warning suggesting enabling `io_uring` or `worker` mode.
- [ ] Post-migration `ANALYZE` is injected for all tables structurally modified by the migration.
- [ ] The `ANALYZE` injection sets `maintenance_work_mem` appropriately for large table analysis.

---

## 5.3 `pg_upgrade` Optimizer Statistics Retention

### What Changed in PG18
`pg_upgrade` in PG18 now preserves optimizer statistics by default. Previously, after a major version upgrade, all statistics were discarded and queries would use sub-optimal plans until `ANALYZE` was run.

### Impact on pg-flux

pg-flux cannot detect whether statistics are stale (post-upgrade or post-migration), but it can **inject preventive `ANALYZE` statements** to ensure statistics are always refreshed after structural migrations.

**`ANALYZE` Injection Rules:**

The differ generates `ANALYZE {table}` statements at the end of the execution plan for any table that has:
- Had columns added or removed
- Had its primary key changed
- Had its clustering index changed
- Had `ALTER TABLE ... SET UNLOGGED` applied

**Optimizer Statistics for New Indexes:**
After `CREATE INDEX CONCURRENTLY` completes, the new index is immediately available, but the planner may not have statistics for it yet. pg-flux injects `ANALYZE {table}` after each new index build to ensure the planner can use the new index statistics.

### pg-flux Requirements

- [ ] `ANALYZE` is always appended after any structural DDL that changes data layout.
- [ ] `ANALYZE` statements are grouped by table (one `ANALYZE table_name` per table, regardless of how many changes were made to it).
- [ ] `ANALYZE` runs within the execution plan (not outside the advisory lock), ensuring it completes before the lock is released.

---

## 5.4 Named `NOT NULL` Constraints (PG18 Catalog Change)

### What Changed in PG18
PostgreSQL 18 changes the storage model for `NOT NULL` constraints. Previously, `NOT NULL` was stored as a boolean flag on `pg_attribute.attnotnull`. In PG18, `NOT NULL` constraints can be **named** and are stored as entries in `pg_constraint` with `contype = 'n'`.

This has major implications for schema diffing:

```sql
-- PG18 named NOT NULL (from pg_constraint)
ALTER TABLE users ADD CONSTRAINT users_email_not_null NOT NULL email;

-- Equivalent (unnamed NOT NULL, still works but stored differently)
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
```

### pg-flux Requirements

**Inspector changes:**
- [ ] Query both `pg_attribute.attnotnull` AND `pg_constraint WHERE contype = 'n'` to build a complete picture of NOT NULL constraints.
- [ ] Merge these two sources: a column marked NOT NULL in `pg_attribute` but with no corresponding `pg_constraint` entry is a legacy unnamed NOT NULL (pre-PG18 behavior).
- [ ] For named NOT NULL constraints, preserve the constraint name in the `Constraint` struct.

**Differ changes:**
- [ ] When a named NOT NULL constraint is added in the desired schema, generate the named constraint form: `ALTER TABLE ... ADD CONSTRAINT {name} NOT NULL {column}`.
- [ ] When an unnamed NOT NULL is present in the desired schema (i.e., `column_name TEXT NOT NULL` without an explicit constraint name), generate the standard form.
- [ ] When a named NOT NULL constraint is renamed, generate `ALTER CONSTRAINT ... RENAME TO ...` (PG18 supports this).

**Hazard Integration:**
- [ ] Named NOT NULL with `NOT VALID` is supported in PG18: `ALTER TABLE ... ADD CONSTRAINT ... NOT NULL ... NOT VALID`. The hazard engine uses this 3-step pattern instead of the 4-step check pattern.

---

## 5.5 Temporal Constraints (`WITHOUT OVERLAPS`)

### What Changed in PG18
PostgreSQL 18 introduces temporal primary key and unique constraints using `WITHOUT OVERLAPS`, and temporal foreign keys using `PERIOD`. These allow modeling time-ranged data with native database constraint enforcement.

```sql
-- Temporal primary key (PG18)
CREATE TABLE reservations (
    room_id     INT,
    during      tsrange,
    CONSTRAINT reservations_room_period_pk
        PRIMARY KEY (room_id, during WITHOUT OVERLAPS)
);

-- Temporal foreign key (PG18)
CREATE TABLE bookings (
    room_id     INT,
    during      tsrange,
    CONSTRAINT bookings_room_fk
        FOREIGN KEY (room_id, PERIOD during)
        REFERENCES reservations (room_id, PERIOD during)
);
```

### pg-flux Requirements

- [ ] The source parser recognizes `WITHOUT OVERLAPS` in `PRIMARY KEY` and `UNIQUE` constraint definitions.
- [ ] The source parser recognizes `PERIOD` in `FOREIGN KEY` constraint definitions.
- [ ] The state inspector queries `pg_constraint.conperiod` to detect temporal constraints in the live database.
- [ ] The differ correctly identifies added/removed temporal constraints.
- [ ] Temporal constraints are treated as regular `CREATE CONSTRAINT` / `DROP CONSTRAINT` changes, with hazard detection applied.
- [ ] `engine inspect` outputs temporal constraints in the correct PG18 syntax.

---

## 5.6 Virtual Generated Columns (PG18 Default)

### What Changed in PG18
PostgreSQL 18 introduces **virtual generated columns** and makes them the default for generated columns. Previously, all generated columns were stored (materialized). Virtual generated columns compute their values at read time, not write time.

```sql
-- PG18 virtual (default — does not need VIRTUAL keyword)
full_name TEXT GENERATED ALWAYS AS (first_name || ' ' || last_name),

-- PG18 stored (explicit)
full_name TEXT GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED,
```

### pg-flux Requirements

- [ ] The source parser distinguishes between `GENERATED ALWAYS AS (...) VIRTUAL` and `GENERATED ALWAYS AS (...) STORED`.
- [ ] When `STORED`/`VIRTUAL` is omitted, the parser assumes `VIRTUAL` (PG18 default behavior).
- [ ] The state inspector reads `pg_attribute.attgenerated`: `'v'` = virtual, `'s'` = stored.
- [ ] The differ detects changes between virtual and stored generated columns as a `COLUMN_TYPE_CHANGE` hazard (requires full column drop/recreate).
- [ ] The differ detects changes to the generation expression as an `ALTER COLUMN` (drop old, add new generated spec).

---

## 5.7 Non-Enforced Constraints

### What Changed in PG18
PostgreSQL 18 adds `NOT ENFORCED` support for `CHECK` and foreign key constraints. These constraints are defined in the schema but are not validated — they exist primarily for documentation and query optimizer hints.

```sql
-- Non-enforced check (for optimizer hints only)
CONSTRAINT age_positive CHECK (age > 0) NOT ENFORCED,

-- Non-enforced foreign key
CONSTRAINT orders_user_fk
    FOREIGN KEY (user_id) REFERENCES users(id) NOT ENFORCED,
```

### pg-flux Requirements

- [ ] The source parser captures the `NOT ENFORCED` attribute.
- [ ] The state inspector queries `pg_constraint.conenforced` (new PG18 column) to detect enforcement status.
- [ ] The differ detects changes between `ENFORCED` and `NOT ENFORCED` as constraint modifications.
- [ ] Changing from `NOT ENFORCED` to `ENFORCED` triggers the `CONSTRAINT_SCAN` hazard (validation run required).

---

## 5.8 Skip Scan Index Optimization

### What Changed in PG18
PostgreSQL 18 adds "skip scan" support for B-tree indexes. This allows multi-column B-tree indexes to be used even when there are no equality conditions on the leading column(s).

### Impact on pg-flux

This affects index diffing and index recommendation:
- [ ] When inspecting an existing multi-column index, pg-flux does not need to recommend additional single-column indexes on leading columns (where PG17 required them) if PG18 skip scan makes the multi-column index more selective.
- [ ] `engine inspect` adds a comment to multi-column indexes explaining that PG18 skip scan is available.
- [ ] Index hazard messages are updated: "This index covers multiple columns. In PostgreSQL 18, skip scans allow this index to be used without an equality condition on leading columns."

---

## 5.9 GIN Parallel Index Builds

### What Changed in PG18
PostgreSQL 18 allows GIN indexes to be built in parallel (previously only B-tree and BRIN supported parallel builds).

### Impact on pg-flux

- [ ] The execution plan for `CREATE INDEX CONCURRENTLY` on a GIN index now benefits from parallel worker allocation.
- [ ] pg-flux adds an advisory note to GIN index builds: "GIN index builds can use parallel workers in PG18. Ensure `max_parallel_maintenance_workers` is configured appropriately."
- [ ] `ANALYZE` after a GIN index build is included in the plan.

---

## 5.10 `RETURNING OLD` / `RETURNING NEW` Support

### What Changed in PG18
PG18 adds `OLD` and `NEW` aliases in `RETURNING` clauses for `INSERT`, `UPDATE`, `DELETE`, and `MERGE`. This is primarily an application-layer feature.

### Impact on pg-flux

pg-flux is a DDL migration tool; DML syntax changes do not directly affect schema diffing. However:
- [ ] The source parser must not choke on `RETURNING OLD.*` or `RETURNING NEW.*` syntax in function bodies or view definitions.
- [ ] PL/pgSQL function bodies containing these new syntax forms are parsed correctly via `pg_query.ParsePlPgSqlToJSON()`.

---

## 5.11 Catalog Breaking Changes in PG18

The following PG18 catalog changes **break** any tool that directly copies catalog queries from PG17 or older:

| Change | Impact on pg-flux |
|--------|------------------|
| `pg_attribute.attcacheoff` removed | Any inspector query selecting this column fails. Must remove from all queries. |
| `pg_class.relallfrozen` added | New column available for freeze status monitoring. Not used in core diffing but available for monitoring hints. |
| `pg_constraint.conenforced` added | Used for non-enforced constraint detection (FR above). |
| `pg_constraint` gains type `'n'` (NOT NULL) | Inspector must handle this new constraint type. |
| `pg_constraint.conperiod` added | Used for temporal constraint detection. |
| `VACUUM`/`ANALYZE` changes: partition children processed by default | `engine apply` must use `ANALYZE ONLY` on parent tables to avoid re-analyzing all partitions after a single parent-level change. |
| `pg_stat_wal` loses `wal_write`/`wal_sync` columns | Monitoring queries against this view must be updated. |

All catalog queries in the inspector module must be tested against a PG18 instance as part of the CI pipeline.
