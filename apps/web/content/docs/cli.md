---
title: CLI reference
group: Reference
order: 1
---

# CLI reference

Every command, every flag.

## Global flags

These work on every subcommand:

| Flag | Description |
|---|---|
| `--db <url>` | Database connection URL. Default: `$DATABASE_URL` |
| `--schema <dir>` | Source schema directory. Default: `./schema` |
| `--schema-file <path>` | Single-file schema mode |
| `--schemas <list>` | Comma-separated list of PG schemas to manage. Default: `public` |
| `--migrations-dir <dir>` | Default: `./migrations` |
| `--tracking-schema <name>` | Default: `_pgflux` |
| `--config <path>` | Default: `.pg-flux.yml` |
| `--log-format <fmt>` | `text` (default) or `json` (machine-parseable) |
| `--verbose` | Debug-level structured logs (per-statement timing, etc.) |
| `--allow-hazards <list>` | Comma-separated hazard names to opt into (e.g. `DATA_LOSS,COLUMN_TYPE_CHANGE`) |
| `--allow-mass-drop` | Bypass the >25% mass-drop guard |
| `--mass-drop-threshold <pct>` | Tune the guard. Default 25 |

## Commands

### `pg-flux init`

Scaffold a project:

```bash
pg-flux init [--dir ./schema] [--migrations-dir ./migrations]
```

### `pg-flux migrate generate`

```bash
pg-flux migrate generate [--label NAME] [--generate-undo]
```

### `pg-flux migrate apply`

```bash
pg-flux migrate apply [--dry-run] [--shadow-dsn URL]
                      [--force-after-drift]
```

### `pg-flux migrate status`

```bash
pg-flux migrate status
```

### `pg-flux migrate repair`

```bash
pg-flux migrate repair
```

### `pg-flux migrate baseline FILE`

```bash
pg-flux migrate baseline migrations/20260520_initial.sql
```

### `pg-flux plan`

Compute the diff without writing a migration file:

```bash
pg-flux plan [--format human|json]
```

### `pg-flux apply`

Apply a fresh plan (skips the migrations folder):

```bash
pg-flux apply [--dry-run] [--statement-timeout 20min]
```

### `pg-flux drift`

```bash
pg-flux drift [--strict]
```

Exit 1 if the live DB differs from `schema/`.

### `pg-flux inspect`

```bash
pg-flux inspect
```

Dump CREATE-style SQL for every catalog object (read-only, no codegen).

### `pg-flux dump`

```bash
pg-flux dump --output ./schema [--layout per-kind|flat] [--force]
```

### `pg-flux verify`

```bash
pg-flux verify [--strict]
```

### `pg-flux pull`

```bash
pg-flux pull [--dry-run] [--output ./schema/_pulled]
```

### `pg-flux gen`

```bash
pg-flux gen [--lang go|ts]... [--out DIR] [--check] [--from-source]
             [--package NAME] [--codegen-config FILE]

# Emit option flags (every config-file field also exposed as a flag):
             [--column-case snake|camel|pascal]
             [--bigint-as bigint|number|string]
             [--date-as Date|string|temporal]
             [--null-style union|undefined|optional]
             [--enum-style union|const-object|ts-enum]
             [--orm-tags sqlx|gorm|bun|ent]
             [--omitempty nullable|defaults|all]
             [--validators zod]
             [--include-tables PATTERN]... [--exclude-tables PATTERN]...
             [--exclude-schemas PATTERN]...
             [--branded-ids] [--insert-update-helpers]
             [--readonly identity|generated|defaults|all]
             [--functions]
```

### `pg-flux gen init`

```bash
pg-flux gen init [--out .pg-flux-codegen.yml] [--force]
```

### `pg-flux version`

```bash
pg-flux version
```

## See also

- [Configuration →](/docs/configuration.html)
- [Hazards →](/docs/hazards.html)
