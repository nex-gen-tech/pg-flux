# 10 — Testing Strategy

**Document:** Testing Strategy & Quality Assurance
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## Overview

pg-flux's testing strategy has four layers:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Test Pyramid                                 │
│                                                                  │
│         ┌──────────────────────┐                                 │
│         │  Shadow DB Validation │  ← End-to-end (PG18 container)│
│         └──────────────────────┘                                 │
│        ┌────────────────────────────┐                            │
│        │   Integration Tests        │  ← Real PG18 container    │
│        └────────────────────────────┘                            │
│       ┌────────────────────────────────┐                         │
│       │   Unit Tests + Fuzz Tests      │  ← In-process, fast    │
│       └────────────────────────────────┘                         │
└─────────────────────────────────────────────────────────────────┘
```

---

## 10.1 Unit Tests

### What to Unit-Test

| Package | Key Test Cases |
|---------|---------------|
| `pkg/src` (parser) | Parse each DDL construct; compare output structs; error cases |
| `pkg/differ` | Table/column/index/constraint diff logic for every change type |
| `pkg/differ/rename` | Rename resolution: valid hints, invalid hints, circular renames, no-op |
| `pkg/dag` | Topological sort: correct ordering, cycle detection |
| `pkg/hazard` | Each hazard rule: trigger condition, auto-rewrite generation, flag check |
| `pkg/inspector` | Catalog query parsing: mock pgx rows, verify struct population |
| `pkg/exec` | Statement ordering, transaction grouping, concurrent vs. transactional split |

### Test Conventions

```go
// pkg/differ/column_test.go
func TestDifferColumnAdded(t *testing.T) {
    desired := &SchemaState{
        Tables: map[string]*Table{
            "public.users": {
                Columns: []*Column{
                    {Name: "id", DataType: "uuid"},
                    {Name: "email", DataType: "text"},  // new
                },
            },
        },
    }
    live := &SchemaState{
        Tables: map[string]*Table{
            "public.users": {
                Columns: []*Column{
                    {Name: "id", DataType: "uuid"},
                },
            },
        },
    }

    plan, err := Diff(desired, live)
    require.NoError(t, err)
    require.Len(t, plan.Statements, 1)
    assert.Equal(t, "ALTER TABLE public.users ADD COLUMN email text", plan.Statements[0].DDL)
    assert.Equal(t, OpAddColumn, plan.Statements[0].OpType)
}
```

### Coverage Target
- ≥ 80% line coverage across all packages.
- 100% coverage of all hazard detection rules.
- 100% coverage of the rename resolution algorithm.

### Running Tests
```bash
go test ./... -count=1 -race
```

The `-race` flag enables Go's data race detector. All tests must pass with `-race`.

---

## 10.2 Fuzz Tests

Fuzz testing is applied to the most security-critical and correctness-critical code paths.

### Fuzzing Targets

```go
// pkg/src/fuzz_test.go
func FuzzParse(f *testing.F) {
    // Seed corpus: known valid SQL
    f.Add("CREATE TABLE users (id uuid PRIMARY KEY, name text NOT NULL)")
    f.Add("CREATE TABLE t (id int, CONSTRAINT pk PRIMARY KEY (id))")

    f.Fuzz(func(t *testing.T, sql string) {
        // Must not panic
        _, _ = ParseSchema(sql)
    })
}
```

```go
// pkg/differ/fuzz_test.go
func FuzzDiff(f *testing.F) {
    // Must not panic or produce invalid SQL for any valid SchemaState pair
    f.Fuzz(func(t *testing.T, tableCount uint8, addRatio float32) {
        desired := generateRandomSchemaState(int(tableCount))
        live := mutateSchemaState(desired, addRatio)
        plan, err := Diff(desired, live)
        if err == nil {
            // All generated SQL must be parseable
            for _, stmt := range plan.Statements {
                _, parseErr := pg_query.Parse(stmt.DDL)
                assert.NoError(t, parseErr, "Generated DDL is invalid: %s", stmt.DDL)
            }
        }
    })
}
```

**Goal:** The parser and differ must NEVER panic on any input, even malformed input.

---

## 10.3 Integration Tests

Integration tests run against a real PostgreSQL 18 database in Docker.

### Infrastructure

```go
// pkg/testutil/pg18.go
func NewPG18Container(t *testing.T) *postgres.PostgresContainer {
    ctx := context.Background()
    container, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:18"),
        postgres.WithDatabase("testdb"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
        testcontainers.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).WithStartupTimeout(60*time.Second)),
    )
    require.NoError(t, err)
    t.Cleanup(func() { container.Terminate(ctx) })
    return container
}
```

### Integration Test Scenarios

Each scenario follows the pattern:
1. Apply the "before" schema to the test database.
2. Run `pg-flux plan --schema <after-schema> --db <test-db>`.
3. Assert the plan contains exactly the expected statements in the expected order.
4. Run `pg-flux apply` and verify the result.
5. Run `pg-flux drift` and assert exit code 0 (no drift).

**Core Scenario List:**

| ID | Scenario | Expected Statements |
|----|----------|---------------------|
| IT-01 | Add column (nullable) | 1x ALTER TABLE ADD COLUMN |
| IT-02 | Add column (NOT NULL with default) | 1x ALTER TABLE ADD COLUMN |
| IT-03 | Add column (NOT NULL, no default) | Hazard: CONSTRAINT_SCAN; 4-step NOT NULL pattern |
| IT-04 | Drop column | Hazard: DATA_LOSS; 1x ALTER TABLE DROP COLUMN |
| IT-05 | Rename column | 1x ALTER TABLE RENAME COLUMN |
| IT-06 | Change column type (safe cast, e.g. int → bigint) | 1x ALTER TABLE ALTER COLUMN TYPE |
| IT-07 | Change column type (unsafe cast) | Hazard: COLUMN_TYPE_CHANGE |
| IT-08 | Add table | 1x CREATE TABLE |
| IT-09 | Drop table | Hazard: DATA_LOSS; 1x DROP TABLE |
| IT-10 | Rename table | 1x ALTER TABLE RENAME TO |
| IT-11 | Add PRIMARY KEY (new table) | 1x CREATE TABLE with PK |
| IT-12 | Add PRIMARY KEY (existing table) | DROP old PK + ADD CONSTRAINT pk |
| IT-13 | Add UNIQUE constraint | 1x ALTER TABLE ADD CONSTRAINT UNIQUE |
| IT-14 | Add FK (non-valid + validate) | ADD CONSTRAINT FK NOT VALID + VALIDATE |
| IT-15 | Drop FK | 1x ALTER TABLE DROP CONSTRAINT |
| IT-16 | Add CHECK constraint | ADD CONSTRAINT CHECK NOT VALID + VALIDATE |
| IT-17 | Add index (new) | 1x CREATE INDEX CONCURRENTLY |
| IT-18 | Drop index | 1x DROP INDEX CONCURRENTLY |
| IT-19 | Modify index (expression change) | DROP CONCURRENTLY + CREATE CONCURRENTLY |
| IT-20 | Add partial index (with WHERE) | 1x CREATE INDEX CONCURRENTLY ... WHERE |
| IT-21 | Add GIN index | 1x CREATE INDEX CONCURRENTLY USING gin |
| IT-22 | Add function | 1x CREATE OR REPLACE FUNCTION |
| IT-23 | Modify function body | 1x CREATE OR REPLACE FUNCTION |
| IT-24 | Drop function | 1x DROP FUNCTION |
| IT-25 | Add trigger | 1x CREATE TRIGGER |
| IT-26 | Drop trigger | 1x DROP TRIGGER |
| IT-27 | Add RLS policy | 1x CREATE POLICY |
| IT-28 | Modify RLS USING expression | 1x DROP POLICY + CREATE POLICY |
| IT-29 | Enable RLS on table | 1x ALTER TABLE ENABLE ROW LEVEL SECURITY |
| IT-30 | No changes (idempotent) | 0 statements, exit code 0 |

**PG18-Specific Scenarios:**

| ID | Scenario | Expected Behavior |
|----|----------|------------------|
| IT-31 | Column default `uuidv7()` | Correctly parsed, no false-positive diff |
| IT-32 | Named NOT NULL constraint (PG18) | Detected from `pg_constraint type='n'` |
| IT-33 | Virtual generated column | `attgenerated='v'` detected; diff on expression change |
| IT-34 | Temporal PK (`WITHOUT OVERLAPS`) | Detected from `pg_constraint.conperiod` |
| IT-35 | Non-enforced FK (`NOT ENFORCED`) | `conenforced=false` detected; change from NOT ENFORCED → ENFORCED triggers CONSTRAINT_SCAN |

---

## 10.4 Shadow Database Validation

The shadow database validation is the final gate before `pg-flux apply` executes against the live database.

### Process

```
Live DB  ─── Clone Schema ──► Shadow DB
                                   │
