# PRD v2 — Robustness & Completeness

**Project:** pg-flux  
**Version:** 2.0 (planning)  
**Status:** Draft  
**Supersedes in intent:** v1.0 PRD scope for *completeness*; v1 documents remain the baseline for shipped behavior until v2 milestones land.

**Related:** [README](./README.md), [11-edge-cases-and-known-limitations](./11-edge-cases-and-known-limitations.md), [remaining-gaps](../remaining-gaps.md), [parser-limitations](../parser-limitations.md).

---

## 1. Purpose

PRD v2 defines what is required for pg-flux to be **robust** in real production settings: the **declared schema in Git** and the **live database** must be represented without silent loss, the **diff and plan** must cover the object classes teams rely on, **ordering** must be sound for those classes, and **validation** must reduce false confidence.

v1 delivered a strong core (heap tables, constraints, many secondary objects, hazards, shadow checks). v2 closes **honesty gaps** (silently ignored SQL, incomplete catalog coverage, heuristic-only ordering) and **operational gaps** (batching, observability, enforcement).

---

## 2. North-Star Outcomes (v2)

| # | Outcome | Measurable signal |
|---|---------|---------------------|
| N1 | **No silent drop** of valid top-level DDL in schema files | Every parseable `RawStmt` is either modeled in `SchemaState`, emitted as explicit `RAW_DDL` with stable identity, or rejected with file+line error |
| N2 | **Inspector parity** for supported object classes | Documented `relkind` / catalog coverage; partitioned parents, or explicit exclusion with drift warning |
| N3 | **Dependency soundness** for modeled objects | DAG or validated ordering covers types → tables → dependent objects for v2 scope; documented limits for the rest |
| N4 | **False confidence reduced** | Structural / semantic validation paths documented; optional coverage gate in CI |
| N5 | **Operable at scale** | Batching, lock strategy, and failure recovery documented and testable (not a single monolithic transaction for all production DDL) |

---

## 3. v1 Gaps v2 Must Address (honest inventory)

### 3.1 Source loader (`pkg/src`)

- **Gap:** `processRawStmt` handles only a small set of AST node types; other statements parse successfully but **do not** populate `SchemaState` (e.g. `CREATE TYPE`, `CREATE DOMAIN`, `CREATE SCHEMA`, `GRANT`, many others).
- **v2 requirement:** **Stmt coverage policy**: either (a) first-class model + differ, (b) ordered `ExtraDDL` / `RAW_DDL` with **deparse+identity**, or (c) **fail closed** for unsupported statements when `--strict-schema` (name TBD) is set.

### 3.2 Inspector (`pkg/inspector`)

- **Gap:** Table load uses `relkind = 'r'` only; **partitioned table parents** (`relkind = 'p'`) are not loaded as tables.
- **Gap:** Foreign tables, some composite replication objects, and “exotic” catalogs are out of scope or partial.
- **v2 requirement:** **Explicit** support or **explicit** exclusion with warnings in plan/drift output.

### 3.3 Schema model (`pkg/schema`)

- **Gap:** No first-class **types** (enum, composite, range, domain), **casts**, **rules**, **publications** / **subscriptions**, **FDW** graph, **comments** (if product wants them), **owner/privilege** model.
- **v2 requirement:** Expand `SchemaState` in phases; **hash/drift** must include new fields or intentionally exclude with documentation.

### 3.4 Differ (`pkg/differ`)

- **Gap:** Diff only exists for object types in the model; “invisible” objects never drift.
- **v2 requirement:** For each supported class: **create/alter/drop** semantics, fingerprint strategy, and hazard rules.

### 3.5 DAG (`pkg/dag`)

- **Gap:** Regex/heuristic ordering; **quoted identifiers** and complex dependencies fail edge cases; not all `CREATE` kinds have producers/consumers.
- **v2 requirement:** Tighten with **libpg_query**-assisted reference extraction where feasible; **tests** for quoted idents; **document** irreducible limits.

### 3.6 Execution (`pkg/exec`)

- **Gap:** All non-`CONCURRENT` DDL in **one transaction** — long lock, all-or-nothing.
- **v2 requirement:** **Phased apply**: batches, optional per-statement transactions, savepoint policy — product decision with clear defaults.

### 3.7 Shadow / equivalence (`pkg/shadow`)

- **Gap:** Validates on **empty** disposable DB; no data-dependent proofs.
- **v2 requirement:** **Optional** “contentful shadow” (fixtures) for constraint validation; keep **structural** equivalence as default; document limits.

### 3.8 Parser & platform

- **Gap:** `pg_query` major version vs **target server** version skew.
- **Gap:** **CGO** / native — Windows story weak.
- **v2 requirement:** **Version support matrix** in docs; optional pure-Go or alternate driver path *deferred* unless NFR demands it; CI on target platforms.

---

## 4. v2 Scope

### 4.1 In scope (candidate backlog)

