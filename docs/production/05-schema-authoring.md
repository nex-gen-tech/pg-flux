# 05 — Schema Authoring Guide

pg-flux reads plain PostgreSQL SQL files. This document covers every supported object type and the hint annotations that control diff behaviour.

---

## File Layout

```
schema/
├── users.sql         # tables, types, domains in one file
├── posts.sql
├── extensions.sql    # extensions
├── functions.sql     # PL/pgSQL functions
└── policies.sql      # RLS policies (can live in same file as table)
```

All `.sql` files under the `--schema` directory are loaded recursively. Order of files within the directory does not matter — pg-flux uses a dependency-aware DAG sorter to order statements correctly.

---

## Supported Object Types

| Object | Supported | Notes |
|--------|-----------|-------|
| `CREATE TABLE` | ✅ | Including constraints, defaults, generated columns |
| `CREATE TYPE ... AS ENUM` | ✅ | Value ordering preserved; ADD VALUE uses BEFORE/AFTER |
| `CREATE DOMAIN` | ✅ | Including named and unnamed CHECK constraints |
| `CREATE INDEX` / `CREATE UNIQUE INDEX` | ✅ | Including partial, expression, and concurrent indexes |
| `CREATE VIEW` | ✅ | Full body diff via AST fingerprint |
| `CREATE MATERIALIZED VIEW` | ✅ | Treated as higher-priority than regular views in DAG sort |
| `CREATE FUNCTION` / `CREATE PROCEDURE` | ✅ | Body diff; supports PL/pgSQL, SQL, plv8 |
| `CREATE TRIGGER` | ✅ | Including BEFORE/AFTER/INSTEAD OF |
| `CREATE EXTENSION` | ✅ | IF NOT EXISTS idiom; version upgrades |
| `CREATE SEQUENCE` | ✅ | ALTER SEQUENCE emitted (not DROP+CREATE) when params change |
| `CREATE POLICY` | ✅ | USING/WITH CHECK normalization prevents false diffs |
| `ALTER TABLE ENABLE/DISABLE ROW LEVEL SECURITY` | ✅ | |
| Partitioned tables (`PARTITION BY`) | ✅ | Partition key changes tracked; child table management |
| Foreign keys | ✅ | Including ON DELETE/UPDATE actions |
| Check constraints | ✅ | AST-normalized to prevent whitespace false-diffs |
| `GRANT` / `REVOKE` | ⚠️ | Emitted as raw DDL; no live-state inspection (idempotency not guaranteed) |
| Table inheritance | ⚠️ | Basic support; complex hierarchies untested |
| Composite types | ❌ | Not yet supported |

---

## Hint Annotations

Hint comments are SQL line comments (`-- @key value`) placed **directly above** the object they annotate.

### `-- @renamed from=<old_name>`

Tells pg-flux that a column or table was renamed. Without this hint, a rename looks like DROP + ADD, which loses data.

```sql
CREATE TABLE public.users (
  id    bigserial PRIMARY KEY,
  -- @renamed from=username
  handle text,
  -- @renamed from=nickname
  screen_name text
);
```

Generated migration:

```sql
ALTER TABLE public.users RENAME COLUMN username TO handle;
ALTER TABLE public.users RENAME COLUMN nickname TO screen_name;
```

**Rules:**
- Place the `-- @renamed` comment on the line immediately before the column definition.
- The `from=` value is the live database column name, not a historical one.
- Remove the annotation after the migration has been applied (it has no effect once the column is renamed).

---

### `-- @using <expr>`

Provides a PostgreSQL `USING` expression for incompatible column type changes — cases where PostgreSQL cannot implicitly cast the existing data.

Required when changing:
- `boolean` → any `ENUM`
- `text` → `integer` (when values are numeric strings)
- Any `ENUM` → a different `ENUM`
- `varchar(N)` → `varchar(M)` where `M < N` (data truncation)

```sql
-- @using CASE is_verified
--          WHEN TRUE  THEN 'verified'::public.verification_status
--          ELSE 'unverified'::public.verification_status
--        END
is_verified public.verification_status NOT NULL DEFAULT 'unverified',
```

pg-flux emits:

```sql
-- Drop default first (PG cannot auto-cast it)
ALTER TABLE public.users ALTER COLUMN is_verified DROP DEFAULT;

-- Type change with your USING expression
ALTER TABLE public.users
  ALTER COLUMN is_verified
  SET DATA TYPE public.verification_status
  USING CASE is_verified
    WHEN TRUE THEN 'verified'::public.verification_status
    ELSE 'unverified'::public.verification_status
  END;

-- Restore the new default
ALTER TABLE public.users ALTER COLUMN is_verified SET DEFAULT 'unverified';
```

