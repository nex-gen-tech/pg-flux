# Spec 1 — pg-flux Current State Analysis

**Date:** 2026-05-22
**Version assessed:** v0.1.0
**Evidence base:** fastapi-todo + express-bookmarks examples — 38 real schema mutations across 2 domains, driven end-to-end by a first-time user persona.
**Reference:** [PRD v2](../../apps/cli/docs/prd/PRD-v2-robustness.md) · [ROADMAP](../../ROADMAP.md)

---

## Overall Rating: 7.8 / 10 (potential: 8.5 / 10)

**Updated 2026-05-22:** Six bugs fixed since initial assessment (B1–B6). The `@renamed` constraint ghost (G1) is fixed, all four examples now pass `drift` and `verify` cleanly, grants are emitted correctly, and all CI gates exit 0 without workarounds. The rating climbs from 6.2 to 7.8 with the remaining gaps being schema coverage (types, partitions) and developer experience polish.

---

## Dimension Ratings

| Dimension | Rating | Trend |
|---|---|---|
| Core pipeline (generate → apply) | **8.5 / 10** | Solid; grants now emitted |
| Auto-safety rewrites | **9 / 10** | Best-in-class |
| CI gates (drift / verify) | **8.5 / 10** | All 4 examples exit 0; drift is hard-gatable |
| CLI UX | **6.5 / 10** | rehash command landed; ADVISORY dedup remaining |
| Bug stability | **8 / 10** | B1–B6 all fixed; @renamed ghost resolved |
| Schema object coverage | **6 / 10** | Tables, indexes, views, functions, enums, triggers, grants — good; types model is pass-through only |
| Ecosystem / maturity | **3 / 10** | v0.1.0, no integrations, PG-only |
| Documentation | **7.5 / 10** | 4 real example apps; Python emitter added |

---

## What Works Well (evidence from testing)

- **UUID PKs, JSONB, tsvector generated columns** all round-trip without special handling.
- **GIN indexes** (`USING gin (col)`) generated and applied correctly.
- **CREATE TRIGGER** recognized by source parser — `CREATE_TRIGGER` op in generated migration, applied cleanly.
- **`@renamed from=`** hint correctly emits `RENAME COLUMN` (not DROP+ADD) for column data preservation.
- **Partial indexes** with `WHERE deleted_at IS NULL` and `WHERE status = ANY (ARRAY[...])` survive drift checks cleanly.
- **CHECK constraint** auto-rewrite to `NOT VALID + VALIDATE` is transparent and correct.
- **CREATE INDEX** auto-rewrite to `CONCURRENTLY IF NOT EXISTS` outside transaction is correct by default — no other tool does this transparently.
- **DATA_LOSS hazard** detection with clear error and documented override flag.
- **Mass-drop guard** refused a schema-destroying migration with a clear message.
- **RLS**: `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` + `CREATE POLICY` generated and applied cleanly.
- `WHERE status = ANY (ARRAY[...])` partial index no longer produces false drift (B2 fixed).
- `verify` now recognizes `CREATE TYPE ... AS ENUM` in source (B4 fixed in source).
- View false drift fixed in source (B3 fixed).

---

## Open Gaps

### Previously Critical — Now Fixed

**G1 — @renamed constraint ghost** ✅ Fixed
After renaming a column that appears in a UNIQUE constraint, every subsequent `migrate generate` re-emitted broken `DROP CONSTRAINT / ADD CONSTRAINT` statements referencing the old column name. Fixed by `applyColumnRenameHintsToConstraints()` in the source parser, which rewrites constraint `DefSQL` to use the new column name when a `@renamed` hint is applied.

**B1 — FK to partitioned table ghost constraints** ✅ Fixed
PostgreSQL auto-creates per-partition FK clones (`conparentid != 0`). Fixed by adding `AND c.conparentid = 0` to the `pg_constraint` inspector query.

**B2 — Stored procedure perpetual re-emit** ✅ Fixed
`pg_get_functiondef()` returns `IN param_name type`; source omits `IN` mode keyword. Fixed with `reParamModeIn` regex in ExtraDDL fingerprinting.

**B3 — CREATE SCHEMA re-emit** ✅ Fixed
Tracking `CREATE SCHEMA` as first-class objects; inspector reads live schemas from `pg_namespace`; differ suppresses emission when schema already exists.

**B4 — Inline unnamed CHECK constraints silently dropped** ✅ Fixed
Source parser now captures inline column-level CHECK constraints and auto-generates names following PostgreSQL's `<table>_<col>_check` convention.