| Area | Items |
|------|--------|
| **Types & domains** | `CREATE TYPE` (enum, composite, range), `CREATE DOMAIN`; inspector from `pg_type` / `pg_enum` / typbasetype; differ for additive enum values, type alterations with hazards |
| **Loader completeness** | `DefineStmt` and other high-traffic nodes routed to model or `RAW_DDL` band |
| **Partitioning** | Inspect `relkind` in `('r','p')` (or policy-driven); parent/child relationship in model; `ATTACH`/`DETACH` ordering with DAG |
| **Inheritance** | Model `INHERITS` / `pg_inherits` for ordering warnings at minimum |
| **Privileges (optional phase)** | `GRANT`/`REVOKE` as modeled objects or as ordered pass-through with diff |
| **FDW (minimal)** | Optional: foreign **server+mapping** in Misc + ordering; full FDW out if cost too high |
| **DAG** | Quoted ident tests; `pg_query` walk for `depends on` where regex insufficient |
| **Execution** | Transaction boundaries, idempotency notes, `lock_timeout` / `statement_timeout` per batch |
| **NFR** | `ENFORCE_COVERAGE` / threshold in CI; performance budget for `inspect` on large catalogs |

### 4.2 Out of scope (v2 unless reprioritized)

- Full **logical replication** (publications, subscriptions) production lifecycle
- **Event triggers** (unless minimal ordering hooks)
- **pg_cron**, **extensions** not already covered
- **100%** PostgreSQL surface area
- Automatic **zero-downtime rewrites** for all lock-heavy alters (advisory automation stays incremental)

### 4.3 Principles

- **No silent success** for DDL the user thought was managed.
- **Every exclusion is user-visible** (flag, log line, or plan annotation).
- **Phased delivery** over one big-bang; each phase is shippable.

---

## 5. Phased delivery (indicative)

| Phase | Theme | Exit criteria |
|-------|--------|----------------|
| **V2-A** | **Loader honesty + types baseline** | `CREATE TYPE`/`CREATE DOMAIN` in model from files; enum/composite/range in inspector; `RAW_DDL` policy for unmodeled nodes is explicit; **tests** for “no silent drop” |
| **V2-B** | **Partitioned tables + graph** | Parents in inspect; DAG edges for type/table/partition; regression tests |
| **V2-C** | **Differ depth** | Type alter/drop hazards; domain changes; enum remove strategy (hazard + manual path documented) |
| **V2-D** | **Execution + ops** | Batched apply with documented semantics; better lock/timeout controls |
| **V2-E (optional)** | **Privileges + FDW minimum** | As customer-driven |

Dependencies: **V2-A** unblocks honest diffs for typified schemas; **V2-B** is critical for partition-heavy users; **D–E** parallelizable after A.

---

## 6. Functional requirements (v2 IDs)

> Checkbox tracking for implementation; wording is acceptance-oriented.

### FR2-01 — Loader coverage (P0)

- [ ] Authoritative list of **supported** `RawStmt` node kinds; all others: `ExtraDDL`, `MiscObject`, or error (configurable).
- [ ] `DefineStmt` (`CREATE TYPE`, etc.) **never** silently ignored when present in schema dir.

### FR2-02 — Inspector: heap + partitioned parents (P0)

- [ ] Query policy includes `relkind` ∈ `('r', 'p')` **or** documented filter with **warning** when `p` is skipped.
- [ ] Integration test against Docker PG with a partitioned parent.

### FR2-03 — SchemaState: types (P0)

- [ ] `SchemaState` (or sub-struct) holds user-defined types with fingerprint inputs compatible with `differ` and `hashstate`.

### FR2-04 — Differ: types and domains (P0)

- [ ] Additive changes generate safe DDL; destructive changes emit **hazards**; enum value removal matches v1 policy or better.

### FR2-05 — DAG: soundness (P1)

- [ ] Type-before-table ordering for modeled type deps; **quoted** identifier test cases; failure mode = cycle error, not apply-time surprise where avoidable.

### FR2-06 — Execution: batching (P1)

- [ ] **Configurable** transaction strategy; default documented; **no** hidden single-txn for all DDL without opt-out path for large plans.

### FR2-07 — Shadow / validation (P1)

- [ ] `ValidateStructuralEquivalence` **documents** that empty DB is assumed; optional extended validation tracked as follow-up.
- [ ] If coverage gate: `scripts/coverage-nfr.sh` or CI threshold tied to v2 NFR.

### FR2-08 — Parser alignment (P0)

- [ ] `go.mod` `pg_query_go` / lib version **documented** against **supported** server minors; upgrade playbook.

### FR2-09 — Platform (P2)

- [ ] CI matrix reflects **supported** OS/arch; Windows stance explicit (supported vs best-effort vs unsupported).

---

## 7. Non-functional requirements (v2)

| ID | Requirement | Target |
|----|-------------|--------|
| NFR2-1 | `inspect` on 10k relations | Completes under documented timeout with concurrency |
| NFR2-2 | Plan size | `plan` output streams or caps with guidance |
| NFR2-3 | Test coverage | Enforceable threshold on critical packages (`differ`, `inspector`, `src`, `dag`) when `ENFORCE_COVERAGE=1` |
| NFR2-4 | Observability | Structured log fields: phase, object class, statement id, duration |
| NFR2-5 | Security | Revisit [12-security-considerations](./12-security-considerations.md) for new surfaces (types, grants) |

