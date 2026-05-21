---
title: Troubleshooting
group: Reference
order: 6
description: Every error you'll hit, what it means, what to do.
---

If pg-flux is yelling at you, find the message below. If it isn't here, [open an issue](https://github.com/nex-gen-tech/pg-flux/issues/new) — these entries get added every time a real user gets stuck.

## Connection errors

### `pgver.Detect: failed to connect`

```text
Error: pgver.Detect: failed to connect to `user=pgflux database=mydb`:
   [::1]:5432 (localhost): server error: FATAL: database "mydb" does not exist
```

The connection succeeded but the database doesn't exist on this server. Either create it first, or fix your DSN.

```bash
# create the DB
psql -h localhost -U postgres -c 'CREATE DATABASE mydb;'

# OR fix the DSN
pg-flux ... --db "postgres://user:pass@localhost:5432/correct_db_name?sslmode=disable"
```

### `dial tcp: connection refused`

PG isn't listening where pg-flux is looking. Confirm with:

```bash
psql "$DATABASE_URL" -c 'SELECT 1'
```

If that also fails, the problem is `psql` ↔ `postgres`, not pg-flux.

### `permission denied for relation pg_attribute`

The role pg-flux is connecting as doesn't have read access to the catalog. This is rare on default PG installs but can happen with locked-down setups. Grant:

```sql
GRANT USAGE ON SCHEMA pg_catalog TO migration_role;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO migration_role;
```

See [Database privileges](/docs/privileges.html) for the full required grant set.

### `SSL is required`

Your PG host requires TLS. Append `?sslmode=require` to your DSN (or `?sslmode=verify-full` if you have the CA cert configured):

```bash
postgres://user:pass@host:5432/db?sslmode=require
```

## Migration errors

### `refusing to apply: blocking hazards`

```text
Error: refusing to apply: blocking hazards; pass --allow-hazards or change schema
```

pg-flux detected a destructive operation. Two options:

**Option A** — confirm you really want it and opt in:

```bash
pg-flux migrate apply --allow-hazards=DATA_LOSS
```

**Option B** — see what the hazard actually is:

```bash
pg-flux plan --format=json | jq '.statements[].hazards'
```

Hazard names: `DATA_LOSS`, `COLUMN_TYPE_CHANGE`, `CONSTRAINT_SCAN`, `MASS_DROP`, `RLS_GAP`. Pass a comma-separated subset.

### `refusing to plan: N of M live objects (X%) would be dropped`

```text
Error: refusing to plan: 5 of 8 live objects (62%) would be dropped,
exceeding the 25% mass-drop threshold; if this is intentional,
re-run with --allow-mass-drop. Examples: TABLE public.users, ...
```

You either:

- Pointed `--schema` at the wrong directory (most common cause)
- Are intentionally wiping the schema (rare)

Common fix:

```bash
# you probably wanted ./schema, not ./scratch
pg-flux migrate generate --schema=./schema
```

If you really do want the wipe:

```bash
pg-flux migrate apply --allow-mass-drop
```

### `live database state has drifted`

```text
Error: refusing to apply 20260520_add_role.sql: live database state has drifted
since this migration was generated (expected baseline=abc123…, live=def456…)
```

Someone (or something) modified the DB between when you ran `migrate generate` and when you're trying to `migrate apply`. Your options:

**Option A** — rebase the migration:

```bash
# this captures the current live state into source first
pg-flux pull --dry-run=false

# then regenerate against the new baseline
pg-flux migrate generate --label rebase
```

**Option B** — force apply if you've manually verified compatibility:

```bash
pg-flux migrate apply --force-after-drift
```

See [Drift recovery](/docs/drift.html) for the full playbook.

### `checksum mismatch for already-applied migration`

```text
Error: checksum mismatch for already-applied migration 20260520_initial.sql:
   recorded=abc123 current=def456 — do not edit applied migrations
```

You edited a migration file *after* it was applied. The tracking table has the old checksum. Two options:

- **Best**: revert your edit. Applied migrations are immutable.
- **If the edit was intentional** (e.g., comment fix only): `pg-flux migrate repair` recomputes checksums.

> [!CAUTION]
> Never use `repair` to "fix" a schema change you made to an applied migration.
> Schema changes belong in a new migration. Use repair only for whitespace
> and comment changes.

### `could not acquire migration advisory lock`

```text
Error: could not acquire migration advisory lock (another apply in progress)
```

Two apply processes are racing against the same database. Wait for the other to finish, or kill the stuck process (and clear the advisory lock):

```sql
-- find the lock holder
SELECT pid, application_name, query FROM pg_stat_activity
WHERE pid IN (SELECT pid FROM pg_locks WHERE locktype = 'advisory');

-- if confirmed stuck:
SELECT pg_terminate_backend(<pid>);
```

## Generate / diff errors

### `dependency cycle detected`

```text
Error: dag: dependency cycle detected: function public.foo() ↔ view public.v
```

A function references a view that references the function (or longer cycle). pg-flux refuses because the migration would be unappliable.

Fix: break the cycle in source. Usually means dropping one direction:

- Have the view inline the logic instead of calling the function
- Have the function return a constant or use a less-circular path

This rarely happens in real schemas — usually a side effect of an in-progress refactor.

### `column X exists in live but not in desired (NOT in @renamed list)`

This isn't a literal error string but a common confusion. If you renamed a column without the `@renamed` hint, pg-flux sees drop-plus-add and treats it as data loss.

Fix: add the hint.

```sql
CREATE TABLE users (
  id    bigint PRIMARY KEY,
  email text NOT NULL  -- @renamed from=email_address
);
```

