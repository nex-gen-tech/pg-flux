# 04 — Functional Requirements

**Document:** Core Functional Requirements
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## Overview

This document defines all functional requirements (FRs) for pg-flux v1.0. Each requirement includes a description, motivation, acceptance criteria, and edge cases.

Requirements are prioritized:
- **P0:** Must ship in v1.0. Blocking.
- **P1:** Should ship in v1.0. High value.
- **P2:** Nice to have in v1.0; acceptable to defer to v1.1.

---

## FR-01: High-Fidelity Declarative SQL Parsing (P0)

**Description:** The tool must accept a directory of `.sql` files or a single `schema.sql` file as the absolute source of truth for the desired schema state.

**Motivation:** The parser is the foundation of the entire system. An imprecise parser produces incorrect diffs. Using `pg_query_go` (which wraps the actual PostgreSQL C parser) guarantees 100% dialect accuracy for PostgreSQL 18 syntax.

**Acceptance Criteria:**

- [ ] The tool accepts `--schema-dir=<path>` to specify a directory. All `.sql` files are loaded recursively.
- [ ] The tool accepts `--schema-file=<path>` to specify a single SQL file.
- [ ] All PostgreSQL 18 DDL constructs are parsed correctly, including:
  - `CREATE TABLE ... WITHOUT OVERLAPS` (temporal constraints)
  - `GENERATED ALWAYS AS (...) VIRTUAL` (virtual generated columns, PG18 default)
  - `CONSTRAINT ... NOT NULL` (named NOT NULL constraints, PG18)
  - `FOREIGN KEY ... NOT ENFORCED` (non-enforced constraints, PG18)
  - `CHECK ... NOT ENFORCED` (non-enforced check constraints, PG18)
  - `DEFAULT uuidv7()` expressions
  - `RETURNING OLD.*`, `RETURNING NEW.*` clauses (PG18)
- [ ] Syntax errors from `pg_query.Parse()` are returned with the source file path and 1-based line number.
- [ ] Files are parsed in deterministic order (lexicographic sort by relative path). Object conflicts across files (same table defined twice) return a validation error.
- [ ] Hint comments (`-- @renamed`, `-- @deprecated`) are extracted and attached to their parent AST nodes.
- [ ] PL/pgSQL function bodies are also parsed via `pg_query.ParsePlPgSqlToJSON()` for semantic comparison.

**Edge Cases:**
- File with a `.sql` extension that contains only comments: parse as empty, no-op.
- Unicode in identifiers (PostgreSQL allows Unicode names): must be handled correctly.
- Quoted identifiers vs. unquoted: `"MyTable"` and `mytable` are different — the parser must preserve case-sensitivity exactly as written.
- Files with Windows line endings (CRLF): must be normalized before parsing.
- Circular `\i` includes in SQL files: not supported; return a clear error.
- Empty schema directory: valid; produces "no changes" diff against any live database.

---

## FR-02: Live Schema Inspection from System Catalogs (P0)

**Description:** The tool must connect to a live PostgreSQL 18 database and construct a complete, accurate representation of its current schema by directly querying system catalog tables.

**Motivation:** The `information_schema` view lacks the detail needed for high-fidelity diffing. Directly querying `pg_class`, `pg_attribute`, `pg_constraint`, etc., provides the exact stored form of every schema object.

**Acceptance Criteria:**

- [ ] Inspects all tables in the target schema (default: `public`). `--schemas=a,b,c` allows multi-schema inspection.
- [ ] Inspects all column types, including array types (`TEXT[]`), domain types, and composite types.
- [ ] Inspects all constraints: PK, FK, UNIQUE, CHECK, NOT NULL (from both `pg_attribute.attnotnull` and `pg_constraint` for PG18 named NOT NULL).
- [ ] Inspects all index definitions, including expression indexes, partial indexes (with `WHERE` clause), and operator class specifications.
- [ ] Inspects all PL/pgSQL function signatures and bodies (via `pg_proc.prosrc`).
- [ ] Inspects all trigger definitions, including timing (`BEFORE`/`AFTER`/`INSTEAD OF`), events, and target function.
- [ ] Inspects all RLS policies, including `polcmd`, `polroles`, and the expression trees of `polqual` (USING) and `polwithcheck` (WITH CHECK) via `pg_get_expr()`.
- [ ] Inspects all sequences, including `INCREMENT BY`, `START WITH`, `MIN/MAXVALUE`, `CACHE`, and owning column.
- [ ] Inspects views (regular and materialized).
- [ ] System schemas (`pg_*`, `information_schema`) are excluded automatically.
- [ ] Uses parameterized queries exclusively — no string concatenation of schema/table names into query text.
- [ ] All catalog queries run concurrently using Go goroutines.

