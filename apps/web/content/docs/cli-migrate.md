---
title: Migration commands
group: Reference
order: 2
description: pg-flux migrate generate / apply / status / repair / baseline / rollback.
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
> check exists to catch the "someone manually changed prod" scenario — bypassing
> it can apply a migration to a state it wasn't designed for.

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
  migration                                    applied_at            down
  20260518_140330_initial_schema.sql           2026-05-18 14:03:30   no
  20260519_091012_add_posts_table.sql          2026-05-19 09:10:12   yes
  20260520_103245_add_users_phone.sql          2026-05-20 10:32:45   yes
  20260521_083011_add_audit_table.sql          (pending)             yes
```

The `down` column shows `yes` when the migration file contains a `-- +migrate Down` section (combined format) or a matching `_undo.sql` file (separate format). Pending migrations with Down SQL will be rolled back if applied and then rolled back.

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
> Use `--dry-run` to inspect the Down SQL before committing to a rollback:
> ```bash
> $ pg-flux migrate rollback --dry-run
> -- dry-run: would roll back 1 migration(s), no changes made
>
> 20260520_103245_add_users_phone.sql:
>   DROP INDEX idx_users_phone;
>   ALTER TABLE users DROP COLUMN phone;
> ```

### Examples

```bash
# Roll back the most recently applied migration
$ pg-flux migrate rollback
rolling back 20260520_103245_add_users_phone.sql ...
      ok
Rolled back 1 migration(s).

# Roll back the last 3 migrations
$ pg-flux migrate rollback 3
rolling back 20260520_103245_add_users_phone.sql ...
      ok
rolling back 20260519_091012_add_posts_table.sql ...
      ok
rolling back 20260518_140330_initial_schema.sql ...
      ok
Rolled back 3 migration(s).
```

See the [Rollback guide →](/docs/rollback.html) for full details on Down SQL formats, combined files, and limitations.

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
