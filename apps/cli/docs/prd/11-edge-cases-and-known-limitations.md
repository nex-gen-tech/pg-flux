# 11 — Edge Cases & Known Limitations

**Document:** Edge Cases, Boundary Conditions & Known Limitations
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## Overview

This document catalogs every known edge case in the pg-flux domain. Each entry specifies the scenario, expected behavior, and whether it is supported in v1.0 or deferred.

**Legend:**
- ✅ Supported in v1.0
- ⚠️ Partially supported with caveats
- ❌ Not supported (deferred to v1.1 or later)

---

## 11.1 Parser Edge Cases

### EC-01: Quoted vs. Unquoted Identifiers
✅ **Supported**

PostgreSQL distinguishes between `"MyTable"` (case-sensitive, preserves case) and `mytable` (case-folded to lowercase). These are different identifiers.

```sql
CREATE TABLE "MyTable" (id INT);   -- table name: MyTable
CREATE TABLE MyTable (id INT);     -- table name: mytable
```

pg-flux preserves the quoted/unquoted distinction exactly as written. A quoted identifier is stored as `"MyTable"` and compared to the live catalog where `relname = 'MyTable'`.

**Edge sub-case:** If the desired schema defines `"MyTable"` and the live schema has `mytable`, this is treated as a new table + drop, not a rename. To rename, use `-- @renamed from="mytable"`.

---

### EC-02: Unicode Identifiers
✅ **Supported**

PostgreSQL allows Unicode characters in identifiers (e.g., `CREATE TABLE "données_utilisateur" (...)`). pg_query_go correctly handles UTF-8 identifiers because it uses the real PostgreSQL parser.

pg-flux passes all identifiers through unchanged. No normalization is applied to Unicode identifiers.

---

### EC-03: Files with Only Comments
✅ **Supported**

A `.sql` file containing only SQL comments (e.g., `-- License header`) is valid. It produces an empty parse result and no objects, which is a no-op. The tool does not error on empty-but-valid files.

---

### EC-04: Duplicate Object Definitions Across Files
✅ **Validated (returns error)**

If two `.sql` files both define a `CREATE TABLE users`, the source parser returns: `"duplicate definition: table public.users defined in tables/users.sql and tables/users_v2.sql"`.

---

### EC-05: `\i` Include Directives
❌ **Not Supported**

PostgreSQL's `psql` tool supports `\i filename` includes. The pg_query_go C parser does not process `\i` directives (it's a psql client feature). If a file contains `\i`, it will fail to parse.

**Recommendation:** Use pg-flux's native directory scanning (`--schema-dir=<path>`) instead of `\i` includes.

---

### EC-06: Dollar-Quoted Function Bodies
✅ **Supported**

Function bodies using dollar-quoting (`$$ ... $$` or `$body$ ... $body$`) are parsed correctly by the PostgreSQL C parser.

```sql
CREATE FUNCTION get_user(p_id UUID) RETURNS users AS $$
BEGIN
    RETURN (SELECT * FROM users WHERE id = p_id);
END;
$$ LANGUAGE plpgsql;
```

---

### EC-07: Functions with Unnamed Parameters (`$1`, `$2`)
✅ **Supported**

Functions can be defined with unnamed parameters referenced as `$1`, `$2`. The parser captures these as unnamed parameters. The differ uses the full function signature (name + parameter types + return type) as the identity key.

---

## 11.2 Inspector Edge Cases

### EC-08: Logically Dropped Columns
✅ **Handled (excluded automatically)**

PostgreSQL marks dropped columns in `pg_attribute` with `attisdropped = true`. These columns still occupy space in the system catalog but are invisible to users. The inspector excludes all rows where `attisdropped = true`.

---

### EC-09: Inherited Tables (Table Inheritance)
⚠️ **Partial Support**

PostgreSQL's table inheritance (`CREATE TABLE child () INHERITS (parent)`) creates complex relationships in system catalogs. The inspector captures parent and child tables as independent objects.

