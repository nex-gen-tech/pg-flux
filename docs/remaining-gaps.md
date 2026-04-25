# Remaining Gaps (post-pass)

This file tracks **residual** product and NFR debt. Several former gaps now have **concrete implementations**; others stay open by design (equivalence proving, 100% graph coverage) or until dependencies move.

## Addressed in tree (where to look)


| Theme                           | What shipped                                                                                                                                                                                                                                                  |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Semantic shadow / second DB** | `shadow.ValidateSemanticApply` / `ValidateSemanticOnDatabase` — sequential **autocommit** apply on a disposable DSN (objects visible between steps). CLI: `--shadow-semantic` with `--shadow-dsn`. Still not a formal *equivalence* proof with production.    |
| **Syntax shadow**               | `ValidateSyntaxInTxn` / `ValidateSyntaxOnDatabase` — rolled-back transaction, unchanged.                                                                                                                                                                      |
| **80% coverage NFR**            | `scripts/coverage-nfr.sh` reports total `go test -coverpkg=./...` %; set `ENFORCE_COVERAGE=1` to fail below a threshold. Hitting **80%** still requires broad integration tests and cmd coverage.                                                             |
| **Dependency graph + cycles**   | `dag.TopologicalSortStatements` + `dag.ErrDependencyCycle` — edges from `statementProduces` / `statementRequires` (FK, FROM/JOIN, etc.). `TopoSort` delegates here. Heuristic: not every DDL kind is modeled; unknown refs may still fail only at apply time. |
| **Extensions (opt-in)**         | `schema.Extension.Version`; live `extversion`; desired `WITH VERSION` / `ALTER EXTENSION ... UPDATE TO` in SQL; diff emits `UPDATE_EXTENSION` (`ALTER EXTENSION name UPDATE TO 'version'`) when versions differ.                                              |
| **pg_query gaps**               | `docs/parser-limitations.md` — ENFORCED forms and version alignment with lib; upgrade path = bump `pg_query_go` in `go.mod`.                                                                                                                                  |


Wider product phases: `docs/prd/08-implementation-roadmap.md`. **Planned v2 scope** (types, loader honesty, partitions, execution, NFR): `docs/prd/PRD-v2-robustness.md`.

## Still open (by design or backlog)

- **Fully formal** equivalence of migrations with a desired model in *arbitrary* production state (reordering, partial apply, out-of-band DDL) — not a single check.
- **100%** global `go test -coverpkg=./...` (expensive; not an absolute product goal).
- **Exhaustive** object kinds in the DDL dependency graph (e.g. every `CREATE` variant, FDW, event triggers) — the graph is extended as needs arise; some refs still only fail at apply time.
- **Inspector** on exotic catalog combinations beyond what integration tests cover.

## Newly in tree (this iteration)


| Area                      | What shipped                                                                                                                                                                                                                                                                                                                                                                                                              |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **DDL graph**             | `pkg/dag`: `CREATE_INDEX` → depends on `ON` table; `CREATE_TRIGGER` → `ON` table + `EXECUTE {FUNCTION|PROCEDURE} name`; `registerProducers` adds a coarse **function** key `schema.name` (from `schema.FunctionDependencyKey`) so triggers order after the right `CREATE_FUNCTION`. `CONCURRENTLY` is handled via the same index-`ON` edge (topological order; `exec` still runs concurrent steps outside the first txn). `depgraph_ddl.go` adds heuristic edges for `CREATE TYPE` / `CREATE DOMAIN` (incl. on `RAW_DDL` first statement), composite attribute deps, domain `AS` and range `SUBTYPE` bases, schema-qualified column types, and qualified `RETURNS` (regex-based; not all PostgreSQL spellings). |
| **Semantic / structural** | `shadow.ValidateStructuralEquivalence`: semantic apply on disposable DSN + `inspector.Inspect` + `differ.Diff` vs desired; `plan` flag `--shadow-equivalence` (with `--shadow-dsn`). This is a **structural** empty-DB check, not a formal proof.                                                                                                                                                                         |
| **Integration**           | `TestE2E_ShadowEquivalence`, `TestInspector_RLSAndPolicies` (Docker, `PGFLUX_E2E=1`); inspector/RLS/policy paths.                                                                                                                                                                                                                                                                                                         |
| **Function key**          | `schema.FunctionDependencyKey` for trigger→function edges.                                                                                                                                                                                                                                                                                                                                                                |


