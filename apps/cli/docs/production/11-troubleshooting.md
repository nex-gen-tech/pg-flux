# 11 — Troubleshooting

---

## `generate` produces unexpected changes on every run

**Symptom:** Running `pg-flux migrate generate` twice in a row produces a new migration the second time, even though nothing changed.

**Cause 1: Default expression normalisation mismatch.**

The live catalog stores default expressions in a normalised form (e.g. `'active'::user_status` instead of `'active'`). pg-flux normalises both sides before comparing, but edge cases can cause false positives.

Diagnostic:
```bash
pg-flux migrate generate --format json | jq '.statements[] | select(.op_type == "ALTER_DEFAULT")'
```

If you see spurious `ALTER_DEFAULT` for a column you did not change, check the exact default stored in the catalog:
```sql
SELECT column_name, column_default
FROM information_schema.columns
WHERE table_schema = 'public' AND table_name = 'users';
```

Fix: Adjust the default expression in the schema file to match the canonical form shown in the catalog (e.g. write `DEFAULT 'active'::user_status` instead of `DEFAULT 'active'`).

---

**Cause 2: Extension toggling (DROP + CREATE loop).**

If you see `DROP_EXTENSION` followed by `CREATE_EXTENSION` on every run, the extension is declared in both the desired schema and the live DB, but their `DefSQL` fingerprints differ.

Fix: Ensure the schema file uses `CREATE EXTENSION IF NOT EXISTS <name>` without a version pin, or match exactly the version installed.

---

**Cause 3: Constraint text whitespace or type cast differences.**

pg-flux uses `pg_query` AST fingerprinting to compare constraint definitions. Very occasionally, pg_query's deparsing produces a form that does not round-trip cleanly.

Diagnostic:
```sql
SELECT conname, pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid = 'public.users'::regclass;
```

Compare with your schema file. If there is a structural difference (not just whitespace), update the schema file to match.

---

## `apply` fails with `checksum mismatch`

```
Error: checksum mismatch for already-applied migration 20260428_120000.sql:
  recorded=abc123 current=def456
  — do not edit applied migrations
```

**Cause:** The migration file was edited after it was applied to the database.

**Fix:**
1. Do NOT edit the file to match the recorded checksum — that hides the discrepancy.
2. Determine what changed and why.
3. If the change was intentional (e.g. a formatting fix), restore the original content, then create a new migration for the intended change.
4. If the file was corrupted (e.g. line ending conversion by your editor), restore it from git history: `git checkout -- migrations/20260428_120000.sql`.

---

## `apply` fails with `cannot cast type X to Y`

```
ERROR: default for column "is_verified" cannot be cast automatically to type verification_status
```

**Cause:** You changed a column type from one type (e.g. `boolean`) to an incompatible type (e.g. `verification_status` ENUM) and the column has a `DEFAULT` expression that PostgreSQL cannot implicitly cast.

**Fix:** Add a `-- @using` annotation to the column in your schema file. See [05-schema-authoring.md — @using annotation](./05-schema-authoring.md#---using-expr).

pg-flux will automatically:
1. Emit `ALTER COLUMN DROP DEFAULT` before the type change.
2. Use your USING expression.
3. Emit `ALTER COLUMN SET DEFAULT <new_default>` after.

---

## `apply` fails with `column "X" cannot be cast automatically to type Y`

Same as above but without the DEFAULT. The USING expression in the migration file is missing or incorrect.

Regenerate with a `-- @using` annotation that produces a valid value of the new type for every row.

---

## View refresh fails after type change

```
ERROR: column "X" is of type old_type but expression is of type new_type
```

**Cause:** A view references a column whose type was changed, and the `DROP VIEW ... CASCADE` / `CREATE VIEW` ordering was incorrect.

pg-flux automatically drops and recreates views that reference type-changed columns, including views being dropped in the same migration. If this error occurs, it means a view was not detected as referencing the changed column (e.g. via a subquery or function).

**Fix:** Drop the view explicitly in the migration before the type change, then recreate it after. This can be done manually by editing the generated migration file.

---

## Sequence keeps getting ALTER'd on every run

**Symptom:** `pg-flux migrate generate` emits an `ALTER SEQUENCE` on every run.

**Cause:** The `CACHE` value in the schema file differs from the live catalog value.

PostgreSQL's `pg_sequences` view shows the declared cache, but `pg_get_serial_sequence` can return slightly different SQL. Check:

```sql
SELECT * FROM pg_sequences WHERE sequencename = 'counter_seq';
```

Match the `cache_size` value exactly in your schema file:
```sql
CREATE SEQUENCE public.counter_seq START 10 INCREMENT 10 CACHE 2;
```

---

## ENUM value not found after migration

```
ERROR: invalid input value for enum user_status: "pending_review"
```

**Cause:** The ENUM value was added to the schema file but the migration has not been applied yet, or the application was deployed before `migrate apply`.

**Fix:** Always apply migrations before deploying application code that uses new ENUM values.

---

## Shadow validation always fails

```
shadow validate 20260428_120000.sql: shadow validate ... (rolled back): ERROR: ...
```

**Cause:** The shadow database schema does not match the production schema closely enough. The migration assumes a column or table exists that is absent in the shadow DB.

**Fix:** Refresh the shadow DB from a production schema dump:
```bash
pg_dump --schema-only $PROD_DATABASE_URL | psql $SHADOW_DATABASE_URL
```

Or, if using `--shadow-semantic`, ensure the shadow DB is an empty database and has all prior migrations applied (it must start from the same state as production).

---

## `pg_query` parse error on generate

```
Error: parse schema: pkg/src/parser.go: pg_query parse: syntax error at position 42
```

**Cause:** One of your schema `.sql` files has invalid PostgreSQL syntax.

**Fix:**
```bash
# Find which file has the error
psql $DATABASE_URL -f schema/suspicious_file.sql
```

Or test the specific statement:
```sql
DO $$ BEGIN
  -- paste the failing statement here
END $$;
```

---

## `_pgflux.migrations` table exists in the wrong schema

pg-flux creates the tracking table in the schema specified by `--tracking-schema` (default: `_pgflux`). If you see `relation "_pgflux.migrations" does not exist`, either:

1. The schema was dropped.
2. You changed `--tracking-schema` after migrations were already applied.
3. You are connecting to a different database.

**Fix:**
```bash
# Check what schema the tracking table is in
psql $DATABASE_URL -c "\dn _pgflux"
psql $DATABASE_URL -c "SELECT schemaname, tablename FROM pg_tables WHERE tablename = 'migrations';"
```

Then pass the correct `--tracking-schema` flag, or run `pg-flux migrate apply` once with the correct flag to let it create the table.

---

## Getting Debug Output

For verbose internal logging:

```bash
PGFLUX_LOG_LEVEL=debug pg-flux migrate generate 2>&1 | head -100
```

For raw differ output (what changes were detected before statement generation):

```bash
pg-flux migrate generate --format json | jq '.statements'
```

For inspecting what pg-flux sees in the live database:

```bash
pg-flux inspect --out /tmp/live-schema
diff -r schema/ /tmp/live-schema/
```
