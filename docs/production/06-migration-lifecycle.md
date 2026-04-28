# 06 — Migration Lifecycle

---

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│  1. AUTHOR   Edit .sql source files (desired state)            │
│  2. GENERATE pg-flux migrate generate  → migrations/<ts>.sql   │
│  3. REVIEW   Read the generated file; check hazard annotations  │
│  4. COMMIT   git commit migrations/<ts>.sql                     │
│  5. CI       Shadow validation (optional but recommended)       │
│  6. APPLY    pg-flux migrate apply  (staging → production)      │
│  7. VERIFY   pg-flux migrate generate  → "No changes detected" │
└─────────────────────────────────────────────────────────────────┘
```

---

## Step 1 — Edit Schema Files

Make your desired changes to `.sql` files in the `schema/` directory. The files are the **single source of truth**. Do not write migration SQL by hand; always let pg-flux generate it from the diff.

---

## Step 2 — Generate

```bash
pg-flux migrate generate [--label short_description]
```

If there are no differences, pg-flux prints `No changes detected — no migration generated.` and exits `0`.

If differences exist, a file named `<YYYYmmdd_HHMMSS>[_label].sql` is written to the migrations directory.

**Always re-run generate after applying** to confirm the state is clean. A clean generate (no output) is your acceptance test.

---

## Step 3 — Review the Generated File

Every generated file has the structure:

```sql
-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

-- [ADVISORY ...]  ← informational; does not affect execution
-- [HAZARD ...]    ← blocking hazard annotation

BEGIN;

-- [N] <OP_TYPE>: <object>
<DDL statement>;

COMMIT;
```

**Advisory annotations** — informational warnings that do not block execution (e.g. column reorder requires table recreation, which pg-flux does not auto-emit).

**Hazard annotations** — produced by the hazard detector; if a blocking hazard was allowed via `--allow-hazards`, the annotation explains what risk was accepted.

**Statement ordering** — statements are sorted by the DAG dependency sorter. The order is always safe: types before tables, parent tables before foreign keys, views dropped before their base table type changes, etc.

---

## Step 4 — Commit the Migration File

```bash
git add migrations/<ts>.sql
git commit -m "migration: add user bio column"
```

Migration files, once committed, **must not be edited**. pg-flux records a SHA-256 checksum on first apply and verifies it on every subsequent run. Editing an applied migration causes apply to abort with a checksum mismatch error.

---

## Step 5 — Shadow Validation (Optional but Recommended)

Before applying to the live database, validate the migration on a disposable shadow DB:

```bash
pg-flux migrate apply \
  --dry-run \
  --shadow-dsn postgres://user:pass@shadow:5432/shadow_db \
  --shadow-semantic
```

Shadow semantic mode applies the plan to the shadow DB (which must be a close structural copy of production). It catches:
- SQL syntax errors
- Missing table/column references
- Type mismatches in USING expressions
- Constraint violations on existing data (on the shadow copy)

---

## Step 6 — Apply

```bash
# Staging
pg-flux migrate apply

# Production (with shadow pre-validation)
pg-flux migrate apply \
  --shadow-dsn postgres://user:pass@shadow-prod:5432/shadow \
  --shadow-semantic
```

pg-flux applies migrations in timestamp order. Each migration is fully transactional except for statements containing `CONCURRENTLY` (which cannot run inside a transaction; they are applied in auto-commit mode after the regular transaction commits).

### Transaction Model

```
For each pending migration file:
  ┌─────────────────────────────────────────────────────┐
  │  BEGIN                                              │
  │    regular DDL statement 1                          │
  │    regular DDL statement 2                          │
  │    ...                                              │
  │    INSERT INTO _pgflux.migrations (filename, chk)   │
  │  COMMIT   ← tracking row committed with DDL         │
  └─────────────────────────────────────────────────────┘
  Then (outside transaction):
    CREATE INDEX CONCURRENTLY ...   ← if any
  Then:
    INSERT INTO _pgflux.migrations ...  ← if CONCURRENTLY stmts present
```

If the transactional block fails, the entire DDL and the tracking insert are rolled back. The file will be retried on the next `apply`.

---

## Step 7 — Verify Clean State

```bash
pg-flux migrate generate
# No changes detected — no migration generated.
```

This is the most important check. A clean generate after apply confirms:
- All DDL was applied successfully.
- pg-flux's inspection of the live catalog matches the desired schema.
- No manual changes have been made to the live DB that created drift.

---

## Tracking Table

pg-flux creates `_pgflux.migrations` automatically:

```sql
CREATE TABLE _pgflux.migrations (
  id          serial      PRIMARY KEY,
  filename    text        NOT NULL UNIQUE,
  checksum    text        NOT NULL,
  applied_at  timestamptz NOT NULL DEFAULT now()
);
```

**Do not:**
- Drop or truncate this table
- Edit `checksum` values
- Delete rows for applied migrations

**You can:**
- Query it to audit migration history
- Add custom indexes for reporting
- Replicate it alongside your application tables

---

## Empty Migrations

If a migration file exists but has no executable SQL (only comments and `BEGIN`/`COMMIT` markers), pg-flux records it in the tracking table without executing DDL. This allows you to create documentation-only or no-op migration files if needed.

---

## Concurrent Index Creation

Indexes declared with `CONCURRENTLY` are handled automatically:

```sql
-- In your schema file:
CREATE INDEX CONCURRENTLY idx_users_email_hash ON public.users USING hash (email);
```

pg-flux emits:

```sql
BEGIN;
-- regular DDL ...
COMMIT;

-- After transaction:
CREATE INDEX CONCURRENTLY idx_users_email_hash ON public.users USING hash (email);
```

**Note:** If the process is killed between the COMMIT and the CONCURRENTLY statement, the migration will not be recorded as applied. Re-running `apply` will attempt the CONCURRENTLY statement again. `CREATE INDEX CONCURRENTLY` is idempotent when combined with `IF NOT EXISTS` (which pg-flux uses).

---

## Column Reorder

PostgreSQL does not support reordering columns in-place (`ALTER TABLE` cannot change column position). pg-flux detects column order mismatches and emits an `[ADVISORY COLUMN_REORDER]` comment, but does **not** automatically generate a table recreation.

If column order matters (e.g. for `SELECT *` queries or external tools), recreate the table manually:

```sql
-- In a hand-written migration:
ALTER TABLE public.users RENAME TO users_old;
CREATE TABLE public.users ( ... columns in desired order ... );
INSERT INTO public.users SELECT ... FROM users_old;
DROP TABLE users_old;
```
