---
title: Migrations
group: Migrations
order: 1
---

# Migrations

pg-flux's migration model is **declarative**: you write the schema you want in `schema/`, the tool generates the minimum DDL to get there, and you apply it. There's no SQL-up / SQL-down pair to maintain.

## The lifecycle

```
edit schema/ → pg-flux migrate generate → review .sql → pg-flux migrate apply
                                                              ↓
                                                       _pgflux.migrations
                                                          (tracking)
```

Every step is observable:

```bash
pg-flux migrate status        # see applied / pending
pg-flux drift                 # see live ≠ source diff
pg-flux verify                # see live ⊃ source delta
```

## Generate

```bash
pg-flux migrate generate --label add_users_email_index
```

This:

1. Loads `schema/**/*.sql` and parses with `pg_query` (PG-grade parser).
2. Inspects the live database via `pg_catalog`.
3. Diffs the two states. The differ has full coverage for tables, indexes, views, functions, triggers, policies, enums, composite types, domains, sequences, extensions, default privileges, event triggers, statistics, foreign servers/tables, ownership, comments, and grants.
4. Embeds a sha256 baseline hash so apply can detect drift.
5. Writes a timestamped file: `migrations/20260520_103245_add_users_email_index.sql`.

The generated file is plain SQL you can review, edit, or run by hand.

## Apply

```bash
pg-flux migrate apply
```

Each pending file runs in a single transaction (except `CREATE INDEX CONCURRENTLY`-class statements which PG forbids inside transactions — those run autocommit after the main txn commits). pg-flux acquires a session-level advisory lock so two concurrent apply runs can't race.

If the baseline-hash check detects drift (someone modified the DB outside pg-flux between generate and apply), apply refuses:

```
refusing to apply 20260520_add_users_email_index.sql: live database state has drifted
since this migration was generated (expected baseline=abc123…, live=def456…).
Re-run `pg-flux migrate generate` to rebase the migration, or pass --force-after-drift
to apply anyway.
```

> [!TIP]
> In a multi-developer team, this error often means a colleague's migration was
> applied before yours. Run `pg-flux migrate rebase` to regenerate your pending
> migration against the current database state. See [Working in a team →](/docs/teamwork.html).

## Status

```bash
pg-flux migrate status
# 20260520_103245_initial_schema.sql       applied 2026-05-20 10:33:01
# 20260520_120100_add_email_index.sql      pending
```

## Repair / baseline / undo

These edge-case commands cover real production scenarios:

| Command | When to use |
|---|---|
| `pg-flux migrate baseline FILE` | Adopting pg-flux against an existing DB — marks files as "already applied" without running them. |
| `pg-flux migrate repair`        | A migration's content was edited after applying. Recomputes the recorded checksum (safe on already-applied content). |
| `pg-flux migrate generate --generate-undo` | Also writes a best-effort reverse-migration alongside the forward one. |
| `pg-flux migrate rebase` | Two developers generated migrations against the same base state. After the other's migrates were applied, regenerate yours on top. |

## Drift detection

`pg-flux drift` compares live DB ↔ source and exits 1 if anything diffs. Pair with `--strict` in CI:

```bash
pg-flux drift --strict --db "$DATABASE_URL"
# diff output:
#   ADD COLUMN users.last_login timestamptz
#   DROP INDEX idx_orphan
```

This is your CI canary: if it fails on `main`, someone ran SQL by hand in production and didn't update source.

## Hazards

pg-flux classifies every statement it emits. `--allow-hazards` gates risky ops behind explicit consent:

```bash
pg-flux migrate apply
# Error: refusing to apply: blocking hazards; pass --allow-hazards or change schema

pg-flux migrate apply --allow-hazards=DATA_LOSS,COLUMN_TYPE_CHANGE
```

The hazard types are documented in [Hazards →](/docs/hazards.html). Sane defaults:

| Hazard | Default |
|---|---|
| `DATA_LOSS` (DROP TABLE / DROP COLUMN) | refuse |
| `COLUMN_TYPE_CHANGE` (table rewrite) | refuse |
| `MASS_DROP` (>25% of live objects dropped) | refuse |
| `CONSTRAINT_SCAN` (ADD FK without NOT VALID) | refuse |

For ADD CHECK / ADD FOREIGN KEY on large tables, pg-flux auto-rewrites to the safe NOT VALID + VALIDATE pattern (`--auto-not-valid` is on by default). VALIDATE runs outside the main transaction under ShareUpdateExclusive lock.

## CONCURRENTLY indexes

`CREATE INDEX CONCURRENTLY` can't run inside a transaction. pg-flux handles this automatically: regular DDL goes in the main txn, concurrent index ops run after the txn commits in autocommit mode.

## See also

- [Hazards →](/docs/hazards.html)
- [Drift recovery →](/docs/drift.html)
- [Configuration →](/docs/configuration.html)
- [Working in a team →](/docs/teamwork.html)
