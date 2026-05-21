# 08 — CI/CD Integration

---

## Recommended Pipeline Structure

```
Pull Request opened
  └─► CI: pg-flux migrate generate --dry-run
              → post diff as PR comment (optional)
              → fail if unexpected hazards present

Merge to main / release branch
  └─► CD: pg-flux migrate apply (staging)
              → pg-flux migrate generate  (assert no drift)

Release approved
  └─► CD: pg-flux migrate apply (production)
              → pg-flux migrate generate  (assert no drift)
```

---

## GitHub Actions

### Drift Detection on Pull Requests

```yaml
# .github/workflows/schema-check.yml
name: Schema Drift Check

on:
  pull_request:
    paths:
      - 'schema/**'
      - 'migrations/**'

jobs:
  schema-check:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:18
        env:
          POSTGRES_USER: pgflux
          POSTGRES_PASSWORD: pgflux
          POSTGRES_DB: testdb
        options: >-
          --health-cmd pg_isready
          --health-interval 5s
          --health-timeout 3s
          --health-retries 10
        ports:
          - 5432:5432

    steps:
      - uses: actions/checkout@v4

      - name: Install pg-flux
        run: |
          go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest

      - name: Apply all migrations to test DB
        env:
          DATABASE_URL: postgres://pgflux:pgflux@localhost:5432/testdb
        run: pg-flux migrate apply

      - name: Assert schema is clean (no drift)
        env:
          DATABASE_URL: postgres://pgflux:pgflux@localhost:5432/testdb
        run: |
          output=$(pg-flux migrate generate 2>&1)
          if echo "$output" | grep -q "Generated:"; then
            echo "DRIFT DETECTED:"
            echo "$output"
            exit 1
          fi
          echo "Schema is clean."
```

---

### Shadow Validation Before Production Apply

```yaml
# .github/workflows/deploy.yml
name: Deploy — Production Migrations

on:
  push:
    branches: [main]

jobs:
  migrate:
    runs-on: ubuntu-latest
    environment: production   # requires manual approval in GitHub Environments

    steps:
      - uses: actions/checkout@v4

      - name: Install pg-flux
        run: go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest

      # Validate against a shadow copy of production before touching live
      - name: Shadow validation
        env:
          PGFLUX_DB: ${{ secrets.PROD_DATABASE_URL }}
          PGFLUX_SHADOW_DSN: ${{ secrets.SHADOW_DATABASE_URL }}
        run: |
          pg-flux migrate apply \
            --dry-run \
            --shadow-dsn "$PGFLUX_SHADOW_DSN" \
            --shadow-semantic

      - name: Apply to production
        env:
          PGFLUX_DB: ${{ secrets.PROD_DATABASE_URL }}
        run: pg-flux migrate apply

      - name: Assert clean state
        env:
          PGFLUX_DB: ${{ secrets.PROD_DATABASE_URL }}
        run: |
          result=$(pg-flux migrate generate 2>&1)
          if echo "$result" | grep -q "Generated:"; then
            echo "Post-deploy drift detected!"
            echo "$result"
            exit 1
          fi
```

---

## GitLab CI

```yaml
# .gitlab-ci.yml
stages:
  - test
  - deploy

variables:
  PGFLUX_DB: $DATABASE_URL

schema-check:
  stage: test
  image: golang:1.22
  services:
    - name: postgres:18
      alias: postgres
  variables:
    POSTGRES_USER: pgflux
    POSTGRES_PASSWORD: pgflux
    POSTGRES_DB: testdb
    PGFLUX_DB: postgres://pgflux:pgflux@postgres:5432/testdb
  script:
    - go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
    - pg-flux migrate apply
    - |
      if pg-flux migrate generate | grep -q "Generated:"; then
        echo "Schema drift detected"; exit 1
      fi
  only:
    - merge_requests

deploy-production:
  stage: deploy
  image: golang:1.22
  environment: production
  when: manual
  script:
    - go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
    - pg-flux migrate apply --shadow-dsn "$SHADOW_DATABASE_URL" --shadow-semantic
  only:
    - main
```

---

## Exit Codes

Use these in shell scripts for conditional logic:

| Command | Exit 0 | Exit non-zero |
|---------|--------|---------------|
| `migrate generate` | No changes or migration written successfully | Blocking hazard / parse error / DB error |
| `migrate apply` | All pending migrations applied | At least one migration failed |
| `migrate status` | Always 0 | DB error |

**Detecting drift in scripts:**

```bash
output=$(pg-flux migrate generate 2>&1)
exit_code=$?

if [ $exit_code -ne 0 ]; then
  echo "Generation failed: $output"
  exit 1
fi

if echo "$output" | grep -q "Generated:"; then
  echo "Drift detected — unexpected schema changes:"
  cat migrations/$(ls -t migrations/ | head -1)
  exit 1
fi

echo "Schema is in sync."
```

---

## JSON Output for Scripting

Every command supports `--format json`:

```bash
pg-flux migrate status --format json
```

```json
[
  {"filename": "20260428_120000.sql", "applied": true,  "applied_at": "2026-04-28T12:00:01Z"},
  {"filename": "20260428_130000.sql", "applied": false, "applied_at": null}
]
```

```bash
# Count pending migrations
pg-flux migrate status --format json | jq '[.[] | select(.applied == false)] | length'
```

---

## Environment Variable Injection

Never hardcode database credentials in CI config files. Use your platform's secret store:

```bash
# GitHub Actions
PGFLUX_DB: ${{ secrets.DATABASE_URL }}

# GitLab CI
PGFLUX_DB: $DATABASE_URL  # set in project CI/CD variables (masked)

# Kubernetes / Helm
env:
  - name: PGFLUX_DB
    valueFrom:
      secretKeyRef:
        name: db-credentials
        key: url
```

---

## Kubernetes Init Container Pattern

For Kubernetes deployments, run pg-flux as an init container so migrations complete before the application starts:

```yaml
initContainers:
  - name: pg-flux-migrate
    image: ghcr.io/nex-gen-tech/pg-flux:latest
    command: ["pg-flux", "migrate", "apply"]
    env:
      - name: PGFLUX_DB
        valueFrom:
          secretKeyRef:
            name: app-db-secret
            key: url
    volumeMounts:
      - name: migrations
        mountPath: /app/migrations
        readOnly: true

volumes:
  - name: migrations
    configMap:
      name: app-migrations   # or use an init container that git-clones the repo
```
