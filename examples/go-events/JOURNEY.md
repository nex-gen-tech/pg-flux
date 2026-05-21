# Journey log

This file is the running log of the build of this example. It exists so contributors can read the friction a real user hits — not the polished marketing story.

The schema in this example wasn't designed up front. It grew across 15 iterations the way a real app's schema would, with pg-flux as the tool driving every change. Every issue below was hit live during that build.

## What worked, with no friction

### `GENERATED ALWAYS AS IDENTITY` PKs

All four tables use `GENERATED ALWAYS AS IDENTITY` for their primary keys instead of `bigserial` or uuid. pg-flux round-trips these perfectly. The generated migration emits:

```sql
id pg_catalog.int8 GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
```

The type is rendered as `pg_catalog.int8` (the internal canonical name for `bigint`), which PostgreSQL accepts without complaint. On apply, the column behaves exactly as `GENERATED ALWAYS AS IDENTITY` — you cannot override it with a value unless you use `OVERRIDING SYSTEM VALUE`. No friction at all.

### `DEFERRABLE INITIALLY DEFERRED` FK

`attendees.user_id` was declared as:

```sql
CONSTRAINT attendees_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id)
  ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED
```

pg-flux preserved `DEFERRABLE INITIALLY DEFERRED` verbatim in the generated migration:

```sql
CONSTRAINT attendees_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users (id)
  ON DELETE CASCADE DEFERRABLE INITIALLY DEFERRED,
```

The drift check also handled this correctly — no false positives from the deferrable attribute. This is the most important FK variant for multi-tenant bulk-insert workflows (insert workspace + users + attendees in one transaction; FK is checked only at commit). Clean round-trip.

### Materialized view: `CREATE MATERIALIZED VIEW`

pg-flux parsed and generated `CREATE MATERIALIZED VIEW` cleanly. The operation type was `CREATE_MATERIALIZED_VIEW` in the migration comment, distinct from a regular view. The `UNIQUE INDEX` on the materialized view was also generated and applied correctly:

```sql
-- [5] CREATE_MATERIALIZED_VIEW: public.event_stats
CREATE MATERIALIZED VIEW public.event_stats AS ...;
...
-- [6] CREATE_INDEX: public.idx_event_stats_event_id
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_event_stats_event_id ON public.event_stats USING btree (event_id);
```

The materialized view was correctly handled in `gen/views.go` output:

```go
// EventStat — Read-only row from materialized view public.event_stats.
type EventStat struct {
    EventID        *int64 `db:"event_id" json:"event_id"`
    WorkspaceID    *int64 `db:"workspace_id" json:"workspace_id"`
    ConfirmedCount *int64 `db:"confirmed_count" json:"confirmed_count"`
    TotalCount     *int64 `db:"total_count" json:"total_count"`
}
```

pg-flux gen correctly distinguished the materialized view from regular tables, placing its struct in `views.go` with a "Read-only row from materialized view" comment.

### GIN index on text[] column

```sql
CREATE INDEX idx_events_tags ON public.events USING GIN (tags);
```

Generated migration emitted `USING gin (tags)` exactly. `pg-flux drift` showed no false positives on subsequent runs — consistent with B2 being fixed in this build.

### `pg-flux gen --lang go` integration

Running `pg-flux gen --lang go --out gen/` generated three files:
- `gen/tables.go` — one struct per table with correct Go types, `db:` and `json:` tags
- `gen/enums.go` — typed string constants for all three enums
- `gen/views.go` — the materialized view struct

The generated structs were used directly as scan targets in the app — no hand-written types. The `pgx.Rows.Scan()` calls in `internal/handler/` map directly to generated struct fields without any adaptation layer. The `json.RawMessage` type for `jsonb` and `[]string` for `text[]` were correct out of the box.

### Updated_at trigger

`CREATE TRIGGER events_set_updated_at BEFORE UPDATE ... EXECUTE FUNCTION public.set_updated_at()` generated correctly. The function + trigger appeared as separate operations in the same migration. The trigger fires on every UPDATE and the `updated_at` column advances — verified by looking at the DB.

### RLS with bigint setting

`workspace_id = current_setting('app.workspace_id', true)::bigint` was generated as:

```sql
CREATE POLICY events_workspace_isolation ON public.events TO public
  USING (workspace_id = current_setting('app.workspace_id', true)::bigint);
```

This is the bigint variant of the same pattern used in the previous two examples (uuid). No friction.

### Soft-delete partial unique index

The partial unique index approximating `UNIQUE NULLS NOT DISTINCT` on `(workspace_id, title)` where `deleted_at IS NULL`:

```sql
CREATE UNIQUE INDEX idx_events_workspace_title_active ON public.events (workspace_id, title)
  WHERE deleted_at IS NULL;
```

Generated, applied, and drifted cleanly. No false positives on the partial index.

## Drift and verify results

### `pg-flux drift`

```
No drift.
exit code: 0
```

Clean after all 15 migrations. The materialized view did NOT cause a false positive on `drift` — unlike regular views (B3 in fastapi-todo and express-bookmarks), materialized views are not tracked by the differ's view normalization code and therefore do not trigger the false-drift loop. This is a meaningful difference: **projects that only use materialized views (not regular CREATE VIEW) can gate CI on `drift` safely.**

The trigger, function, RLS policy, all GIN indexes, partial indexes, and generated columns all drifted cleanly.

