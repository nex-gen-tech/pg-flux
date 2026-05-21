---
title: Migration commands
group: Reference
order: 2
description: pg-flux migrate generate / apply / status / repair / baseline.
---

## `pg-flux migrate generate`

Inspect the live database, diff against `schema/`, and write a timestamped `.sql` migration file.

```bash
pg-flux migrate generate [--label NAME] [--generate-undo]
```

| Flag | Description |
|---|---|
| `--label <text>` | Appended to the timestamped filename: `20260520_103245_<label>.sql` |
| `--generate-undo` | Also write a best-effort reverse-migration alongside |

The generated file embeds a `pg-flux-baseline-hash` header so `apply` can detect drift.

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

Lists every file in `migrations/` with its applied/pending state and apply timestamp.

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