**Limitation:** pg-flux does not model inheritance relationships in the dependency graph. Changes to parent table columns are not automatically propagated to child tables in the execution plan. v1.0 treats them as independent tables.

---

### EC-10: Partitioned Tables
⚠️ **Partial Support (parent only)**

Partitioned tables (`CREATE TABLE orders ... PARTITION BY RANGE (created_at)`) are supported at the parent level. pg-flux inspects the partition declaration (`relkind = 'p'`) and the partition strategy.

**Limitations:**
- Individual partition creation/deletion is NOT diffed. Partitions are user-managed.
- Changing the partitioning strategy (e.g., RANGE → LIST) is treated as a `DROP TABLE + CREATE TABLE` (DATA_LOSS hazard).
- Partition index management (each partition gets an index copy) is tracked separately per partition.

---

### EC-11: Foreign Tables (FDW)
❌ **Not Supported**

Foreign tables created via `CREATE FOREIGN TABLE ... SERVER ...` are excluded from inspection and diffing. They require external FDW servers and connection parameters that cannot be managed declaratively.

---

### EC-12: Extension-Owned Objects
✅ **Excluded Automatically**

Objects owned by PostgreSQL extensions (identified by `pg_depend.classid` linking to the extension OID) are excluded from inspection. For example, `uuid-ossp` creates its own functions and types; these are not diffed.

**Edge case:** If a user creates a table that uses an extension-provided type (e.g., `PostGIS geometry`), the column type is captured as a string reference to the type name. pg-flux does not manage the type itself.

---

### EC-13: Views That Reference Each Other (View Dependencies)
✅ **Handled via DAG**

