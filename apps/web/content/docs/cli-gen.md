---
title: Codegen commands
group: Reference
order: 3
description: pg-flux gen — generate Go, TypeScript, Python, and Rust types from the schema.
---

## `pg-flux gen`

Generate application-language types from the schema. Default output is `./internal/dbgen` for Go.

```bash
pg-flux gen [--lang go|ts|python|rust]... [--out DIR] [--check] [--from-source]
            [--package NAME] [--codegen-config FILE]
```

### Source control

| Flag | Default | Description |
|---|---|---|
| `--lang <list>` | `go` | Target language(s); repeatable: `go,ts,python,rust` |
| `--out <dir>` | `./internal/dbgen` | Output directory (single-output mode) |
| `--package <name>` | `dbgen` | Go package name |
| `--from-source` | off | Read from `schema/` instead of the live DB |
| `--check` | off | Exit 1 if on-disk files differ from emitter output. Exit code 3 when files are stale. |
| `--codegen-config <file>` | `.pg-flux-codegen.yml` | Multi-output config |

### Emit option flags

Every config-file option also has a flag for the common single-output case:

| Flag | Choices |
|---|---|
| `--column-case` | `snake` (default) · `camel` · `pascal` |
| `--bigint-as` | `bigint` (default) · `number` · `string` (TS) |
| `--date-as` | `Date` (default) · `string` · `temporal` (TS) |
| `--null-style` | `union` (default) · `undefined` · `optional` (TS) |
| `--enum-style` | `union` (default) · `const-object` · `ts-enum` (TS) |
| `--orm-tags` | `sqlx` · `gorm` · `bun` · `ent` (Go) |
| `--omitempty` | `nullable` · `defaults` · `all` (Go) |
| `--validators` | `zod` (TS, opt-in) |
| `--branded-ids` | bool (TS) |
| `--insert-update-helpers` | bool (TS) |
| `--readonly` | `identity` · `generated` · `defaults` · `all` |
| `--functions` | bool — emit function/procedure types |
| `--include-tables <pat>` | repeatable; glob patterns |
| `--exclude-tables <pat>` | repeatable; glob patterns |
| `--exclude-schemas <pat>` | repeatable |

### Example

```bash
pg-flux gen --lang go,ts --validators=zod --column-case=camel --branded-ids
```

Produces:

```text
internal/dbgen/    (Go)
  tables.go enums.go types.go views.go functions.go

src/db/            (TS — when --out=./src/db)
  tables.ts enums.ts types.ts views.ts functions.ts
  brands.ts validators.ts index.ts

gen/               (Python — when --out=./gen)
  models.py

src-rust/db/       (Rust — when --out=./src-rust/db)
  tables.rs enums.rs views.rs types.rs functions.rs mod.rs
```

> [!TIP]
> Prefer a config file for projects with more than one output. Drive every
> output from one place: `pg-flux gen` (no flags) processes them all.

---

## `pg-flux gen init`

```bash
pg-flux gen init [--out .pg-flux-codegen.yml] [--force]
```

Scaffolds a `.pg-flux-codegen.yml` with every option inline-documented as a comment. Refuses to overwrite an existing file unless `--force` is set.

The scaffolded file has two pre-populated outputs (Go + TS) covering the common case; users delete what they don't need.
