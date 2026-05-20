# 09 — Operations Runbook

Day-2 operations: what to do when things go wrong in production.

---

## Checking Migration Status

```bash
# Human-readable
pg-flux migrate status

# Machine-readable
pg-flux migrate status --format json | jq '.'

# Count pending
pg-flux migrate status --format json | jq '[.[] | select(.applied == false)] | length'

# Query the tracking table directly
psql $DATABASE_URL -c "
  SELECT filename, checksum, applied_at
  FROM _pgflux.migrations
  ORDER BY applied_at DESC
  LIMIT 20;
"
```

---

## Detecting Schema Drift

Drift occurs when someone has made manual changes to the live database that are not reflected in the schema files.

```bash
pg-flux migrate generate
```

If a migration is generated when you did not expect one, you have drift. The generated file shows exactly what changed.

**Recommended response:**

1. Review the generated migration. Determine whether the live DB change was intentional.
2. If intentional, update the schema source files to match. Regenerate — the generate should now produce nothing.
3. If unintentional, apply the generated migration to revert the live DB to the desired state.
4. Post an incident report and investigate how the manual change occurred.

---

## Rolling Back a Migration

pg-flux does not generate automatic rollback scripts. Rollbacks in PostgreSQL DDL are non-trivial (you cannot `ALTER TABLE ... DROP COLUMN IF NOT EXISTS` on a column that never existed, etc.).

**Manual rollback process:**

1. Identify the last applied migration:
   ```bash
   psql $DATABASE_URL -c "
     SELECT filename FROM _pgflux.migrations ORDER BY applied_at DESC LIMIT 1;
   "
   ```

2. Write a manual rollback SQL file (do not put it in the `migrations/` directory):
   ```sql
   -- rollback-20260428_120000.sql  (keep separate; do not commit to migrations/)
   BEGIN;
   ALTER TABLE public.users DROP COLUMN IF EXISTS bio;
   DELETE FROM _pgflux.migrations WHERE filename = '20260428_120000.sql';
   COMMIT;
   ```

3. Apply it directly via psql:
   ```bash
   psql $DATABASE_URL < rollback-20260428_120000.sql
   ```

4. Revert the schema source file change and delete (or archive) the migration file from `migrations/`.

5. Verify with `pg-flux migrate generate` → should produce nothing.

> **Important:** Step 3 deletes the tracking row so pg-flux does not see the migration as applied. Without this, pg-flux will detect a checksum mismatch if the migration file is re-generated with a different timestamp.

---

## Recovering from a Failed Migration

If `pg-flux migrate apply` fails mid-way:

**Regular (transactional) statements:**

All transactional DDL rolls back atomically. The tracking row is also rolled back. The database is in its pre-migration state. Simply fix the migration file (or the schema source) and re-run apply.

**CONCURRENTLY statements:**

If the process was killed after the transactional block committed but before the `CONCURRENTLY` statement ran:
1. The tracking row is not yet inserted.
2. The regular DDL has been applied.
3. Re-running apply will re-execute the `CONCURRENTLY` statement (which is safe because pg-flux uses `IF NOT EXISTS`).

If the `CONCURRENTLY` statement itself failed (e.g. index build failed due to a constraint violation):
1. Inspect the error: `psql $DATABASE_URL -c "\d+ table_name"`
2. Fix the root cause (bad data, conflicting index, etc.)
3. Re-run `pg-flux migrate apply` — the CONCURRENTLY statement will be retried.

---

## Emergency Hotfix (Manual DDL)

Sometimes you must apply a DDL change directly in an emergency, outside of the normal pg-flux pipeline.

**After the emergency:**

1. Apply the emergency DDL directly:
   ```bash
   psql $DATABASE_URL -c "ALTER TABLE public.orders ADD COLUMN refund_status text;"
   ```

2. Immediately update the schema source file to match.

3. Run `pg-flux migrate generate` → it should now produce nothing (confirming the source file matches the live DB).

4. If a migration file was already generated before the emergency hotfix, delete it and regenerate.

5. If the hotfix changed a column that is already tracked, make sure the schema file exactly matches the live catalog (pay attention to type normalisation, default expressions, etc.).

---

## Recreating the Tracking Table

If `_pgflux.migrations` is accidentally dropped:

```sql
CREATE SCHEMA IF NOT EXISTS _pgflux;

CREATE TABLE _pgflux.migrations (
  id          serial      PRIMARY KEY,
  filename    text        NOT NULL UNIQUE,
  checksum    text        NOT NULL,
  applied_at  timestamptz NOT NULL DEFAULT now()
);
```

Then re-register already-applied migrations. pg-flux will otherwise try to re-apply them all. You can bulk-insert them using checksums from the migration files (pg-flux stores SHA-256 of the file content):

```bash
# Re-register all migration files as applied (use only when DB is already up-to-date)
for f in migrations/*.sql; do
  base=$(basename "$f")
  chk=$(sha256sum "$f" | awk '{print $1}')
  psql $DATABASE_URL -c "
    INSERT INTO _pgflux.migrations (filename, checksum)
    VALUES ('$base', '$chk')
    ON CONFLICT (filename) DO NOTHING;
  "
done
```

---

## Shadow Database Maintenance

The shadow database should be a structural (schema-only) copy of production. Refresh it regularly:

```bash
# Dump schema only (no data) from production
pg_dump --schema-only $PROD_DATABASE_URL > schema_dump.sql

# Restore to shadow (drop and recreate)
psql $SHADOW_DATABASE_URL -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
psql $SHADOW_DATABASE_URL < schema_dump.sql
```

Alternatively, use `pg_restore` with a custom-format dump for faster restores on large schemas.

---

## Monitoring Recommendations

Set up alerts on:

| Alert | Query / Check | Threshold |
|-------|---------------|-----------|
| Pending migrations in production | `SELECT COUNT(*) FROM files WHERE applied = false` | > 0 after deploy |
| Schema drift detected | Run `pg-flux migrate generate` in a cron job | Any output |
| `_pgflux.migrations` table missing | `SELECT 1 FROM _pgflux.migrations LIMIT 1` fails | Any error |
| Long-running DDL locks | `SELECT * FROM pg_locks WHERE mode = 'AccessExclusiveLock'` | Duration > 30s |
| Replication lag spike | `SELECT EXTRACT(EPOCH FROM now() - pg_last_xact_replay_timestamp())` | > 60s after migration |

---

## Sequence Counter Preservation

pg-flux emits `ALTER SEQUENCE` (not `DROP + CREATE`) when sequence parameters change. The current counter value (`nextval`) is always preserved. However, if you manually drop and recreate a sequence, the counter resets to `START`.

To check current sequence values:

```bash
psql $DATABASE_URL -c "
  SELECT schemaname, sequencename, last_value
  FROM pg_sequences
  WHERE schemaname = 'public'
  ORDER BY sequencename;
"
```