**Rules:**
- The expression must produce a value of the new column type for every existing row.
- Multi-line USING expressions: each continuation line must also start with `--` but does not need `@using`.
- Remove the annotation after the migration is applied (it has no effect afterwards).

---

## Tables

### Column Types

Prefer PostgreSQL-native types. pg-flux normalises common aliases to prevent false diffs:

| Alias you can write | Canonical form stored in catalog |
|--------------------|----------------------------------|
| `int`, `int4` | `integer` |
| `int8` | `bigint` |
| `int2` | `smallint` |
| `decimal`, `numeric` | `numeric` |
| `timestamp` | `timestamp without time zone` |
| `timestamptz` | `timestamp with time zone` |
| `bool` | `boolean` |
| `varchar(n)` | `character varying(n)` |
| `char` | `character(1)` |
| `float4` | `real` |
| `float8` | `double precision` |

### Generated Columns

```sql
search_name text GENERATED ALWAYS AS (upper(coalesce(full_name, handle))) STORED,
```

pg-flux detects generated columns and never emits `SET DEFAULT` or `NOT NULL` alters for them.

### Constraints

```sql
CONSTRAINT users_email_unique   UNIQUE (email),
CONSTRAINT users_email_format   CHECK (email LIKE '%@%'),
CONSTRAINT users_score_range    CHECK (test_score >= 0 AND test_score <= 100)
```

**Constraint fingerprinting:** pg-flux round-trips constraint definitions through `pg_query` and normalises PostgreSQL catalog representations (which add `::type` casts) to prevent false diffs.

---

## ENUMs

```sql
CREATE TYPE public.post_status AS ENUM ('draft', 'published', 'archived');
```

**Adding a value:**

Simply add the value in the desired position in the source file:

```sql
CREATE TYPE public.post_status AS ENUM ('draft', 'pending_review', 'published', 'archived');
```

pg-flux emits:

```sql
ALTER TYPE public.post_status ADD VALUE 'pending_review' BEFORE 'published';
```

**Removing a value:**

PostgreSQL does not support `ALTER TYPE ... DROP VALUE`. If you remove an ENUM value from the source file, pg-flux emits a **blocking** `DATA_LOSS` hazard and refuses to generate the migration. You must handle this manually (typically by migrating all rows away from the value first, then doing a `CREATE TYPE ... AS ... (new list)` + table update).

---

## Extensions

```sql
-- extensions.sql
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
```

pg-flux only manages extensions that are **explicitly declared** in the desired schema. Extensions installed by a DBA that are not in the schema files are left untouched — they will never be auto-dropped.

---

## Sequences

```sql
CREATE SEQUENCE public.invoice_number START 1000 INCREMENT 1 CACHE 10;
```

When sequence parameters change, pg-flux emits `ALTER SEQUENCE IF EXISTS ... INCREMENT BY x MINVALUE x MAXVALUE x CACHE x [NO CYCLE]` — it does **not** emit DROP+CREATE, which would reset the current counter.

`START` is only used on the initial `CREATE SEQUENCE` and is never put into `ALTER SEQUENCE` (to preserve the live counter value).

---

## Views

```sql
CREATE VIEW public.published_posts AS
  SELECT p.id, p.title, u.handle AS author
  FROM public.posts p
  JOIN public.users u ON u.id = p.user_id
  WHERE p.status = 'published';
```

pg-flux uses AST fingerprinting (via `pg_query`) rather than string comparison. Whitespace changes and `::type` casts added by `pg_get_viewdef` do not generate false diffs.

When a column type changes on a table referenced by a view, pg-flux automatically promotes the view to `DROP_VIEW_EARLY` before the type change and recreates it after. This applies to both currently-live views and views being removed in the same migration.

---

## Row-Level Security (RLS)

```sql
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;

CREATE POLICY users_select ON public.users
  FOR SELECT USING (true);

CREATE POLICY users_update ON public.users
  FOR UPDATE USING (id = current_setting('app.user_id')::bigint);
```

Policy `USING` and `WITH CHECK` expressions are normalised via `pg_query` deparser before comparison. Whitespace and minor syntactic differences between the source file and `pg_get_expr` output do not generate false diffs.
