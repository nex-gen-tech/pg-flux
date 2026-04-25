# 02 — Personas & User Stories

**Document:** Target Audience, Personas, and User Stories
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## 2.1 Target Personas

### Persona 1: "The Backend Engineer" (Alex)
**Role:** Backend or Full-Stack Software Engineer at a growth-stage startup or mid-size company.
**Background:** Writes application code daily. Manages database schema changes as part of feature development. Knows SQL well but is not a DBA. Frustrated by the boilerplate of writing `ALTER TABLE` statements and the fear of accidentally dropping a column.

**Pain Points:**
- Has to manually write `ALTER TABLE ADD COLUMN`, `ALTER TABLE DROP COLUMN`, and `CREATE INDEX` for every change.
- Afraid to rename a column because no tool can do it safely without data loss.
- Not sure which of their index changes require `CONCURRENTLY` to avoid locking the table.

**Goals:**
- Update `schema.sql`, run one command, and get the right migration generated.
- Never lose data due to a misclassified rename.
- Have the tool warn them before they take the database down.

**pg-flux Value:** Automated, safe diff generation. Hint-based rename resolution. Automatic hazard warnings.

---

### Persona 2: "The DevOps / Platform Engineer" (Jordan)
**Role:** Infrastructure or DevOps engineer responsible for CI/CD pipelines and deployment reliability.
**Background:** Does not write application SQL daily. Cares deeply about deployment safety, auditability, and reliability. Needs tools that produce stable exit codes and structured output for automation.

**Pain Points:**
- No reliable way to detect schema drift in CI without running migration scripts and hoping they fail.
- Migration tools produce logs that are hard to parse programmatically.
- Cannot enforce "DBA approval" on risky migrations without custom tooling.

**Goals:**
- A single binary that can be dropped into a Docker image with no JVM or Python dependency.
- Deterministic exit codes: `0` means no drift, `1` means drift detected.
- Structured JSON output that can feed into Slack notifications or PR comments.

**pg-flux Value:** `engine drift` command with clean exit codes. `--format=json` flag. Single static binary for `linux/amd64` and `linux/arm64`.

---

### Persona 3: "The DBA" (Sam)
**Role:** Database Administrator or Senior Database Engineer at an enterprise or high-traffic platform.
**Background:** Expert-level PostgreSQL knowledge. Responsible for production stability, query performance, and data integrity. Reviews all schema changes before they hit production. Highly skeptical of automated tooling.

**Pain Points:**
- Has seen automated migration tools lock production tables for 20 minutes at 2pm on a Tuesday.
- Developers submit schema changes that bypass the DBA review process.
- After major schema changes, the query planner uses stale statistics and queries regress.

**Goals:**
- Every generated plan is auditable and shows exactly which DDL will run.
- Any operation that could lock a table or lose data is blocked unless explicitly approved.
- `ANALYZE` is automatically injected after significant structural changes.

**pg-flux Value:** Full execution plan preview (`engine plan`). Named Hazard system with required CLI acknowledgment. Automatic `ANALYZE` injection post-migration.

---

### Persona 4: "The AI/Platform Integrator" (Riley)
**Role:** AI Engineer or Platform Architect building AI-assisted development workflows.
**Background:** Building systems where AI agents assist with code review, code generation, or automated testing. Wants to hook schema changes into an AI validation loop.

**Pain Points:**
- LLMs cannot reliably inspect a live database; they need structured, serializable input.
- Existing migration tools produce unstructured text output that is hard for AI agents to consume.
- No way to ask an AI "is this migration logically correct for our business model?" before it runs.

**Goals:**
- Machine-readable output (JSON) of the exact diff and proposed DDL.
- Ability to pipe the diff into an AI agent for semantic review.
- Structured output that identifies hazard types, affected tables, and DDL operations.

**pg-flux Value:** `--dry-run --format=json` with full delta serialization. Named hazard types as structured fields.

---

## 2.2 User Stories

Each user story has a unique ID, persona, title, story, and acceptance criteria.

---

### US-01: Drift Detection in CI
**Persona:** Jordan (DevOps Engineer)
**Priority:** P0 (Must Have)

> *As a DevOps engineer, I want to run `engine drift` as a step in our GitHub Actions pipeline, so that it exits with code `1` when production schema doesn't match the repository schema, blocking deployments when drift is detected.*

**Acceptance Criteria:**
- [ ] `engine drift --db=$DATABASE_URL --schema=./schema` returns exit code `0` when live schema matches desired state.
- [ ] Returns exit code `1` when any structural difference (table, column, index, constraint, function, policy) is detected.
- [ ] When `--format=json`, outputs a machine-readable summary of differences including: object type, object name, change type (`ADD`, `DROP`, `MODIFY`).
- [ ] Supports `DATABASE_URL` environment variable so credentials are never passed as CLI arguments.
- [ ] Completes in under 5 seconds for databases with up to 500 tables.

