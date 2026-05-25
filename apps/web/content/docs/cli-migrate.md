---
title: Migration commands
group: Reference
order: 2
description: pg-flux migrate generate / apply / rebase / status / repair / baseline / rollback.
---

## `pg-flux migrate generate`

Inspect the live database, diff against `schema/`, and write a timestamped `.sql` migration file.

```bash
pg-flux migrate generate [--label NAME] [--generate-undo] [--format separate|combined]
```

| Flag | Description |
|---|---|
| `--dry-run` | Print the SQL to stdout without writing any file |
| `--label <text>` | Appended to the timestamped filename: `20260520_103245_<label>.sql` |
| `--generate-undo` | Also write a best-effort reverse-migration as a separate `_undo.sql` file |
| `--format <fmt>` | Migration file format: `separate` (default) or `combined` (single file with `-- +migrate Up` / `-- +migrate Down` sections) |

The generated file embeds a `pg-flux-baseline-hash` header so `apply` can detect drift.

> [!TIP]
> Use `--dry-run` to preview a migration before committing it:
> ```bash
> $ pg-flux migrate generate --dry-run
> -- dry-run: 2 statement(s), no file written
> ALTER TABLE users ADD COLUMN phone text;
> CREATE INDEX idx_users_phone ON users (phone);
> ```

> [!NOTE]
> Generation is read-only — it queries `pg_catalog` and writes a file.
> Nothing mutates the database.

### Example

```bash
$ pg-flux migrate generate --label add_users_phone
Generated: migrations/20260520_103245_add_users_phone.sql (1 statement)
```

---

## `pg-flux migrate apply`

Apply all pending migration files in timestamp order.

```bash
pg-flux migrate apply [--dry-run] [--shadow-dsn URL] [--force-after-drift]
```

| Flag | Description |
|---|---|
| `--dry-run` | Print what would be applied; touch nothing |
| `--shadow-dsn <url>` | Optional disposable DB for pre-flight syntax / semantic validation |
| `--force-after-drift` | Apply even if the baseline-hash drift check fails |

Each migration runs inside a transaction (CONCURRENTLY statements run autocommit after). A session-level advisory lock prevents concurrent applies against the same database.

pg-flux retries acquisition for up to **30 seconds** (1-second intervals). If it still cannot acquire the lock, the error message includes the exact SQL to release it manually:

```sql
SELECT pg_advisory_unlock(7040926865817495040);
```

> [!WARNING]
> Skipping the drift check with `--force-after-drift` should be rare. The
> check exists to catch two scenarios: a manual change applied directly to the
> database outside pg-flux, or a parallel-branch conflict where a colleague's
> migration was deployed while yours was pending. In both cases the correct fix
> is to resolve the root cause — use `migrate rebase` for the parallel-branch
> scenario, or capture the manual change in a new migration — rather than
> bypassing the check. Bypassing it can apply a migration to a schema state it
> wasn't designed for.

> [!NOTE]
> `migrate apply` emits a warning when it detects an out-of-order migration
> (a file whose timestamp is earlier than the most recently applied migration).
> This is the signal that a parallel-branch conflict occurred and `migrate rebase`
> is needed before the migration can apply cleanly.

### Example

```bash
$ pg-flux migrate apply
apply 20260520_103245_add_users_phone.sql ...
      ok
Applied 1 migration(s), 0 already up to date.
```

---

## `pg-flux migrate status`

```bash
pg-flux migrate status
```

Lists every file in `migrations/` with its applied/pending state, apply timestamp, and whether Down SQL is available.

```bash
$ pg-flux migrate status
STATUS   FILENAME                                APPLIED AT                     DOWN
applied  20260518_140330_initial_schema.sql      2026-05-18 14:03:30.000000+00  no
applied  20260519_091012_add_posts_table.sql     2026-05-19 09:10:12.000000+00  yes
applied  20260520_103245_add_users_phone.sql     2026-05-20 10:32:45.000000+00  yes
pending  20260521_083011_add_audit_table.sql                                    yes
```

The `DOWN` column shows `yes` when the migration file contains a `-- +migrate Down` section (combined format) or a matching `_undo.sql` file exists (separate format).

---

## `pg-flux migrate rollback`

