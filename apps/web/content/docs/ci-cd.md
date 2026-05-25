---
title: CI / CD integration
group: Migrations
order: 4
description: GitHub Actions, GitLab CI, CircleCI, pre-commit hooks — actual YAML you can copy.
---

You don't want migrations applying to prod from someone's laptop. This page shows you how to wire pg-flux into CI so:

- Pull requests fail if generated code is stale, the live DB has undeclared objects, or source has drifted from a target environment
- Merges to `main` auto-apply migrations to staging (and optionally production with manual approval)
- Local commits are guarded by pre-commit hooks before they even leave the dev machine

Every snippet below is real, copy-pasteable, and tested in the wild.

## The four CI gates

Pair these as your standard pipeline:

| Gate | Command | Exit code | What it catches |
|---|---|---|---|
| **Schema parses** | `go vet` / `tsc --noEmit` on generated output | non-zero | Generated types don't compile |
| **Drift** | `pg-flux drift --strict` | `2` | Live ≠ source (someone changed prod) |
| **Undeclared objects** | `pg-flux verify --strict` | `4` | Live has stuff not in source (manual hotfix) |
| **Generated code stale** | `pg-flux gen --check` | `3` | Source changed but codegen wasn't run |

Run all four on every PR.

## Multi-developer PR workflow

When two developers open PRs that both modify the schema, the PR that merges second needs a rebase step before it can deploy. pg-flux's CI detects this automatically and gives actionable guidance.

**What the tooling does for you:**
- Advisory lock on `migrate apply` — two pipelines can't apply simultaneously; the second waits up to 30 seconds, then either succeeds (first is done) or errors with a clear lock-release command
- Out-of-order detection — if a pending migration has an earlier timestamp than the last applied migration, `migrate apply` emits a warning and a step-by-step fix
- Drift error with parallel-branch guidance — the error message distinguishes "parallel development" from "manual schema change" and gives a 4-step fix

**The developer workflow for the second PR:**

```bash
# After your colleague's PR merged to main and was applied to staging:
git pull origin main            # pull their migration into your branch
pg-flux migrate apply           # apply it to your local DB
pg-flux migrate rebase          # regenerate your migration on top
git add migrations/
git commit -m "rebase: regenerate migration after teammate's changes"
git push
```

**Adding a rebase check to your PR CI:**

You can fail CI explicitly when a migration is detected as needing a rebase, before it even reaches the apply step:

```yaml
- name: check for out-of-order migrations
  env:
    DATABASE_URL: ${{ secrets.STAGING_DATABASE_URL }}
  run: |
    # pg-flux migrate apply --dry-run prints out-of-order warnings
    # exit non-zero if any out-of-order migration is detected
    output=$(pg-flux migrate apply --dry-run 2>&1)
    echo "$output"
    if echo "$output" | grep -q "out-of-order migration"; then
      echo "❌ Out-of-order migrations detected. Run: pg-flux migrate rebase"
      exit 1
    fi
```

This surfaces the problem earlier (on the PR itself) rather than at the deploy step.

## GitHub Actions

Drop this in `.github/workflows/pg-flux.yml`:

```yaml
name: pg-flux

on:
  pull_request:
  push:
    branches: [main]

jobs:
  schema-check:
    name: schema + codegen drift
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:17
        env:
          POSTGRES_USER: pgflux
          POSTGRES_PASSWORD: pgflux
          POSTGRES_DB: pgflux
        ports: ["5432:5432"]
        options: >-
          --health-cmd="pg_isready -U pgflux"
          --health-interval=5s
          --health-retries=10

    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: apps/cli/go.mod
          cache: true
          cache-dependency-path: apps/cli/go.sum

      - name: install pg-flux
        run: go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest

      - name: bring schema to current
        env:
          DATABASE_URL: postgres://pgflux:pgflux@localhost:5432/pgflux?sslmode=disable
        run: pg-flux migrate apply

      # 1. Generated code must be in sync with source.
      - name: codegen check
        env:
          DATABASE_URL: postgres://pgflux:pgflux@localhost:5432/pgflux?sslmode=disable
        run: pg-flux gen --check

      # 2. No undeclared objects in the CI database.
      - name: verify
        env:
          DATABASE_URL: postgres://pgflux:pgflux@localhost:5432/pgflux?sslmode=disable
        run: pg-flux verify --strict

      # 3. Generated Go must compile.
      - name: go build generated
        run: |
          if [ -d internal/dbgen ]; then
            cd internal/dbgen && go build ./...
          fi

      # 4. Generated TS must type-check (if applicable).
      - name: tsc generated
        run: |
          if [ -d src/db ]; then
            npx tsc --noEmit -p src/db
          fi

  prod-drift:
    name: prod drift canary
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    needs: schema-check
    steps:
      - uses: actions/checkout@v4
      - run: go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
      - name: drift check vs production
        env:
          DATABASE_URL: ${{ secrets.PROD_DATABASE_URL }}
        run: pg-flux drift --strict
```

