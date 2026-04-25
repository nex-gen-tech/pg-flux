# 12 — Security Considerations

**Document:** Security Threat Model & Controls
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## 12.1 Threat Model

pg-flux is a CLI tool that:
1. Reads `.sql` files from the local filesystem.
2. Connects to a PostgreSQL database.
3. Queries system catalogs.
4. Executes DDL statements.

### Trust Boundaries

```
┌──────────────────────────────────────────────────────────────────┐
│  Developer Workstation / CI Runner                                │
│                                                                  │
│   .sql files ──► pg-flux binary ──► PostgreSQL 18 Database      │
│   (trusted)                          (trusted, but credentials  │
│                                        must not be leaked)       │
│                    ↑                                             │
│             .pg-flux.yml (trusted)                               │
└──────────────────────────────────────────────────────────────────┘
```

### Assets to Protect

| Asset | Risk |
|-------|------|
| Database credentials (connection string, password) | Credential leakage in logs or error messages |
| Live database schema (structure) | Unintended structural changes (data definition modification) |
| Live database data | Data loss from `DROP TABLE` / `DROP COLUMN` hazards |
| Shadow database | Privilege escalation via shadow DB to live DB |

---

## 12.2 OWASP Top 10 Mapping

### A01: Broken Access Control

**Control:** pg-flux operates with the privileges of the connected database user. It does not attempt to escalate privileges.

**Requirement:** The database user used for migrations should have exactly the permissions needed:
```sql
-- Minimum required permissions for pg-flux apply
GRANT CONNECT ON DATABASE myapp TO migrations_user;
GRANT USAGE ON SCHEMA public TO migrations_user;
GRANT CREATE ON SCHEMA public TO migrations_user;
-- For RLS operations:
ALTER TABLE ... OWNER TO migrations_user;  -- or use SUPERUSER for RLS

-- For shadow DB validation:
GRANT CREATEDB TO migrations_user;  -- Only if using shadow DB validation
```

**Documentation Requirement:** The README must document the minimum required permissions for each operation mode (`inspect`, `plan`, `apply`, `drift`).

---

### A03: Injection

**SQL Injection via Schema Names:**

All catalog queries use parameterized queries via pgx/v5. Schema names and table names are NEVER interpolated into query strings.

```go
// CORRECT — parameterized
rows, err := conn.Query(ctx, `
    SELECT c.relname, n.nspname
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = ANY($1)
`, targetSchemas)

// WRONG — never do this (SQL injection risk)
rows, err := conn.Query(ctx, fmt.Sprintf(
    "SELECT ... WHERE n.nspname = '%s'", userInput))
```

**DDL Statement Construction:**

DDL statements are constructed from the parsed AST objects, not from user-controlled strings. The pg_query_go library handles all SQL generation via its deparsing API.

However, DDL statements use identifiers (table names, column names) that must be properly quoted when included in DDL. The quoting function uses `pg_query.QuoteIdentifier()` or equivalent — never manual string concatenation.

```go
// CORRECT
func quoteIdent(name string) string {
    return pg_query.QuoteIdentifier(name)  // adds " quotes and escapes internal "
}

// WRONG
func quoteIdentBad(name string) string {
    return `"` + name + `"`  // injection possible if name contains "
}
```

**Hint Comment Injection:**

Hint comments (`-- @renamed from=X`) are parsed from `.sql` files. The `from=` value is validated to be a valid PostgreSQL identifier (alphanumeric + underscores, or a properly double-quoted name). Any `from=` value that fails identifier validation results in a parse error — it is never used as a raw string in query construction.

---

### A05: Security Misconfiguration

**SSL/TLS for Database Connections:**

By default, pgx/v5 requires SSL (`sslmode=require`) unless explicitly overridden. pg-flux inherits this default.

For production use, the recommended connection string should include SSL:
```bash
DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=verify-full&sslrootcert=/path/to/ca.crt"
```

**Configuration File Permissions:**