See [Recipes / rename a column](/docs/recipes.html#rename-a-column-without-data-loss).

### `unsupported feature: PG18 virtual generated column requires server >= 18`

pg-flux's version gates work at generate time. The fix is either:

- Upgrade your target PG to a version that supports the feature
- Don't use the feature

If you think pg-flux is wrong about the gate, [file an issue](https://github.com/nex-gen-tech/pg-flux/issues/new).

## Codegen errors

### Generated Go code doesn't compile

The most common cause: a custom type override that's missing an import. If your `.pg-flux-codegen.yml` says:

```yaml
type_overrides:
  numeric: my.package.Decimal
```

…but `my.package` doesn't exist in your `go.mod`, compilation fails. Fix the override or `go get` the package.

Other compile-fail causes:

- A column type pg-flux doesn't know about (file an issue with the type)
- A comment hint with a `gotype=` referring to a non-existent type

Run `cd internal/dbgen && go build ./...` directly to see the exact compile error, then fix.

### `pg-flux gen --check` fails in CI but works locally

You committed generated code from a different `.pg-flux-codegen.yml` than the one CI is using. Common causes:

- CI is running against an empty DB (no objects → empty `tables.go`); local DB has the full schema
- Different override values between environments

Fix: run `pg-flux gen` against the same DSN the CI is using, commit the output.

### Generated TypeScript doesn't type-check

Likely a `tstype` comment hint with invalid syntax. Check:

```sql
COMMENT ON COLUMN posts.metadata IS 'pg-flux: tstype={ source: string; ip?: string }';
```

The TS expression must be valid on its own. If you wrote `tstype=Array<>` (with no generic), you'll get a `tsc` error pointing at the generated file.

### `comment hint not applying`

You wrote:

```sql
COMMENT ON COLUMN posts.metadata IS 'pg-flux: tstype=string';
```

…but the generated code still has `metadata: unknown`. Likely cause:

- The comment was set in PG but the inspector hasn't seen the change yet. Re-run `pg-flux gen` after the migration that adds the comment is applied.
- You have a `type_overrides` for `jsonb` in your codegen config that's winning. Per-column hints win over per-type overrides; check the precedence.

### `enum value rename not detected`

You renamed an enum value but pg-flux is emitting `ADD VALUE` + data-loss advisory instead of `RENAME VALUE`. The rename detector requires *exact* same-position swap. If the value list also has insertions or deletions, the detector bails out for safety.

Workaround: in a separate migration first, do only the rename. Then in a later one, add/remove other values.

## Performance issues

### `pg-flux inspect is slow`

Inspect runs ~25 catalog queries. On a schema with 1000+ tables, expect 1–3 seconds. If it's much slower:

- Your `pg_catalog` queries are slow — check for catalog bloat (`VACUUM FULL pg_catalog.pg_attribute`, with appropriate caution)
- Your DB is under load and competing for resources
- You're connecting over a high-latency link — each round trip multiplies up

Use `--from-source` for codegen when you don't need live state:

```bash
pg-flux gen --from-source
```

### `pg-flux gen is rebuilding files that haven't changed`

It shouldn't be. Generated files are written only when content differs from on-disk. If it's repeatedly rewriting:

- Compare two consecutive runs with `diff -r ./internal/dbgen ./internal/dbgen.bak` to see what's actually different
- Likely: a non-deterministic ordering somewhere. File an issue with the diff.

## Adoption / setup

### "How do I start using pg-flux on an existing project?"

```bash
# 1. point pg-flux at your live DB and dump
pg-flux dump --db "$DATABASE_URL" --output ./schema --force

# 2. verify the dump captured everything
pg-flux verify --strict   # should print "clean"

# 3. confirm round-trip
pg-flux migrate generate --label baseline
# Expected output: "Generated: migrations/...baseline.sql (0 statements)"

# 4. baseline that migration (don't run it; it's a no-op anyway)
pg-flux migrate baseline migrations/<timestamp>_baseline.sql

# 5. write a real migration to test the loop
echo "ALTER TABLE users ADD COLUMN test text;" >> schema/users.sql
pg-flux migrate generate --label test
pg-flux migrate apply

# 6. commit everything
git add . && git commit -m "adopt pg-flux"
```

### "How do I write my own migrations alongside pg-flux?"

Don't. The whole point is to remove migrations as a separate source of truth. If you have an existing migrations folder, you have two options:

- **Migrate to declarative**: use the dump-baseline workflow above. Your old migrations stay where they are; new changes come from pg-flux.
- **Keep both**: not recommended. The two systems will fight over baseline state. Pick one.

### "How do I exclude a table from codegen?"

In `.pg-flux-codegen.yml`:

```yaml
exclude_tables:
  - "_pgflux_*"     # migration tracking
  - "audit_*"       # audit log tables
  - "schema_migrations"  # whatever your old tool left behind
```

Or per-output, exclude entire schemas:

```yaml
exclude_schemas:
  - "_pgflux"
  - "audit"
```

### "How do I generate types for only a few tables?"

```yaml
include_tables:
  - "users"
  - "orders"
  - "public.posts"
```

When `include_tables` is non-empty, it acts as an allowlist — only matching tables get types. Everything else is silent-dropped (no error).

## When all else fails

```bash
# 1. structured logs
pg-flux ... --log-format=json --verbose

# 2. shadow validation
pg-flux migrate apply --shadow-dsn="postgres://...disposable..."

# 3. inspect the live state
pg-flux inspect > /tmp/live.sql

# 4. compare to source state
pg-flux drift
```

If you've exhausted these and pg-flux is still not behaving — [open a bug](https://github.com/nex-gen-tech/pg-flux/issues/new?template=bug_report.md) with the structured log output, the exact pg-flux + PG versions, and the smallest reproducing schema.
