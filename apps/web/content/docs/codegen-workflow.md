---
title: Codegen workflow
group: Codegen
order: 4
---

# The codegen workflow

How pg-flux fits into a typical app dev loop.

## Day-to-day

```bash
# 1. Edit schema
$ vim schema/tables/users.sql       # add a column

# 2. Generate + apply migration
$ pg-flux migrate generate --label add_users_phone
Generated: migrations/20260520_104301_add_users_phone.sql (1 statement)

$ pg-flux migrate apply
apply 20260520_104301_add_users_phone.sql ... ok

# 3. Regenerate types
$ pg-flux gen
[go] wrote 1 file (4 already up to date)
[ts] wrote 2 files (6 already up to date)
```

Only the files that *actually* changed are rewritten — pg-flux compares emitter output against on-disk content and skips identical files. Your `git diff` stays localised.

## Pre-commit hook

Catch out-of-date generated code locally before pushing:

```bash
# .git/hooks/pre-commit
#!/usr/bin/env bash
pg-flux gen --check || {
  echo "Generated types are stale — run 'pg-flux gen' to refresh."
  exit 1
}
```

## CI gate

```yaml
- name: codegen drift
  run: pg-flux gen --check --db "${{ secrets.DATABASE_URL }}"
```

This pairs nicely with `pg-flux verify --strict` for full drift coverage:

```yaml
- run: pg-flux verify --strict     # live ≠ source → fail
- run: pg-flux gen --check         # generated ≠ schema → fail
```

## Source-mode generation (offline CI)

Use `--from-source` to generate from `schema/` directly without contacting the database. Useful when the CI runner can't reach prod and you've checked in source files that fully describe the schema:

```bash
pg-flux gen --from-source --check
```

This relies entirely on the SQL parser. Function param/return types only appear when generated against a live DB (the inspector reads pg_proc); other kinds work the same in both modes.

## Multi-output

A single `.pg-flux-codegen.yml` can drive multiple output directories with different option sets:

```yaml
outputs:
  - lang: go
    out: ./internal/db
    orm_tags: sqlx

  - lang: ts
    out: ./apps/web/src/db
    column_case: camel
    branded_ids: true
    validators: zod

  - lang: ts
    out: ./apps/mobile/src/db
    column_case: camel
    bigint_as: string         # mobile prefers strings for IDs
```

`pg-flux gen` (no flags) runs all outputs in sequence. Each emits independently so a partial failure leaves others intact.

## Filtering

Don't generate types for tracking / audit / migration tables:

```yaml
exclude_tables:
  - "_pgflux_*"
  - "audit_*"
  - "schema_migrations"

exclude_schemas:
  - "_pgflux"
  - "internal"
```

Patterns use `filepath.Match` glob syntax (`*`, `?`, `[abc]`).

## Watching for changes (dev mode)

There's no built-in watch mode (yet). For local dev, pair `pg-flux gen` with any file-watcher:

```bash
# requires entr (apt/brew install entr)
ls schema/**/*.sql | entr -c pg-flux gen --from-source
```

## See also

- [Codegen overview →](/docs/codegen.html)
- [Configuration →](/docs/configuration.html)
- [Drift recovery →](/docs/drift.html)
