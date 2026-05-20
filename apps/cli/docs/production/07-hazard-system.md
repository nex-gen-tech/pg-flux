# 07 — Hazard System

pg-flux classifies every DDL operation by the risks it carries in production. Hazardous operations are blocked by default; they must be explicitly allowed.

---

## Hazard Severities

| Severity | Behaviour | Example |
|----------|-----------|---------|
| `BLOCKING` | Generation fails; migration file is not written | `DROP COLUMN`, type change |
| `ADVISORY` | Migration is generated; hazard annotation added as a SQL comment | Column reorder detected |

---

## Hazard Types

### `DATA_LOSS`

Severity: **BLOCKING**

The operation permanently destroys data with no automatic recovery path.

Triggered by:
- `DROP COLUMN` — all data in the column is deleted
- `DROP TABLE` — all rows deleted
- `DROP VIEW CASCADE` — cascades may drop dependent objects
- Removing an ENUM value — PostgreSQL does not support `ALTER TYPE DROP VALUE`; any rows using that value will break
- `DROP EXTENSION CASCADE` — may drop dependent objects

**How to allow:**

```bash
pg-flux migrate generate --allow-hazards DATA_LOSS
```

**Best practice:** Before allowing `DATA_LOSS` for a column drop, first set the column to `NULL` or move data to another column in a prior migration.

---

### `COLUMN_TYPE_CHANGE`

Severity: **BLOCKING**

Changing a column's data type may rewrite the entire table (acquiring an `AccessExclusiveLock` for the duration), and may fail if existing data cannot be cast to the new type.

Triggered by any `ALTER COLUMN SET DATA TYPE` change.

**How to allow:**

```bash
pg-flux migrate generate --allow-hazards COLUMN_TYPE_CHANGE
```

**Incompatible casts:** If PostgreSQL cannot implicitly cast the existing data (e.g. `boolean` → `enum`), you must provide a `-- @using` expression. See [05-schema-authoring.md — @using annotation](./05-schema-authoring.md#---using-expr).

---

### `CONSTRAINT_SCAN`

Severity: **BLOCKING**

Adding a `NOT NULL` constraint or a `CHECK` constraint on a populated table requires a full sequential scan of the table, holding an `AccessExclusiveLock` for the duration.

**Zero-downtime alternative (PostgreSQL 18 pattern):**

```sql
-- Step 1: add as NOT VALID (no scan, no lock held long)
ALTER TABLE public.users ADD CONSTRAINT users_score_range
  CHECK (test_score >= 0) NOT VALID;

-- Step 2 (separate migration, short ShareUpdateExclusiveLock):
ALTER TABLE public.users VALIDATE CONSTRAINT users_score_range;
```

Use `--append-validate-not-valid` to have pg-flux automatically append the `VALIDATE CONSTRAINT` step:

```bash
pg-flux migrate generate --append-validate-not-valid
```

---

### `NOT_REPLICA_SAFE`

Severity: **ADVISORY**

The operation is not safe to run on a logical replication subscriber, or may cause replication lag.

Triggered by:
- `CREATE EXTENSION` / `ALTER EXTENSION ... UPDATE` (DDL that may not replicate)
- Certain `ALTER TABLE` forms that are not replicated

---

### `STAGED_SET_NOT_NULL`

Severity: **ADVISORY** (emitted when `--set-not-null-reltuple-hint > 0`)

A `SET NOT NULL` is being applied to a table that has more than the configured estimated row count. This operation holds an `AccessExclusiveLock` for the duration of the full-table scan.

**Zero-downtime alternative:**
1. First migration: add a `CHECK (col IS NOT NULL) NOT VALID` constraint.
2. Second migration: `VALIDATE CONSTRAINT`.
3. Third migration: `ALTER COLUMN SET NOT NULL` (PostgreSQL 18 will use the validated constraint to skip the scan).

---

### `COLUMN_REORDER` (Advisory)

Severity: **ADVISORY**

The declared column order in the schema file differs from the live column order. PostgreSQL does not support in-place column reordering. pg-flux will not auto-generate a table recreation.

No action is required unless column order is critical to your application. See [06-migration-lifecycle.md — Column Reorder](./06-migration-lifecycle.md#column-reorder).

---

## Allowlist

Pass a comma-separated list to `--allow-hazards` or set `allow_hazards` in `.pg-flux.yml`:

```bash
# Allow multiple hazard types in one generate
pg-flux migrate generate \
  --allow-hazards COLUMN_TYPE_CHANGE,DATA_LOSS

# Allow in config file (not recommended for production pipelines)
# allow_hazards: COLUMN_TYPE_CHANGE
```

**Recommended practice:** Never set `allow_hazards` in the config file. Always pass it explicitly on the CLI so the decision is visible in CI logs and git history.

---

## Hazard Annotation in Generated Files

Every allowed hazard is annotated in the migration file as a comment immediately before the relevant statement:

```sql
-- [HAZARD DATA_LOSS] Drops column data
-- [2] DROP_COLUMN: public.users.legacy_field
ALTER TABLE public.users DROP COLUMN IF EXISTS legacy_field CASCADE;
```

This creates a permanent audit trail: anyone reading the migration file in the future can see that the data loss hazard was known and accepted.

---

## Enum Removal — Special Case

PostgreSQL does not have `ALTER TYPE ... DROP VALUE`. If you remove an ENUM value from a source file, pg-flux emits a blocking `DATA_LOSS` advisory and refuses to generate the migration.

To remove an ENUM value safely:

1. Migrate all rows that use the value to a different value (application migration + SQL UPDATE in a prior migration).
2. Rename the existing type:
   ```sql
   ALTER TYPE public.user_status RENAME TO user_status_old;
   ```
3. Create the new type without the removed value.
4. `ALTER TABLE ... ALTER COLUMN ... SET DATA TYPE new_type USING col::text::new_type`.
5. Drop the old type.

This is a multi-migration process and cannot be automated safely. Plan accordingly.
