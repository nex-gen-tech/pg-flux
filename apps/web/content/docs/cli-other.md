---
title: Other commands
group: Reference
order: 5
description: init, plan, apply, drift, inspect, update, version.
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
pg-flux inspect [--type <kind>] [--object <name>] [--out <file>] [--summary]
```

Connects to the live database and prints every schema object as complete CREATE-style SQL. Read-only — nothing is written to your schema directory.

| Flag | Description |
|---|---|
| `--type <kind>` | Filter by object type: `table`, `view`, `function`, `index`, `enum`, `sequence`, `trigger`, `extension`, `policy`, `domain` |
| `--object <name>` | Filter by name; supports `*` glob (e.g. `--object 'public.*'`) |
| `--out <file>` | Write SQL to a file instead of stdout |
| `--summary` | Print a compact inventory (TYPE / SCHEMA / NAME) instead of full SQL |

```bash
# Full schema to stdout
pg-flux inspect

# Quick inventory of everything
pg-flux inspect --summary

# All tables
pg-flux inspect --type table

# One specific function
pg-flux inspect --type function --object normalize_email

# All views, save to file
pg-flux inspect --type view --out views.sql
```

Output includes complete DDL: storage parameters, constraints, defaults, `OWNER TO`, `COMMENT ON`, `GRANT` — the same SQL that `dump` writes to files.

## `pg-flux update`

```bash
pg-flux update [--version <tag>]
```

Interactively pick a version to install from a scrollable list of all GitHub releases, or pass `--version` to skip the prompt.

```bash
pg-flux update                    # interactive version picker
pg-flux update --version v0.1.5   # install a specific version
```

Downloads the binary for your OS and architecture, verifies the SHA-256 checksum, and atomically replaces the running binary in-place.

## `pg-flux version`

```bash
pg-flux version
# pg-flux v0.1.6
```

Prints the binary's version string.