Source Schema ────────────────► Differ
                                   │
                              Apply Plan
                                   │
                         Inspect Shadow DB
                                   │
                    Compare shadow to Source Schema
                                   │
                     ✓ Match? ──── proceed to live apply
                     ✗ No Match? ── error, do not apply
```

### Implementation

```go
func ValidatePlan(ctx context.Context, plan *Plan, sourceState *SchemaState, conn *pgx.Conn) error {
    shadowDB := fmt.Sprintf("pgflux_shadow_%d_%s", time.Now().UnixMilli(), uuid.New())

    _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE template0", shadowDB))
    if err != nil { return err }

    defer conn.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", shadowDB))

    // Connect to shadow DB
    shadowConn, err := pgx.Connect(ctx, shadowConnString(conn, shadowDB))
    if err != nil { return err }
    defer shadowConn.Close(ctx)

    // Clone current live schema into shadow DB
    if err := cloneSchemaSQLInto(ctx, shadowConn, plan.LiveState); err != nil {
        return fmt.Errorf("shadow clone failed: %w", err)
    }

    // Apply the plan to shadow DB
    if err := applyPlan(ctx, shadowConn, plan); err != nil {
        return fmt.Errorf("plan failed on shadow DB: %w", err)
    }

    // Inspect the shadow DB
    postState, err := InspectSchema(ctx, shadowConn, plan.TargetSchemas)
    if err != nil { return err }

    // Compare shadow result to source schema
    residualDiff, err := Diff(sourceState, postState)
    if err != nil { return err }

    if len(residualDiff.Statements) > 0 {
        return fmt.Errorf("shadow validation failed: %d residual differences after applying plan",
            len(residualDiff.Statements))
    }

    return nil
}
```

---

## 10.5 CI/CD Pipeline

```yaml
# .github/workflows/ci.yml
name: CI

