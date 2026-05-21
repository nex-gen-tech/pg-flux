---
title: Database privileges
group: Configuration
order: 2
description: What permissions pg-flux needs, how to set up a dedicated migration role.
---

pg-flux executes DDL. That requires real permissions. This page is the reference for what to grant, who to grant it to, and how to set up a least-privilege role for production.

## The short version

For local development, the postgres superuser works. For everything else, create a dedicated role:

```sql
-- in psql, connected as a superuser
CREATE ROLE pg_flux_migrator WITH LOGIN PASSWORD 'change_me';

-- read pg_catalog (default for any role; here for completeness)
GRANT USAGE ON SCHEMA pg_catalog TO pg_flux_migrator;

-- own the target schemas (or be granted CREATE on them)
GRANT ALL ON SCHEMA public TO pg_flux_migrator;
GRANT ALL ON SCHEMA _pgflux TO pg_flux_migrator;  -- tracking schema

-- if you want pg-flux to manage RLS / triggers / event triggers
ALTER ROLE pg_flux_migrator SUPERUSER;  -- nuclear option
-- OR more surgically, see below
```

The tension: pg-flux needs to do almost anything to a database (CREATE/ALTER/DROP every kind of object, manage RLS, install extensions, change ownership). The least-privilege role for pg-flux ends up being most-privilege in practice.

## What pg-flux actually does

| Operation | What permission |
|---|---|
| Inspect catalog (read pg_catalog) | Any role can do this on a normal install |
| Create / alter / drop tables, indexes, views in a schema | `CREATE` on the schema, or own it |
| Create types (enum, composite, domain, range) | `CREATE` on the schema |
| Create functions and procedures | `CREATE` on the schema |
| Create triggers | Own the table the trigger is on |
| Create event triggers | `pg_event_trigger` role or superuser (PG13+) |
| Enable / disable RLS | Own the table, or `pg_signal_backend` |
| Create policies | Own the table |
| Grant/revoke privileges | `WITH GRANT OPTION` on the privilege you're granting |
| Install extensions | superuser, or member of `pg_create_extension` (PG13+) where applicable |
| Manage statistics objects | Own the table the statistics are on |
| Manage foreign servers / tables | superuser, or member of `pg_create_foreign_data_wrapper` |

Most application schemas use 80% of this. The rest is what makes a dedicated migrator role tricky.

## The pragmatic options

### Option 1: superuser

```sql
CREATE ROLE pg_flux_migrator WITH LOGIN SUPERUSER PASSWORD '...';
```

Works for everything. Sleeps the lawyers in your ops team. Common in self-hosted setups where the migration role IS the operator.

### Option 2: schema owner

```sql
CREATE ROLE pg_flux_migrator WITH LOGIN PASSWORD '...';
ALTER SCHEMA public OWNER TO pg_flux_migrator;
ALTER SCHEMA _pgflux OWNER TO pg_flux_migrator;
```

Works for everything inside owned schemas, including triggers, policies, RLS toggles. Doesn't work for cross-schema GRANTs to roles that pg_flux_migrator doesn't have admin on. Doesn't work for cluster-level operations (extensions, event triggers).

This is the sweet spot for most teams.

### Option 3: granular grants

Long. Painful. Necessary in compliance-heavy shops.

```sql
CREATE ROLE pg_flux_migrator WITH LOGIN PASSWORD '...';

-- Schema-level
GRANT USAGE, CREATE ON SCHEMA public TO pg_flux_migrator;
GRANT USAGE, CREATE ON SCHEMA _pgflux TO pg_flux_migrator;

-- Existing object ownership (one-time)
DO $$
DECLARE
  obj record;
BEGIN
  FOR obj IN SELECT n.nspname, c.relname, c.relkind
    FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relkind IN ('r','v','m','S','f')
  LOOP
    EXECUTE format('ALTER %s %I.%I OWNER TO pg_flux_migrator',
      CASE obj.relkind WHEN 'r' THEN 'TABLE'
                       WHEN 'v' THEN 'VIEW'
                       WHEN 'm' THEN 'MATERIALIZED VIEW'
                       WHEN 'S' THEN 'SEQUENCE'
                       WHEN 'f' THEN 'FOREIGN TABLE'
      END, obj.nspname, obj.relname);
  END LOOP;
END $$;

-- Future objects auto-owned
ALTER DEFAULT PRIVILEGES FOR USER pg_flux_migrator IN SCHEMA public
  GRANT ALL ON TABLES TO pg_flux_migrator;

-- Allow pg-flux to grant to app roles
GRANT app_reader TO pg_flux_migrator WITH ADMIN OPTION;
GRANT app_writer TO pg_flux_migrator WITH ADMIN OPTION;
```

Even this is incomplete. For event triggers / extensions / foreign data wrappers, you still need superuser or specific bootstrap roles.

