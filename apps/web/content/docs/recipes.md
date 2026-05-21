---
title: Migration recipes
group: Migrations
order: 3
description: Real-world scenarios — rename a column, drop a FK safely, NOT NULL on a huge table, and more.
---

The migrations docs explain the lifecycle. This page is the cookbook: copy-paste solutions to the migrations that real teams need to run on real production databases.

Every recipe assumes you've already read [Quick start](/docs/quick-start.html) and have pg-flux pointed at your DB.

## Rename a column (without data loss)

The naïve approach — change `users.email_address` to `users.email` in source and run `migrate generate` — does the wrong thing. pg-flux sees it as "drop `email_address`, add `email`". You lose the data.

Use the `@renamed` hint:

```sql
CREATE TABLE users (
  id    bigint PRIMARY KEY,
  email text NOT NULL  -- @renamed from=email_address
);
```

Now `migrate generate` emits:

```sql
ALTER TABLE users RENAME COLUMN email_address TO email;
```

> [!IMPORTANT]
> The hint must be on the *column line* and must say `from=<old_name>`.
> Whitespace around `=` is fine. The hint is consumed at generate time;
> you can remove it from source after the migration applies.

## Rename a table

Same pattern, but at the table level. pg-flux looks for `Table.OldName`:

```sql
-- @renamed-table from=user_accounts
CREATE TABLE users (
  id bigint PRIMARY KEY,
  email text NOT NULL
);
```

Emits:

```sql
ALTER TABLE user_accounts RENAME TO users;
```

The old name is internally tracked through subsequent diffs so foreign-key references update correctly.

## Add NOT NULL to a column on a huge table

The dangerous version:

```sql
-- 50M-row table; this locks for ~6 minutes
ALTER TABLE events ALTER COLUMN tenant_id SET NOT NULL;
```

pg-flux's auto-rewrite (controlled by `--set-not-null-reltuple-hint`, default `10000`) gives you the four-step pattern PostgreSQL itself optimizes for:

```sql
-- 1. Add CHECK constraint NOT VALID — instantaneous, short AccessExclusive lock
ALTER TABLE events
  ADD CONSTRAINT chk_events_tenant_id_notnull
  CHECK (tenant_id IS NOT NULL) NOT VALID;

-- 2. VALIDATE the constraint — long scan but only ShareUpdateExclusive lock (non-blocking for writes)
ALTER TABLE events VALIDATE CONSTRAINT chk_events_tenant_id_notnull;

-- 3. SET NOT NULL — fast because PG 14+ sees the proven CHECK and skips the rescan
ALTER TABLE events ALTER COLUMN tenant_id SET NOT NULL;

-- 4. DROP the helper constraint — no longer needed
ALTER TABLE events DROP CONSTRAINT chk_events_tenant_id_notnull;
```

You write `NOT NULL` in your schema. pg-flux notices the reltuples are large, rewrites to this pattern, splits steps 2 across the txn boundary (runs autocommit), and skips the table rewrite entirely. The migration applies in seconds even on tables with hundreds of millions of rows.

## Add a foreign key without locking the parent

Same idea, different shape:

```sql
ALTER TABLE orders
  ADD CONSTRAINT orders_user_fk
  FOREIGN KEY (user_id) REFERENCES users(id);
```

By default pg-flux auto-rewrites this (`--auto-not-valid`, on by default) to:

```sql
ALTER TABLE orders
  ADD CONSTRAINT orders_user_fk
  FOREIGN KEY (user_id) REFERENCES users(id) NOT VALID;

ALTER TABLE orders VALIDATE CONSTRAINT orders_user_fk;
```

Step 1 takes a brief AccessExclusive lock on `orders` to add the catalog entry. Step 2 runs autocommit and scans `orders` under ShareUpdateExclusive, which doesn't block writes. Net result: brief blip, no extended lock.

> [!TIP]
> If you ever need to opt out — say, you're confident the data is valid and want
> to skip the VALIDATE pass — set `--auto-not-valid=false`. The constraint is
> added in its fully-valid form, with a full table scan under AccessExclusive.

## Drop a column safely

Dropping columns is destructive — pg-flux refuses without `--allow-hazards=DATA_LOSS`:

```bash
$ pg-flux migrate apply
Error: refusing to apply: blocking hazards; pass --allow-hazards or change schema
```

To proceed:

```bash
$ pg-flux migrate apply --allow-hazards=DATA_LOSS
```

A safer pattern: deprecate in place first, then drop in a later migration:

```sql
-- Step 1: mark the column NOT NULL → nullable, deprecate via comment
ALTER TABLE users ALTER COLUMN legacy_phone DROP NOT NULL;
COMMENT ON COLUMN users.legacy_phone IS 'deprecated, will be removed in 0.4';

-- ... time passes, application stops writing to it ...

-- Step 2: drop in a later migration (with explicit --allow-hazards=DATA_LOSS)
ALTER TABLE users DROP COLUMN legacy_phone;
```

