---
title: Working in a team
group: Migrations
order: 6
description: Multi-developer collaboration — branching, rebasing, and conflict resolution.
---

## Overview

pg-flux uses a single shared schema directory. The files in `schema/` are committed to git alongside migration files in `migrations/`. All developers on the team edit the same schema files and generate migrations from them — the combination of those two directories is the source of truth for your database schema.

The recommended branching model: each developer works on a feature branch, makes schema changes, runs `migrate generate` to capture a migration, and opens a PR. The migration file is part of the PR, just like the application code.

---

## The shared schema directory model

- One `schema/` directory in the repo, shared by all developers
- Migration files in `migrations/` are also committed to the repo
- pg-flux tracks which migrations have been applied per database via the `_pgflux.migrations` table
- All developers generate migrations against the same schema directory — the diff is always "current live DB state → desired schema"

This means two developers can branch at the same time, each add tables or columns, and each generate a migration independently. The two migrations will interleave by timestamp when merged.

---

## Scenario A — the happy path (no conflicts)

Dev A and Dev B work on completely unrelated tables. They generate migrations independently. Their timestamps differ because they generated at different times.

```
main:   ──────────────────────────┬─────────────────────────┬──────▶
                                  │ Dev A merges             │ Dev B merges
                                  │ 20260601_103010_...      │ 20260601_143022_...
```

When both PRs merge, the migrations are applied in timestamp order:

1. `20260601_103010_dev_a_add_payments.sql` — applied first
2. `20260601_143022_dev_b_add_notifications.sql` — applied second

No drift, no conflicts. Each migration was generated against a schema state that includes the other developer's changes only in alphabetical-timestamp order.

---

## Scenario B — parallel branch conflict (most common)

This is the most common collaboration issue. Both devs branch from the same commit. Both run `migrate generate` against the same live database. Their migration files have the **same baseline hash** because the live DB was in the same state when each was generated.

### Sub-case B1: Dev A merges first (later timestamp gets a drift error)

- Dev A generates at 10:00 → `20260601_100000_dev_a_add_tags.sql`
- Dev B generates at 10:05 → `20260601_100500_dev_b_add_labels.sql`
- Dev A's PR merges first, migration applies to staging/prod
- Dev B's PR tries to apply `dev_b_add_labels.sql` — **drift error**:

```text
Error: refusing to apply 20260601_100500_dev_b_add_labels.sql: baseline hash mismatch.
Expected: 1dcf92a  Live: c28c4c3
The schema was modified outside pg-flux between generate and apply.
Re-run `pg-flux migrate rebase` to regenerate your pending migration against current state.
```

The live DB (post-Dev-A) no longer matches the baseline that was recorded when Dev B generated.