---

### US-02: Safe Column Rename
**Persona:** Alex (Backend Engineer)
**Priority:** P0 (Must Have)

> *As a developer, I want to rename `users.name` to `users.full_name` in my `schema.sql` file using a rename hint, so the tool generates an `ALTER TABLE ... RENAME COLUMN` statement instead of a DROP/ADD pair that would delete all user names.*

**Acceptance Criteria:**
- [ ] The parser correctly extracts `-- @renamed from=<old_name>` annotations from column definitions.
- [ ] When a column is present in the file with a rename hint but absent in the live database, the differ generates `ALTER TABLE {table} RENAME COLUMN {old_name} TO {new_name}` — not a DROP/ADD pair.
- [ ] If the old column does not exist in the live database either, the tool reports an error: "Rename source column `{old_name}` does not exist in the live schema."
- [ ] The rename hint survives a round-trip through the hint extraction system (extracted and re-written without mutation).
- [ ] Table renames are supported using `-- @renamed from=<old_table_name>` on the `CREATE TABLE` statement.

---

### US-03: RLS Policy Update
**Persona:** Sam (DBA) & Alex (Engineer)
**Priority:** P0 (Must Have)

> *As a security engineer, I want to modify the `USING` clause of an RLS policy in my `.sql` file and have pg-flux generate the correct `ALTER POLICY` or `DROP/CREATE POLICY` statement, without false-positive diffs on every run.*

**Acceptance Criteria:**
- [ ] RLS policy `USING` and `WITH CHECK` expressions are parsed and normalized before comparison.
- [ ] Equivalent expressions (e.g., `(auth.uid() = user_id)` vs. `auth.uid() = user_id`) are treated as identical.
- [ ] When a `USING` expression changes semantically, the differ generates `DROP POLICY ... ON ...; CREATE POLICY ...` or `ALTER POLICY ... USING (...)`.
- [ ] `FOR` clause changes (e.g., `ALL` → `SELECT`) are detected and generate a full `DROP/CREATE`.
- [ ] `WITH CHECK` changes are detected and generate the correct DDL.
- [ ] No diff is generated for policies that have not changed.

---

### US-04: Hazard Prevention — Table Lock
**Persona:** Sam (DBA)
**Priority:** P0 (Must Have)

> *As a DBA, I want the tool to automatically rewrite `CREATE INDEX` to `CREATE INDEX CONCURRENTLY` and warn me with a named hazard if any index build cannot be made concurrent, so that no migration I apply takes an extended Access Exclusive lock on a large table.*

**Acceptance Criteria:**
- [ ] All `CREATE INDEX` statements in the generated plan are automatically rewritten to `CREATE INDEX CONCURRENTLY`.
- [ ] A `HazardType_TableLock` hazard is emitted for any index build that cannot use `CONCURRENTLY` (e.g., on expressions that require the full table to be available during build — this case should be documented but is rare).
- [ ] Concurrent index builds are validated by a shadow database before being included in the final plan.
- [ ] The `--allow-hazards TABLE_LOCK` flag allows the operator to explicitly acknowledge and proceed.
- [ ] The `engine plan` output clearly labels each statement with its hazard type and a plain-English description.

---

### US-05: Hazard Prevention — Data Loss
**Persona:** Sam (DBA)
**Priority:** P0 (Must Have)

> *As a DBA, I want the tool to block any `DROP TABLE` or `DROP COLUMN` operation unless I pass an explicit `--allow-hazards DATA_LOSS` flag, so that no accidental schema deletion happens in a deployment.*

**Acceptance Criteria:**
- [ ] Any `DROP TABLE` in the generated plan emits a `HazardType_DataLoss` hazard and blocks execution.
- [ ] Any `DROP COLUMN` in the generated plan emits a `HazardType_DataLoss` hazard and blocks execution.
- [ ] Passing `--allow-hazards DATA_LOSS` allows execution to proceed.
- [ ] The hazard message includes the affected table and column name and an estimated row count (queried from `pg_class.reltuples`) to help the operator assess impact.
- [ ] Cascading drops (e.g., dropping a table that has dependent views or foreign keys) enumerate all cascade targets in the hazard message.

---

### US-06: Hazard Prevention — Constraint Scan
**Persona:** Sam (DBA)
**Priority:** P1 (Should Have)

> *As a DBA, I want the tool to automatically use the `NOT VALID` multi-step constraint validation pattern when adding `NOT NULL` or `CHECK` constraints to populated tables, so that no full table scan blocks production traffic.*

