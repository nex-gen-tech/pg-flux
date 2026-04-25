# 06 — CLI Interface & UX Design

**Document:** Command-Line Interface Design
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## 6.1 Design Philosophy

The pg-flux CLI is designed around three principles:

1. **Terraform-like mental model:** Developers already understand `plan` / `apply` from Terraform. pg-flux mirrors this workflow for database schemas.
2. **Safe by default:** Hazardous operations block by default. Destructive actions require explicit flags.
3. **CI/CD first:** Every command has a `--format=json` flag, deterministic exit codes, and supports secret injection via environment variables.

---

## 6.2 Command Structure

```
pg-flux [global flags] <command> [command flags]
```

### Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--log-level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `--no-color` | bool | `false` | Disable ANSI color output |
| `--format` | string | `human` | Output format: `human`, `json`, `sql` |
| `--config` | string | `.pg-flux.yml` | Path to config file |

---

## 6.3 Command Reference

### `pg-flux init`

**Purpose:** Bootstrap a new schema project with best-practice directory structure and example files.

```
pg-flux init [flags]
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--dir` | No | `./schema` | Target directory for schema files |
| `--db-name` | No | `myapp` | Database name used in example files |
| `--with-examples` | No | `true` | Generate example table/function/policy files |

**Behavior:**
1. Creates the directory if it does not exist.
2. Creates subdirectories: `tables/`, `functions/`, `policies/`, `indexes/`, `types/`.
3. Generates `.pg-flux.yml` configuration file.
4. Generates `tables/example_users.sql` with PG18 best-practice patterns.
5. Prints a getting-started guide to stdout.

**Exit Codes:**
- `0`: Success
- `1`: Directory already contains a `.pg-flux.yml` (would overwrite)

**Example Output:**
```
✓ Initialized pg-flux schema project in ./schema/

Directory structure:
  schema/
  ├── .pg-flux.yml
  ├── tables/
  │   └── example_users.sql
  ├── functions/
  ├── policies/
  ├── indexes/
  └── types/

Next steps:
  1. Edit schema/tables/example_users.sql to define your schema
  2. Run: pg-flux plan --db $DATABASE_URL --schema ./schema
  3. If the plan looks good: pg-flux apply --db $DATABASE_URL --schema ./schema
```

---

### `pg-flux inspect`

**Purpose:** Reverse-engineer an existing PostgreSQL 18 database into normalized `.sql` schema files.

```
pg-flux inspect [flags]
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--db` | Yes | `$DATABASE_URL` | PostgreSQL connection string |
| `--out` | No | `./schema` | Output directory for generated files |
| `--schemas` | No | `public` | Comma-separated list of schemas to inspect |
| `--exclude-tables` | No | — | Comma-separated table names to exclude |
| `--split-by` | No | `type` | How to split output files: `type` (one dir per object type) or `table` (one file per table) |

**Environment Variable Support:**
`--db` can be omitted if `DATABASE_URL` is set in the environment.

**Exit Codes:**
- `0`: Success
- `1`: Connection failed
- `2`: Permission denied on system catalogs

**Example Output (human mode):**
```
Connecting to postgres://...@localhost:5432/myapp... ✓

Inspecting schema: public
  Tables:          47
  Indexes:        112
  Functions:       18
  Triggers:         9
  RLS Policies:    14
  Sequences:       12

Writing to ./schema/...

✓ Inspection complete.

schema/
├── tables/
│   ├── users.sql
│   ├── orders.sql
│   └── ...
├── functions/
│   └── ...
├── policies/
│   └── ...
└── indexes/
    └── ...

TIP: Run `pg-flux plan --db $DATABASE_URL --schema ./schema` to verify the
     generated files match the live database (should produce zero changes).
```

---

### `pg-flux plan`

**Purpose:** Calculate the diff between desired schema and live database, run hazard detection, and output the execution plan.

```
pg-flux plan [flags]
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--db` | Yes | `$DATABASE_URL` | PostgreSQL connection string |
| `--schema` | Yes | `./schema` | Path to schema directory or file |
| `--allow-hazards` | No | — | Comma-separated list of hazard types to allow (e.g., `DATA_LOSS,TABLE_LOCK`) |
| `--out` | No | stdout | Write plan to file instead of stdout |
| `--format` | No | `human` | Output format: `human`, `json`, `sql` |
| `--schemas` | No | `public` | Database schemas to include in the diff |
| `--no-analyze` | No | `false` | Disable auto-inject of ANALYZE statements |
| `--validate` | No | `true` | Validate plan against a shadow database |