> [!IMPORTANT]
> In practice, almost everyone running pg-flux in production ends up at
> Option 2 (schema owner) or Option 1 (superuser, scoped to a separate
> migration database connection). The granular path is purity for its own
> sake unless you have an audit requirement that demands it.

## Tracking schema

pg-flux writes to `_pgflux.migrations` to track which migrations have been applied. The role needs write access:

```sql
GRANT USAGE ON SCHEMA _pgflux TO pg_flux_migrator;
GRANT ALL ON _pgflux.migrations TO pg_flux_migrator;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA _pgflux TO pg_flux_migrator;
```

The schema name is configurable via `--tracking-schema` (default `_pgflux`). If you choose a different name, replace `_pgflux` everywhere above.

## Read-only commands

These commands need read access only — `pg_catalog` SELECT, plus optional read access to your tables if you query them with `inspect`:

| Command | Permissions |
|---|---|
| `pg-flux drift` | catalog SELECT |
| `pg-flux verify` | catalog SELECT |
| `pg-flux dump` | catalog SELECT |
| `pg-flux pull` | catalog SELECT |
| `pg-flux inspect` | catalog SELECT |
| `pg-flux plan` | catalog SELECT |
| `pg-flux gen` | catalog SELECT |
| `pg-flux migrate status` | catalog SELECT + tracking schema SELECT |

For CI canaries that only run drift/verify/gen-check against production, this means you can use a **read-only role** — no DDL grant needed.

```sql
-- read-only role for CI canaries
CREATE ROLE pg_flux_readonly WITH LOGIN PASSWORD '...';
GRANT USAGE ON SCHEMA public TO pg_flux_readonly;
GRANT USAGE ON SCHEMA _pgflux TO pg_flux_readonly;
GRANT SELECT ON _pgflux.migrations TO pg_flux_readonly;
```

Use this DSN for nightly drift checks against production.

## Connection pooling

pg-flux doesn't long-hold connections, so connection limits are usually a non-issue. One operation = one transaction = one connection, released when done.

Exception: `migrate apply` with many `CONCURRENTLY` statements. Each autocommit statement uses the same connection but in autocommit mode. Still one connection total.

If you're running multiple pg-flux processes against the same database in parallel, the **session advisory lock** serializes them so they don't race. The other processes wait, not error.

## TLS

For production DBs, always use TLS:

```bash
postgres://migrator:secret@host:5432/db?sslmode=require
```

| sslmode | When to use |
|---|---|
| `disable` | Local dev only |
| `prefer` | Don't (pretends to be safe but isn't) |
| `require` | Production minimum; verifies the connection is TLS but not the cert |
| `verify-ca` | Verifies the cert is signed by a trusted CA |
| `verify-full` | Verifies CA AND hostname; recommended for prod |

pg-flux uses the underlying [pgx](https://github.com/jackc/pgx) driver's TLS support — anything pgx accepts, pg-flux accepts.

## Connection examples

```bash
# Local development (no TLS)
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/mydb?sslmode=disable"

# Staging via SSL with CA verification
export DATABASE_URL="postgres://pg_flux_migrator:secret@staging-db.internal:5432/mydb?sslmode=verify-ca&sslrootcert=/etc/ssl/certs/ca.pem"

# Cloud-managed Postgres (e.g., AWS RDS) — full TLS verification
export DATABASE_URL="postgres://pg_flux_migrator:secret@prod.cluster-x.us-east-1.rds.amazonaws.com:5432/mydb?sslmode=verify-full&sslrootcert=/etc/ssl/certs/rds-ca-2019-root.pem"
```

## Switching roles mid-session

pg-flux executes everything under whichever role is in the DSN. If you need different operations to run under different roles (e.g., RLS-enable as schema owner, but data inserts as app_writer) — that's not pg-flux's concern. pg-flux is purely schema, no data.

If you do need privilege switching inside a migration, write a raw `SET ROLE` in your `schema/` SQL — pg-flux passes it through.

## What about `ALTER USER`?

pg-flux **doesn't manage cluster-level roles**. Creating users, granting role memberships, changing passwords — none of that is in scope. Use a separate tool (Terraform, your IaC, a CRUD service).

The reason: roles are cluster-wide, not schema-wide. A pg-flux project is per-database. Cluster-wide objects don't fit the per-database model.

## Audit logging

If your compliance requires logging every DDL pg-flux executes, the cleanest path:

1. Enable `log_statement = 'ddl'` in `postgresql.conf` (logs all DDL)
2. Ship Postgres logs to your aggregator
3. Each pg-flux apply will appear there with the role, timestamp, statement, duration

Inside pg-flux, structured JSON logs (`--log-format=json`) capture the same statements plus pg-flux's own metadata (which migration file, which hazard ratings).
