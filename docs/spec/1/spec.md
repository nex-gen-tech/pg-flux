# Spec 1 — pg-flux Current State Analysis

**Date:** 2026-05-22
**Version assessed:** v0.1.0
**Evidence base:** fastapi-todo + express-bookmarks examples — 38 real schema mutations across 2 domains, driven end-to-end by a first-time user persona.
**Reference:** [PRD v2](../../../apps/cli/docs/prd/PRD-v2-robustness.md) · [ROADMAP](../../../ROADMAP.md)

---

## Overall Rating: 6.2 / 10 (potential: 8.5 / 10)

The core pipeline is better than most tools at v0.1.0. Two things prevent recommending it to a real team today: the `@renamed` constraint ghost breaks interactive development, and the CI gates leak too much to hard-gate on. Fix those and the rating climbs to 8+ without touching anything else.

---

## Dimension Ratings

| Dimension | Rating | Trend |
|---|---|---|
| Core pipeline (generate → apply) | **8 / 10** | Solid |
| Auto-safety rewrites | **9 / 10** | Best-in-class |
| CI gates (drift / verify) | **4.5 / 10** | Improving — 2 bugs fixed in source |
| CLI UX | **5.5 / 10** | Several polish fixes landed; a few remain |
| Bug stability | **5 / 10** | @renamed ghost is critical |
| Schema object coverage | **6 / 10** | Tables, indexes, views, functions, enums, triggers — good; types model is pass-through only |
| Ecosystem / maturity | **3 / 10** | v0.1.0, no integrations, PG-only |
| Documentation | **7 / 10** | Strong for v0.1.0; real example apps exist |

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

### Critical (blocking production use)

**G1 — @renamed constraint ghost in differ**
After renaming a column that appears in a UNIQUE constraint, every subsequent `migrate generate` re-emits broken `DROP CONSTRAINT / ADD CONSTRAINT` statements referencing the old column name. Fails on apply with `column "old_name" named in key does not exist`. Requires manual surgery on every migration file generated after a rename.

- Found in: express-bookmarks — affected 7 consecutive migrations after renaming `username` → `handle`.
- Severity: Critical — breaks interactive development, not just CI.
- Not present in the installed binary test for fastapi-todo because the fastapi-todo schema worked around it differently.
- Reference: PRD v2 §3.3 (schema model gaps), §3.4 (differ).

### CI Gate Leaks (open in installed binary; fixed in source build)

**G2 — View false drift** (fixed in source — commit 9530b73)
PostgreSQL canonicalizes view bodies in the catalog. Differ compared source text to catalog text directly. Fixed in source; not yet in release binary.

**G3 — verify misses CREATE TYPE enums** (fixed in source — commit 529cb8d)
verify source scanner did not load `CREATE TYPE ... AS ENUM`. Fixed in source.

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

**G8 — --force-after-drift on every apply after manual migration edit**
Any manual edit to a generated migration (e.g., removing a broken statement) changes the file hash. Every subsequent `migrate apply` requires `--force-after-drift`. There is no `pg-flux migrate rehash` or `accept` command to accept an edited file without blanket overriding all drift checks.

**G9 — init can silently overwrite schema/users.sql**
`pg-flux init` writes a sample `schema/users.sql`. If the user already has that file, it is overwritten without warning.

### Codegen

**G10 — No Python type emitter**
pg-flux gen targets Go and TypeScript. fastapi-todo required hand-written Pydantic models. A Python emitter is on the ROADMAP (v0.3) but not yet built.

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

| Priority | What | Why it unlocks |
|---|---|---|
| P0 | Fix G1 (@renamed ghost) | Unblocks interactive use; removes manual migration surgery |
| P0 | Lock in CI gates (G2, G3 verified in release; G4 triggers in verify) | Makes `drift --strict` safely gatable in CI |
| P1 | G7 ADVISORY dedup + G8 rehash command | Developer friction reduction |
| P1 | G5 first-class type model (V2-A) | Honest schema coverage; unblocks type alter hazards |
| P2 | G10 Python emitter | Closes fastapi-todo hand-written model gap |
| P2 | G6 partitioned tables (V2-B) | Partition-heavy users |