`.pg-flux.yml` should not contain credentials. It is designed for non-sensitive configuration only. The documentation warns against storing passwords in the config file.

**Mitigation:** pg-flux detects if the config file contains a `password:` or `db:` key that includes a password in the URL and emits a warning: "WARNING: Do not store database credentials in .pg-flux.yml. Use environment variables or a secrets manager."

---

### A09: Security Logging and Monitoring

**Password Redaction in Logs:**

All log output from pg-flux redacts database passwords:

```go
func sanitizeConnString(connStr string) string {
    u, err := url.Parse(connStr)
    if err != nil { return "[unparseable connection string]" }
    if u.User != nil {
        u.User = url.User(u.User.Username())  // drops password
    }
    return u.String()
}
```

**Audit Logging:**

When pg-flux applies a migration, it logs:
- Timestamp of migration start and end.
- Number of statements applied.
- Whether the migration succeeded or failed.
- The current database user.

This audit log is written to stderr (or to a log file if `--log-file` is configured).

In v1.1, an audit table (`public.pgflux_migrations`) can be written to the database itself to record applied migrations.

---

## 12.3 Credential Management Best Practices

### Environment Variables (Recommended)
```bash
export DATABASE_URL="postgres://migrations_user@host:5432/myapp"
export PGPASSWORD="my-secret-password"  # standard PG env var

pg-flux apply --schema ./schema
```

### Secrets Manager Integration (Advanced)
For CI/CD environments, use a secrets manager to inject credentials:

```yaml
# GitHub Actions — inject from GitHub Secrets
- name: Run pg-flux
  env:
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
  run: pg-flux apply --schema ./schema
```

### Never
- Do NOT hardcode passwords in `.pg-flux.yml`.
- Do NOT pass passwords via command-line flags (they appear in `ps aux` output).
- Do NOT commit connection strings with passwords to source control.

---

## 12.4 Advisory Lock Security

The advisory lock (`pg_advisory_lock`) prevents concurrent migrations. However, advisory locks are per-session and are released when the session disconnects.

**Security Implication:** If pg-flux crashes mid-migration, the advisory lock is automatically released when the database connection closes (PostgreSQL cleans up session-level locks).

**Lock Key Generation:**
The advisory lock key is derived from the target schema name(s) using a stable hash:
```go
func advisoryLockKey(schemas []string) int64 {
    h := fnv.New64a()
    sort.Strings(schemas)
    for _, s := range schemas {
        h.Write([]byte(s))
    }
    return int64(h.Sum64())
}
```

This ensures that migrations targeting different schemas don't block each other.

---

## 12.5 Shadow Database Security

The shadow database validation creates a temporary PostgreSQL database. Security considerations:

1. **Naming:** Shadow databases are named `pgflux_shadow_{timestamp}_{uuid}` to avoid conflicts and be easily identifiable.
2. **Cleanup:** The shadow database is always dropped after validation (via `defer`), even on error.
3. **Network isolation:** Shadow databases are created on the same server as the live database. No external network access is made.
4. **No sensitive data:** Shadow databases receive only schema DDL (no data), so there is no data exposure risk.
5. **Required privilege:** Shadow database creation requires `CREATEDB`. This should be scoped to the migration user only during migration runs.

---

## 12.6 Dependency Security

| Dependency | Security Consideration |
|------------|----------------------|
| `pg_query_go/v6` | Bundles libpg_query C code; C memory safety is the upstream's responsibility. No known CVEs. Monitor for upstream security advisories. |
| `pgx/v5` | Well-audited PostgreSQL driver; uses TLS by default; parameterized queries built-in. |
| `cobra` | CLI framework; no network access; low risk. |
| `viper` | Configuration file parsing; do not parse config from untrusted sources. |

**Vulnerability Scanning:**
- `govulncheck ./...` runs in CI on every pull request.
- `go mod tidy` is run in CI to prevent unnecessary dependencies.
- Dependency updates via `dependabot` for automated security patch PRs.
