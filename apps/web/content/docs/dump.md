---
title: Dump · verify · pull
group: Dump & sync
order: 1
---

# Dump · verify · pull

These three read-only commands handle the bidirectional flow: live DB → source files. Combined, they let teams adopt pg-flux against existing databases and detect / capture out-of-band changes.

## dump

```bash
pg-flux dump --db "$DATABASE_URL" --output ./schema
```

Extracts every catalog object the inspector knows about — tables, enums, composite types, domains, views, sequences, functions, triggers, policies, event triggers, statistics, extensions, foreign tables/servers, default privileges — into source SQL files organised by kind:

```
schema/
  tables/<schema>.<name>.sql
  views/<schema>.<name>.sql
  types/<schema>.<name>.sql
  functions/<schema>.<name>.sql
  ...
```

The output is **round-trip clean**: running `pg-flux migrate generate` immediately after a dump produces zero pending changes. This property is enforced by an integration test that runs the dump → reload → diff loop against 4 representative fixtures.

### Options

- `--layout per-kind` (default) | `flat` (single `schema.sql`)
- `--schemas public,app` — limit dump scope
- `--force` — overwrite a non-empty target directory

## verify

```bash
pg-flux verify --db "$DATABASE_URL"
```

Read-only asymmetric diff: lists every catalog object present in the live DB but **not** declared in source. Use `--strict` in CI to fail the build:

```bash
pg-flux verify --strict
# verify: 2 undeclared live object(s):
#
#   Tables (1):
#     - public.hotfix_overrides
#
#   Indexes (1):
#     - public.idx_users_legacy
#
# Run `pg-flux pull` to capture these into schema/_pulled/<ts>.sql for review.
```

## pull

```bash
pg-flux pull --dry-run --db "$DATABASE_URL"
```

Renders the live-only objects into a quarantine file via the same emitters `dump` uses. `--dry-run` prints what would be captured; without it (or with `--dry-run=false`), pg-flux writes:

```
schema/_pulled/20260520_103245_pulled.sql
```

Source files you've edited are **never modified**. Users review the quarantine file and decide which blocks to promote into the regular source set.

## Typical workflow: adopting pg-flux against an existing DB

```bash
# 1. point pg-flux at the live DB and dump everything
pg-flux dump --db "$DATABASE_URL" --output ./schema --force

# 2. verify nothing escaped the dump
pg-flux verify --strict
# verify: clean — every live object is declared in source.

# 3. round-trip check
pg-flux migrate generate --label baseline-dump
# Generated: migrations/20260520_103245_baseline-dump.sql (0 statements)
# (zero statements = perfect dump)
```

## Continuous workflow: catching prod hotfixes

In CI on main:

```yaml
- run: pg-flux verify --strict --db "${{ secrets.DATABASE_URL }}"
```

If someone runs SQL by hand on production:

```bash
$ pg-flux verify --strict
verify: 1 undeclared live object(s):

  Indexes (1):
    - public.idx_emergency_perf

$ pg-flux pull
Wrote 1 object(s) to ./schema/_pulled/20260520_103245_pulled.sql
# review the file, move the relevant block into schema/indexes/public.users.sql, commit.
```

## See also

- [Migrations →](/docs/migrations.html) — how the round-trip closes
- [Codegen →](/docs/codegen.html) — pulled objects automatically flow into generated types
