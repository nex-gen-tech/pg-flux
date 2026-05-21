---
title: Schema authoring
group: Migrations
order: 5
description: How to organize, name, and structure the SQL files pg-flux reads.
---

pg-flux loads `schema/**/*.sql`. That's it. There's no special DSL, no preprocessor, no transformation. You write the SQL you would have written by hand and pg-flux figures out what to do with it.

This page is about the conventions that make that scale — from "one schema.sql file" to "200+ tables across 6 schemas."

## File layout

There's no required layout. The loader walks the tree, parses every `.sql` file, and merges. But the layout you pick affects how easy your repo is to navigate.

### Option 1: single file (good for &lt; 20 objects)

```text
schema/
└── schema.sql
```

Everything in one file. Easy to grep, but every diff touches the same file.

### Option 2: per-kind (the dump default)

```text
schema/
├── tables/
│   ├── public.users.sql
│   ├── public.posts.sql
│   └── public.events.sql
├── views/
│   ├── public.active_users.sql
│   └── public.user_stats.sql
├── types/
│   ├── public.user_role.sql      # the enum
│   └── public.address.sql        # composite type
├── functions/
│   ├── public.set_updated_at.sql
│   └── public.calc_score.sql
├── triggers/
│   └── public.posts.set_updated_at.sql
└── policies/
    └── public.users.users_select.sql
```

This is what `pg-flux dump` produces by default. Each file holds one object. Diffs stay local — touching `posts` only changes `tables/public.posts.sql`.

> [!TIP]
> If you adopted pg-flux via `dump`, you're already in this layout. Don't
> fight it — the round-trip-clean guarantee depends on the per-kind structure.

### Option 3: per-domain (recommended for &gt; 50 objects)

Group by business domain:

```text
schema/
├── accounts/
│   ├── users.sql               # CREATE TABLE users + indexes + triggers
│   ├── sessions.sql
│   └── role.sql                # CREATE TYPE role
├── billing/
│   ├── subscriptions.sql
│   ├── invoices.sql
│   └── plans.sql
├── content/
│   ├── posts.sql
│   ├── comments.sql
│   └── reactions.sql
└── infrastructure/
    ├── audit_log.sql
    └── event_triggers.sql
```

Mirror your application architecture. The trade-off vs per-kind: you lose the "all enums in one place" benefit, but you gain "everything about billing in one place" — which is what humans usually want.

### Option 4: hybrid

```text
schema/
├── public/                  # all top-level public schema objects
│   ├── tables/
│   ├── views/
│   ├── functions/
│   └── types/
├── audit/                   # audit schema, namespaced
│   ├── tables/
│   └── functions/
└── _global/                 # cluster-level objects
    ├── extensions.sql       # CREATE EXTENSION ...
    ├── event_triggers.sql
    └── default_privileges.sql
```

If you manage multiple Postgres schemas (`public`, `audit`, `analytics`), this separates them clearly while still using the per-kind subdivision within each.

## Load order doesn't matter (mostly)

pg-flux walks the file tree in name-sorted order. **But the order in which statements are encountered doesn't determine the order they're emitted.**

The differ doesn't care if `CREATE TYPE foo` appears in file A and `CREATE TABLE bar (x foo)` in file B. It builds the full `SchemaState` from all files, then sorts changes by dependency at emit time.

What this means in practice:

- You can split definitions across files freely
- You can name files however you want; pg-flux doesn't read filename intent
- Cross-file forward references resolve themselves

> [!NOTE]
> One exception: `ALTER POLICY` and `ALTER TABLE ... ENABLE/DISABLE TRIGGER`
> are stashed in pending lists during the first pass and applied in a second
> pass. That works for the cross-file case. The only thing that truly can't
> resolve is a literal SQL parse error in a single file.

## Naming conventions

These aren't enforced, but they make grepping easier:

| Convention | Example | Notes |
|---|---|---|
| Snake case for everything | `user_accounts`, `event_log` | Matches PG's identifier style |
| Plural for table names | `users`, `posts`, `orders` | Common, not universal |
| Singular for enum types | `role`, `status`, `event_kind` | An enum is one value |
| `<table>_<column>_<kind>` for constraint names | `users_email_unique`, `posts_user_fk` | Predictable when reading errors |
| `idx_<table>_<columns>` for indexes | `idx_posts_user_id_created_at` | Matches the convention `pg_dump` follows |
| `_<schema>_<purpose>` for internal schemas | `_pgflux`, `_audit_history` | Underscore-prefix marks "system" |

## Identifying objects pg-flux manages

pg-flux only manages what its `--schemas` flag includes. Default is `public` only. To manage more:

```bash
pg-flux migrate generate --schemas=public,audit,analytics
```

Or in `.pg-flux.yml`:

```yaml
target_schemas: [public, audit, analytics]
```

Any object in a schema not in this list is *invisible* to pg-flux. It won't generate drops for it. It won't include it in dumps. It won't generate types for it.

