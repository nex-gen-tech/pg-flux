# 10 — Security

---

## Threat Model

pg-flux is a CLI tool that:

1. Reads `.sql` files from the local filesystem or CI workspace.
2. Connects to PostgreSQL via a standard DSN.
3. Queries system catalogs (read-only during inspect/generate).
4. Executes DDL statements (during apply).

Trust boundaries:

```
[ CI Runner / Developer Workstation ]
        │
        ▼
[ pg-flux binary ]  ──── DDL only ────►  [ Live PostgreSQL ]
        │
        └── schema files (trusted input)
        └── .pg-flux.yml (trusted config)
```

**pg-flux never executes DML (INSERT/UPDATE/DELETE) on user tables.** The only writes it performs are to the `_pgflux.migrations` tracking table and the DDL statements in the migration files you generated and reviewed.

---

## Least-Privilege Database Role

Create a dedicated role for migrations. Never use the `postgres` superuser or your application's runtime role.

```sql
-- Create the migrations role
CREATE ROLE migrations_user WITH LOGIN PASSWORD 'strong-random-password';

-- Grant connection
GRANT CONNECT ON DATABASE myapp TO migrations_user;

-- Grant schema-level DDL
GRANT USAGE, CREATE ON SCHEMA public TO migrations_user;

-- pg-flux needs to read system catalogs (granted by default to all roles)
-- GRANT SELECT ON pg_catalog tables is not required; they are readable by default.

-- For RLS management (ENABLE ROW LEVEL SECURITY, CREATE POLICY):
-- These require table ownership or superuser.
-- Option A: grant ownership of specific tables:
ALTER TABLE public.users OWNER TO migrations_user;

-- Option B (simpler, use with care in truly isolated environments):
ALTER ROLE migrations_user SUPERUSER;

-- For creating extensions (requires superuser or pg_extension_owner):
GRANT pg_extension_owner TO migrations_user;  -- PG 15+
-- Or:
ALTER ROLE migrations_user SUPERUSER;  -- simpler but broader
```

**Shadow database role:**

The shadow DB role only needs `CONNECT` + full DDL on the shadow schema. It must **not** have access to the production DB.

```sql
-- On the shadow database server
CREATE ROLE shadow_migrations WITH LOGIN PASSWORD 'shadow-password';
GRANT CONNECT ON DATABASE shadow_myapp TO shadow_migrations;
GRANT USAGE, CREATE ON SCHEMA public TO shadow_migrations;
ALTER ROLE shadow_migrations SUPERUSER;  -- needed for RLS on shadow
```

---

## Connection String Security

**Never hardcode credentials in:**
- `.pg-flux.yml` committed to git
- CI workflow YAML files
- Shell scripts committed to git
- Docker Compose files committed to git

**Always use:**
- Environment variables (`PGFLUX_DB`, `DATABASE_URL`)
- Secret stores (GitHub Secrets, GitLab CI Variables, Vault, AWS Secrets Manager)
- `.pg-flux.yml` that reads from env: `db: ${DATABASE_URL}`

**.gitignore template:**

```
# Never commit credentials
.pg-flux.yml.local
.env
.env.*
!.env.example
```

**Verify no credentials in git history:**

```bash
git log --all --full-diff -S "password" -- "*.yml" "*.yaml" "*.env"
```

---

## SQL Injection Prevention

pg-flux uses parameterized queries (`pgx/v5`) for all catalog reads. Schema names, table names, and column names are **never** string-interpolated into query text in the inspector.

For DDL generation, identifiers are quoted via the `ident()` function which wraps the identifier in double quotes and escapes any embedded double quotes:

```go
func ident(s string) string {
    return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
```

**Caution:** The `-- @using` annotation expression is inserted verbatim into the generated `USING` clause. Review USING expressions carefully before applying, especially if they are sourced from untrusted input.

---

## Migration File Integrity

pg-flux stores a SHA-256 checksum of every applied migration file in the tracking table. On every `migrate apply`, all previously-applied files are re-checksummed and compared. If a file has been tampered with, apply aborts immediately.

```
Error: checksum mismatch for already-applied migration 20260428_120000.sql:
  recorded=abc123... current=def456...
  — do not edit applied migrations
```

This means:
- Applied migration files are immutable by design.
- If someone edits an applied file to cover up a change, pg-flux will catch it on the next deploy.
- The tracking table itself should be protected: revoke `DELETE` and `UPDATE` on `_pgflux.migrations` from the application runtime role.

```sql
-- Lock down the tracking table
REVOKE ALL ON _pgflux.migrations FROM app_runtime_user;
GRANT SELECT ON _pgflux.migrations TO app_runtime_user;  -- read-only for monitoring
```

---

## Audit Trail

The `_pgflux.migrations` table provides an immutable audit trail of every schema change:

```sql
SELECT filename, checksum, applied_at
FROM _pgflux.migrations
ORDER BY applied_at;
```

For enhanced auditing, enable PostgreSQL statement logging for the migrations role:

```sql
ALTER ROLE migrations_user SET log_min_duration_statement = 0;
ALTER ROLE migrations_user SET log_statement = 'ddl';
```

This logs every DDL statement with timestamp and duration to the PostgreSQL log.

---

## Shadow Database Isolation

The shadow database is used for pre-flight validation. It must be completely isolated from production:

- Different server (or at least a different PostgreSQL instance).
- The shadow role must not have `postgres_fdw` or `dblink` access to production.
- Network firewall: the shadow DB should not be accessible from production application servers.
- Shadow DB credentials must be different from production DB credentials.

---

## Secrets Rotation

When rotating the migrations database password:

1. Create the new password in your secret store.
2. Update the PostgreSQL role: `ALTER ROLE migrations_user PASSWORD 'new-password';`
3. Update the secret in CI.
4. Verify the next deploy succeeds.
5. Revoke the old password (if it was used via a connection pool, flush the pool).

---

## OWASP Top 10 Mapping

| OWASP | Control in pg-flux |
|-------|-------------------|
| A01 Broken Access Control | Dedicated migrations role with minimum privileges; revoke DML access from runtime role |
| A02 Cryptographic Failures | SHA-256 checksum on all migration files; TLS enforced on PostgreSQL connections via libpq `sslmode=require` |
| A03 Injection | Parameterized queries for catalog reads; `ident()` quoting for all DDL identifiers |
| A04 Insecure Design | Hazard system blocks destructive operations by default; explicit opt-in required |
| A05 Security Misconfiguration | No hardcoded defaults for credentials; config validation on startup |
| A06 Vulnerable Components | `go.sum` pins all dependency checksums; `govulncheck` recommended in CI |
| A07 Auth Failures | Credentials only in env vars / secret store; never logged |
| A09 Logging Failures | Migration filename + checksum logged on every apply; PostgreSQL DDL audit logging recommended |
