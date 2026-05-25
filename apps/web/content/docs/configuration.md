---
title: Configuration
group: Configuration
order: 1
---

# Configuration files

pg-flux uses two YAML files, both optional. CLI flags always override config-file values.

## `.pg-flux.yml` — tool config

Tool-level settings the CLI reads on every command:

```yaml
version: 1

# Database connection (env $DATABASE_URL overrides; --db CLI flag wins over both).
db: postgres://postgres:postgres@localhost:5432/mydb?sslmode=disable

# Directories.
schema_dir: ./schema
migrations_dir: ./migrations

# Which PG schemas to manage.
target_schemas: [public]

# Schema that holds pg-flux's own tracking table (_pgflux.migrations).
tracking_schema: _pgflux

# Migration generation defaults (all optional).
migrate:
  generate_undo: true   # auto-write Down SQL on every migrate generate
  format: combined      # "separate" (default) | "combined"
```

Created by `pg-flux init`.

### `migrate:` keys

The `migrate:` block configures migration generation defaults. All keys are optional; omit the block entirely to use the built-in defaults.

| Key | Default | Description |
|---|---|---|
| `generate_undo` | `false` | When `true`, every `migrate generate` also writes a best-effort reverse migration without needing `--generate-undo` |
| `format` | `separate` | `separate` — forward SQL in one file, reverse SQL in a separate `_undo.sql` file; `combined` — a single file with `-- +migrate Up` and `-- +migrate Down` sections |

If you only need `generate_undo`, you can omit `format`:

```yaml
migrate:
  generate_undo: true
```

If you only need the combined format (and are happy writing Down SQL by hand):

```yaml
migrate:
  format: combined
```

Both keys can be overridden per invocation:

```bash
pg-flux migrate generate --generate-undo          # one-off separate undo file
pg-flux migrate generate --format combined         # one-off combined format
```

> [!NOTE]
> `migrate.format: combined` and `migrate.generate_undo: true` are independent.
> Combined format embeds the Down section in the same file; `generate_undo` writes
> a separate `_undo.sql`. Setting both will produce a combined file (the
> `generate_undo` config is superseded when `format: combined` is active).

See the [Rollback guide →](/docs/rollback.html) for end-to-end usage.

## `.pg-flux-codegen.yml` — codegen config

Per-output codegen settings:

```yaml
outputs:
  - lang: go
    out: ./internal/db
    package: db
    orm_tags: sqlx              # "" | sqlx | gorm | bun | ent
    omitempty: nullable         # "" | nullable | defaults | all
    readonly: identity          # none | identity | generated | defaults | all
    functions: true             # emit function/procedure types
    column_case: snake          # snake | camel | pascal

    type_overrides:
      numeric: github.com/shopspring/decimal.Decimal
      uuid:    github.com/google/uuid.UUID

  - lang: ts
    out: ./src/db
    column_case: camel
    bigint_as: number           # bigint | number | string
    date_as: string             # Date | string | temporal
    null_style: optional        # union | undefined | optional
    enum_style: const-object    # union | const-object | ts-enum
    branded_ids: true
    insert_update_helpers: true
    validators: zod             # "" | zod
    functions: true

    json_shapes:
      public.users.metadata: '{ source: string; ip?: string }'

    type_overrides:
      numeric: string
      uuid:    string

  - lang: python
    out: ./gen
    null_style: optional          # optional | union
    enum_style: strenum           # strenum | enum
    functions: false
    type_overrides:
      numeric: decimal.Decimal

  - lang: rust
    out: ./src/db
    functions: false
    type_overrides:
      numeric: rust_decimal::Decimal

# Filtering — applies per output.
exclude_tables: ["_pgflux_*", "audit_*"]
exclude_schemas: ["_pgflux", "audit"]
```

Created by `pg-flux gen init` with every option documented inline.

## Comment hints

Inline per-column / per-table overrides via SQL COMMENT:

```sql
COMMENT ON COLUMN posts.metadata IS 'pg-flux: tstype={ source: string; ip?: string } gotype=encoding/json.RawMessage';
```

Recognised tokens:

| Token | Effect |
|---|---|
| `gotype=<qualified type>` | Per-column Go type. May be fully-qualified: `gotype=github.com/foo/bar.Baz` |
| `tstype=<TS expression>` | Per-column TS type. Verbatim; can be complex objects |
| `nullable=force` | Force nullable even when column is NOT NULL (rare) |

Any text before `pg-flux:` becomes the field's documentation comment.

## Environment variables

| Variable | Used by | Equivalent CLI flag |
|---|---|---|
| `DATABASE_URL` | every command | `--db` |
| `PGFLUX_SHADOW_DSN` | migrate/apply | `--shadow-dsn` |
| `PGFLUX_TEST_DSN` | integration tests | (test-only) |

## CLI flag precedence

1. CLI flag (highest)
2. Environment variable
3. `.pg-flux.yml` / `.pg-flux-codegen.yml`
4. Built-in default (lowest)

## Config key validation

pg-flux validates `.pg-flux.yml` on every run. Unknown keys trigger a warning with a spelling suggestion:

```text
warning: unknown config key "migraitons" in .pg-flux.yml — did you mean "migrations"?
```

The suggestion uses Levenshtein distance, so it catches common typos. If no close match exists, the key is still listed so you can find it.

## See also

- [CLI reference →](/docs/cli-overview.html)
- [Codegen overview →](/docs/codegen.html)