**Fix**: Dev B runs `migrate rebase`. See [The migrate rebase workflow](#the-migrate-rebase-workflow) below.

### Sub-case B2: Out-of-order — Dev B deploys before Dev A

- Dev A generates at 10:00 → `20260601_100000_dev_a_add_tags.sql`
- Dev B generates at 10:05 → `20260601_100500_dev_b_add_labels.sql`
- Dev B's PR merges **first** (reviews moved faster, etc.)
- `20260601_100500_dev_b_add_labels.sql` is applied to the shared DB
- Dev A's PR merges — the migrations directory now contains A's file (10:00) **before** B's already-applied file (10:05)
- When `migrate apply` runs on the next deploy: A's file has an earlier timestamp than the last applied migration → out-of-order warning + drift error with a step-by-step fix guide

```text
Warning: out-of-order migration detected.
  20260601_100000_dev_a_add_tags.sql has a timestamp earlier than the last applied
  migration (20260601_100500_dev_b_add_labels.sql).
Error: refusing to apply: baseline hash mismatch (expected 1dcf92a, live c28c4c3).
To fix:
  1. Pull the latest migrations from your main branch.
  2. Run `pg-flux migrate apply` on your local DB to bring it up to date.
  3. Run `pg-flux migrate rebase` to regenerate this migration on top of current state.
  4. Commit the updated migration file and re-open your PR.
```

**Fix**: Dev A runs `migrate rebase`. The rebase keeps the original `100000` filename — ordering vs. already-applied migrations is preserved in the tracking table.

---

## The `migrate rebase` workflow

`migrate rebase` regenerates all pending (unapplied) migration files against the current live database state. It keeps the original filenames (timestamps + labels) so ordering is preserved — only the SQL content and baseline hash are updated.

Here is the full step-by-step fix for Sub-case B1 (Dev B's migration after Dev A merged):

```bash
# 1. Pull the latest main (which now has Dev A's migration)
git pull origin main

# 2. Apply Dev A's migration to your local database
pg-flux migrate apply --db "$DATABASE_URL"

# 3. Rebase your pending migration on top of current state
pg-flux migrate rebase --db "$DATABASE_URL"
# Output: rebased  20260601_100500_dev_b_add_labels.sql
# Next: review the updated migration file, then run pg-flux migrate apply

# 4. Review the updated migration file
cat migrations/20260601_100500_dev_b_add_labels.sql

# 5. Apply the rebased migration locally to confirm it works
pg-flux migrate apply --db "$DATABASE_URL"

# 6. Commit the updated migration file
git add migrations/
git commit -m "rebase: regenerate migration on top of Dev A's changes"
git push
```

Important notes about rebase:

- **Keeps the original filename** — ordering vs. already-applied migrations is preserved in the tracking table
- **Review before applying** — if the rebase produces SQL that conflicts with Dev A's changes at the SQL level (e.g., you both added a column with the same name), you'll see it clearly when you review the updated file
- **Multiple pending files** — when more than one pending file exists, rebase folds all changes into the first file (earliest timestamp) and removes the rest. Commit both the updated file and the deletions.

---

## When `--force-after-drift` is appropriate

`--force-after-drift` skips the baseline hash check entirely. Use it **only** when both of the following are true:

1. You've reviewed the migration SQL manually and confirmed it is safe to apply against the current live state
2. You're recovering from a specific known-safe scenario — for example, a hotfix was applied directly to the database and you've already captured that change in source

Never use `--force-after-drift` as a default or as a workaround for rebase. It masks real problems and allows migrations to run against an unexpected schema state.

---

## Concurrent CI pipelines

If two CI pipelines fire `migrate apply` against the same database simultaneously (for example, two PRs merge in quick succession), pg-flux uses a PostgreSQL advisory lock to serialize them. The second pipeline waits up to 30 seconds. If the first completes within that time, the second runs and finds all migrations already applied:

```text
0 migration(s) applied, 2 already up to date.
```

If the lock wait times out:

```text
Error: could not acquire pg-flux migration lock after 30s — another `migrate apply` may be running.
To release manually: SELECT pg_advisory_unlock(7040926865817495040);
```

The lock ID is printed in the error. This is safe to release manually once you have confirmed no apply is actively running against that database.

---

## Recommended team conventions

1. **Keep `schema/` in the same repo as `migrations/`** — never split them into separate repositories. pg-flux needs both directories to generate accurate diffs.

2. **One PR = one migration** — generate the migration as part of the feature branch, not after merging. The migration file is part of the review.

3. **Rebase before merging (not after)** — if a colleague's PR merged while yours was in review, run `migrate rebase` on your branch before opening or updating your PR. Don't wait until merge.

4. **Never edit applied migrations** — use `migrate repair` only for comment or whitespace changes. Schema changes always require a new migration file.

5. **Protect `migrations/` in CI** — add a job that fails if `migrations/` changed without a corresponding change to `schema/`, or vice versa. This prevents a migration file being added without the schema source changing, and vice versa.

6. **Use shadow validation in staging CI** — `migrate apply --shadow-dsn <url>` runs the migration against a disposable copy of the database before touching the real one. This catches SQL errors before they reach a shared environment.

---

## Checking for conflicts before opening a PR

Run these checks before pushing your branch to confirm your migration won't conflict with recent changes on main:

```bash
# Fetch latest main
git fetch origin main

# Check if any migrations were added on main since your branch diverged
git diff --name-only origin/main...HEAD -- migrations/

# If main has new migrations, apply them locally and rebase
git stash          # stash uncommitted schema changes if any
git rebase origin/main
pg-flux migrate apply
pg-flux migrate rebase
git stash pop
```

If `git diff --name-only` shows no new migration files on main, your migration applies cleanly and no rebase is needed.

---

## See also

- [Migrations overview →](/docs/migrations.html)
- [CI/CD integration →](/docs/ci-cd.html)
- [Drift recovery →](/docs/drift.html)
- [Migration commands →](/docs/cli-migrate.html)
