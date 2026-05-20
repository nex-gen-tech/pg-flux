# 03 — Configuration Reference

---

## Config File

pg-flux looks for `.pg-flux.yml` in the current working directory, or at the path given by `--config`.

```yaml
# .pg-flux.yml — full example with all available keys

# Connection string. Can use $DATABASE_URL env var substitution.
# Accepts any libpq-compatible DSN or keyword=value string.
db: postgres://migrations_user:secret@db.prod.example.com:5432/myapp

# Directory that contains .sql source files (the "desired" state).
# All .sql files are read recursively.
schema: ./schema

# Alias for `schema`; both are accepted.
schema_dir: ./schema

# Directory where generated migration files are written.
migrations_dir: ./migrations

# Comma-separated list of PostgreSQL schemas to manage.
# pg-flux only inspects and diffs objects inside these schemas.
schemas: public,billing

# Schema used for the migration tracking table (_pgflux.migrations by default).
tracking_schema: _pgflux

# Output format: human (default) or json.
format: human

# When set, pg-flux validates each pending migration file against this DSN
# by running it inside a rolled-back transaction before touching the live DB.
# Requires a throwaway / shadow database, NOT the live production DB.
shadow_dsn: postgres://migrations_user:secret@shadow.internal:5432/shadow_myapp

# Comma-separated list of hazard types that are allowed without an error.
# Overridden per-run by --allow-hazards on the CLI.
allow_hazards: ""

# When > 0, emit a STAGED_SET_NOT_NULL advisory hazard for SET NOT NULL
# on tables with more than this many estimated rows (from pg_class.reltuples).
set_not_null_reltuple_hint: 100000

# Emit a synthetic VALIDATE CONSTRAINT after ADD CONSTRAINT ... NOT VALID.
append_validate_not_valid: false
```

---

## Environment Variables

Any `--flag` can also be supplied as an environment variable using the pattern `PGFLUX_<FLAG_UPPERCASED>`.

| Environment variable | Equivalent flag |
|---------------------|----------------|
| `DATABASE_URL` | `--db` (fallback only) |
| `PGFLUX_DB` | `--db` |
| `PGFLUX_SHADOW_DSN` | `--shadow-dsn` |
| `PGFLUX_TRACKING_SCHEMA` | `--tracking-schema` |
| `PGFLUX_ALLOW_HAZARDS` | `--allow-hazards` |
| `PGFLUX_FORMAT` | `--format` |

The lookup order is: **CLI flag → env var → config file → built-in default**.

---

## Flag Reference

All flags are available on both sub-commands (`migrate generate`, `migrate apply`) unless noted.

### Connection & Target

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--db` | string | `postgres://pgflux:pgflux@localhost:5432/exampledb` | PostgreSQL DSN |
| `--schemas` | string | `public` | Comma-separated schema list to manage |
| `--schema` / `--schema-dir` | string | `./schema` | Directory containing `.sql` source files |
| `--schema-file` | string | — | Single `.sql` file instead of a directory |
| `--migrations-dir` | string | `./migrations` | Where migration files are read/written |
| `--tracking-schema` | string | `_pgflux` | Schema for the tracking table |
| `--config` | string | `.pg-flux.yml` | Config file path |

### Safety & Hazards

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--allow-hazards` | string | — | Comma-separated hazard types to allow through (e.g. `DATA_LOSS,COLUMN_TYPE_CHANGE`) |
| `--set-not-null-reltuple-hint` | float | `0` | Warn on SET NOT NULL when table has more than N estimated rows |
| `--append-validate-not-valid` | bool | `false` | Auto-append `VALIDATE CONSTRAINT` after `NOT VALID` adds |

### Validation

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--shadow-dsn` | string | — | DSN of a throwaway DB; validates each migration in a rolled-back transaction before applying to live |
| `--shadow-semantic` | bool | `false` | With `--shadow-dsn`: apply the full plan on the shadow DB (mutates it); requires a disposable DB |
| `--shadow-equivalence` | bool | `false` | With `--shadow-dsn`: after semantic apply, inspect shadow DB and require it to match desired schema |
| `--validate-sql` | bool | `false` | Re-parse every emitted statement through pg_query (FR-01 check) |
| `--validate-plpgsql` | bool | `false` | Parse-check PL/pgSQL function bodies with pg_query |

### Output

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `human` | Output format: `human` or `json` |

### `migrate generate` only

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--label` | string | — | Short description appended to the generated filename |

### `migrate apply` only

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Print what would be applied; do not execute |

---

## Minimal Production Config

```yaml
# .pg-flux.yml (production)
db: ${DATABASE_URL}
schema: ./schema
migrations_dir: ./migrations
schemas: public
tracking_schema: _pgflux
shadow_dsn: ${SHADOW_DATABASE_URL}
set_not_null_reltuple_hint: 50000
append_validate_not_valid: true
```
