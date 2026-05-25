---
title: Rollback
group: Reference
order: 3
description: Roll back applied migrations using embedded Down SQL — overview, formats, commands, and limitations.
---

## Overview

`pg-flux migrate rollback` undoes the last N applied migrations by executing the **Down SQL** embedded in each migration file, then removing the tracking row so the migration is marked as pending again.

Rollback is useful during local development when you need to re-run or revise a migration. In production, prefer writing a new forward migration — rollback is intentionally limited to the most recently applied migrations and requires pre-authored Down SQL.

What rollback can and cannot do automatically:

| Operation | Auto-reversible? |
|---|---|
| `CREATE TABLE` | Yes — `DROP TABLE` |
| `ADD COLUMN` | Yes — `DROP COLUMN` |
| `CREATE INDEX` | Yes — `DROP INDEX` |
| `CREATE TYPE` (enum) | Yes — `DROP TYPE` |
| `DROP TABLE` | No — data is gone |
| `DROP COLUMN` | No — data is gone |
| `ALTER COLUMN … TYPE` | No — requires explicit cast |
| Complex data transforms | No — must be hand-authored |

pg-flux does not auto-generate Down SQL for destructive operations. You must write those by hand in the `-- +migrate Down` section.

---

## Enabling Down SQL

There are three ways to get Down SQL into your migration files.

### Option A: Combined file format (recommended)

Set `migrate.format: combined` in `.pg-flux.yml`:

```yaml
migrate:
  format: combined
```

Every `migrate generate` will now write a **single file** with both sections:

```text
migrations/20260520_103245_add_users_phone.sql
```