**Edge Cases:**
- Partitioned tables: inspect parent + children; emit diffing results for the parent definition. Partition creation/deletion is modeled as a separate change type.
- Tables with no columns (edge case in PG, but valid): must not panic.
- Functions with `$n` parameter references (unnamed parameters): captured correctly.
- Aggregates and window functions: captured but tagged as non-diff-able in v1.0 (emit informational note).
- Views that reference functions that are also in the schema: both objects captured; dependency established in DAG.
- Tables with `pg_attribute.attisdropped = true` columns: excluded from inspection (these are logically deleted columns).

---

## FR-03: Hint-Based Rename Resolution (P0)

**Description:** Developers annotate rename intent in SQL source files using structured comments. The differ uses these hints to distinguish column/table renames from drop/add pairs.

**Motivation:** Without rename hints, any declarative differ will interpret a column rename as DROP old + ADD new, destroying all data in that column. This is the most critical safety feature of pg-flux.

**Hint Syntax:**
```sql
CREATE TABLE users (
    id uuid DEFAULT uuidv7() PRIMARY KEY,
    -- @renamed from=name
    full_name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL
);
```

For table renames:
```sql
-- @renamed from=user_accounts
CREATE TABLE users (
    ...
);
```

**Acceptance Criteria:**

- [ ] `-- @renamed from=<identifier>` on the line immediately before a column definition is recognized as a column rename hint.
- [ ] `-- @renamed from=<identifier>` on the line immediately before a `CREATE TABLE` statement is recognized as a table rename hint.
- [ ] The `from=` value must be a valid unquoted or quoted PostgreSQL identifier.
- [ ] If the source identifier does not exist in the live schema, the tool returns an error: `"rename source '<old_name>' not found in live schema for table '<table>'."` — it does NOT silently create a new column.
- [ ] If the source identifier already matches an existing column with the new name (i.e., the rename was already applied), the hint is a no-op.
- [ ] Multiple renames within the same table are all resolved correctly, independently.
- [ ] Circular renames (A renamed to B, B renamed to A) return a validation error.
- [ ] The `-- @deprecated` hint marks an object as "will be dropped in a future migration" — it is NOT diffed. Objects with `@deprecated` are excluded from the current execution plan but logged as informational.

**Edge Cases:**
- Rename hint on a column that also changes type: rename is applied first (as `ALTER TABLE ... RENAME`), then the type change is a separate statement.
- Rename hint with incorrect `from=` value (typo): returns a clear error with the misspelled name and a suggestion.
- Rename hint followed by a DROP in the same migration (rename then immediate drop): resolve rename, then drop.

---

## FR-04: Topological Dependency Sorting (P0)

**Description:** The differ must order all generated DDL statements according to the dependency graph of the schema objects they operate on.

**Acceptance Criteria:**

- [ ] Creation DDL is always ordered: Tables → Functions → Triggers → Foreign Keys → Non-null Constraints → RLS Enable → Policies → Views.
- [ ] Deletion DDL is always ordered in reverse: Views → Policies → RLS Disable → Constraints → Triggers → Functions → Tables.
- [ ] Within a single table: column additions precede constraint additions that reference those columns.
- [ ] Indexes that reference expressions or functions are created after those functions exist.
- [ ] Foreign key constraints are created after both the referencing and referenced tables and their primary keys are created.
- [ ] The DAG detects and reports cycles (should not occur in valid PostgreSQL schema).
- [ ] Concurrent operations (e.g., `CREATE INDEX CONCURRENTLY`) are sorted to run after all transactional DDL on the same table, because `CONCURRENTLY` cannot run inside a transaction.

