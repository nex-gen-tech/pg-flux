# go-blog

A minimal Go blog API demonstrating the full **pg-flux** workflow:
combined up/down migrations, schema evolution, codegen, and rollback.

## Stack

- **Go** 1.22+ — `net/http` + `pgx/v5`
- **PostgreSQL** 14+
- **pg-flux** for migrations and type generation

## Schema

| Table | Description |
|---|---|
| `users` | Blog authors (handle, email, bio) |
| `posts` | Articles with status enum (`draft` / `published` / `archived`) |
| `tags` / `post_tags` | M:N tag relationship |
| `comments` | Post comments |

Views: `published_posts`  
Triggers: `set_updated_at` on users and posts

## Quick start

```bash
# 1. Create the database
createdb pgflux_blog

# 2. Apply migrations
pg-flux migrate apply

# 3. Generate Go types
pg-flux gen --lang go --out ./gen --package blog

# 4. Run the API
DATABASE_URL=postgres://localhost/pgflux_blog go run .
```

## pg-flux config highlights

```yaml
# .pg-flux.yml
migrate:
  generate_undo: true   # always write Down SQL
  format: combined      # Up + Down in a single .sql file
```

Every migration file has both `-- +migrate Up` and `-- +migrate Down` sections.

## Rollback demo

```bash
# Roll back the last migration
pg-flux migrate rollback

# Roll back last N migrations
pg-flux migrate rollback 2

# Dry-run first
pg-flux migrate rollback --dry-run
```

See [JOURNEY.md](JOURNEY.md) for the complete step-by-step walkthrough.

## API endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/posts` | List published posts |
| `GET` | `/users` | List users |
| `GET` | `/health` | Health check |