> [!IMPORTANT]
> `secrets.PROD_DATABASE_URL` is a GitHub Actions secret. Never put a
> production DSN in workflow YAML.

### Auto-apply to staging on merge

Append a job that runs only on `main`:

```yaml
  apply-staging:
    name: apply migrations to staging
    if: github.ref == 'refs/heads/main' && github.event_name == 'push'
    runs-on: ubuntu-latest
    needs: schema-check
    environment: staging  # gates on GitHub environment protection rules
    steps:
      - uses: actions/checkout@v4
      - run: go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest

      - name: apply migrations
        env:
          DATABASE_URL: ${{ secrets.STAGING_DATABASE_URL }}
        run: |
          pg-flux migrate apply \
            --shadow-dsn="${{ secrets.STAGING_SHADOW_DSN }}"

      - name: verify post-apply
        env:
          DATABASE_URL: ${{ secrets.STAGING_DATABASE_URL }}
        run: pg-flux drift --strict
```

For production, gate behind a manual approval — GitHub's `environment` protection rules let you require a reviewer to approve before the job runs.

## GitLab CI

```yaml
stages: [test, deploy]

variables:
  DATABASE_URL: postgres://pgflux:pgflux@postgres:5432/pgflux?sslmode=disable

schema:check:
  stage: test
  image: golang:1.25
  services:
    - name: postgres:17
      alias: postgres
      variables:
        POSTGRES_USER: pgflux
        POSTGRES_PASSWORD: pgflux
        POSTGRES_DB: pgflux
  script:
    - go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
    - pg-flux migrate apply
    - pg-flux gen --check
    - pg-flux verify --strict
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

migrate:staging:
  stage: deploy
  image: golang:1.25
  variables:
    DATABASE_URL: ${STAGING_DATABASE_URL}
  script:
    - go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
    - pg-flux migrate apply
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  environment:
    name: staging
```

## CircleCI

```yaml
version: 2.1

orbs:
  go: circleci/go@1.11

jobs:
  schema-check:
    docker:
      - image: cimg/go:1.25
      - image: postgres:17
        environment:
          POSTGRES_USER: pgflux
          POSTGRES_PASSWORD: pgflux
    steps:
      - checkout
      - run: go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
      - run:
          name: wait for postgres
          command: |
            for i in $(seq 1 30); do
              pg_isready -h localhost -U pgflux && break
              sleep 1
            done
      - run:
          name: pg-flux drift + verify + gen --check
          environment:
            DATABASE_URL: postgres://pgflux:pgflux@localhost:5432/pgflux?sslmode=disable
          command: |
            pg-flux migrate apply
            pg-flux gen --check
            pg-flux verify --strict

workflows:
  test:
    jobs:
      - schema-check
```

## Pre-commit hook

Catch drift before the commit even leaves the dev machine. Drop this in `.git/hooks/pre-commit` and `chmod +x`:

```bash
#!/usr/bin/env bash
set -e

# Only run if schema/ changed
if git diff --cached --name-only | grep -q '^schema/'; then
  echo "schema/ changed — running pg-flux checks"
  pg-flux drift || {
    echo "❌ drift detected: run 'pg-flux migrate generate' before committing"
    exit 1
  }
  pg-flux gen --check || {
    echo "❌ generated code is stale: run 'pg-flux gen' before committing"
    exit 1
  }
fi
```

