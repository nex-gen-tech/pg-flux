---
title: Other commands
group: Reference
order: 5
description: init, plan, apply, drift, inspect, version.
---

## `pg-flux init`

```bash
pg-flux init [--dir ./schema] [--migrations-dir ./migrations]
```

Scaffolds a project: writes `.pg-flux.yml`, an empty `schema/` with an example file, and an empty `migrations/`. The example schema is a starting point you replace.

## `pg-flux plan`

```bash
pg-flux plan [--format human|json]
```

Compute the diff without writing a migration file. `--format=json` produces structured output for editor integrations and CI.

## `pg-flux apply`

```bash
pg-flux apply [--dry-run] [--statement-timeout 20min]
```

Apply the in-memory plan directly (no migration file involved). Useful for ad-hoc fixes or non-production scratch databases.

> [!CAUTION]
> Prefer `migrate generate` + `migrate apply` in any environment with a
> ledger. `pg-flux apply` leaves no record of what ran.

## `pg-flux drift`

```bash
pg-flux drift [--strict]
```

Symmetric diff: live ↔ source. Exits 1 when anything differs.

```bash
$ pg-flux drift --strict
diff:
  ADD COLUMN users.last_login timestamptz
  DROP INDEX idx_orphan
```

Exit codes: `0` clean, `2` drift detected.

## `pg-flux inspect`

```bash
pg-flux inspect
```

Dump every catalog object as CREATE-style SQL to stdout. Read-only, no codegen. Mainly used for debugging the inspector.

## `pg-flux version`

```bash
pg-flux version
# pg-flux v0.1.3
```

Prints the binary's version string.