**Acceptance Criteria:**
- [ ] When adding a `NOT NULL` constraint to a column on a table with data, the tool generates the 4-step pattern:
  1. `ADD CONSTRAINT ... CHECK (col IS NOT NULL) NOT VALID`
  2. `VALIDATE CONSTRAINT ...`
  3. `ALTER COLUMN ... SET NOT NULL`
  4. `DROP CONSTRAINT ...`
- [ ] When adding a `CHECK` constraint to a table with data, the tool generates:
  1. `ADD CONSTRAINT ... CHECK (...) NOT VALID`
  2. `VALIDATE CONSTRAINT ...`
- [ ] `HazardType_ConstraintScan` is emitted if the two-step pattern is not applicable for a specific constraint type.
- [ ] `--allow-hazards CONSTRAINT_SCAN` bypasses this protection.
- [ ] The generated intermediate constraint names are deterministic (based on a hash of table + constraint) to be idempotent on retry.

---

### US-07: Fresh Schema Initialization
**Persona:** Alex (Backend Engineer)
**Priority:** P0 (Must Have)

> *As a developer starting a new project, I want to run `engine init` to get a best-practice schema directory scaffold, so I can start defining my schema immediately without needing to understand the tooling internals.*

**Acceptance Criteria:**
- [ ] `engine init --dir=./schema` creates the directory if it doesn't exist.
- [ ] Creates a `schema/.pg-flux.yml` configuration file with sensible defaults.
- [ ] Creates a `schema/tables/` subdirectory with an example `example_users.sql` file.
- [ ] Creates a `schema/functions/`, `schema/policies/`, and `schema/indexes/` directory for organization.
- [ ] The example files contain valid PostgreSQL 18 DDL including a `uuidv7()` default.
- [ ] The tool prints a getting-started guide to stdout after initialization.

---

### US-08: Live DB Inspection / Reverse Engineering
**Persona:** Alex & Jordan
**Priority:** P1 (Should Have)

> *As a developer onboarding onto an existing project, I want to run `engine inspect` against the production database to generate the current schema as `.sql` files, so I can start using pg-flux without rewriting the entire schema from scratch.*

**Acceptance Criteria:**
- [ ] `engine inspect --db=$DATABASE_URL --out=./schema` generates normalized `.sql` files in the output directory.
- [ ] All tables, columns, constraints, indexes, functions, triggers, and RLS policies are captured.
- [ ] The generated files are valid input for `engine plan` and `engine drift`.
- [ ] Column ordering in generated files matches `pg_attribute.attnum` ordering.
- [ ] System tables (e.g., `pg_*`, `information_schema.*`) are excluded.
- [ ] Generated files include hint comments for objects that have non-obvious configurations (e.g., partial indexes, expression indexes).

---

### US-09: Atomic Transactional Apply
**Persona:** Jordan (DevOps) & Sam (DBA)
**Priority:** P0 (Must Have)

> *As a DevOps engineer, I want all non-concurrent DDL statements in a migration plan to be executed within a single transaction, so that if any statement fails, the entire migration is rolled back and the database is left in a consistent state.*

**Acceptance Criteria:**
- [ ] Non-concurrent statements are wrapped in `BEGIN; ... COMMIT;`.
- [ ] Concurrent statements (e.g., `CREATE INDEX CONCURRENTLY`) are executed outside the transaction, as required by PostgreSQL.
- [ ] If a concurrent statement fails, the tool reports the exact failure, the current dirty state of the schema, and the remaining steps that need to be applied.
- [ ] A `--dry-run` flag runs the full plan validation without executing any DDL.
- [ ] Advisory lock `pg_try_advisory_lock` is acquired at the start of `engine apply` to prevent concurrent migrations on the same database.

---

### US-10: AI-Assisted Schema Review
**Persona:** Riley (AI Integrator)
**Priority:** P2 (Nice to Have)

> *As an AI engineer, I want `engine plan --format=json` to emit a complete, structured representation of the proposed migration, so I can pipe it to a review agent that checks for business logic regressions before the migration is applied.*

**Acceptance Criteria:**
- [ ] JSON output includes: `version`, `generated_at`, `source_schema_hash`, `live_schema_hash`, `hazards[]`, `statements[]`.
- [ ] Each statement in `statements[]` includes: `ddl`, `object_type`, `object_name`, `operation_type`, `hazards[]`, `is_concurrent`, `estimated_lock_duration_ms`.
- [ ] The JSON schema is versioned and documented.
- [ ] The tool exits with code `0` when `--format=json` is used (even if hazards exist), enabling non-blocking pipeline steps.
- [ ] A `--schema-only` flag outputs just the computed diff without DDL, for use in documentation generation.