on: [push, pull_request]

jobs:
  test:
    strategy:
      matrix:
        pg_version: ["17", "18"]
        go_version: ["1.22", "1.23"]
        os: [ubuntu-latest, macos-latest]

    runs-on: ${{ matrix.os }}

    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go_version }}

      - name: Setup Docker (for testcontainers)
        if: runner.os == 'Linux'
        run: sudo apt-get install -y docker.io

      - name: Run unit tests
        run: go test ./... -count=1 -race -short

      - name: Run integration tests (PG${{ matrix.pg_version }})
        env:
          PG_VERSION: ${{ matrix.pg_version }}
        run: go test ./... -count=1 -race -run Integration -timeout 10m

      - name: Run fuzz tests (30s budget)
        run: |
          go test -fuzz FuzzParse -fuzztime 30s ./pkg/src/
          go test -fuzz FuzzDiff -fuzztime 30s ./pkg/differ/

      - name: Check coverage
        run: |
          go test ./... -coverprofile=coverage.out
          go tool cover -func=coverage.out | grep total | awk '{if ($3 < "80.0%") exit 1}'
```

---

## 10.6 Regression Test Corpus

A corpus of 100+ known schema migration scenarios is maintained in `testdata/scenarios/`. Each scenario is a directory containing:

```
testdata/scenarios/
├── add_column_nullable/
│   ├── before.sql         ← Live schema before migration
│   ├── after.sql          ← Desired schema
│   ├── expected_plan.sql  ← Expected DDL output
│   └── scenario.yaml      ← Metadata (pg_version, hazards, description)
├── rename_column/
│   ├── before.sql
│   ├── after.sql          ← Contains @renamed hint
│   ├── expected_plan.sql
│   └── scenario.yaml
└── ...
```

The scenario test runner:
1. Applies `before.sql` to a test DB.
2. Runs pg-flux plan with `after.sql`.
3. Compares output to `expected_plan.sql` (modulo whitespace normalization).
4. If `scenario.yaml` includes `hazards:`, verifies they are detected.

**Adding new scenarios:** When a bug is reported, a regression scenario is always added to the corpus before the fix, following the red-green-refactor TDD pattern.
