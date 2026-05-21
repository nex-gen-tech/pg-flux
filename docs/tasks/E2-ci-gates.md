# Epic 2 — CI Gate Reliability

**Priority:** P0 — drift + verify must be safe to hard-gate in CI.
**Spec ref:** [Spec 1 §Open Gaps G2, G3, G4](../spec/1/spec.md)
**Depends on:** E1-T1 (express-bookmarks regeneration requires the rename fix first)

---

## E2-T1 — Add triggers to verify scanner

**Summary:** `pg-flux verify` does not scan `pg_trigger`. Triggers declared in `schema/*.sql` (via `CREATE TRIGGER`) are applied correctly but are never checked by verify. An out-of-band trigger added directly to the DB is invisible to CI.

**Acceptance criteria:**
- `verify` reports undeclared triggers that exist in `pg_trigger` for monitored schemas but have no corresponding `CREATE TRIGGER` in source.
- `verify` does not flag triggers that are declared in source schema files.
- express-bookmarks: `verify` passes after applying all migrations (trigger `bookmarks_set_updated_at` is declared in `schema/triggers.sql`).
- Unit test: trigger in DB, not in source → verify exits non-zero with the trigger name listed.

**Dependencies:** None.

---

## E2-T2 — Regenerate express-bookmarks migrations after E1-T1

**Summary:** The 7 express-bookmarks migration files currently contain `-- WORKAROUND: pg-flux B1` comments where spurious DROP/ADD CONSTRAINT statements were manually removed. Once E1-T1 is fixed, regenerate all migrations from a fresh DB so they are clean pg-flux artifacts.

**Acceptance criteria:**
- All `WORKAROUND` comments removed from `examples/express-bookmarks/migrations/`.
- Migrations regenerated from scratch on a fresh `pgflux_express_bookmarks` DB.
- `migrate apply` on a fresh DB succeeds with no flags.
- `examples/express-bookmarks/JOURNEY.md` updated: B1 marked fixed, WORKAROUND notes removed from the bug description.

**Dependencies:** E1-T1.

---

## E2-T3 — Both examples pass CI gate end-to-end

**Summary:** The `examples` job in `.github/workflows/test.yml` replays both fastapi-todo and express-bookmarks. Both must exit 0 on `drift` and `verify` using the source build (which includes all fixes).

**Acceptance criteria:**
- `examples` CI job passes for fastapi-todo: `drift` exits 0, `verify` exits 0.
- `examples` CI job passes for express-bookmarks: `drift` exits 0, `verify` exits 0.
- No `|| true` or exit-code suppression in the CI steps.

**Dependencies:** E2-T1, E2-T2.