**Edge Cases:**
- Mutual foreign keys (table A has FK to B, B has FK to A): not supported in PostgreSQL's own schema, but the tool should detect this and return a clear error instead of infinite-looping.
- Self-referencing foreign keys (table A has FK to itself): supported; order is CREATE TABLE, then ADD CONSTRAINT FK.
- Function that depends on a type that is also being created in the same migration: type → function ordering enforced.

---

## FR-05: RLS and Security Policy Management (P0)

**Description:** Row-Level Security policies must be treated as first-class schema objects, diffed and patched with the same precision as tables and indexes.

**Acceptance Criteria:**

- [ ] RLS policies are parsed from source `.sql` files including:
  - `CREATE POLICY ... ON ... AS ... FOR ... TO ... USING (...) WITH CHECK (...)`
  - `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`
  - `ALTER TABLE ... FORCE ROW LEVEL SECURITY`
- [ ] Policy expressions (`USING`, `WITH CHECK`) are normalized using `pg_query.Fingerprint()` before comparison to eliminate false-positive diffs from formatting differences.
- [ ] When a `USING` or `WITH CHECK` expression changes semantically, the plan generates a `DROP POLICY` + `CREATE POLICY` sequence (PostgreSQL does not support `ALTER POLICY` to change the expression in all cases).
- [ ] When only the `TO` (roles) list changes, an `ALTER POLICY ... TO ...` is generated.
- [ ] `ENABLE/DISABLE ROW LEVEL SECURITY` changes are detected on the table level.
- [ ] `FORCE/NO FORCE ROW LEVEL SECURITY` changes are detected on the table level.
- [ ] Policies are dropped before the tables they belong to, and created after.

**Edge Cases:**
- Policy that references a function in its `USING` clause: the function must be created/updated before the policy is applied.
- Multiple policies on the same table with the same name: PostgreSQL does not allow this; the source parser returns a validation error.
- Policy `FOR ALL` vs. separate `FOR SELECT`, `FOR INSERT`, etc.: these are distinct policies; both the policy name and `FOR` clause are used as the identity key.

---

## FR-06: Zero-Downtime Hazard Detection & Rewriting (P0)

**Description:** The hazard detection engine intercepts dangerous DDL and either rewrites it into a safe equivalent or emits a named hazard requiring explicit acknowledgment.

**Full Hazard Registry:**

| Hazard ID | Trigger | Auto-Rewrite | CLI Flag |
|-----------|---------|-------------|---------|
| `DATA_LOSS` | `DROP TABLE`, `DROP COLUMN`, `DROP SCHEMA` | None | `--allow-hazards DATA_LOSS` |
| `TABLE_LOCK` | `CREATE INDEX` (non-concurrent) | → `CREATE INDEX CONCURRENTLY` | `--allow-hazards TABLE_LOCK` |
| `TABLE_LOCK` | `DROP INDEX` (non-concurrent) | → `DROP INDEX CONCURRENTLY` | `--allow-hazards TABLE_LOCK` |
| `CONSTRAINT_SCAN` | `ADD CHECK` on populated table without `NOT VALID` | → NOT VALID + VALIDATE | `--allow-hazards CONSTRAINT_SCAN` |
| `CONSTRAINT_SCAN` | `SET NOT NULL` on populated column | → 4-step check pattern | `--allow-hazards CONSTRAINT_SCAN` |
| `CONSTRAINT_SCAN` | `ADD FOREIGN KEY` without `NOT VALID` | → FK NOT VALID + VALIDATE | `--allow-hazards CONSTRAINT_SCAN` |
| `COLUMN_TYPE_CHANGE` | `ALTER COLUMN TYPE` (requires rewrite) | None | `--allow-hazards COLUMN_TYPE_CHANGE` |
| `INDEX_REBUILD` | Building new index (advisory warning) | Already concurrent | None required (advisory only) |
| `ENUM_VALUE_DROP` | `ALTER TYPE ... DROP VALUE` | None | `--allow-hazards ENUM_VALUE_DROP` |
| `FUNCTION_SIGNATURE_CHANGE` | Changing function parameter types | None | `--allow-hazards FUNCTION_SIGNATURE_CHANGE` |
| `NOT_REPLICA_SAFE` | `SEQUENCE` reset below current `last_value` | None | `--allow-hazards NOT_REPLICA_SAFE` |

