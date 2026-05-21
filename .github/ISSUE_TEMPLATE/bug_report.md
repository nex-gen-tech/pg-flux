---
name: Bug report
about: Something pg-flux did that it shouldn't have, or didn't do that it should have.
title: "[bug] "
labels: bug, needs-triage
---

## What happened

A clear description of what went wrong. One sentence is fine; a wall of text is also fine.

## What you expected

What you thought pg-flux would do instead.

## Repro

The shortest possible sequence of commands and SQL that demonstrates the bug. If we can't reproduce it we can't fix it.

```bash
# the exact commands you ran
```

```sql
-- the schema or DDL involved
```

## Versions

- pg-flux: `pg-flux version` → ___
- PostgreSQL: `SELECT version();` → ___
- OS: macOS / Linux / Windows + version
- Go: `go version` → ___ (if building from source)

## Logs

If pg-flux emitted any output (especially `--log-format=json --verbose`), paste it:

```text

```

## Other context

Anything else that might matter — connection pool size, parallel workers, weird PG extensions installed, prior migration history that might've left state behind.