The file contains both the forward and reverse SQL (see [Combined file format](#combined-file-format) below).

### Option B: Auto-generate separate undo files

Set `migrate.generate_undo: true` in `.pg-flux.yml`:

```yaml
migrate:
  generate_undo: true
```

Every `migrate generate` will write two files:

```text
migrations/20260520_103245_add_users_phone.sql
migrations/20260520_103245_add_users_phone_undo.sql
```

The `_undo.sql` file holds the best-effort reverse SQL. pg-flux will not treat it as a forward migration.

### Per-invocation override

Both options can be set on a single `generate` call without changing config:

```bash
# Write a combined up/down file this time only
pg-flux migrate generate --format combined --label add_users_phone

# Write a separate _undo.sql file this time only
pg-flux migrate generate --generate-undo --label add_users_phone
```

---

## Combined file format

A combined migration file uses `-- +migrate Up` and `-- +migrate Down` section markers:

```sql
-- pg-flux-baseline-hash: sha256:a3f2...
-- +migrate Up

ALTER TABLE users ADD COLUMN phone text;
CREATE INDEX idx_users_phone ON users (phone);

-- +migrate Down

DROP INDEX idx_users_phone;
ALTER TABLE users DROP COLUMN phone;
```

Rules:
- `-- +migrate Up` marks the start of the forward section. Everything before it (comments, the baseline-hash header) is ignored.
- `-- +migrate Down` marks the start of the reverse section.
- Both sections are optional — a file with no `-- +migrate Down` section has no Down SQL and will be skipped during rollback.
- Statements in the Down section should be the exact inverse of the Up section, in reverse order.

> [!NOTE]
> For destructive operations (`DROP TABLE`, `DROP COLUMN`, type changes), pg-flux
> leaves the `-- +migrate Down` section empty. You must fill it in manually before
> relying on rollback for those migrations.

---

## Running rollback

Roll back the most recently applied migration:

```bash
$ pg-flux migrate rollback
rolling back 20260520_103245_add_users_phone.sql ...
      ok
Rolled back 1 migration(s).
```

Roll back the last 3 applied migrations:

```bash
$ pg-flux migrate rollback 3
rolling back 20260520_103245_add_users_phone.sql ...
      ok
rolling back 20260519_091012_add_posts_table.sql ...
      ok
rolling back 20260518_140330_initial_schema.sql ...
      ok
Rolled back 3 migration(s).
```

Migrations are rolled back in reverse-apply order (most recent first).

---

## Dry run

Use `--dry-run` to preview what would be rolled back without touching the database:

```bash
$ pg-flux migrate rollback --dry-run
-- dry-run: would roll back 1 migration(s), no changes made

20260520_103245_add_users_phone.sql:
  DROP INDEX idx_users_phone;
  ALTER TABLE users DROP COLUMN phone;
```

---

## Checking Down SQL availability

`migrate status` includes a `down` column showing whether each migration has Down SQL available:

```bash
$ pg-flux migrate status
  migration                                    applied_at            down
  20260518_140330_initial_schema.sql           2026-05-18 14:03:30   no
  20260519_091012_add_posts_table.sql          2026-05-19 09:10:12   yes
  20260520_103245_add_users_phone.sql          2026-05-20 10:32:45   yes
```

A value of `no` in the `down` column means rollback will skip that migration.

---

## When there is no Down SQL

If a migration has no Down SQL, `migrate rollback` skips it and prints a tip:

```text
skipping 20260518_140330_initial_schema.sql — no Down SQL (add a -- +migrate Down section to enable rollback)
```

If **all** requested migrations are skipped due to missing Down SQL, pg-flux exits with code **6**:

```bash
$ pg-flux migrate rollback
skipping 20260518_140330_initial_schema.sql — no Down SQL (add a -- +migrate Down section to enable rollback)
All 1 migration(s) skipped — no Down SQL available.
$ echo $?
6
```

See [exit code 6](/docs/cli-overview.html) in the CLI reference.

---

## Operations that cannot be auto-reversed

pg-flux generates a best-effort Down section, but some DDL has no safe automatic inverse:

| DDL | Why it cannot be auto-reversed |
|---|---|
| `DROP TABLE` | Table data is permanently gone |
| `DROP COLUMN` | Column data is permanently gone |
| `ALTER COLUMN … TYPE` | Requires an explicit cast expression that depends on your data |
| `TRUNCATE` | Row data is permanently gone |

For these, pg-flux leaves the `-- +migrate Down` section empty (or omits it). You must write the reverse SQL manually before relying on rollback:

```sql
-- +migrate Down

-- TODO: restore the archived_posts table from backup before running this
CREATE TABLE archived_posts (
  id      bigint PRIMARY KEY,
  user_id bigint NOT NULL,
  title   text NOT NULL
);
```

> [!WARNING]
> Never rely on auto-generated Down SQL for migrations that delete data or change
> column types without reviewing the generated section first.

---

## Transaction safety

Each rollback runs inside a transaction. If the Down SQL fails partway through:

- The transaction is rolled back — the database is left unchanged.
- The tracking row is **not** deleted — the migration remains marked as applied.
- The error is printed to stderr and pg-flux exits with code 1.

This means a partial rollback failure leaves the database in its pre-rollback state: safe to retry once the Down SQL is corrected.

---

## FAQ

**Can I add Down SQL retroactively?**

Yes. Open the existing migration file and add a `-- +migrate Down` section at the end with the reverse SQL. If you use the separate-file format, create a matching `_undo.sql` file. After saving, `migrate status` will show `down=yes` for that migration.

**What if the Down SQL fails partway through?**

pg-flux wraps each rollback in a transaction. On failure, the transaction is rolled back and the tracking row is left intact — the migration remains marked as applied. Fix the Down SQL and retry.

**Does rollback work on migrations that used `CONCURRENTLY`?**

`CREATE INDEX CONCURRENTLY` and `DROP INDEX CONCURRENTLY` run outside a transaction. pg-flux will attempt to execute the Down SQL as written. If the Down section uses `CONCURRENTLY`, it will run autocommit — failure leaves the index in a partially-built state. In that case, inspect `pg_index.indisvalid` and clean up manually if needed.

**Can I roll back past the baseline migration?**

No. A baseline migration is marked as applied without running its SQL. It has no Down SQL and will always be skipped.

---

## See also

- [Migration commands →](/docs/cli-migrate.html)
- [Configuration →](/docs/configuration.html)
- [Exit codes →](/docs/cli-overview.html)
