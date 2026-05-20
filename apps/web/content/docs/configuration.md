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
```

Created by `pg-flux init`.

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

## See also

- [CLI reference →](/docs/cli.html)
- [Codegen overview →](/docs/codegen.html)
