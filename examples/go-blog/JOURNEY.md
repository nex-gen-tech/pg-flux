# go-blog: pg-flux User Journey

A step-by-step walkthrough of the complete pg-flux workflow using this example.
Demonstrates **combined up/down migrations**, **rollback**, and **codegen** from scratch.

## Prerequisites

- pg-flux installed (`go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest`)
- PostgreSQL running
- Go 1.22+

## Setup

```bash
cd examples/go-blog
cp .env.example .env
# Edit DATABASE_URL in .env if needed

# Create the database
createdb pgflux_blog
# or: psql postgres://... -c "CREATE DATABASE pgflux_blog;"
```

---

## Step 1 — Initial schema → generate migration

The `.pg-flux.yml` has `migrate.format: combined` so every generated migration
is a single file with both forward (`-- +migrate Up`) and reverse (`-- +migrate Down`) SQL.

```bash
pg-flux migrate generate --label initial_schema
```

Output:
```
Generated: migrations/20260525_064851_083_initial_schema.sql (12 statements)
Format:    combined (up/down in one file)
Next: pg-flux migrate apply
```

Open the migration file — it has both sections in one file:

```sql
-- +migrate Up

-- pg-flux generated migration
...
BEGIN;
CREATE TABLE public.users (...);
CREATE TABLE public.posts (...);
...
COMMIT;
-- CREATE INDEX CONCURRENTLY ...

-- +migrate Down

BEGIN;
DROP VIEW IF EXISTS "public"."published_posts" CASCADE;
DROP FUNCTION IF EXISTS "public"."set_updated_at"() CASCADE;
DROP TABLE IF EXISTS "public"."posts" CASCADE;
DROP TABLE IF EXISTS "public"."users" CASCADE;
DROP TYPE IF EXISTS "public"."post_status" CASCADE;
COMMIT;
```

---

## Step 2 — Apply

```bash
pg-flux migrate apply
```

Output:
```
apply 20260525_064851_083_initial_schema.sql ...
      ok

Applied 1 migration(s), 0 already up to date.
Next: pg-flux gen    (refresh generated types)
```

---

## Step 3 — Generate Go types

```bash
pg-flux gen --lang go --out ./gen --package blog
```

Output:
```
[go] wrote 3 file(s)
```

The `gen/` directory now has typed Go structs matching every table, enum, and view:

```go
// Post mirrors public.posts.
type Post struct {
    ID          int64      `db:"id" json:"id"`
    AuthorID    int64      `db:"author_id" json:"author_id"`
    Slug        string     `db:"slug" json:"slug"`
    Title       string     `db:"title" json:"title"`
    Body        string     `db:"body" json:"body"`
    Status      PostStatus `db:"status" json:"status"`
    PublishedAt *time.Time `db:"published_at" json:"published_at"`
    ...
}
```

---

## Step 4 — Check status

```bash
pg-flux migrate status
```

Output:
```
STATUS   FILENAME                                APPLIED AT                     DOWN
applied  20260525_064851_083_initial_schema.sql  2026-05-25 06:48:51.151914+00  yes
```

The `DOWN yes` column confirms rollback SQL is available for this migration.

---

## Step 5 — Evolve schema: add comments table

Add `schema/comments.sql` then generate a second migration:

```bash
pg-flux migrate generate --label add_comments
```

Output:
```
Generated: migrations/20260525_064855_375_add_comments.sql (2 statements)
Format:    combined (up/down in one file)
Next: pg-flux migrate apply
```

Apply it:

```bash
pg-flux migrate apply
```

Output:
```
skip  20260525_064851_083_initial_schema.sql (already applied)
apply 20260525_064855_375_add_comments.sql ...
      ok

Applied 1 migration(s), 1 already up to date.
```

---

## Step 6 — Status (two migrations, both have Down SQL)

```bash
pg-flux migrate status
```

Output:
```
STATUS   FILENAME                                APPLIED AT                     DOWN
applied  20260525_064851_083_initial_schema.sql  2026-05-25 06:48:51.151914+00  yes
applied  20260525_064855_375_add_comments.sql    2026-05-25 06:48:55.435033+00  yes
```

---

## Step 7 — Dry-run rollback

```bash
pg-flux migrate rollback --dry-run
```

Output:
```
would rollback 20260525_064855_375_add_comments.sql

Dry run: would roll back 1 migration(s).
```

---

## Step 8 — Rollback last migration

```bash
pg-flux migrate rollback
```

Output:
```
rollback 20260525_064855_375_add_comments.sql ...
         ok

Rolled back 1 migration(s).
```

The `comments` table is gone from the database, and the migration is back to `pending`:

```bash
pg-flux migrate status
# STATUS   FILENAME                                APPLIED AT                    DOWN
# applied  20260525_064851_083_initial_schema.sql  2026-05-25 06:48:51...        yes
# pending  20260525_064855_375_add_comments.sql                                  yes
```

Re-apply when ready:

```bash
pg-flux migrate apply
```

---

## Step 9 — Rollback everything (N=2)

Roll back all applied migrations at once:

```bash
pg-flux migrate rollback 2
```

Output:
```
rollback 20260525_064855_375_add_comments.sql ...
         ok
rollback 20260525_064851_083_initial_schema.sql ...
         ok

Rolled back 2 migration(s).
```

The public schema is now completely empty — all tables, functions, enums, views, and indexes are gone. The migration files are untouched and can be re-applied at any time:

```bash
pg-flux migrate apply   # re-applies both migrations cleanly
```

---

## config that makes this work

The `.pg-flux.yml` has two lines that enable this workflow:

```yaml
migrate:
  generate_undo: true   # always generate Down SQL
  format: combined      # Up + Down in one file
```

Without these, you'd need `--generate-undo --format=combined` on every `migrate generate` call.