If view `B` depends on view `A` (i.e., `B`'s definition references `A`), the DAG ensures:
- On creation: `A` is created before `B`.
- On deletion: `B` is dropped before `A`.
- On modification of `A`: a `CREATE OR REPLACE VIEW A` does not require dropping `B` unless the columns change.

---

### EC-14: Materialized Views and CONCURRENT Refresh
⚠️ **Inspect only in v1.0**

Materialized views (`CREATE MATERIALIZED VIEW`) are inspected. The differ detects changes to their definition. However:
- pg-flux does NOT generate `REFRESH MATERIALIZED VIEW [CONCURRENTLY]` as part of migrations.
- Changing a materialized view's definition requires `DROP MATERIALIZED VIEW + CREATE MATERIALIZED VIEW`.
- `CONCURRENTLY` refresh requires a unique index on the materialized view; the differ warns if a materialized view is created without a unique index.

---

## 11.3 Differ Edge Cases

### EC-15: Self-Referential Foreign Keys
✅ **Supported**

Tables with a FK that references themselves (e.g., `category` with `parent_id INT REFERENCES category(id)`) are handled correctly. The DAG creates the table first, then the FK constraint in a second step.

---

### EC-16: Mutual Foreign Keys (A → B and B → A)
✅ **Detected, returns error**

PostgreSQL supports this via `DEFERRABLE INITIALLY DEFERRED` constraints. The DAG detects this cycle and handles it by:
1. Creating both tables without their mutual FK constraints.
2. Adding both FK constraints after both tables exist.

If the FKs are non-deferrable, PostgreSQL itself will reject the circular reference. pg-flux follows PostgreSQL's rules.

---

### EC-17: Column Rename + Type Change in Same Migration
✅ **Supported (ordered correctly)**

When a column is both renamed AND has its type changed in the same migration:
1. `ALTER TABLE ... RENAME COLUMN old_name TO new_name` (no lock contention)
2. `ALTER TABLE ... ALTER COLUMN new_name TYPE ...` (COLUMN_TYPE_CHANGE hazard if unsafe)

The DAG ensures the rename happens first to avoid a "column not found" error on the type change.

---

### EC-18: Primary Key Change
⚠️ **Supported with caveats**

Changing a primary key (e.g., adding a column to a composite PK, or changing the PK column) requires:
1. `DROP CONSTRAINT pk_name` — drops the old PK.
2. `ADD CONSTRAINT pk_name PRIMARY KEY (new_columns)` — creates the new PK.

**Limitation:** Dropping a PRIMARY KEY takes an exclusive lock on the table. There is no zero-downtime path for PK changes. This is surfaced as a `TABLE_LOCK` hazard with an advisory note.

---

### EC-19: Enum Type Modifications
⚠️ **Add-only in v1.0**

PostgreSQL allows adding enum values but not removing them without recreating the type.

- **Adding enum values:** ✅ `ALTER TYPE status ADD VALUE 'pending_review'` generated automatically.
- **Removing enum values:** The `ENUM_VALUE_DROP` hazard blocks this. The tool does not auto-generate the DROP+RECREATE+MIGRATE sequence in v1.0.

---

### EC-20: Column Default Expression Changes
✅ **Supported**

Changing a column default (e.g., from `DEFAULT NULL` to `DEFAULT uuidv7()`) generates `ALTER TABLE ... ALTER COLUMN ... SET DEFAULT ...`. The differ uses `pg_query.Fingerprint()` to normalize the expression and avoid false-positive diffs.

---

### EC-21: Generated Column Expression Changes
✅ **Supported (with DATA_LOSS hazard)**

Changing the expression of a generated column (virtual or stored) requires:
1. `ALTER TABLE ... DROP COLUMN old_generated_col`
2. `ALTER TABLE ... ADD COLUMN new_generated_col GENERATED ALWAYS AS (...)`

This is a `DATA_LOSS` hazard for stored generated columns (data is dropped). For virtual generated columns, there is no data loss (the expression is recomputed at read time), but it still requires DROP+ADD.

---

## 11.4 Execution Edge Cases

### EC-22: Statement Timeout During `CREATE INDEX CONCURRENTLY`
✅ **Handled with guidance**

If `CREATE INDEX CONCURRENTLY` is interrupted (timeout, `SIGINT`, connection loss), PostgreSQL leaves the index as INVALID. The inspector detects INVALID indexes (`pg_index.indisvalid = false`) on the next run and:
- Shows the INVALID index in the plan diff.
- Recommends: `DROP INDEX CONCURRENTLY idx_name` before recreating.

---

### EC-23: Advisory Lock Acquisition Failure
✅ **Fails fast with clear error**

If another `pg-flux apply` is running, `pg_try_advisory_lock()` returns false. pg-flux immediately exits with:
```
Error: advisory lock could not be acquired — another migration may be in progress.
       If no migration is running, release the lock manually:
       SELECT pg_advisory_unlock(pg_flux_lock_key());
```

---

### EC-24: Dry-Run on Production
✅ **Supported**

`pg-flux apply --dry-run` runs the entire plan against a shadow database without touching the live database. This is safe to run against production for validation.

---

## 11.5 Known Limitations

| # | Limitation | Deferred To |
|---|-----------|-------------|
| L-01 | `\i` include directives in SQL files not supported | v1.1 |
| L-02 | Partitioned table partition management not included in diff | v1.1 |
| L-03 | Foreign tables (FDW) not supported | v1.1 |
| L-04 | Enum value removal not auto-generated (hazard only) | v1.1 |
| L-05 | Materialized view refresh not automated | v1.1 |
| L-06 | PG18-specific SQL syntax requires `pg_query_go/v7` | When pg_query_go v7 is released |
| L-07 | Aggregates and window functions inspected but not diffed | v1.1 |
| L-08 | Windows OS not supported (CGO limitation) | Indefinite / v2 |
| L-09 | Multi-schema FK dependencies require all schemas to be in scope | v1.0 (already supported) |
| L-10 | Declarative partitioning strategy changes require manual intervention | v1.1 |
| L-11 | Large object (lo) types and management | Not planned |
| L-12 | Publication/subscription (logical replication) objects | Not planned |
| L-13 | pg_cron jobs | Not planned (out of scope) |
