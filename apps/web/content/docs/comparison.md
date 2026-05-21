---
title: vs alternatives
group: Getting started
order: 4
description: Honest comparison vs Atlas, sqlc, Prisma, Drizzle, goose, golang-migrate, pg-schema-diff.
---

You probably already use one of the tools below. This page is about deciding whether pg-flux *replaces* it, *complements* it, or *isn't worth the switch*.

We try to be honest. Where pg-flux loses, we say so.

## TL;DR matrix

| Tool | Replaces it? | Or works alongside? |
|---|---|---|
| [**Atlas**](#vs-atlas) | Mostly yes, for PG-only projects | Atlas wins for multi-DB |
| [**sqlc**](#vs-sqlc) | No — sqlc generates queries, pg-flux generates schema types | Use both: pg-flux for schema + types, sqlc for typed queries |
| [**Prisma**](#vs-prisma) | If you want SQL-first, yes | Prisma wins for ORM-style apps |
| [**Drizzle**](#vs-drizzle) | Different category; you can use Drizzle Studio + pg-flux | Drizzle gives you ORM; pg-flux gives you types + safe migrations |
| [**goose / golang-migrate**](#vs-goose--golang-migrate) | Yes, completely | These are file-based migration runners |
| [**pg-schema-diff**](#vs-pg-schema-diff) | Yes, for the diff portion | pg-flux is the broader product |
| [**raw SQL + checked-in migrations**](#vs-rolling-your-own) | Yes | This is the default people start with |

## vs Atlas

[Atlas](https://atlasgo.io) is the closest direct competitor. Both are declarative-schema migration tools. Where they diverge:

| Dimension | Atlas | pg-flux |
|---|---|---|
| Languages supported | PG, MySQL, MariaDB, SQLite, SQL Server, ClickHouse, Redshift | PostgreSQL only |
| Schema definition | Atlas HCL DSL, or SQL, or both | SQL only |
| Migration generation | Yes | Yes |
| Drift detection | Yes (Atlas Cloud) | Yes (CLI: `drift`, `verify`) |
| PG 18 coverage | Lagging (Atlas tracks slowly) | First-class (NULLS NOT DISTINCT, NOT ENFORCED, virtual generated, etc.) |
| Type generation | No | Go + TypeScript, with comment hints, branded IDs, zod schemas |
| Hosted control plane | Atlas Cloud (paid) | None (self-hosted only) |
| Hazard model | Lint rules + manual blocks | Built-in hazard classification with `--allow-hazards` |
| Round-trip dump | Limited | First-class, integration-test-enforced |
| Schema dump | Yes | Yes, round-trip clean |

**When to choose Atlas**: you manage multiple DB engines (PG + MySQL), or you want the HCL DSL, or you're on Atlas Cloud already.

**When to choose pg-flux**: PG-only, you want SQL-as-source-of-truth, you want the codegen layer, you want PG 18 features today.

**Use both?** Not practically. They'd fight over baseline state.

## vs sqlc

[sqlc](https://sqlc.dev/) generates type-safe query code from `.sql` files. Different problem space.

| Dimension | sqlc | pg-flux |
|---|---|---|
| Schema migrations | No | Yes |
| Query-to-code generation | Yes | No |
| Type generation for tables | Yes | Yes |
| Type generation for query results | Yes (its main feature) | No |
| Multi-language | Go + Kotlin + Python | Go + TypeScript |
| Manages the DB | No | Yes |

**You should use both.** pg-flux handles schema + migration + table-row types. sqlc generates the typed query layer on top.

```text
schema/users.sql ──► pg-flux ──► users table + User struct
                                                ▲
queries/users.sql ──► sqlc ─────────────────────┘
                          │
                          └──► GetUserByID(ctx, id) (User, error)
```

The handoff is clean because both consume the same SQL schema files. pg-flux owns "how does the schema get there"; sqlc owns "how does the application call it with types".

## vs Prisma

[Prisma](https://www.prisma.io/) is full-stack ORM territory. Schema in Prisma DSL, migrations auto-generated, JS/TS-only client.

| Dimension | Prisma | pg-flux |
|---|---|---|
| Schema definition | `schema.prisma` (own DSL) | SQL |
| Migration generation | Yes | Yes |
| ORM client | Yes (Prisma Client) | No (you write SQL or use sqlc/raw) |
| Multi-database | PG, MySQL, SQLite, SQL Server, MongoDB | PG only |
| PG-specific features | Lagging (no NULLS NOT DISTINCT, no partition syntax, etc.) | First-class |
| Generated types | TypeScript only | TypeScript + Go |
| Runtime | Required (Prisma Client engine) | None (just SQL output) |

**When to choose Prisma**: you want a turnkey ORM with auto-generated client, you're TS-only, you don't need PG-specific features.

**When to choose pg-flux**: SQL-first team, you want PG superpowers (RLS, partitioning, GIST, range types, etc.), you mix Go and TS.

**Use both?** Hard. You'd give up Prisma's auto-migrations to use pg-flux's, which removes most of Prisma's value.

## vs Drizzle

[Drizzle](https://orm.drizzle.team/) is a TS-first ORM with a SQL-like query builder. Schema is defined in TS, migrations are SQL.

| Dimension | Drizzle | pg-flux |
|---|---|---|
| Schema definition | TypeScript files | SQL files |
| Migration generation | Drizzle Kit | Yes |
| Drift detection | Limited | Yes |
| ORM | Yes (drizzle-orm) | No |
| Multi-database | PG, MySQL, SQLite | PG only |

**When to choose Drizzle**: you want TS-as-source-of-truth, you want the ORM, you like the SQL-like query syntax.

**When to choose pg-flux**: you want SQL-as-source-of-truth (the actual SQL syntax, not a JS DSL), you mix Go + TS clients.

**Use both?** Possible: use pg-flux for the schema + migration + Go types, use Drizzle as the TS client against the same DB. But you'd be defining the schema twice, which defeats the point.

## vs goose / golang-migrate

These are imperative migration runners. You write `001_up.sql` / `001_down.sql` pairs. They apply them in order.

| Dimension | goose / golang-migrate | pg-flux |
|---|---|---|
| Migration source | Hand-written up/down pairs | Generated from declarative SQL |
| Declarative schema | No | Yes |
| Drift detection | No | Yes |
| Hazard model | No | Yes |
| Codegen | No | Yes |
| Maturity | Older, broader install base | Newer, more focused |

**When to choose goose/golang-migrate**: you genuinely want imperative migrations and don't trust generators. You have a tiny schema. You're maintaining a legacy codebase.

**When to choose pg-flux**: anything else.

**Migrating from goose to pg-flux**: see [Recipes / adoption](/docs/recipes.html). Short version: `pg-flux dump` your live DB, mark all existing migrations as baselined, switch tools.

## vs pg-schema-diff

[pg-schema-diff](https://github.com/stripe/pg-schema-diff) (from Stripe) does exactly what its name says — diffs two PG schemas. Excellent at the diff itself.

| Dimension | pg-schema-diff | pg-flux |
|---|---|---|
| Diff quality | Excellent | Comparable; we test against the same matrix |
| Migration lifecycle | No | Yes (generate / apply / status / repair / baseline) |
| Drift / verify | No | Yes |
| Codegen | No | Yes |
| Dump | No | Yes |
| Hazard model | Stripe's lints | Built-in, opt-in via flag |

**When to choose pg-schema-diff**: you just need a diff library to embed in your own tool.

**When to choose pg-flux**: you want the whole product.

These are complementary in spirit: if pg-schema-diff didn't exist, pg-flux's differ would be smaller. We share inspiration.

## vs rolling your own

The default in most projects: a `migrations/` folder with hand-written `.sql` files, applied by `psql -f` in CI.

This works until it doesn't. The failure modes:

- No drift detection. Someone runs SQL in prod, nobody knows.
- No hazard guards. `DROP COLUMN` runs and nobody notices.
- No type generation. Your Go and TS drift further from the DB every week.
- No PG version awareness. PG 18 features you wrote work locally but fail in prod (running PG 14).
- No round-trip. Re-creating the dev DB from scratch is hours of manual work.

pg-flux is what you'd build if you tried to write all of that yourself, except we already did.

## When pg-flux is the wrong choice

We try to be straight about this. **Don't use pg-flux if:**

- You manage MySQL / SQLite / SQL Server. We don't support them. Use Atlas.
- You want a turnkey ORM. We don't generate query code; we generate row types. Pair with sqlc.
- You can't trust generated SQL. Some teams have a policy of human-written-only migrations. pg-flux can still emit the SQL for you to review and commit, but you'll lose the auto-apply guarantees.
- Your schema is genuinely tiny (under 10 tables, no changes for months). The setup cost isn't justified.
- You need migration code (not just SQL) — i.e., per-migration Go functions that do conditional logic. pg-flux migrations are pure SQL. Use a different tool.

For everything else PG-shaped, pg-flux is built for you.

## Decision tree

```text
Do you use PostgreSQL?
├── No  → Atlas (cross-DB), or your engine's native tool
└── Yes
    │
    ├── Do you want declarative SQL as source of truth?
    │   ├── No  → Prisma (TS DSL) / Drizzle (TS code) / Atlas (HCL)
    │   └── Yes
    │       │
    │       ├── Do you also need typed queries (not just typed rows)?
    │       │   ├── Yes → pg-flux + sqlc
    │       │   └── No  → pg-flux alone
    │       │
    │       └── Do you use PG-specific features (RLS, partitions, GIST, range types)?
    │           ├── Yes → pg-flux (others lag here)
    │           └── No  → Either works; pg-flux is still our pick for PG-only shops
```

If you ended up at "pg-flux," head to [quick start](/docs/quick-start.html).