### `pg-flux verify`

```
verify: clean — every live object is declared in source.
exit code: 0
```

This is a significant improvement over the previous two examples. Both fastapi-todo and express-bookmarks saw B4 (verify misses `CREATE TYPE` declarations) — verify reported the enums as undeclared live objects. In this build, verify correctly recognized all three enums (`user_role`, `event_status`, `attendee_status`) as declared in source and returned clean.

**B4 appears to be fixed.** The verify source scanner now handles `CREATE TYPE ... AS ENUM`.

## Real bugs hit

### B1 — Enum type name truncation in `gen --lang go` (cosmetic but affects usability)

The three enum types `user_role`, `event_status`, and `attendee_status` generated the following Go type names:

```go
type AttendeeStatu string   // should be AttendeeStatus
type EventStatu string      // should be EventStatus
type UserRole string        // correct
```

The trailing `s` of `attendee_status` and `event_status` is silently stripped. The name-generation logic appears to singularize the last word of snake_case type names — and `status` becomes `statu`. `user_role` is not affected because `role` doesn't singularize oddly.

This is a cosmetic bug in `pg-flux gen --lang go`'s naming heuristic: the generated names are valid Go identifiers, but they look wrong to any reader of the code and will cause confusion when referenced from application code.

**Workaround:** The app uses the generated names as-is (e.g., `dbgen.AttendeeStatu`, `dbgen.EventStatu`). Renaming them would break on the next `pg-flux gen` run. Document the oddity, live with it.

### B2 — `drift --strict` silently exits 1

`pg-flux drift` does not have a `--strict` flag. Running `pg-flux drift --strict` exits 1 with no output and no error message. The CI documentation in the README and the previous two examples references `drift --strict` as a CI gate — that flag does not exist on the `drift` command (it exists on `verify`).

`pg-flux drift` always exits 1 when there is drift and 0 when clean — `--strict` is the default behavior. The silent exit 1 from an unknown flag is surprising; ideally it would print `unknown flag: --strict`.

### B3 — COLUMN_REORDER advisory appears on every subsequent migration after column additions

Starting from migration 6 (add_event_columns), every subsequent migration file includes advisory comments like:

```
-- [ADVISORY COLUMN_REORDER] Column order in public.events differs from desired schema;
-- reordering requires table recreation. Desired order (surviving cols): ...
```

This is because columns are added in a different order than declared in the source file. The advisory is technically correct, but repeating it on every migration for the rest of the build is noise — it appeared in 8 of the 15 migrations. The B3 note from express-bookmarks applies here too: this advisory should appear once when the divergence first happens, not repeat forever.

## Polish-level issues hit

- The migration number in the filename (e.g., `20260521_221525_154_add_workspaces.sql`) includes a 3-digit counter as a tiebreaker for same-second collisions (a fix for the B1 from fastapi-todo). This example hit no same-second collisions since migrations were generated with enough time between them, but the counter is a good addition.
- `migrate apply` mixes human output lines with structured log lines — same issue as the previous two examples. The human lines (`apply X ... ok`) and the structured lines (`level=INFO msg=migrate.applied`) interleave differently depending on terminal vs piped output.
- The `pg-flux gen --lang go` output package name is `dbgen` (not `tables` or a project-derived name). This can't be configured. Acceptable but worth noting.

## Materialized view: does pg-flux drift handle it?

Yes. The materialized view `event_stats` does NOT trigger false drift. This is the opposite of regular views (B3 in earlier examples). The drift checker appears to skip materialized views entirely rather than comparing their body text — so neither a false positive nor a false negative on the view body. The unique index on the materialized view (`idx_event_stats_event_id`) IS tracked and drifts cleanly.

The practical implication: you can use `pg-flux drift` as a CI gate in projects that use materialized views. Regular views remain a false-positive risk.

## Does `verify` report the materialized view as undeclared?

No. `pg-flux verify` exits 0 cleanly. The materialized view is not reported as an undeclared object. This is correct behavior — it is declared in `schema/views.sql`.

## Does the updated_at trigger survive drift/verify?

Yes. The trigger `events_set_updated_at` and its function `set_updated_at()` are both declared in source, both applied, and both handled cleanly by drift (no false positive) and verify (not reported as undeclared). Consistent with the express-bookmarks finding.

## Bottom line

This is the best drift/verify result across all three examples:

- `drift` exits 0 — no false positives from any object type (materialized view, trigger, GIN index, partial unique index, RLS, generated column, DEFERRABLE FK).
- `verify` exits 0 — all three enums, the materialized view index, the function, the trigger, and all tables are recognized as declared. **B4 from the previous two examples (verify misses enums) is confirmed fixed.**

The generate/apply pipeline handled every advanced PG feature cleanly: `GENERATED ALWAYS AS IDENTITY`, `DEFERRABLE INITIALLY DEFERRED`, `CREATE MATERIALIZED VIEW`, GIN indexes on both `jsonb` and `text[]`, partial unique indexes, generated columns, RLS with `::bigint` cast, and the updated_at trigger pattern.

The one new gen bug (enum names truncated to `AttendeeStatu` / `EventStatu`) is cosmetic and doesn't affect correctness. The CI story for this example is clean: both `drift` and `verify` can be used as gates without workarounds.

This example was built in one sitting against pg-flux built from the current main branch on PostgreSQL 17, acting as a first-time user.
