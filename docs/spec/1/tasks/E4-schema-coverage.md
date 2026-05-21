# Epic 4 — Schema Object Coverage

**Priority:** P1 (type model) / P2 (partitions) — aligns with PRD v2 V2-A and V2-B milestones.
**Spec ref:** [Spec 1 §Open Gaps G5, G6](../spec.md)
**PRD ref:** [PRD v2 §3.3, FR2-01 – FR2-05](../../../apps/cli/docs/prd/PRD-v2-robustness.md)

---

## E4-T1 — First-class enum type model in SchemaState (V2-A)

**Summary:** `CREATE TYPE ... AS ENUM` is currently handled as `ExtraDDL` pass-through. The source loader deparses it (V2-A partial) but `SchemaState` has no typed entry for enums. The differ compares hash only — it cannot detect additive value changes, order changes, or drops with hazards. `verify` (after E2-T1) will flag unseen enums. The type needs to be a first-class modeled object.

**Scope:** Enums first. Composite types, domains, and range types are follow-on.

**Acceptance criteria:**
- `SchemaState` holds a typed `Enum` struct (name, schema, values in order).
- Inspector reads from `pg_type` / `pg_enum` for live enum state.
- Differ detects: new enum value (safe → `ALTER TYPE ... ADD VALUE`), renamed value (hazard if data exists), removed value (DATA_LOSS hazard).
- `verify` does not flag declared enums as undeclared (fixes G3 properly at the model level, not just scanner).
- `drift` does not false-positive on a schema that has only enum types added/unchanged.
- Unit tests for add-value, rename-value, drop-value diff paths.
- PRD v2 FR2-01, FR2-03, FR2-04 (enum subset) satisfied.

**Dependencies:** None (but benefits from E2-T1 being done first so verify coverage is tested together).

---

## E4-T2 — Loader completeness: GrantStmt and CommentStmt as ExtraDDL

**Summary:** `GRANT` and `COMMENT ON` statements in schema files are currently silently dropped by the source loader's `processRawStmt`. Per PRD v2 principle "no silent success for DDL the user thought was managed," these should either be routed to `ExtraDDL` (pass-through with stable identity) or fail closed with a clear error when `--strict-schema` is set.

**Acceptance criteria:**
- `GRANT` statements in schema files are preserved as `ExtraDDL` and included in the generated migration in dependency order.
- `COMMENT ON` statements are preserved as `ExtraDDL`.
- Neither silently disappears from the plan.
- `pg-flux plan` output lists them as `RAW_DDL` operations.
- PRD v2 FR2-01 (loader coverage) satisfied for these two node kinds.

**Dependencies:** None.

---

## E4-T3 — Partitioned table parents in inspector (V2-B)

**Summary:** Inspector queries include `relkind IN ('r', 'p')` (partial V2-B). However, child partition graph, `ATTACH PARTITION` / `DETACH PARTITION` ordering, and partition strategy changes (RANGE/LIST/HASH) are not modeled. A schema with partitioned tables will silently have its partition structure ignored by drift/verify.

**Acceptance criteria:**
- Inspector loads partitioned parent tables with their partition strategy and key column(s).
- `drift` detects: new partition added (safe), partition dropped (DATA_LOSS hazard), strategy changed (hazard).
- DAG includes type → partitioned table ordering.
- Integration test against a Docker PG with a partitioned table.
- PRD v2 FR2-02 satisfied.

**Dependencies:** E4-T1 (type → table DAG ordering should include partitioned parents).
