---
title: Drift recovery
group: Migrations
order: 3
---

# Drift recovery

> Drift = the live database has been modified outside of pg-flux. Usually a manual hotfix, sometimes another tool, occasionally a developer who forgot.

pg-flux has three layers of drift detection. Pick the one that fits your scenario.

## Layer 1: `pg-flux drift`

Bidirectional structural diff. Detects anything in source that's not in live AND anything in live that's not in source.

```bash
pg-flux drift --strict
```

In CI on `main`, this catches "production diverged from source" within hours of it happening. Exit 1 on any difference.

## Layer 2: `pg-flux verify`

Asymmetric: live ⊃ source. Lists objects that exist in live but aren't declared in source. Doesn't flag missing-in-live objects (those would be caught at the next `migrate apply`).

```bash
pg-flux verify --strict
# verify: 1 undeclared live object(s):
#
#   Indexes (1):
#     - public.idx_emergency_perf
#
# Run `pg-flux pull` to capture these into schema/_pulled/<ts>.sql for review.
```

## Layer 3: baseline-hash check

Per-migration drift detection between `migrate generate` and `migrate apply`. Every generated migration embeds a sha256 of the live schema at generation time. `apply` recomputes and refuses on mismatch:

```bash
pg-flux migrate apply
# refusing to apply 20260520_add_role.sql: live database state has drifted
# since this migration was generated (expected baseline=abc123…, live=def456…).
# Re-run `pg-flux migrate generate` to rebase the migration,
# or pass --force-after-drift to apply anyway.
```

## Recovery playbook

### Scenario A: someone added an emergency index

```bash
$ pg-flux verify --strict
verify: 1 undeclared live object(s):
  Indexes (1):
    - public.idx_emergency_perf

$ pg-flux pull
Wrote 1 object(s) to ./schema/_pulled/20260520_103245_pulled.sql

# Review the file; move the CREATE INDEX block into the appropriate
# schema/indexes/public.users.sql file. Commit.

$ git diff schema/
# +CREATE INDEX idx_emergency_perf ON public.users USING btree (last_login);

$ pg-flux verify --strict
verify: clean — every live object is declared in source.
```

### Scenario B: drift detected during apply

You ran `pg-flux migrate generate` last week. Today you `migrate apply` and pg-flux refuses:

```bash
$ pg-flux migrate apply
refusing to apply 20260512_add_users.sql: live database state has drifted
```

Two paths:

1. **Reconcile and regenerate**. Run `pg-flux pull` to capture whatever changed, merge into source, then `pg-flux migrate generate --label rebase` to write a new migration on top of the new live state.

2. **Force-apply** (when you've verified the changes are compatible). `pg-flux migrate apply --force-after-drift`. Subsequent generates will compare against the new live state.

### Scenario C: live database state ≠ tracked migration history

The DB has objects that don't correspond to any migration file. Use `pg-flux migrate baseline` to mark existing migration files as "already applied" without running them:

```bash
$ pg-flux migrate baseline migrations/20260101_initial.sql
Marked 20260101_initial.sql as applied (checksum recorded)
```

Now subsequent applies pick up where you left off.

## CI patterns

### Pre-merge

```yaml
- run: pg-flux drift --strict
- run: pg-flux gen --check
```

### Daily on `main`

```yaml
on:
  schedule: [{ cron: "0 7 * * *" }]
steps:
  - run: pg-flux verify --strict --db "${{ secrets.DATABASE_URL }}"
```

## See also

- [Migrations →](/docs/migrations.html)
- [Dump · verify · pull →](/docs/dump.html)
