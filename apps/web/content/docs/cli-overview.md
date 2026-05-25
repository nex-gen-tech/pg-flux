---
title: CLI overview
group: Reference
order: 1
description: pg-flux command tree, global flags, and exit-code conventions.
---

The pg-flux CLI is one binary with focused subcommands. Each command is read-only by default — anything that mutates state requires an explicit opt-in (a hazard allowance, `--force`, or running an `apply`).

## Command tree

```text
pg-flux
├── init                     scaffold .pg-flux.yml + schema/ + migrations/
├── migrate
│   ├── generate            diff source vs live, write a .sql file
│   ├── apply               apply pending migration files
│   ├── status              list applied / pending / down-sql availability
│   ├── rollback [N]        roll back the last N applied migrations
│   ├── repair              recompute checksums after editing applied files
│   └── baseline FILE       mark a file as already-applied
├── plan                     compute diff without writing a file
├── apply                    apply the in-memory plan
├── drift                    live ≠ source? exit 2
├── verify [--strict]        live ⊃ source? exit 1
├── inspect                  dump every catalog object as CREATE-style SQL
├── dump                     extract live schema to source files
├── pull                     capture undeclared live objects to quarantine
├── gen [init]               generate Go / TypeScript / Python / Rust types
└── version
```

## Global flags

Every subcommand inherits these:

| Flag | Default | Description |
|---|---|---|
| `--db <url>` | `$DATABASE_URL` | PostgreSQL connection URL |
| `--schema <dir>` | `./schema` | Source schema directory |
| `--schema-file <path>` | — | Single-file source mode |
| `--schemas <list>` | `public` | PG schemas to manage (comma-separated) |
| `--migrations-dir <dir>` | `./migrations` | Migration file directory |
| `--tracking-schema <name>` | `_pgflux` | Schema holding the tracking table |
| `--config <path>` | `.pg-flux.yml` | Tool config file |
| `--log-format <fmt>` | `text` | `text` or `json` |
| `--verbose` | off | Debug-level structured logs |
| `--allow-hazards <list>` | — | Comma-separated hazard names to opt in |
| `--allow-mass-drop` | off | Bypass the >25% mass-drop guard |
| `--mass-drop-threshold <pct>` | `25` | Tune the mass-drop guard |

## Exit codes

| Code | When |
|---|---|
| `0` | Command succeeded with no notable conditions |
| `1` | Generic error (parse failure, connection refused, blocking hazard) |
| `2` | Drift detected by `drift --strict` |
| `3` | Stale codegen detected by `gen --check` |
| `4` | Undeclared live objects detected by `verify --strict` |
| `5` | Hazard blocked — `migrate apply` refused a blocking hazard; re-run with `--allow-hazards` |
| `6` | `migrate rollback` — all requested migrations had no Down SQL and were skipped |

> [!TIP]
> CI pipelines should treat any non-zero exit as a hard failure unless the
> command was invoked with the matching `--allow-*` flag or expected status.

## Configuration precedence

For any setting, pg-flux uses the first non-empty source:

1. **CLI flag** (highest)
2. **Environment variable** (`DATABASE_URL`, `PGFLUX_SHADOW_DSN`, etc.)
3. **Config file** (`.pg-flux.yml` and `.pg-flux-codegen.yml`)
4. **Built-in default**

## What's next

- [Migration commands →](/docs/cli-migrate.html)
- [Codegen commands →](/docs/cli-gen.html)
- [Dump / verify / pull →](/docs/cli-dump.html)
- [Other commands →](/docs/cli-other.html)