Roll back the last N applied migrations by executing the Down SQL embedded in each file and removing the tracking row.

```bash
pg-flux migrate rollback [N] [--dry-run]
```

`N` is a positional argument that defaults to `1`.

| Flag | Description |
|---|---|
| `--dry-run` | Print what would be rolled back without touching the database |

Migrations are rolled back in reverse-apply order (most recent first). Migrations with no Down SQL are skipped with a tip message. If all requested migrations are skipped, pg-flux exits with code **6**.

Each rollback runs inside a transaction. On failure the transaction is rolled back and the tracking row is left intact — the migration stays marked as applied.

> [!TIP]
> Use `--dry-run` to preview what would be rolled back without touching the database:
> ```bash
> $ pg-flux migrate rollback --dry-run
> would rollback 20260520_103245_add_users_phone.sql
>
> Dry run: would roll back 1 migration(s).
> ```

### Examples

```bash
# Roll back the most recently applied migration
$ pg-flux migrate rollback
rollback 20260520_103245_add_users_phone.sql ...
         ok
Rolled back 1 migration(s).

# Roll back the last 3 migrations
$ pg-flux migrate rollback 3
rollback 20260520_103245_add_users_phone.sql ...
         ok
rollback 20260519_091012_add_posts_table.sql ...
         ok
rollback 20260518_140330_initial_schema.sql ...
         ok
Rolled back 3 migration(s).
```

See the [Rollback guide →](/docs/rollback.html) for full details on Down SQL formats, combined files, and limitations.

---

## `pg-flux migrate rebase`

Regenerate all pending (unapplied) migration files against the current live database state.

```bash
pg-flux migrate rebase
```

Use this after pulling a colleague's merged migrations that were applied to a shared environment while your own migrations were pending. Rebase keeps the original filenames (timestamps + labels) so ordering is preserved — only the SQL content and baseline hash are updated.

| Situation | What rebase does |
|---|---|
| One pending file | Rewrites the file with new DDL (diff of desired schema vs. current live) and updates the baseline hash |
| Multiple pending files | Folds all changes into the first file (earliest timestamp), removes the rest. Commit the deletions. |
| No pending files | Prints "No pending migrations to rebase." and exits cleanly |
| Desired schema already matches live | Marks the file(s) as unchanged — they will be no-ops when applied |

> [!TIP]
> After rebasing, always review the updated file before applying. The new SQL reflects
> what still needs to change from the current live state — some statements from the
> original migration may no longer be needed.

### When to use rebase

The canonical signal that you need rebase is the drift error on `migrate apply`:

```text
Error: refusing to apply 20260601_100500_dev_b.sql: this migration was generated before
other changes were applied (expected baseline=1dcf92…, live=c28c4c…).
This is a parallel-development conflict: two branches were developed against the same
schema state and another branch was deployed first. To fix:
  1. Pull the latest migrations from your main branch.
  2. Run `pg-flux migrate apply` on your local DB to bring it up to date.
  3. Run `pg-flux migrate rebase` to regenerate this migration on top of current state.
  4. Commit the updated migration file and re-open your PR.
```

### Full rebase workflow

```bash
git pull origin main            # get the latest migrations from main
pg-flux migrate apply           # apply any new migrations to your local DB
pg-flux migrate rebase          # regenerate your pending migration
cat migrations/<your-file>.sql  # review the updated content
pg-flux migrate apply           # confirm it applies cleanly
git add migrations/
git commit -m "rebase migration"
git push
```

See [Working in a team →](/docs/teamwork.html) for full parallel-branch scenarios.

---

## `pg-flux migrate repair`

```bash
pg-flux migrate repair
```

Recomputes the tracking-table checksum for every already-applied file. Use after intentionally editing an applied migration's content (e.g. fixing a typo in a comment).

> [!CAUTION]
> Never edit the SQL statements of an already-applied migration. Repair is
> only for comment / whitespace fixes. Schema changes belong in a new migration.

---

## `pg-flux migrate baseline FILE`

```bash
pg-flux migrate baseline migrations/20260101_initial.sql
```

Marks a migration file as "already applied" without running its SQL. Used when adopting pg-flux against an existing database — the baseline file represents the starting state.