> [!IMPORTANT]
> The `--schemas` list is THE control over scope. If you accidentally
> omit a schema you manage, pg-flux will see all its tables as
> undeclared in source (`verify` will complain) but won't drop them
> (because it doesn't see them at all).

## Renaming via `@renamed` hints

pg-flux can't telepathically know that `email_address` became `email`. Tell it:

```sql
CREATE TABLE users (
  id    bigint PRIMARY KEY,
  email text NOT NULL  -- @renamed from=email_address
);
```

The hint must be:

- On the column's line (not a separate line)
- The exact form `@renamed from=<old_name>` (no spaces around `=` is preferred but accepted)

For tables, the hint goes on a comment before the `CREATE TABLE`:

```sql
-- @renamed-table from=user_accounts
CREATE TABLE users (
  id bigint PRIMARY KEY,
  email text NOT NULL
);
```

You can remove the hint after the migration applies — pg-flux only consumes it at `migrate generate` time.

## Comment-as-doc

Use `COMMENT ON` for application-facing documentation. pg-flux preserves comments and surfaces them in:

- Generated migrations (`COMMENT ON ...` statements)
- Generated Go (Godoc on the struct/field)
- Generated TypeScript (TSDoc on the interface/property)
- Database introspection tools (pgAdmin, DataGrip, etc.)

```sql
COMMENT ON TABLE users IS 'Application user accounts';
COMMENT ON COLUMN users.email IS 'Unique login email; lowercase by convention';
```

In the generated Go:

```go
// User mirrors public.users.
// Application user accounts
type User struct {
  ID int64 `db:"id"`
  // Unique login email; lowercase by convention
  Email string `db:"email"`
}
```

This is documentation that survives the round-trip: it lives in your schema, propagates to your code, and shows up in your IDE.

## Comment hints for codegen

A subset of the comment string can carry directives for the codegen pipeline. See [Configuration](/docs/configuration.html#comment-hints) for the full grammar.

```sql
COMMENT ON COLUMN posts.metadata IS 'Per-post metadata blob. pg-flux: tstype={ source: string; ip?: string }';
```

The text before `pg-flux:` becomes the doc comment; the directives after configure the emitter.

## Declaring privileges (GRANTs)

Put GRANTs in source if you want pg-flux to manage them. Otherwise pg-flux will report them as "live has, source doesn't" via `verify`.

```sql
GRANT SELECT, INSERT, UPDATE, DELETE ON users TO app_writer;
GRANT SELECT ON users TO app_reader;
```

For schema-wide defaults (so newly-created tables auto-grant):

```sql
ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT ON TABLES TO app_reader;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_writer;
```

pg-flux tracks both per-object and default privileges. See [Privileges](/docs/privileges.html) for setting up the migration role itself.

## Partitioning

Partitioned parents look like:

```sql
CREATE TABLE events (
  id          bigserial PRIMARY KEY,
  occurred_at timestamptz NOT NULL,
  payload     jsonb NOT NULL DEFAULT '{}'::jsonb
) PARTITION BY RANGE (occurred_at);

CREATE TABLE events_2026 PARTITION OF events
  FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

CREATE TABLE events_2027 PARTITION OF events
  FOR VALUES FROM ('2027-01-01') TO ('2028-01-01');
```

pg-flux tracks the parent in `Table.PartitionBy` and children in `SchemaState.PartitionChildren`. Adding a new partition to an existing parent is a CREATE TABLE on the child, which pg-flux handles.

> [!NOTE]
> Migrating an unpartitioned table to a partitioned one is *not* something
> pg-flux automates — it requires data movement. See
> [Recipes / partition an existing table](/docs/recipes.html#partition-an-existing-table).

## Splitting and consolidating

Need to reorganize? Here's the rule:

**Splitting a file** — move statements from one file to another. pg-flux sees no diff (same statements, different files).

**Consolidating files** — same; merge files freely. Just don't introduce duplicates (`CREATE TABLE users` in two files is an error).

**Moving objects between schemas** — that's a real schema change. Either use `ALTER TABLE ... SET SCHEMA`, or recreate.

## What goes where (cheat sheet)

| Object | Typical home |
|---|---|
| `CREATE TABLE` | `schema/tables/<schema>.<name>.sql` (or domain-grouped) |
| `CREATE INDEX` | In the same file as its table, after the CREATE |
| `CREATE VIEW` / `MATERIALIZED VIEW` | `schema/views/...` |
| `CREATE TYPE ... AS ENUM` | `schema/types/...` |
| `CREATE TYPE ... AS (...)` (composite) | `schema/types/...` |
| `CREATE DOMAIN` | `schema/types/...` |
| `CREATE FUNCTION` / `PROCEDURE` | `schema/functions/...` |
| `CREATE TRIGGER` | Same file as the function it calls, or `schema/triggers/...` |
| `CREATE POLICY` | Same file as its table |
| `CREATE EXTENSION` | `schema/_global/extensions.sql` |
| `CREATE EVENT TRIGGER` | `schema/_global/event_triggers.sql` |
| `GRANT` / `ALTER DEFAULT PRIVILEGES` | `schema/_global/privileges.sql` or in-file with the object |
| `COMMENT ON` | Right after the object it comments |

## When schema files get unwieldy

Around 50+ files, you should start splitting by domain rather than kind. At 200+ files, you'll want to introduce subdirectories. The loader walks recursively — go as deep as you want.

What you should NOT do:

- Have one mega-file with everything (becomes a merge-conflict factory)
- Spread one object across multiple files (impossible to grep)
- Mix non-pg-flux SQL (data migrations, ad-hoc queries) into `schema/`

If you have SQL that isn't part of the declarative schema — seed data, test fixtures, one-off backfills — put it outside `schema/`. pg-flux only loads what's under your `schema_dir` config.
