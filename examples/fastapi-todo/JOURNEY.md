# Journey log

This file is the running log of the build of this example. It exists so contributors can read the friction a real user hits — not the polished marketing story.

The schema in this example wasn't designed up front. It grew across ~20 iterations the way a real app's schema would, with pg-flux as the tool driving every change. Every issue below was hit live during that build.

## What worked, with no friction

- `curl -sSfL .../install.sh | sh` installed a working binary in 5 seconds.
- The first 6 schema mutations (create tables, add FK columns, partial indexes, junction tables) generated and applied cleanly with no surprises.
- `migrate generate` consistently produced a single timestamped file with embedded baseline hash. The file is readable SQL — you could hand-review every change.
- `CREATE INDEX` was always auto-rewritten to `CREATE INDEX CONCURRENTLY IF NOT EXISTS` and split out of the surrounding txn. That's the right default.
- `CHECK` constraints were auto-rewritten to `NOT VALID + VALIDATE`. Same — right default.
- `@renamed from=username` hint preserved data when renaming `users.username` → `users.handle`. The migration emitted `ALTER TABLE ... RENAME COLUMN`, not `DROP + ADD`.
- `DATA_LOSS` hazard correctly refused dropping `tags.label`. Pass `--allow-hazards=DATA_LOSS` to override. Clear error message.
- Mass-drop guard refused a migration that would drop 30%+ of the schema. Clear error message; correct mitigation flag suggested.
- Soft-delete partial index (`WHERE deleted_at IS NULL`) survives drift checks cleanly.
- `GENERATED ALWAYS AS (lower(title)) STORED` round-trips correctly through generate + apply.
- The FastAPI app talks to the resulting schema without any glue layer. Generated columns auto-populate. The SQL function (`count_user_todos`) is callable from psycopg. Constraint violations surface the constraint name in the error.

## Real bugs hit

### B1 — Same-second migration filenames collide and break apply order (critical)

Generating multiple migrations within the same wall-clock second produces filenames like:

```
20260521_190103_add_priority_enum.sql
20260521_190103_add_priority_column.sql
```

These sort by label alphabetically, not by generation order. `add_priority_column` (which references `todo_priority`) ends up *before* `add_priority_enum` (which creates the type) in apply order. On a fresh database, `migrate apply` fails:

```
ERROR: type "public.todo_priority" does not exist (SQLSTATE 42704)
```

This is the kind of bug that's invisible during interactive development (you generate one migration at a time, with seconds between them) and disastrous in CI/scripted workflows.

**Workaround in this example:** I renamed the 4 colliding files in the `190103` cluster with `a/b/c/d` suffixes to force the correct order. See `migrations/20260521_190103[a-d]_*.sql`. The same workaround is applied to the other same-second clusters.

**Proper fix in pg-flux:** filename timestamps need sub-second resolution (ms) or a monotonic counter as tiebreaker.

### B2 — Drift falsely positive after partial index with `WHERE col IN (...)`

I wrote:
```sql
CREATE INDEX idx_todos_priority ON public.todos (priority)
  WHERE priority IN ('high', 'urgent');
```

PostgreSQL normalized that in `pg_indexes.indexdef` to:
```sql
WHERE (priority = ANY (ARRAY['high'::todo_priority, 'urgent'::todo_priority]))
```

pg-flux's differ treats the two forms as different, so `pg-flux drift` reports false drift forever. Workaround: rewrite the source to use the `= ANY` form. That's what `schema/todos.sql` does in this example.

This makes `pg-flux drift --strict` unsafe to gate CI on once any partial index uses `IN`.

### B3 — Views falsely drift on every check

Even when the source view definition matches what was applied, `pg-flux drift` reports a `DROP VIEW + CREATE VIEW` cycle for `active_todos`. PostgreSQL canonicalizes view bodies in the catalog (quoting, schema-qualification, AST normalization), and pg-flux's differ doesn't apply the same canonicalization to the parsed source. Net effect: any project with views can't use `drift --strict` in CI.

### B4 — `verify` ignores `CREATE TYPE` declarations in source

`schema/types.sql` contains:
```sql
CREATE TYPE public.todo_priority AS ENUM ('low','normal','high','urgent');
```

`pg-flux verify` reports `public.todo_priority` as a live object not declared in source. `pg-flux drift` parses the same file and sees the enum fine. The verify command's source loader doesn't see types.

## Polish-level issues hit

- `pg-flux init` is interactive; prompts swallow stdin when piped. Should detect non-tty.
- `init` writes a sample `schema/users.sql`; the README example overwrites it without warning. Either skip the sample or reference it.
- `--schema` and `--schema-dir` are both documented as separate flags with identical defaults. Should be aliased.
- Help text contains internal references like "PRD P3-14". Should be removed from user-facing strings.
- `migrate apply` mixes human output (`apply X ... ok`) with structured logs (`level=INFO msg=migrate.applied ...`). Pick one.
- Every non-zero exit dumps the full --help block, including for legitimate signals like "drift detected" or "hazard refused". Real errors get buried under 30 lines of flag listings.
- Mass-drop refusal lists `VIEW public.active_todos` as a dropped object even when nothing the view references is being changed.
- The README `curl | sh` example doesn't mention sudo will be needed (default install dir is `/usr/local/bin`); `PGFLUX_BIN_DIR` documented in docs but not in the README.

## Bottom line

The "write SQL once and pg-flux figures out the rest" promise holds for *generating and applying* changes — that pipeline is solid through 18 real-world mutations and 5 tables of structure. The CI gates (`drift`, `verify`) leak too much to gate on safely today; same-second filename collisions break script-driven workflows. Both are fixable.

The example you're reading was built in one sitting against pg-flux v0.1.0 on PostgreSQL 17, with no insider knowledge of the tool. The bugs above are exactly what a first-time user would hit.
