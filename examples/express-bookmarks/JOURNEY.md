# Journey log

This file is the running log of the build of this example. It exists so contributors can read the friction a real user hits — not the polished marketing story.

The schema in this example wasn't designed up front. It grew across ~18 iterations the way a real app's schema would, with pg-flux as the tool driving every change. Every issue below was hit live during that build.

## What worked, with no friction

- First migration: `CREATE TABLE public.users` with `uuid DEFAULT gen_random_uuid()` primary key. pg-flux handled the UUID default without complaint — `id uuid DEFAULT gen_random_uuid() PRIMARY KEY` round-trips through the differ cleanly.
- `migrate generate` always produced a single timestamped file. Unlike the fastapi-todo build, no same-second filename collisions were hit — the commands ran far enough apart.
- `CREATE INDEX CONCURRENTLY IF NOT EXISTS` auto-rewrite continued to work on every index, including GIN indexes (`USING gin (search_vector)`, `USING gin (metadata)`). The GIN syntax was preserved exactly.
- `CHECK` constraints on bookmarks (`url LIKE 'http%'`, `length(notes) <= 5000`) were auto-rewritten to `NOT VALID + VALIDATE` and split out of the transaction correctly.
- FK with `ON DELETE CASCADE` and `ON DELETE SET NULL` both generated correctly.
- `@renamed from=title` on `collections.title` → `collections.name` worked cleanly: the differ emitted `RENAME COLUMN` + `RENAME CONSTRAINT` (not drop-and-recreate). That's the right pattern.
- `GENERATED ALWAYS AS (lower(title)) STORED` round-trips without issue. The generated column appears in `ADD COLUMN` in the migration.
- `tsvector GENERATED ALWAYS AS (to_tsvector(...)) STORED` also round-trips — pg-flux added both generated columns in a single migration with no error.
- `jsonb NOT NULL DEFAULT '{}'` was recognized and generated correctly. The GIN index on the `metadata` column was emitted as `USING gin (metadata)`.
- `CREATE TRIGGER ... EXECUTE FUNCTION ...` — pg-flux recognized the trigger syntax and generated a `CREATE_TRIGGER` operation in the migration. The function + trigger applied cleanly in a single migration.
- `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` + `CREATE POLICY` generated and applied cleanly.
- Soft-delete partial index (`WHERE deleted_at IS NULL`) generated correctly and `pg-flux drift` did not flag it as drifted.
- The `WHERE status = ANY (ARRAY['unread'::bookmark_status, 'reading'::bookmark_status])` index (B2 in fastapi-todo) generated with the `ANY(ARRAY[...])` form automatically. Drift check did not flag it — confirming B2 is fixed.

## Real bugs hit

### B1 — `@renamed` hint leaves a permanent stale constraint ghost in the differ (critical)

After renaming `users.username` → `users.handle` with the `@renamed from=username` hint, the first migration correctly emitted `ALTER TABLE users RENAME COLUMN username TO handle`. So far, so good.

But the very next migration generated — and every subsequent migration for the rest of the build — included these two spurious statements:

```sql
-- [HAZARD DATA_LOSS] Drops constraint
ALTER TABLE public.users DROP CONSTRAINT IF EXISTS users_username_unique;

-- [HAZARD CONSTRAINT_SCAN] Adding constraint may scan table
ALTER TABLE public.users ADD CONSTRAINT users_username_unique UNIQUE (username);
```

The constraint `users_username_unique` in the live DB references `handle` (the new column name). The differ's source model appears to keep generating it against the old column name `username` — indefinitely, across every subsequent migration. On apply, the first migration that carries these statements fails:

```
ERROR: column "username" named in key does not exist (SQLSTATE 42703)
```

This is not a one-time issue. Every subsequent `migrate generate` re-emits the same broken statements. This means that after any `@renamed` column that is referenced in a constraint, the differ is permanently broken for that constraint until the issue is fixed in pg-flux.

**Workaround in this example:** Manually removed the two spurious statements from every affected migration file (7 migrations total). Added `-- WORKAROUND: pg-flux B1` comments to mark each edit so the fixes are auditable.

**Proper fix in pg-flux:** After a column rename is applied, the differ must update its internal model of the constraint to use the new column name. The `@renamed` hint should be resolved in the constraint's column reference list before the diff is computed.

**Fixed** in pg-flux source (`pkg/src/parser.go` — `applyColumnRenameHintsToConstraints`). The express-bookmarks migrations were regenerated clean after this fix. No WORKAROUND comments remain. The single generated migration (`initial_schema`) applies without error and both `drift` and `verify` exit 0 cleanly.