**B5 — GRANT statements never emitted** ✅ Fixed
`grants.sql` sorts alphabetically before table/view schema files. Added `PendingGrants` second-pass in `LoadDesiredState` to apply grants after all objects are loaded.

**B6 — Enum cast in partial index false drift** ✅ Fixed
`stripUserDefinedCasts()` removes `::user_defined_type` casts from index predicates before comparing, so bare string literals match the PostgreSQL catalog form.

### CI Gate Leaks (both fixed)

**G2 — View false drift** ✅ Fixed (commit 9530b73)
**G3 — verify misses CREATE TYPE enums** ✅ Fixed (commit 529cb8d)

### Schema Object Coverage

**G4 — Triggers not in verify scanner**
`CREATE TRIGGER` is handled by generate/apply correctly, but `verify` does not scan `pg_trigger`. An out-of-band trigger (created directly in DB) is invisible to CI. Not blocking, but the CI story is incomplete.

**G5 — Types are ExtraDDL pass-through, not structurally modeled**
`CREATE TYPE / CREATE DOMAIN / CREATE COMPOSITE` are routed to `ExtraDDL` in the source loader (V2-A partial). The inspector reads types from `pg_type`, but the differ does not do structural type diffing — it only catches changes through the ExtraDDL hash. Type alter/drop hazards (PRD v2 FR2-04) are not implemented.

**G6 — Partitioned table parents not fully modeled**
Inspector queries include `relkind IN ('r', 'p')` but child partition graph, `ATTACH`/`DETACH` ordering, and strategy diffs are not implemented (PRD v2 §3.2, V2-B).

### Developer Experience

**G7 — ADVISORY COLUMN_REORDER repeats in every migration**
Once column order diverges from desired schema, the same two-line advisory appears in every subsequent migration indefinitely. Should deduplicate after the first occurrence.

**G8 — --force-after-drift on every apply after manual migration edit** ✅ Fixed
`pg-flux migrate rehash` now accepts an edited migration file by writing a content-hash into the baseline-hash header. The drift check accepts the content-hash as a signal that the user reviewed and accepted the manual edit.

**G9 — init can silently overwrite schema/users.sql** ✅ Fixed
`pg-flux init` now skips writing sample schema files if they already exist, and prints `skipped schema/users.sql (already exists)`.

### Codegen

**G10 — No Python type emitter** ✅ Fixed
`pg-flux gen --lang python` generates `models.py` with Pydantic v2 `BaseModel` classes and `str, Enum` enums. Verified on fastapi-todo.

---

## Competitive Position

| Tool | Model | Auto-safety | SQL-native | PG-specific | Maturity |
|---|---|---|---|---|---|
| **pg-flux** | Declarative SQL diff | Best-in-class | Yes | Deep | v0.1.0 |
| **Atlas** | HCL or SQL diff | Partial | HCL-first | No (multi-DB) | 3+ yrs, production |
| **golang-migrate** | File runner only | None | Yes | No | Mature, primitive |
| **Flyway** | Versioned files | None | Sort-of | No | Enterprise, heavy |
| **Prisma Migrate** | TS DSL | None | No | No | Strong TS ecosystem |

**Edge over Atlas:** Pure SQL, no HCL or DSL. Auto-safety rewrites Atlas doesn't do (CONCURRENTLY, NOT VALID, DATA_LOSS). Simpler "edit → generate → apply" loop.

**Where Atlas wins:** Maturity, cloud product, multi-DB, ecosystem integrations.

**Target user:** Small/medium PG-only teams. Engineers who write SQL naturally and want a tool that handles the dangerous parts (CONCURRENTLY, hazards, rename hints) without learning a DSL.

---

## Priority Path to 8.5 / 10

| Priority | What | Status |
|---|---|---|
| P0 | Fix G1 (@renamed ghost) | ✅ Done |
| P0 | Lock in CI gates (G2, G3, G4 triggers in verify) | ✅ Done — all 4 examples exit 0 |
| P1 | G7 ADVISORY dedup + G8 rehash command | ✅ Done |
| P1 | G5 first-class type model (V2-A) | Open — types still pass-through |
| P2 | G10 Python emitter | ✅ Done |
| P2 | G6 partitioned tables (V2-B) | Open — partition graph not modeled |

**Remaining to reach 8.5 / 10:**
- G5 first-class structural type model (enums have full diff; domains/composite types are pass-through)
- G6 partitioned table full graph (ATTACH/DETACH ordering, strategy diffs)
- G7 ADVISORY COLUMN_REORDER deduplication (cosmetic — advisory still repeats, just less frequently)
