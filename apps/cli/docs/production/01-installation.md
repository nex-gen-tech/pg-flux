# 01 — Installation

---

## Prerequisites

| Requirement | Minimum version | Notes |
|-------------|----------------|-------|
| Go | 1.22+ | Only needed when building from source |
| PostgreSQL | 16+ | Target database; pg_query_go requires libpg_query which ships statically |
| Docker | 20+ | Optional; used by embedded-test suite and shadow DB mode |
| `pg_query_go` | v6 | Bundled as a Go module dependency; no system lib needed |

---

## Build from Source

```bash
git clone https://github.com/nex-gen-tech/pg-flux.git
cd pg-flux
go build -o pg-flux ./cmd/pg-flux
```

To install directly into `$GOPATH/bin`:

```bash
go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
```

Verify the build:

```bash
pg-flux --version
# pg-flux v0.1.0 (go1.22, linux/amd64)
```

---

## Binary Distribution

Pre-built binaries for Linux (amd64/arm64) and macOS (arm64) are attached to each GitHub release.

```bash
# macOS arm64
curl -Lo pg-flux https://github.com/nex-gen-tech/pg-flux/releases/latest/download/pg-flux-darwin-arm64
chmod +x pg-flux
sudo mv pg-flux /usr/local/bin/
```

---

## Verify Installation

```bash
pg-flux --help
```

Expected output:

```
pg-flux — declarative PostgreSQL schema migration engine

Usage:
  pg-flux [command]

Available Commands:
  migrate     Generate and apply schema migrations
  inspect     Reverse-engineer live DB into schema files

Use "pg-flux [command] --help" for more information about a command.
```

---

## Database Requirements

pg-flux connects using a standard PostgreSQL DSN. The database user must have:

```sql
-- Inspect (read-only mode):
GRANT CONNECT ON DATABASE mydb TO migrations_user;
GRANT USAGE ON SCHEMA pg_catalog, information_schema TO migrations_user;
GRANT SELECT ON ALL TABLES IN SCHEMA pg_catalog TO migrations_user;

-- Apply (DDL mode): needs schema ownership or superuser for some operations
GRANT CONNECT ON DATABASE mydb TO migrations_user;
GRANT CREATE ON DATABASE mydb TO migrations_user;  -- CREATE SCHEMA
GRANT USAGE, CREATE ON SCHEMA public TO migrations_user;
-- For RLS and policy management:
ALTER ROLE migrations_user SUPERUSER;  -- recommended for production migrations role
-- OR grant table ownership explicitly per table
```

> **Tip:** Use a dedicated `migrations` database role. Never use the `postgres` superuser in CI pipelines. See [10-security.md](./10-security.md) for a least-privilege setup.

---

## Tracking Schema

pg-flux records applied migrations in a `_pgflux.migrations` table that it creates automatically on first run. The tracking schema can be changed via `--tracking-schema`.

```sql
-- Created automatically; do not drop manually
SELECT filename, checksum, applied_at
FROM _pgflux.migrations
ORDER BY applied_at;
```