### B2 — Trigger support exists but is not tracked in drift/verify

`CREATE TRIGGER` was recognized by the source parser — it generated a `CREATE_TRIGGER` step in the migration and applied without error. However, `pg-flux drift` does not track triggers: after applying the `bookmarks_set_updated_at` trigger, running `pg-flux drift` reports no drift for the trigger (correct), but `pg-flux verify` also does not flag it as undeclared (correct). This suggests trigger support is symmetric and complete.

However: if you manually create a trigger directly in the DB (outside of pg-flux), `pg-flux verify` will not catch it as an undeclared object. Triggers are not in the verify scanner's object set. Not a blocking issue for the generate/apply pipeline, but worth noting for the CI story.

### B3 — View falsely drifts on every check

The view `unread_bookmarks` drifts on every `pg-flux drift` run even when the source definition has not changed:

```
DROP_VIEW public.unread_bookmarks: DROP VIEW IF EXISTS public.unread_bookmarks CASCADE
CREATE_VIEW public.unread_bookmarks: CREATE OR REPLACE VIEW public.unread_bookmarks AS SELECT b.id, b.user_id, b.title, b.url, b.status, b.created_at FROM public.bookmarks b WHERE b.status = 'unread'
```

PostgreSQL normalizes view bodies in the catalog (single-line, fully-qualified, AST form), and the differ compares the source text to the catalog text directly. The two forms never match. This is the same B3 from fastapi-todo — it is not fixed.

In addition: every migration that adds a new column to `bookmarks` also regenerated a `DROP_VIEW / CREATE_VIEW` cycle for `unread_bookmarks` (7 times across the build). This isn't incorrect — adding a column to a base table doesn't break the view in PG, but pg-flux drops and recreates it anyway as a precaution. That's conservative but noisy.

### B4 — `verify` does not see `CREATE TYPE` in source

`pg-flux verify` reports:

```
verify: 1 undeclared live object(s):

  Enums (1):
    - public.bookmark_status
```

This is the same B4 from fastapi-todo. The `verify` command's source scanner does not load `CREATE TYPE` declarations from `schema/*.sql`. `pg-flux drift` parses the same files and sees the enum correctly. The two commands use different source loaders. Not fixed.

## Polish-level issues hit

- Every `migrate apply` failure dumps the full `--help` block (30+ lines of flags) before the actual error message. When B1 fires, the error is `column "username" named in key does not exist (SQLSTATE 42703)` — it takes scrolling past the help block to read it.
- `migrate apply` mixes human output lines (`apply ... ok`) with structured log lines (`level=INFO msg=migrate.applied`). Inconsistent; pick one.
- The `ADVISORY COLUMN_REORDER` comment appeared in every migration from migration 3 onward, for both `bookmarks` and `users`. Column reorder is advisory (no action taken), but having the same two-line advisory in every single migration file is noise. It should appear once when the column order first diverges and not repeat until it changes again.
- `pg-flux drift` exits 1 and prints the full `--help` block even when the only drift is the view B3 false positive. This makes it unusable as a CI gate without `|| true`.
- `--force-after-drift` was required on every `migrate apply` after the first workaround edit, because editing the generated migration file changes its hash. The flag is correct behavior, but it's needed on every subsequent apply for the rest of the build once you touch any migration.

## Bottom line

The generate/apply pipeline is solid across 18 schema mutations, 5 tables, 1 view, 1 function, 1 trigger, 1 enum, RLS, and 9 indexes — including GIN, partial, and generated-column indexes. UUID primary keys, JSONB columns, and tsvector generated columns all work without any special handling. The `ANY(ARRAY[...])` partial index drift false positive from fastapi-todo B2 is confirmed fixed.

**B1 is now fixed.** The `@renamed` constraint ghost bug has been resolved in pg-flux source. The express-bookmarks migrations were regenerated clean from scratch — a single `initial_schema` migration covers the full schema (5 tables, 1 view, 1 enum, 1 function, 1 trigger, RLS, 9 indexes) with no WORKAROUND comments anywhere. Both `drift` and `verify` exit 0 after apply.

The CI gates (`drift`, `verify`) leak too much to gate on safely: B3 (view false drift) and B4 (verify misses types) are both still open. The trigger apply/drift story is clean.

This example was built in one sitting against pg-flux v0.1.0 on PostgreSQL 17, acting as a first-time user with no insider knowledge of the tool.