---

## 8. Risk register (v2-specific)

| Risk | Mitigation |
|------|------------|
| “Complete” model explodes in complexity | **Phased** FRs; **Misc** + warnings for long tail |
| `pg_query` / server skew | **Matrix**, integration tests, parse-on-load errors |
| DAG false negatives | **Tests** + **apply** errors acceptable only when **documented**; optional `--validate-apply` |
| Batching breaks atomicity | **Hazard** and **idempotency** documentation; user choice |

---

## 9. Testing strategy (delta from v1)

- **Property-style:** loader must not drop statements (golden tests per node kind).
- **Catalog fixtures:** Docker with partitioned sets, custom types, domains.
- **Regression:** every new DAG rule has **positive and negative** ordering tests.
- **E2E:** `PGFLUX_E2E=1` extended for v2 milestones.

---

## 10. Document control

- **Author:** product + engineering (repo).  
- **Review cadence:** quarterly or each phase exit.  
- **Traceability:** v2 FR IDs should appear in issues/PR titles (`FR2-0x`).

When v2-A ships, update [README](./README.md) “Version” and add a **Changelog** entry; consider deprecating duplicate claims in v1 FRs that contradict implemented behavior (parser “all PG18 DDL” vs loader dispatch).

---

## 11. Implementation status (this repository)

Snapshot of v2 work **landed in tree** vs still planned. Update this section when closing milestones.

| Milestone / FR | Status | Notes |
|----------------|--------|--------|
| **V2-A** Loader: type/schema DDL not silent | **Partial** | `processExtraNode` in `pkg/src/extra_stmt.go` deparses `DefineStmt`, `CreateDomainStmt`, `CompositeTypeStmt`, `CreateEnumStmt`, `CreateSchemaStmt`, `AlterTypeStmt` into `ExtraDDL` (plan as `RAW_DDL` / `ChangeRawSQL`). Unit coverage: `pkg/src/coverage_load_test.go` asserts `CREATE TYPE … AS ENUM` appears in `ExtraDDL`. |
| **V2-A** First-class `SchemaState` types + differ | **Not done** | Types are pass-through, not structurally diffed against `pg_type`. |
| **V2-B** Inspector: partitioned parents | **Partial** | `loadTablesMap` includes `relkind IN ('r','p')`; `Reltuples` same. No separate partition-child graph or strategy diff. |
| **V2-C** Differ: type alter/drop hazards | **Not done** | |
| **V2-D** Execution batching / txn strategy | **Not done** | `pkg/exec/apply.go` still one txn for non-concurrent batch. |
| **V2** DAG: quoted idents, full type edges | **Partial** | `depgraph_ddl.go` heuristics; quotes still weak. |
| **NFR2-3** Coverage gate in CI | **Optional** | `scripts/coverage-nfr.sh` + `ENFORCE_COVERAGE` exist; not wired to required CI by default. |
| **FR2-08** Parser/server matrix doc | **Partial** | [parser-limitations.md](../parser-limitations.md) + `go.mod` pin; no formal matrix table. |

**Caveat — `ExtraDDL` ordering:** pass-through statements follow **differ** ordering (`sortChanges` + `buildStatements`), not strictly file order for all change kinds. For types that must run before `CREATE TABLE` in the same desired state, prefer **types in separate files listed first** in the schema dir **and** validate with `plan` / shadow apply. Full **type → table** DAG for mixed modeled + `ExtraDDL` is a **remaining** item.

---

## 12. Remaining backlog (explicit)

Work **not** completed in this pass; do not treat as implemented until issues close.

1. **Type model** — `schema.SchemaState` entries for enums/composites/domains; inspector from catalogs; `differ` beyond `ExtraDDL` pass-through.  
2. **Strict mode** — optional `--strict-schema` (TBD) failing on *any* unhandled `RawStmt` node kind.  
3. **Loader coverage** — remaining node kinds (e.g. `GrantStmt`, `CommentStmt`, `CreateCastStmt`, …) → `ExtraDDL`, `MiscObject`, or error.  
4. **Partitioning** — child partitions, `ATTACH`/`DETACH` ordering vs parents, strategy changes with hazards.  
5. **Execution** — batching, savepoints, per-statement `lock_timeout` / `statement_timeout`.  
6. **CI matrix** — `pg_query` / server version; OS matrix per NFR2-9.  
7. **Formal PRD errata** — v1 [04-functional-requirements](./04-functional-requirements.md) claims vs implementation (update or watermark “aspirational”).

---

## 13. Summary

**PRD v2** is not a rewrite promise—it is a **roadmap to honesty and robustness**: model what you claim to manage, inspect what you claim to diff, order what you claim to run safely, and prove what you claim in CI—without silently ignoring real-world PostgreSQL. **Section 11** records what the repo already does; **section 12** is the honest “still to build” list.
