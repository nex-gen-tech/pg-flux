---
title: Examples
group: Getting started
order: 10
description: Five real-world reference apps that demonstrate pg-flux migrations, drift detection, and codegen across Go, TypeScript, Python, and Rust.
---

The `examples/` directory in the repository contains five standalone applications. Each one is a complete project — real schema, real migrations, generated types, and a running HTTP server — designed to show how pg-flux fits into different stacks. Every example passes `pg-flux drift` and `pg-flux verify` cleanly and runs end-to-end in CI.

Use them as reference implementations when setting up pg-flux in your own project.

## Summary

| Example | Stack | Key PG features demonstrated |
|---|---|---|
| [`fastapi-todo`](#fastapi-todo) | Python + FastAPI + psycopg3 | UUID PKs, enums, RLS, Python codegen |
| [`express-bookmarks`](#express-bookmarks) | TypeScript + Express + node-postgres | JSONB, `tsvector`, GIN indexes, DB-side trigger |
| [`go-events`](#go-events) | Go + chi + pgx/v5 | IDENTITY columns, materialized view, deferrable FK, `text[]` GIN |
| [`go-shop`](#go-shop) | Go + chi + pgx/v5 | Multiple schemas, partitioned table, EXCLUDE constraint, BRIN index, INCLUDE index, domains, composite type, SECURITY DEFINER, stored procedure, grants |
| [`rust-hrm`](#rust-hrm) | Rust + Actix-web + sqlx | Everything in go-shop plus `daterange`, `tstzrange`, `pg_trgm` trigram GIN, window function in matview, multiple EXCLUDE constraints, self-referential table, Rust codegen |

---

## fastapi-todo

**[View on GitHub →](https://github.com/nex-gen-tech/pg-flux/tree/main/examples/fastapi-todo)**

| | |
|---|---|
| **Stack** | Python 3.12 · FastAPI · psycopg3 · Pydantic v2 |
| **PG version tested** | 14 – 18 |

The canonical Python example. Demonstrates the full pg-flux Python codegen path from schema to Pydantic model to FastAPI route.

What it exercises:

- UUID primary keys (`gen_random_uuid()`) and the corresponding `uuid.UUID` Python mapping
- `ENUM` type → `class TodoPriority(str, Enum)` generated model
- Row-level security policies declared in schema and tracked by pg-flux
- `pg-flux gen --lang python` producing `gen/models.py` with `BaseModel`, `Create`, and `Update` helpers
- `pg-flux drift --strict` and `pg-flux verify --strict` in CI

Quick start:

```bash
cd examples/fastapi-todo
docker compose up -d db
pg-flux migrate apply
pg-flux gen --lang python --out gen/
uvicorn app.main:app --reload
```

---

## express-bookmarks

**[View on GitHub →](https://github.com/nex-gen-tech/pg-flux/tree/main/examples/express-bookmarks)**

| | |
|---|---|
| **Stack** | TypeScript · Express · node-postgres (pg) · Zod |
| **PG version tested** | 14 – 18 |

A bookmarks service that demonstrates pg-flux TypeScript codegen, full-text search via `tsvector`, and JSONB metadata columns.

What it exercises:

- `jsonb` column with a known shape declared via a pg-flux comment hint (`tstype=...`)
- `tsvector` column populated by a DB-side trigger; column type mapped to `string` in TS
- GIN index on `tsvector` and on a JSONB path for fast full-text and structured search
- `pg-flux gen --lang ts --validators=zod` producing `tables.ts`, `enums.ts`, and `validators.ts`
- Branded IDs (`--branded-ids`) so `BookmarkId` and `UserId` are not interchangeable at the type level
- Insert/Update helpers (`UserCreate`, `BookmarkCreate`, `BookmarkUpdate`) used directly in Express route handlers

Quick start:

```bash
cd examples/express-bookmarks
docker compose up -d db
pg-flux migrate apply
pg-flux gen --lang ts --out src/db --branded-ids --insert-update-helpers --validators=zod
npm run dev
```

---

## go-events

**[View on GitHub →](https://github.com/nex-gen-tech/pg-flux/tree/main/examples/go-events)**

| | |
|---|---|
| **Stack** | Go 1.25 · chi · pgx/v5 |
| **PG version tested** | 14 – 18 |

An event-management service that exercises several PG features that are more complex to diff and regenerate correctly: IDENTITY columns, materialized views, and deferrable foreign keys.

What it exercises:

- `GENERATED ALWAYS AS IDENTITY` primary keys; identity columns excluded from `Insert` helpers
- A materialized view (`event_stats`) with a UNIQUE index for concurrent refresh; typed as a read-only Go struct
- A deferrable `INITIALLY DEFERRED` foreign key; pg-flux tracks deferability in the schema model
- `text[]` column with a GIN index; mapped to `[]string` in Go
- `pg-flux gen --lang go --orm-tags=sqlx` with `db:""` struct tags
- Multiple file layout: `tables.go`, `enums.go`, `views.go`, `types.go`, `functions.go`

Quick start:

```bash
cd examples/go-events
docker compose up -d db
pg-flux migrate apply
pg-flux gen --lang go --out internal/db --orm-tags=sqlx
go run ./cmd/server
```

---

## go-shop

**[View on GitHub →](https://github.com/nex-gen-tech/pg-flux/tree/main/examples/go-shop)**

| | |
|---|---|
| **Stack** | Go 1.25 · chi · pgx/v5 |
| **PG version tested** | 14 – 18 |

The most feature-dense Go example. Uses two PG schemas (`public` and `catalog`), a range-partitioned table, and several advanced index and constraint types that are tricky to diff correctly.

What it exercises:

- Two schemas (`public`, `catalog`) with cross-schema foreign keys; both tracked in `target_schemas`
- Range-partitioned `orders` table (`PARTITION BY RANGE (created_at)`) with child partitions declared in schema
- `EXCLUDE USING gist` constraint on a `tstzrange` column
- BRIN index on the partitioned table's timestamp column; `INCLUDE` index on a unique constraint
- Domain type (`order_ref`) and composite type (`money_amount`) both used as column types
- `SECURITY DEFINER` function and stored procedure; grants on schema objects declared and diffed
- `pg-flux gen --lang go --functions` — all function and procedure param types emitted
- `--omitempty=nullable` so nullable pointer fields get `json:",omitempty"` tags

Quick start:

```bash
cd examples/go-shop
docker compose up -d db
pg-flux migrate apply
pg-flux gen --lang go --out internal/db --orm-tags=sqlx --functions --omitempty=nullable
go run ./cmd/server
```

---

## rust-hrm

**[View on GitHub →](https://github.com/nex-gen-tech/pg-flux/tree/main/examples/rust-hrm)**

| | |
|---|---|
| **Stack** | Rust 1.78 · Actix-web 4 · sqlx 0.8 |
| **PG version tested** | 14 – 18 |

A human-resources management API that is the most comprehensive example in the repository. It covers everything in `go-shop` and adds range types, trigram search, window functions in materialized views, and Rust codegen.

What it exercises:

- `daterange` columns for employment periods; `tstzrange` for shift scheduling
- `pg_trgm` extension with a GIN trigram index on `employees.name` for similarity search
- A materialized view with a window function (`RANK() OVER (...)`) to compute org-chart rank; typed as a read-only Rust struct with all fields `Option<T>`
- Multiple `EXCLUDE USING gist` constraints; pg-flux tracks each constraint name and expression independently
- Self-referential `employees` table (`manager_id REFERENCES employees(id)`) — nullable FK column maps to `Option<i64>`
- `pg-flux gen --lang rust --functions --out gen/` producing the full six-file module
- `type_overrides: { numeric: rust_decimal::Decimal }` for salary columns
- All generated structs wired to sqlx via `#[derive(sqlx::FromRow)]` and enums via `#[derive(sqlx::Type)]`

Quick start:

```bash
cd examples/rust-hrm
docker compose up -d db
pg-flux migrate apply
pg-flux gen --lang rust --functions --out gen/
cargo run
```

## See also

- [Codegen overview →](/docs/codegen.html)
- [Python codegen →](/docs/codegen-python.html)
- [Rust codegen →](/docs/codegen-rust.html)
- [Migrations →](/docs/migrations.html)
