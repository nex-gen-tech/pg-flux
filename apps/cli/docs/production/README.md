# pg-flux Production Documentation

**pg-flux** is a declarative PostgreSQL schema migration engine. You define what your schema *should* look like in `.sql` files; pg-flux computes the safest migration path, flags hazards, and applies changes.

---

## Documentation Index

| Document | Description |
|----------|-------------|
| [01-installation.md](./01-installation.md) | Building, installing, and verifying pg-flux |
| [02-quick-start.md](./02-quick-start.md) | First project in 10 minutes |
| [03-configuration.md](./03-configuration.md) | `.pg-flux.yml` and all CLI flags |
| [04-cli-reference.md](./04-cli-reference.md) | Complete command and flag reference |
| [05-schema-authoring.md](./05-schema-authoring.md) | Writing `.sql` schema files, hint annotations, and supported objects |
| [06-migration-lifecycle.md](./06-migration-lifecycle.md) | Generate → review → apply loop, tracking table, checksums |
| [07-hazard-system.md](./07-hazard-system.md) | Hazard types, severities, and how to allow/suppress them |
| [08-cicd-integration.md](./08-cicd-integration.md) | GitHub Actions, GitLab CI, shadow validation, drift detection |
| [09-operations-runbook.md](./09-operations-runbook.md) | Day-2 operations: rollbacks, drift recovery, emergency patches |
| [10-security.md](./10-security.md) | DB user permissions, secret handling, audit trail |
| [11-troubleshooting.md](./11-troubleshooting.md) | Common errors, FAQs, and debug techniques |

---

## Core Concepts (60-second version)

```
[ .sql schema files ]  ──► desired state
[ live PostgreSQL DB ] ──► current state
                                │
                         differ engine
                                │
                    ┌───────────┴───────────┐
                    │                       │
             hazard check            DAG sort
                    │                       │
                    └───────────┬───────────┘
                                │
                     generated .sql migration
                                │
                         apply (or review)
```

1. **Schema files** are plain SQL (`CREATE TABLE`, `CREATE TYPE`, `CREATE INDEX`, etc.) stored in a directory you control.
2. **`migrate generate`** diffs live vs desired, produces a timestamped `.sql` migration file.
3. **`migrate apply`** runs pending migrations in order, recording each in a tracking table.
4. **Hazards** (data loss, lock, type change) block generation unless explicitly allowed.

---

## Supported PostgreSQL Versions

| Version | Support |
|---------|---------|
| PostgreSQL 18 | Primary target |
| PostgreSQL 17 | Supported |
| PostgreSQL 16 | Supported |
| < 16 | Not tested; use at your own risk |