**Exit Codes:**
- `0`: Plan generated successfully, no unacknowledged hazards
- `1`: Unacknowledged hazards in plan (use `--allow-hazards` to proceed)
- `2`: Connection failed or schema parse error
- `3`: Shadow database validation failed

**Example Output (human mode — no hazards):**
```
pg-flux plan ─────────────────────────────────────

  Database:  postgres://...@localhost:5432/myapp
  Schema:    ./schema/ (12 files)
  Timestamp: 2026-04-25T14:32:01Z

Changes
───────
  + CREATE TABLE public.products (4 columns)
  ~ ALTER TABLE public.orders ADD COLUMN discount_pct numeric(5,2)
  ~ ALTER TABLE public.users RENAME COLUMN name TO full_name
  + CREATE INDEX CONCURRENTLY idx_orders_user_id ON public.orders (user_id)

Execution Plan  (4 statements)
──────────────
  [1]  ALTER TABLE public.users RENAME COLUMN name TO full_name
         lock_timeout: 3s

  [2]  ALTER TABLE public.orders ADD COLUMN discount_pct numeric(5,2)
         lock_timeout: 3s

  [3]  CREATE TABLE public.products ( ... )
         lock_timeout: 3s

  [4]  CREATE INDEX CONCURRENTLY idx_orders_user_id ON public.orders (user_id)
         ⚠ advisory: INDEX_BUILD — concurrent builds may impact I/O performance
         statement_timeout: 20min  |  lock_timeout: 3s

✓ No blocking hazards. Ready to apply.

  Run: pg-flux apply --db $DATABASE_URL --schema ./schema
```

**Example Output (human mode — with hazards):**
```
pg-flux plan ─────────────────────────────────────
  ...

⛔ Blocking Hazards Detected
─────────────────────────────
  [1]  DROP COLUMN public.users.legacy_token
         ✗ HAZARD: DATA_LOSS
         This will permanently delete all data in column 'legacy_token'.
         Estimated rows affected: ~2,400,000

         To proceed: --allow-hazards DATA_LOSS

  [2]  ALTER TABLE public.payments ALTER COLUMN amount TYPE bigint
         ✗ HAZARD: COLUMN_TYPE_CHANGE
         Changing column type from 'numeric(10,2)' to 'bigint' requires
         a full table rewrite. Table size: ~8.2GB. This will lock the
         table for an extended period.

         To proceed: --allow-hazards COLUMN_TYPE_CHANGE

✗ 2 blocking hazard(s). Cannot apply without acknowledgment.

  Use --allow-hazards DATA_LOSS,COLUMN_TYPE_CHANGE to proceed.
```

**JSON output (`--format=json`):**
```json
{
  "version": "1.0",
  "generated_at": "2026-04-25T14:32:01Z",
  "source_schema_hash": "a1b2c3d4",
  "live_schema_hash": "e5f6g7h8",
  "has_blocking_hazards": false,
  "hazards": [],
  "statements": [
    {
      "id": 1,
      "ddl": "ALTER TABLE public.users RENAME COLUMN name TO full_name",
      "operation_type": "RENAME_COLUMN",
      "object_type": "COLUMN",
      "object_name": "public.users.full_name",
      "is_concurrent": false,
      "hazards": [],
      "lock_timeout_ms": 3000,
      "statement_timeout_ms": 3000,
      "estimated_lock_duration_ms": 10
    },
    {
      "id": 4,
      "ddl": "CREATE INDEX CONCURRENTLY idx_orders_user_id ON public.orders USING btree (user_id)",
      "operation_type": "CREATE_INDEX",
      "object_type": "INDEX",
      "object_name": "public.idx_orders_user_id",
      "is_concurrent": true,
      "hazards": [
        {
          "type": "INDEX_BUILD",
          "severity": "advisory",
          "message": "Concurrent index builds may impact database I/O performance during build."
        }
      ],
      "lock_timeout_ms": 3000,
      "statement_timeout_ms": 1200000,
      "estimated_lock_duration_ms": 0
    }
  ]
}
```

---

### `pg-flux apply`

**Purpose:** Execute a generated migration plan against the live database.

```
pg-flux apply [flags]
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--db` | Yes | `$DATABASE_URL` | PostgreSQL connection string |
| `--schema` | Yes\* | `./schema` | Schema dir/file (generates plan on-the-fly) |
| `--plan` | Yes\* | — | Pre-generated plan file from `engine plan --out` |
| `--allow-hazards` | No | — | Hazard types to allow |
| `--dry-run` | No | `false` | Validate without executing |
| `--no-analyze` | No | `false` | Skip post-migration ANALYZE |
| `--lock-timeout` | No | `3s` | Statement lock timeout |
| `--statement-timeout` | No | `3s` (10min for CONCURRENTLY) | Statement execution timeout |

