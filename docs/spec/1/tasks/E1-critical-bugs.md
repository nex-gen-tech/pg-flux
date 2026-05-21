# Epic 1 — Critical Bug Fixes

**Priority:** P0 — must land before Epic 2 and before any new example apps.
**Spec ref:** [Spec 1 §Open Gaps G1, G7, G8](../spec.md)

---

## E1-T1 — Fix @renamed constraint ghost in differ

**Summary:** After a `@renamed from=<old>` column rename, every subsequent `migrate generate` re-emits spurious `DROP CONSTRAINT / ADD CONSTRAINT` statements that reference the old column name. Fails on `migrate apply` with `column "old_name" named in key does not exist`.

**Root cause (known):** The differ builds constraint DDL by looking up the column name in its source model. After the rename hint resolves the old→new mapping for the column itself, the constraint's column list is not updated to use the new name. Subsequent diffs re-compare a constraint pointing at `old_name` (source model) against a constraint pointing at `new_name` (live DB), emitting a false DROP+ADD.

**Location:** `apps/cli/pkg/differ/` — constraint diffing path; where `@renamed` hint is resolved.

**Acceptance criteria:**
- `migrate generate` after a column rename emits no DROP/ADD for constraints whose only change is the renamed column.
- The express-bookmarks migration sequence (`@renamed from=username` on `users.handle`) applies cleanly on a fresh DB with no `WORKAROUND` comments needed.
- fastapi-todo migration sequence still applies cleanly (regression).
- Unit test: schema with `@renamed` column in UNIQUE constraint → generate → generate again → second generate emits empty plan for that constraint.

**Dependencies:** None.

---

## E1-T2 — Deduplicate ADVISORY COLUMN_REORDER warnings

**Summary:** Once column order in a live table diverges from the desired schema, every subsequent `migrate generate` emits the same `[ADVISORY COLUMN_REORDER]` comment for the same tables, indefinitely. It appears in every single migration file for the rest of the project's lifetime.

**Acceptance criteria:**
- `ADVISORY COLUMN_REORDER` for a given table appears at most once — in the first migration where the divergence is detected.
- If the desired column order changes again (new column added in a different position), the advisory re-emits for that change only.
- No regression: the advisory still appears when column order first diverges.
