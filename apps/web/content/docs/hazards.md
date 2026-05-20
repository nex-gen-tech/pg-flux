---
title: Hazards
group: Migrations
order: 2
---

# Hazard system

pg-flux classifies every DDL statement it emits. **Blocking** hazards refuse to apply unless the user explicitly opts in; **advisory** hazards print a notice but don't block.

```bash
pg-flux migrate apply
# Error: refusing to apply: blocking hazards; pass --allow-hazards or change schema

pg-flux migrate apply --allow-hazards=DATA_LOSS
```

## Hazard types

| Type | Severity | Triggered by |
|---|---|---|
| `DATA_LOSS` | Blocking | DROP TABLE, DROP COLUMN, DROP TYPE, DROP DOMAIN, DROP VIEW |
| `MASS_DROP` | Blocking | a single migration that drops >25% of live tables/views/sequences (or wipes a non-empty DB entirely) |
| `COLUMN_TYPE_CHANGE` | Blocking | ALTER COLUMN … TYPE (full table rewrite, AccessExclusiveLock for the duration) |
| `CONSTRAINT_SCAN` | Blocking | ADD CONSTRAINT CHECK / FOREIGN KEY without NOT VALID (full table scan) |
| `RLS_GAP` | Blocking | DROP POLICY followed by CREATE POLICY in the same plan (brief window without RLS) |
| `STAGED_SET_NOT_NULL` | Advisory | a SET NOT NULL on a large table (auto-rewritten to the 4-step safe pattern when `--reltuple-threshold` exceeded) |
| `VALIDATE_CONSTRAINT_SCAN` | Advisory | ALTER TABLE … VALIDATE CONSTRAINT (ShareUpdateExclusive scan; safer than the original full constraint add) |

## Auto-rewrites

By default pg-flux applies these safety transformations:

### NOT VALID + VALIDATE for ADD CHECK / FK

When `--auto-not-valid` is on (default):

```sql
ALTER TABLE orders ADD CONSTRAINT orders_fk FOREIGN KEY (user_id) REFERENCES users(id);
```

Becomes:

```sql
ALTER TABLE orders ADD CONSTRAINT orders_fk FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;
-- runs in main txn, takes short AccessExclusive lock
ALTER TABLE orders VALIDATE CONSTRAINT orders_fk;
-- runs autocommit after main txn, ShareUpdateExclusive lock — non-blocking for writes
```

### CREATE INDEX CONCURRENTLY

```sql
CREATE INDEX idx_users_email ON users (email);
```

Becomes:

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email ON users (email);
```

Skipped for partitioned tables (PG doesn't support CONCURRENTLY there).

### Staged SET NOT NULL (large tables)

When `--reltuple-threshold` is set (default 10000) and a `SET NOT NULL` would scan a large table:

```sql
ALTER TABLE users ADD CONSTRAINT chk_email_notnull CHECK (email IS NOT NULL) NOT VALID;
ALTER TABLE users VALIDATE CONSTRAINT chk_email_notnull;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users DROP CONSTRAINT chk_email_notnull;
```

PG14+ knows about the proven NOT NULL constraint and the `SET NOT NULL` is a fast catalog flip, not a scan.

## Drift safety: baseline-hash check

Every generated migration embeds the live schema hash at generation time. If the live DB drifts before apply:

```bash
pg-flux migrate apply
# refusing to apply 20260520_add_role.sql: live database state has drifted
# (expected baseline=abc123…, live=def456…)
# Re-run `pg-flux migrate generate` to rebase the migration,
# or pass --force-after-drift to apply anyway.
```

Mid-batch checking only happens on the **first pending file**; subsequent files are assumed to be a coherent sequence.

## Concurrency: advisory locks

`migrate apply` acquires a session-level advisory lock keyed by `host:port/db`. Two concurrent applies against the same database see only one acquire the lock; the other returns `could not acquire migration advisory lock (another apply in progress)`.

## Mass-drop guard

```bash
pg-flux migrate apply
# refusing to plan: 5 of 8 live objects (62%) would be dropped, exceeding the 25% mass-drop threshold;
# if this is intentional, re-run with --allow-mass-drop. Examples: TABLE public.orders, TABLE public.users, ...
```

Tunable via `--mass-drop-threshold=50` (percent) or bypass with `--allow-mass-drop`.

## See also

- [Migrations →](/docs/migrations.html)
- [Drift recovery →](/docs/drift.html)