> [!CAUTION]
> Don't drop columns and add new ones with similar names in the same migration.
> If you really mean "rename", use `@renamed` (see above). If you really mean
> "drop X and add Y", do it in two migrations so a rollback doesn't conflate them.

## Add a unique constraint without a long lock

```sql
ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);
```

That holds AccessExclusive for the duration of the index build. The pattern PG uses internally:

```sql
-- Build the unique index concurrently (no AccessExclusive)
CREATE UNIQUE INDEX CONCURRENTLY users_email_unique
  ON users (email);

-- Attach the index as the backing index for a UNIQUE constraint (catalog flip, instantaneous)
ALTER TABLE users
  ADD CONSTRAINT users_email_unique UNIQUE USING INDEX users_email_unique;
```

pg-flux doesn't auto-rewrite this yet (`#TODO: filed`). For now, write the two statements explicitly in your migration file after `migrate generate` writes the naïve form.

## Change a column type without rewriting the table

Some type changes are free — they don't rewrite the table:

| Change | Free? | Why |
|---|---|---|
| `varchar(10)` → `varchar(20)` | ✓ | Same on-disk format, just a metadata check |
| `varchar(20)` → `varchar(10)` | ✗ | Has to validate every row fits |
| `text` → `varchar(N)` | ✗ | Has to validate every row |
| `varchar(N)` → `text` | ✓ | PG14+ knows text is a superset |
| `int4` → `int8` | ✗ | Different storage width |
| `int8` → `int4` | ✗ | Same |
| `timestamp` → `timestamptz` | ✗ | Different storage |
| `numeric` → `numeric(N,M)` | depends | Free if widening, scan if tightening |

If you're doing one of the "✗" changes on a large table, the only real solution is the **shadow column** pattern:

```sql
-- Step 1: add the new column as nullable, no rewrite
ALTER TABLE events ADD COLUMN event_time_new timestamptz;

-- Step 2: backfill in batches from application code
-- (not pg-flux territory — your worker, in chunks of 10k rows)

-- Step 3: switch reads to the new column in app code, deploy, validate

-- Step 4: drop the old column (with --allow-hazards=DATA_LOSS)
ALTER TABLE events DROP COLUMN event_time;
ALTER TABLE events RENAME COLUMN event_time_new TO event_time;
```

This is genuinely PG ecosystem state-of-the-art — there's no good escape from full-table rewrites for incompatible types.

## Add row-level security without breaking existing queries

The dangerous pattern — enable RLS first, then add policies — locks every session out the moment RLS goes on:

```sql
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;  -- now nothing can read documents until a policy permits
CREATE POLICY documents_read ON documents FOR SELECT TO authenticated USING (...);
```

The safe pattern — create policies first, then enable:

```sql
-- Step 1: create the policies. Policies on non-RLS tables do nothing.
CREATE POLICY documents_read ON documents
  FOR SELECT TO authenticated
  USING (owner_id = current_setting('app.user_id')::bigint);

CREATE POLICY documents_admin ON documents
  TO admin
  USING (true)
  WITH CHECK (true);

-- Step 2: enable RLS (now the policies take effect)
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;
```

pg-flux's DAG sort handles this automatically: policies sort with priority 12 (CREATE_POLICY), RLS toggles sort with priority 65 (ChangeToggleRLS). Policies always emit before RLS enable.

## Migrate to a new enum value

PostgreSQL forbids dropping enum values entirely — there's no `ALTER TYPE ... DROP VALUE`. You can only add (`ADD VALUE`) and rename (`RENAME VALUE`, PG 12+).

To replace a value, use the rename:

```sql
-- before:
CREATE TYPE status AS ENUM ('draft', 'review', 'published');

-- after — rename, don't drop+add:
CREATE TYPE status AS ENUM ('draft', 'in_review', 'published');
```

pg-flux's enum-rename detector (added in v0.1) sees the same position changing value and emits:

```sql
ALTER TYPE status RENAME VALUE 'review' TO 'in_review';
```

If you need to actually remove a value, the only path is recreate-the-type:

```sql
-- Step 1: create a new type with the right values
CREATE TYPE status_new AS ENUM ('draft', 'published');

-- Step 2: switch every column over (this WILL fail if any row has the removed value)
ALTER TABLE posts ALTER COLUMN status TYPE status_new USING status::text::status_new;

-- Step 3: drop the old type and rename
DROP TYPE status;
ALTER TYPE status_new RENAME TO status;
```

> [!WARNING]
> Step 2 will fail loudly if any row has the value you're removing. Either
> backfill it first or accept the migration as a hard-stop until the data
> is clean.

## Add a generated column

Generated columns are computed on write:

```sql
CREATE TABLE orders (
  id           bigint PRIMARY KEY,
  subtotal     numeric NOT NULL,
  tax_rate     numeric NOT NULL,
  tax_amount   numeric GENERATED ALWAYS AS (subtotal * tax_rate) STORED
);
```

pg-flux emits this verbatim. Adding a generated column to an existing table rewrites the table — same hazard rules as a column type change.

PG 18 supports `VIRTUAL` generated columns (computed on read, no storage). pg-flux gates these with `pgver.FeatureVirtualGenerated` — try to use one against PG 17 and you'll get a clear error at `migrate generate` time, not at apply.

## Convert a JSON column to a typed column

You shipped `payload jsonb` because you didn't know the shape yet. Now you know. The migration:

```sql
-- Step 1: add the typed columns nullable
ALTER TABLE events ADD COLUMN user_id    bigint;
ALTER TABLE events ADD COLUMN action     text;
ALTER TABLE events ADD COLUMN timestamp_ timestamptz;

-- Step 2: backfill from the JSON (in your app, in batches)
UPDATE events SET
  user_id   = (payload->>'user_id')::bigint,
  action    = payload->>'action',
  timestamp_ = (payload->>'timestamp')::timestamptz
WHERE user_id IS NULL;

-- Step 3: enforce NOT NULL with the staged pattern (see "Add NOT NULL to a huge table")
ALTER TABLE events ALTER COLUMN user_id   SET NOT NULL;
ALTER TABLE events ALTER COLUMN action    SET NOT NULL;
ALTER TABLE events ALTER COLUMN timestamp_ SET NOT NULL;

-- Step 4 (later, when the app no longer reads payload): drop with --allow-hazards=DATA_LOSS
ALTER TABLE events DROP COLUMN payload;
```

For codegen, you can also keep the JSON typed against the documented shape via the `tstype` comment hint — see [Codegen overview](/docs/codegen.html#json-shapes).

## Partition an existing table

Partitioning is the one schema change pg-flux can't fully automate, because converting a regular table to a partitioned one requires data movement that pg-flux doesn't do. The pattern:

```sql
-- Step 1: create the new partitioned parent
CREATE TABLE events_new (
  id          bigint NOT NULL,
  occurred_at timestamptz NOT NULL,
  ...
) PARTITION BY RANGE (occurred_at);

CREATE TABLE events_2025 PARTITION OF events_new FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');
CREATE TABLE events_2026 PARTITION OF events_new FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

-- Step 2: backfill in your app, in batches
-- INSERT INTO events_new SELECT * FROM events WHERE occurred_at >= '...' LIMIT 10000;

-- Step 3: when caught up, swap (locking briefly)
BEGIN;
  ALTER TABLE events RENAME TO events_old;
  ALTER TABLE events_new RENAME TO events;
COMMIT;

-- Step 4: drop the old table when verified
DROP TABLE events_old;
```

pg-flux tracks partitions in `Table.PartitionBy` and as separate child tables in `SchemaState.PartitionChildren`. It manages adding new partitions to an existing partitioned parent automatically.

## Rebuild a view's dependencies

PostgreSQL is unhappy when you `ALTER COLUMN ... TYPE` on a column that any view references — even if the view's output type doesn't change. The error:

```text
ERROR: cannot alter type of a column used by a view or rule
DETAIL: rule _RETURN on view active_users depends on column "users.email"
```

pg-flux's DAG sort handles this automatically via the **drop-early** pass: if a column type change in source affects a column referenced by a view's body, the differ injects a `ChangeDropViewEarly` (priority 3) before the `ChangeAlterColumn` (priority 50) and a `ChangeCreateView` (priority 40) after.

You don't have to think about this. The migration `pg-flux migrate generate` produces is correct ordering.

## Migrate between schemas

Moving a table from `public` to `app`:

```sql
-- target schema must exist
CREATE SCHEMA IF NOT EXISTS app;

-- move
ALTER TABLE public.users SET SCHEMA app;
```

pg-flux's `--schemas` flag controls which schemas the inspector scans. To manage objects across multiple schemas:

```bash
pg-flux migrate generate --schemas=public,app
```

If you don't include a schema in the list, pg-flux pretends its objects don't exist (won't generate drops for them).

## What's not in this recipe book

Some operations are genuinely outside pg-flux's purview:

- **Data migrations** — pg-flux moves schema, not data. Use your application layer for `UPDATE`-shaped backfills, ideally batched.
- **Cross-database migrations** — pg-flux operates against one database at a time. Use logical replication or `pg_dump | pg_restore` for cross-server moves.
- **Vacuuming and reindexing** — these are maintenance, not migration. Schedule them separately.

If your migration needs application-coordinated steps, write it as a sequence of pg-flux migrations interspersed with application deploys. The "expand-then-contract" pattern is the universal answer: add the new schema, dual-write from your app, switch reads, drop the old schema, in that order, with deploys in between.