**Acceptance Criteria:**

- [ ] All hazards listed in the registry are detected and attached to their respective `Statement` in the plan.
- [ ] `engine plan` exits with code `1` if any blocking hazards exist and are not acknowledged.
- [ ] Multiple hazards can be acknowledged in a single flag: `--allow-hazards DATA_LOSS,TABLE_LOCK`.
- [ ] In `--format=json` mode, hazards are serialized as a typed array on each statement.
- [ ] Auto-rewrites are validated against a shadow database before inclusion in the final plan.
- [ ] The table size heuristic (from `pg_class.reltuples`) is used to skip safe-rewrites on empty tables (e.g., no need for the 4-step NOT NULL pattern on a table with 0 rows).

---

## FR-07: Transactional Execution (P0)

**Description:** All non-concurrent DDL statements must be applied atomically within a single transaction.

**Acceptance Criteria:**

- [ ] Non-concurrent statements are wrapped: `BEGIN; SET LOCAL lock_timeout = ...; {DDL}; COMMIT;`
- [ ] If any non-concurrent statement fails, the transaction is rolled back and the database state is unchanged.
- [ ] Concurrent statements (`CREATE/DROP INDEX CONCURRENTLY`) are executed outside the main transaction (PG requirement).
- [ ] A session-level advisory lock prevents concurrent `engine apply` runs on the same database.
- [ ] `engine apply` outputs real-time progress for each statement.
- [ ] `engine apply --dry-run` validates the plan against a shadow database without modifying the live DB.

---

## FR-08: Drift Detection Command (P0)

**Description:** A dedicated command that compares the live DB to the desired schema and exits with a non-zero code if any difference is detected.

**Acceptance Criteria:**

- [ ] `engine drift --db=... --schema=...` exits `0` if no difference, `1` if drift detected.
- [ ] When drift is detected in `--format=json` mode, outputs a structured diff report.
- [ ] When drift is detected in human mode, outputs a colored summary of all differences.
- [ ] `--ignore=<object_type>` allows ignoring specific object types in the drift check (e.g., `--ignore=statistics` to ignore optimizer statistics differences).
- [ ] Completes in under 5 seconds for 500-table databases.

---

## FR-09: AI Integration JSON Output (P1)

**Description:** Structured JSON output for integration with AI agents and external automation.

**Acceptance Criteria:**

- [ ] `engine plan --format=json` outputs a JSON document with:
  - Top-level metadata: `version`, `generated_at`, `source_schema_hash`, `live_schema_hash`
  - `hazards[]`: array of `{ type, message, affected_object, severity }`
  - `statements[]`: array of `{ id, ddl, operation_type, object_type, object_name, is_concurrent, hazards[], estimated_lock_duration_ms }`
- [ ] The JSON schema is stable within a major version (semver-compatible).
- [ ] `engine drift --format=json` outputs: `{ is_drift: bool, differences: [{object_type, object_name, change_type, details}] }`
- [ ] The JSON output is valid against a published JSON Schema definition.

---

## FR-10: Schema Bootstrap / Init (P1)

**Description:** A bootstrap command that scaffolds a new schema directory with best-practice templates.

**Acceptance Criteria:**

- [ ] `engine init --dir=./schema` creates the directory structure.
- [ ] Generates a `.pg-flux.yml` config file.
- [ ] Generates example `.sql` files using PostgreSQL 18 best practices (e.g., `uuidv7()`, named NOT NULL constraints, virtual generated columns).
- [ ] The generated files pass `engine plan --dry-run` without errors.
- [ ] Prints a step-by-step guide to stdout after initialization.

---

## FR-11: Reverse Engineering / Inspect (P1)

**Description:** Generates `.sql` source files from an existing live database.

**Acceptance Criteria:**

- [ ] `engine inspect --db=... --out=./schema` writes normalized `.sql` files.
- [ ] Output files are valid as input for `engine plan`.
- [ ] System tables, internal PostgreSQL objects, and extension-owned objects are excluded.
- [ ] The `--schemas=` flag allows selecting specific schemas for inspection.
- [ ] Generates separate files per object type for clarity: `tables/`, `functions/`, `policies/`, `indexes/`, etc.
