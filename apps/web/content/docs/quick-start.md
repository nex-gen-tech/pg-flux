---
title: Quick start
group: Getting started
order: 1
---

# Quick start

Get from zero to a working pg-flux setup in five minutes. By the end of this guide you'll have a managed schema, a generated migration, and typed Go + TypeScript modules.

## Install

```bash
go install github.com/nexg/pg-flux/cmd/pg-flux@latest
```

Or grab a binary from the [releases page](https://github.com/nexg/pg-flux/releases).

Verify the install:

```bash
pg-flux version
# pg-flux v0.1.0
```

You'll also need:

- **PostgreSQL 14 or newer** running somewhere you can reach.
- For codegen: **Go 1.25+** (if generating Go types) and/or **TypeScript** (any modern version) in the target project.

## Scaffold a project

In a fresh directory:

```bash
pg-flux init
```

This creates:

```
.pg-flux.yml          # tool-level config (DB URL, schema dir, migrations dir)
schema/               # your declarative SQL lives here
  schema.sql          # example seed file
migrations/           # generated migrations land here
```

Edit `.pg-flux.yml` to point at your database:

```yaml
version: 1
db: postgres://postgres:postgres@localhost:5432/mydb?sslmode=disable
schema_dir: ./schema
migrations_dir: ./migrations
target_schemas: [public]
```

(Or set `DATABASE_URL` in the environment — the CLI flag and `--db` always win.)

## Write your schema declaratively

Replace `schema/schema.sql` with the schema you want — exactly the SQL you'd run by hand:

```sql
CREATE TYPE user_role AS ENUM ('admin', 'member', 'guest');

CREATE TABLE users (
  id         bigint PRIMARY KEY,
  email      text NOT NULL,
  role       user_role NOT NULL DEFAULT 'member',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE posts (
  id      bigint PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES users(id),
  title   text NOT NULL,
  body    text NOT NULL DEFAULT ''
);
```

You can split across many files — pg-flux loads `schema/**/*.sql`.

## Generate a migration

```bash
pg-flux migrate generate --label initial_schema
```

pg-flux inspects your live database, computes the minimum-DDL diff, and writes a migration file:

```
Generated: migrations/20260520_103245_initial_schema.sql (8 statements)
```

The file embeds a baseline hash so `apply` can detect drift if anyone manually modifies the DB before you apply.

## Apply

```bash
pg-flux migrate apply
```

Output:

```
apply 20260520_103245_initial_schema.sql ...
      ok
Applied 1 migration(s), 0 already up to date.
```

The migration runs inside an advisory-locked transaction. If you re-run it, pg-flux notices it's already applied and skips it.

## Generate types

For Go:

```bash
pg-flux gen --lang go --out ./internal/dbgen
```

For TypeScript (with the opinionated options most apps want):

```bash
pg-flux gen --lang ts --out ./src/db \
  --column-case=camel --bigint-as=number --date-as=string \
  --null-style=optional --enum-style=const-object \
  --branded-ids --insert-update-helpers --validators=zod
```

You'll get one file per object kind:

```
internal/dbgen/
  tables.go         # struct per table
  enums.go          # typed constants + sql.Scanner/Valuer
  types.go          # composite types + domains
  views.go          # read-only view rows
  functions.go      # function param + result types

src/db/
  tables.ts         # interfaces + Insert/Update helpers
  enums.ts          # const-object + derived type
  brands.ts         # type UserId = number & { __brand: "UserId" }
  validators.ts     # parallel zod schemas
  index.ts          # barrel re-export
```

Want to scaffold a full codegen config?

```bash
pg-flux gen init
```

Writes `.pg-flux-codegen.yml` with every option documented inline.

## Iterate

Whenever you change `schema/`, run the same two commands:

```bash
pg-flux migrate generate --label add_column
pg-flux migrate apply
pg-flux gen
```

For CI, use the `--check` flags to fail builds on stale state:

```bash
pg-flux drift --strict       # exit 1 if live ≠ source
pg-flux verify --strict      # exit 1 if live has undeclared objects
pg-flux gen --check          # exit 1 if generated files are stale
```

## What next?

- **[Migrations →](/docs/migrations.html)** — generate / apply / status / repair / baseline / undo
- **[Codegen →](/docs/codegen.html)** — every emit option, hints, config file reference
- **[Dump · verify · pull →](/docs/dump.html)** — bootstrap pg-flux against an existing DB
- **[Hazards →](/docs/hazards.html)** — how pg-flux keeps migrations safe under load
- **[CLI reference →](/docs/cli.html)** — every command and flag
