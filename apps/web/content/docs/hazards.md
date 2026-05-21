---
title: Hazards
group: Migrations
order: 2
description: What pg-flux refuses to apply by default, and how to opt in when you mean it.
---

Migrations break production in predictable ways. pg-flux classifies the predictable failures, refuses to apply them by default, and asks you to confirm in writing.

This page is the catalog of every hazard pg-flux knows about, the message you'll see, and what to do about it.

## The hazard model in one paragraph

Every change pg-flux emits gets classified. Blocking hazards require an explicit `--allow-hazards` opt-in. Advisory hazards print a warning but don't block. You can pre-allow specific hazards in config; you can audit what was allowed via structured logs. The point: nothing destructive happens silently.

## Severity levels

| Level | What it means | Default behavior |
|---|---|---|
| **Blocking** | "If I run this, I might destroy data or take a multi-minute lock." | `apply` refuses |
| **Advisory** | "This is interesting but probably fine." | Prints a warning, proceeds |

## The catalog

### DATA_LOSS

```text
[HAZARD DATA_LOSS] Drops table public.legacy_users
[HAZARD DATA_LOSS] Drops column public.users.legacy_phone
```

Triggered by:

- `DROP TABLE`
- `DROP COLUMN`
- `DROP TYPE` (when used by columns)
- `DROP DOMAIN`
- `DROP VIEW`

**To allow:**

```bash
pg-flux migrate apply --allow-hazards=DATA_LOSS
```

> [!CAUTION]
> Once `DATA_LOSS` is allowed and applied, there's no automatic rollback.
> Take a backup first. We're not your dad, but we'd take a backup.

### COLUMN_TYPE_CHANGE

```text
[HAZARD COLUMN_TYPE_CHANGE] ALTER COLUMN public.events.tenant_id TYPE bigint USING tenant_id::bigint
```

Triggered by any `ALTER COLUMN ... TYPE` that can't be done as a metadata flip — i.e., almost all type changes.

The hazard exists because column type changes rewrite the table (in PG14+, in some cases) and hold AccessExclusiveLock for the duration. On a 50M-row table, that's 5–15 minutes of full lock.

**To allow:**

```bash
pg-flux migrate apply --allow-hazards=COLUMN_TYPE_CHANGE
```

