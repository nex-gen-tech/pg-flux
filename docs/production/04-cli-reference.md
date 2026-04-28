# 04 — CLI Command Reference

---

## `pg-flux migrate generate`

Compares the live database against the desired schema files and writes a new timestamped migration file if any differences are found.

```
pg-flux migrate generate [flags]
```

**What it does:**

1. Reads all `.sql` files under `--schema` (recursively).
2. Parses them into a desired `SchemaState`.
3. Connects to `--db` and inspects system catalogs into a live `SchemaState`.
4. Diffs desired vs live; detects renames via `-- @renamed` annotations.
5. Runs the DAG dependency sorter to order statements safely.
6. Checks for hazards; blocks if any blocking hazard is found (unless `--allow-hazards`).
7. Writes the migration to `--migrations-dir/<timestamp>[_label].sql`.

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0` | Success — migration written (or no changes detected) |
| `1` | Blocking hazard found; migration not written |
| `2` | DB connection error |
| `3` | Schema parse error |

**Flags specific to this command:**

| Flag | Description |
|------|-------------|
| `--label <text>` | Appended to the filename: `20260428_120000_<label>.sql` |

**Example — generate with label:**

```bash
pg-flux migrate generate --label add_user_bio
# Generated: migrations/20260428_120000_add_user_bio.sql (2 statements)
```

**Example — allow a hazardous change:**

```bash
pg-flux migrate generate --allow-hazards COLUMN_TYPE_CHANGE,DATA_LOSS
```

**Example — JSON output (for CI scripts):**

```bash
pg-flux migrate generate --format json | jq .
```

```json
{
  "file": "migrations/20260428_120000.sql",
  "statements": 3,
  "hazards": [
    {
      "type": "COLUMN_TYPE_CHANGE",
      "severity": "BLOCKING",
      "object": "public.users.is_verified",
      "message": "Column type change may rewrite table"
    }
  ]
}
```

---

## `pg-flux migrate apply`

Applies all pending migration files in timestamp order.

```
pg-flux migrate apply [flags]
```

**What it does:**

1. Reads the tracking table (`_pgflux.migrations`) to find already-applied files.
2. For each pending file (in timestamp order):
   - (Optional) Validates via shadow DB in a rolled-back transaction.
   - Executes `regular` (transactional) statements inside a transaction.
   - Executes `CONCURRENTLY` statements outside the transaction (auto-commit).
   - Inserts the migration filename + SHA-256 checksum into the tracking table.
3. Reports applied / skipped counts.

**Tamper detection:** If a previously-applied migration file has been edited (checksum mismatch), apply aborts with an error. Never edit committed migration files.

**Exit codes:**

| Code | Meaning |
|------|---------|
| `0` | All pending migrations applied successfully |
| `1` | At least one migration failed; transaction rolled back |
| `2` | DB connection error |
| `3` | Checksum mismatch (applied file was edited) |

**Flags specific to this command:**

| Flag | Description |
|------|-------------|
| `--dry-run` | Print what would be applied; do not execute |
| `--shadow-dsn <dsn>` | Validate each migration in a rolled-back transaction on this DSN before touching the live DB |
| `--shadow-semantic` | With `--shadow-dsn`: apply plan to shadow DB (stronger validation; shadow DB is mutated) |
| `--shadow-equivalence` | With `--shadow-dsn`: after semantic apply, inspect shadow and assert it matches desired schema |

**Example — dry run:**

```bash
pg-flux migrate apply --dry-run
# would apply  20260428_120000.sql
# would apply  20260428_120100_add_user_bio.sql
```

**Example — with shadow validation:**

```bash
pg-flux migrate apply \
  --shadow-dsn postgres://user:pass@shadow-db:5432/shadow \
  --shadow-semantic
```

---

## `pg-flux migrate status`

Lists all migration files and whether they have been applied.

```
pg-flux migrate status [flags]
```

**Example output:**

```
APPLIED  20260428_115909_create_users.sql            2026-04-28 11:59:09
APPLIED  20260428_120000_add_user_bio.sql             2026-04-28 12:00:01
PENDING  20260428_120500_add_posts.sql
```

**JSON output:**

```bash
pg-flux migrate status --format json | jq '.[] | select(.applied == false)'
```

---

## `pg-flux inspect`

Reverse-engineers the live database into normalized `.sql` files suitable for use as schema source files.

```
pg-flux inspect [flags]
```

Useful for bootstrapping a pg-flux project against an existing database.

**Example:**

```bash
pg-flux inspect --db postgres://... --out ./schema-from-db
```

This writes one file per object type into `./schema-from-db/`.

---

## Global Flags (all commands)

See [03-configuration.md](./03-configuration.md) for the full flag reference.