Or, if you use [pre-commit](https://pre-commit.com/), add to `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: local
    hooks:
      - id: pg-flux-drift
        name: pg-flux drift
        entry: pg-flux drift
        language: system
        pass_filenames: false
        files: ^schema/
      - id: pg-flux-gen-check
        name: pg-flux gen --check
        entry: pg-flux gen --check
        language: system
        pass_filenames: false
        files: ^schema/
```

## Pre-push hook

If pre-commit feels too aggressive, do it on push instead. `.git/hooks/pre-push`:

```bash
#!/usr/bin/env bash
set -e

# Skip if no schema changes in this push
range="${1:-origin/main..HEAD}"
if git diff --name-only "$range" 2>/dev/null | grep -q '^schema/'; then
  pg-flux drift || exit 1
  pg-flux gen --check || exit 1
fi
```

## Production deploy pipeline

Production migrations are different from CI. The recipe:

1. **PR merged to main** → CI runs full checks, auto-deploys to staging
2. **Staging green for X hours** → manual promotion job triggered
3. **Production apply** runs with:
   - Shadow DB validation first (`--shadow-dsn`)
   - Statement timeout cap (`--statement-timeout=20min`)
   - Structured JSON logs to your log aggregation
   - Slack notification on success/failure
4. **Post-apply** → run `pg-flux drift --strict` to confirm

Example GitHub Actions production job:

```yaml
  apply-prod:
    name: apply migrations to production
    if: github.ref == 'refs/heads/main'
    needs: apply-staging
    runs-on: ubuntu-latest
    environment:
      name: production
      url: https://your-app.example.com
    steps:
      - uses: actions/checkout@v4
      - run: go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest

      - name: shadow-validate first
        env:
          DATABASE_URL: ${{ secrets.PROD_DATABASE_URL }}
          PGFLUX_SHADOW_DSN: ${{ secrets.PROD_SHADOW_DSN }}
        run: |
          pg-flux migrate apply \
            --shadow-dsn="$PGFLUX_SHADOW_DSN" \
            --dry-run

      - name: real apply
        env:
          DATABASE_URL: ${{ secrets.PROD_DATABASE_URL }}
        run: |
          pg-flux migrate apply \
            --statement-timeout=20min \
            --log-format=json \
            2>&1 | tee /tmp/pg-flux-apply.json

      - name: post-apply drift check
        env:
          DATABASE_URL: ${{ secrets.PROD_DATABASE_URL }}
        run: pg-flux drift --strict

      - name: ship logs
        if: always()
        run: |
          curl -F "file=@/tmp/pg-flux-apply.json" "${{ secrets.LOG_INGEST_URL }}"
```

> [!CAUTION]
> Never auto-apply to production without manual approval gates. The `environment:`
> block in GitHub Actions lets you require reviewer approval before the job runs.
> Equivalent gates exist in GitLab (`when: manual`) and CircleCI (approval jobs).

## When to run which command

| Trigger | Command(s) | Why |
|---|---|---|
| Every PR | `drift --strict`, `verify --strict`, `gen --check` | Fail fast on stale state |
| Merge to `main` | apply to staging, then `drift --strict` | Catch broken migrations before prod |
| Nightly (cron) | `verify --strict` against prod | Catch hotfixes that escaped source |
| Production deploy | `apply` with `--shadow-dsn` + post-apply `drift` | Sanity check |
| Local pre-commit | `drift`, `gen --check` | Save the CI round-trip |

## Failure recovery

When CI fails on `drift`, the workflow is:

```bash
# 1. capture what's actually in live
pg-flux pull --dry-run=false

# 2. review schema/_pulled/<timestamp>_pulled.sql
# 3. move the relevant blocks into the proper schema/*.sql files
# 4. commit and re-push
git add schema/
git commit -m "capture hotfix from prod"
git push
```

When CI fails on `gen --check`, just run gen locally and commit:

```bash
pg-flux gen
git add internal/dbgen src/db
git commit -m "regenerate types"
git push
```

When CI fails on `verify --strict`, same playbook as drift — pull, review, commit.

## Secrets management

| Secret type | Stored where |
|---|---|
| `DATABASE_URL` for CI's ephemeral DB | Public — it's an empty container |
| Staging DSN | CI secret (`STAGING_DATABASE_URL`) |
| Production DSN | CI secret with environment protection rules |
| Shadow DSN | CI secret, separate from prod |

Never commit a real DSN to a workflow file. Always use the platform's secret store and reference via `${{ secrets.X }}` or equivalent.