**Safer alternative:** use the shadow-column pattern — see [Recipes / change a column type](/docs/recipes.html#change-a-column-type-without-rewriting-the-table).

### CONSTRAINT_SCAN

```text
[HAZARD CONSTRAINT_SCAN] Adding constraint may scan table
```

Triggered by `ADD CONSTRAINT CHECK` or `ADD CONSTRAINT FOREIGN KEY` without `NOT VALID`. Implies a full table scan under AccessExclusive.

> [!TIP]
> pg-flux auto-rewrites these to the safe `NOT VALID + VALIDATE` pattern
> by default (`--auto-not-valid=true`). The hazard only fires if you
> explicitly disable the rewrite or write `ADD CONSTRAINT` literally
> in your migration.

If you really want the all-at-once scan:

```bash
pg-flux migrate apply --allow-hazards=CONSTRAINT_SCAN
```

### MASS_DROP

```text
refusing to plan: 5 of 8 live objects (62%) would be dropped, exceeding the 25% mass-drop threshold
```

Triggered when a single migration would drop more than 25% of your live tables/views/sequences, OR would wipe a non-empty database entirely.

**Why this exists**: the most common way to nuke a database with pg-flux is to point `--schema` at the wrong directory. The mass-drop guard catches this before it runs.

**To allow:**

```bash
pg-flux migrate apply --allow-mass-drop
```

**Or tune the threshold:**

```bash
pg-flux migrate apply --mass-drop-threshold=50  # allow up to 50%
```

> [!WARNING]
> If you find yourself wanting `--allow-mass-drop` more than once a year,
> something is wrong with your workflow. Double-check the `--schema` flag
> first. Re-confirm twice. Then proceed.

### RLS_GAP

```text
[HAZARD RLS_GAP] Brief window between DROP POLICY and CREATE POLICY where the table is unprotected
```

Triggered when a migration drops a policy and creates a replacement on the same table in the same migration. The transaction wraps both, so there's no real window — but it's a hazard severity advisory because the differ is conservative.

The differ also has a smarter path: when only the policy's `USING` / `WITH CHECK` / role list change, it emits `ALTER POLICY` instead of DROP+CREATE, eliminating the gap entirely.

**You don't need to allow this** — it's advisory, not blocking. The message is informational.

### STAGED_SET_NOT_NULL

```text
[ADVISORY STAGED_SET_NOT_NULL] Rewrote SET NOT NULL on public.events.tenant_id (50M rows) to 4-step pattern
```

Advisory. pg-flux saw a `SET NOT NULL` on a large table (above `--set-not-null-reltuple-hint`, default 10000 rows) and rewrote it to the staged `ADD CHECK NOT VALID + VALIDATE + SET NOT NULL + DROP CHECK` pattern.

**This is good news.** Your migration that would have locked for 5 minutes now applies in seconds. The advisory exists so you know the rewrite happened.

To disable the auto-rewrite (not recommended):

```bash
pg-flux migrate generate --set-not-null-reltuple-hint=0
```

### VALIDATE_CONSTRAINT_SCAN

```text
[ADVISORY VALIDATE_CONSTRAINT_SCAN] Validates constraint under ShareUpdateExclusive lock
```

Advisory. The `VALIDATE CONSTRAINT` half of the NOT VALID + VALIDATE pattern emits this. ShareUpdateExclusive doesn't block writes — only schema changes — but the table is being scanned, which uses I/O.

**You don't need to allow this.** It's informational.

## Allowing hazards in CI

For CI environments that need to apply hazards regularly (test fixtures, ephemeral databases):

```yaml
- name: apply migrations
  env:
    DATABASE_URL: postgres://ci:ci@localhost:5432/test?sslmode=disable
  run: pg-flux migrate apply --allow-hazards=DATA_LOSS
```

For production, **don't** put hazard allowances in your default workflow. Require manual approval (GitHub environment protection rules) for any deploy that uses `--allow-hazards`.

## Hazards in `migrate generate` vs `migrate apply`

| Phase | What happens |
|---|---|
| `migrate generate` | Hazards are computed and embedded as `-- [HAZARD ...]` comments in the generated `.sql` file. Generate always succeeds (it doesn't mutate the database). |
| `migrate apply` | Hazards are re-computed at apply time. If any blocking hazard is found and not allowed, apply refuses BEFORE running any statement. |

This means you can review hazards in the generated file before you ever try to apply:

```bash
$ pg-flux migrate generate --label drop_legacy
Generated: migrations/20260520_drop_legacy.sql (1 statement)

$ grep '\[HAZARD\]' migrations/20260520_drop_legacy.sql
-- [HAZARD DATA_LOSS] Drops column public.users.legacy_phone
```

## Auto-rewrites

pg-flux applies safety transformations by default. The current list:

| Pattern | Auto-rewrite | Flag to disable |
|---|---|---|
| `ADD CONSTRAINT CHECK ... ` | → `ADD ... NOT VALID; VALIDATE CONSTRAINT ...` | `--auto-not-valid=false` |
| `ADD CONSTRAINT FOREIGN KEY ...` | → `ADD ... NOT VALID; VALIDATE CONSTRAINT ...` | `--auto-not-valid=false` |
| `CREATE INDEX ...` (non-partitioned) | → `CREATE INDEX CONCURRENTLY ... IF NOT EXISTS` | (no toggle; always rewritten) |
| `SET NOT NULL` on large table | → 4-step staged pattern | `--set-not-null-reltuple-hint=0` |
| `DROP INDEX ...` | → `DROP INDEX CONCURRENTLY IF EXISTS` | (no toggle) |

Each rewrite emits a separate change in the plan, so they appear in `--dry-run` output and in the generated migration file.

> [!IMPORTANT]
> `CREATE INDEX CONCURRENTLY` can't run inside a transaction. pg-flux handles
> this by splitting the migration: non-concurrent DDL runs in the main txn;
> concurrent statements run autocommit after the txn commits. The tracking
> row is written after the last concurrent statement succeeds.

## Drift safety: baseline-hash check

Hazards aren't the only safety layer. Every generated migration embeds a sha256 of the live state at generate time. Apply refuses if live drifted:

```text
refusing to apply 20260520_add_role.sql: live database state has drifted
since this migration was generated (expected baseline=abc123…, live=def456…).
Re-run `pg-flux migrate generate` to rebase the migration,
or pass --force-after-drift to apply anyway.
```

See [Drift recovery](/docs/drift.html) for the playbook.

## Advisory locking

Two `migrate apply` runs against the same database serialize via a session-level advisory lock keyed by `host:port/db`. The losing process sees:

```text
Error: could not acquire migration advisory lock (another apply in progress)
```

Wait for the first to finish, or kill it if stuck.

## Mass-drop guard tuning

Default threshold is 25%. Lower for very small databases (where dropping 1 of 4 tables is already concerning), higher for tightly-coupled domains where reorganization is expected:

```yaml
# .pg-flux.yml
mass_drop_threshold: 50  # allow up to 50% drops by default
```

Or per-command:

```bash
pg-flux migrate apply --mass-drop-threshold=50
```

The guard only fires when DROPs exceed the percentage AND the live database has objects. An empty live database never trips it (nothing to lose).

## When hazards are wrong

pg-flux is conservative by design — it'd rather refuse a safe migration than apply a dangerous one. If you hit a `DATA_LOSS` hazard on what's genuinely a safe operation (e.g., dropping a column that's already empty and unused for months), the right move is:

```bash
pg-flux migrate apply --allow-hazards=DATA_LOSS
```

If you hit a hazard you think is mis-classified (i.e., pg-flux is wrong about whether it's dangerous), [file an issue](https://github.com/nexg/pg-flux/issues/new) with the migration. Hazard classifications get refined over time as real cases come in.
