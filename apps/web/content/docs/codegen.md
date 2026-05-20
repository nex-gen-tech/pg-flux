---
title: Codegen overview
group: Codegen
order: 1
---

# Codegen

pg-flux generates **application-language type definitions** for every catalog object that has a row shape. Go structs, TypeScript interfaces, enum constants, composite types, domains, views (with inferred column types), function parameters, and procedure parameters — all derived from the same schema model that powers migrations.

```bash
pg-flux gen --lang go,ts
```

Two opt-in flavors (every option configurable per output):

```bash
pg-flux gen init   # scaffold .pg-flux-codegen.yml with every option documented
pg-flux gen        # respects the config; multi-output capable
pg-flux gen --check  # exit 1 if generated files are stale (CI gate)
```

## What gets generated

| PG object | Go | TypeScript |
|---|---|---|
| Table | `type User struct { … }` | `interface User { … }` |
| View / matview | struct with typed fields | interface with typed fields |
| Enum | `type Role string` + constants + `Scan`/`Value` | union / const-object / ts-enum |
| Composite type | struct | interface |
| Domain | type alias | type alias |
| Function | `<Name>Params` + `<Name>Result`/`Row` | `<Name>Params` + `<Name>Result`/`Row` |
| Procedure | `<Name>Params` only | `<Name>Params` only |

Sequences, indexes, triggers, policies aren't generated — they don't have row shapes. (See [Function signatures →](/docs/codegen-functions.html) for the rationale.)

## Per-output options

The config file or CLI can drive every shape decision:

| Option | Choices | Applies to |
|---|---|---|
| `column_case` | snake (default) / camel / pascal | both |
| `readonly` | none / identity / generated / defaults / all | both |
| `bigint_as` | bigint (default) / number / string | TS |
| `date_as` | Date (default) / string / temporal | TS |
| `null_style` | union (default) / undefined / optional | TS |
| `enum_style` | union (default) / const-object / ts-enum | TS |
| `branded_ids` | bool | TS |
| `insert_update_helpers` | bool | TS |
| `validators` | zod / "" | TS |
| `orm_tags` | sqlx / gorm / bun / ent / "" | Go |
| `omitempty` | nullable / defaults / all / "" | Go |
| `functions` | bool — emit function param/result types | both |
| `json_shapes` | map of `"schema.table.column" → TS type` | TS |
| `type_overrides` | per PG type → custom language type | both |
| `include_tables` / `exclude_tables` / `exclude_schemas` | globs | both |

## Example: multi-target config

```yaml
# .pg-flux-codegen.yml
outputs:
  - lang: go
    out: ./internal/db
    package: db
    orm_tags: sqlx
    omitempty: nullable
    functions: true
    type_overrides:
      numeric: github.com/shopspring/decimal.Decimal

  - lang: ts
    out: ./apps/web/src/db
    column_case: camel
    bigint_as: number
    date_as: string
    null_style: optional
    enum_style: const-object
    branded_ids: true
    insert_update_helpers: true
    validators: zod
    functions: true

  - lang: ts
    out: ./apps/mobile/src/db
    column_case: camel
    bigint_as: string         # mobile prefers strings for IDs
    date_as: Date
    null_style: union
```

One schema, three outputs, all idiomatic for their stack.

## Comment hints

Add per-column overrides via PG comments:

```sql
COMMENT ON COLUMN posts.metadata IS 'pg-flux: tstype={ source: string; ip?: string } gotype=*postsmeta.Metadata';
```

Tokens:

- `gotype=...` — fully-qualified Go type (may include import path)
- `tstype=...` — verbatim TS type expression
- Anything before `pg-flux:` becomes the field's documentation comment

The tokenizer balances braces/brackets/parens, so complex type expressions don't need quoting:

```sql
pg-flux: tstype=Array<{ k: string; v: number }> gotype=map[string][]byte
```

## See also

- [Function signatures →](/docs/codegen-functions.html)
- [zod validators →](/docs/codegen-zod.html)
- [Migration → codegen integration →](/docs/codegen-workflow.html)