\* One of `--schema` or `--plan` is required.

**Execution Output:**
```
pg-flux apply ────────────────────────────────────

  Database:  postgres://...@localhost:5432/myapp
  Timestamp: 2026-04-25T14:35:01Z

Applying 4 statements...

  [1/4] ALTER TABLE public.users RENAME COLUMN name TO full_name
        ✓ OK  (12ms)

  [2/4] ALTER TABLE public.orders ADD COLUMN discount_pct numeric(5,2)
        ✓ OK  (8ms)

  [3/4] CREATE TABLE public.products (...)
        ✓ OK  (15ms)

[Transactional statements committed ✓]

  [4/4] CREATE INDEX CONCURRENTLY idx_orders_user_id ON public.orders (user_id)
        ⏳ Building index concurrently... (this may take a while)
        ✓ OK  (4m 12s)

Post-migration:
  ANALYZE public.users      ✓
  ANALYZE public.orders     ✓
  ANALYZE public.products   ✓

✓ Migration complete. 4 statements applied in 4m 13s.
```

**Exit Codes:**
- `0`: All statements applied successfully
- `1`: Migration failed (transactional statements rolled back, concurrent state reported)
- `2`: Plan had unacknowledged hazards
- `3`: Advisory lock could not be acquired (another migration in progress)
- `4`: Dry-run completed (no changes made)

---

### `pg-flux drift`

**Purpose:** Compare live database schema to desired schema and exit with a non-zero code if any drift is detected. Designed for CI/CD pipelines.

```
pg-flux drift [flags]
```

**Flags:**

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--db` | Yes | `$DATABASE_URL` | PostgreSQL connection string |
| `--schema` | Yes | `./schema` | Schema directory or file |
| `--format` | No | `human` | Output format: `human`, `json` |
| `--schemas` | No | `public` | Schemas to check |
| `--ignore` | No | — | Object types to ignore: `statistics`, `comments`, `sequences` |
| `--strict` | No | `false` | Fail on advisory-only differences (e.g., column comments) |

**Exit Codes:**
- `0`: No drift detected — schemas match exactly
- `1`: Drift detected — schemas differ

**Example GitHub Actions Integration:**
```yaml
- name: Check schema drift
  env:
    DATABASE_URL: ${{ secrets.PRODUCTION_DB_URL }}
  run: |
    pg-flux drift \
      --db "$DATABASE_URL" \
      --schema ./schema \
      --format json | tee drift-report.json

    # Exit code from pg-flux determines if the step passes or fails
```

---

## 6.4 Configuration File (`.pg-flux.yml`)

```yaml
# .pg-flux.yml
version: 1

# Default schema directory
schema_dir: ./schema

# Default target schemas in the database
target_schemas:
  - public

# Hazards that are always allowed (use with caution)
allowed_hazards: []

# Timeout settings
timeouts:
  lock_timeout: 3s
  statement_timeout: 3s
  concurrent_statement_timeout: 20m

# Post-migration actions
post_migration:
  analyze: true

# Disable drift checking for specific object types
drift_ignore:
  - statistics

# File glob patterns to include/exclude
include_patterns:
  - "**/*.sql"
exclude_patterns:
  - "**/migrations/**"
  - "**/*.draft.sql"
```

---

## 6.5 Environment Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | Default database connection string (used when `--db` is not provided) |
| `PGFLUX_ALLOWED_HAZARDS` | Override allowed hazards (comma-separated) |
| `PGFLUX_LOG_LEVEL` | Override log level |
| `PGFLUX_NO_COLOR` | Set to `1` to disable color output |
| `PGFLUX_CONFIG` | Path to config file |
| `PGPASSWORD` | PostgreSQL password (standard PG env var, passed through to pgx) |
| `PGSSLMODE` | SSL mode (standard PG env var) |

**Security Note:** Connection strings should never contain passwords in plaintext when used in CI logs. pg-flux redacts the password from all log output automatically.

---

## 6.6 Output Colors & Symbols

| Symbol | Meaning |
|--------|---------|
| `✓` (green) | Success / no change |
| `+` (green) | New object created |
| `~` (yellow) | Existing object modified |
| `-` (red) | Object dropped |
| `⚠` (yellow) | Advisory hazard |
| `⛔` (red) | Blocking hazard |
| `⏳` | In progress (concurrent operation) |

Colors are disabled automatically when stdout is not a TTY (e.g., in CI pipelines).
