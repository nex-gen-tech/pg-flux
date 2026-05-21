# Roadmap

What's planned, what's done, what's deliberately not happening. Updated as priorities shift.

## In flight

Nothing actively in flight beyond bug-fix triage. Open an issue if you have a request that fits one of the buckets below.

## Next (v0.2)

| Item | Why | Rough size |
|---|---|---|
| ALTER OWNER for SCHEMA | The one ALTER OWNER gap — needs schema-level modeling in `pkg/schema` | ~1 day |
| ENUM RENAME VALUE inference | Currently detected via heuristic (same-position swap). Add an explicit `@renamed-value 'a' → 'b'` hint for precision | ~half day |
| TypeScript watch mode | `pg-flux gen --watch` rebuilds on schema-file change. Pair with `pg-flux migrate apply --gen` for full pipeline. | ~2 days |
| Function signatures: VARIADIC / polymorphic / RETURNS SETOF table | Edge cases the current emitter glosses over | ~1 day |
| Per-object file layout in codegen | `layout: per-object` emits `users.go` / `orders.go` instead of one big `tables.go` | ~1 day |

## Soon (v0.3)

| Item | Why |
|---|---|
| Foreign-key cross-reference types in TS (`user_id: User["id"]`) | Turns the schema graph into compile-time relationships |
| `pg-flux dump --diff` | "Show me what's in live but not source, AS source-shaped SQL" — combines verify + pull into one call |
| Python emitter | Same architecture as Go/TS; dataclasses + Enum + Optional |
| Custom comment-hint plugins | Allow projects to define their own `pg-flux: <key>=<value>` handlers |

## Later (no version pinned)

- **Rust emitter** — serde derive, Optional<T>, sqlx integration
- **JSON Schema → typed jsonb** — point at a `.schema.json`, get the TS type + zod validator
- **Migration scripting hooks** — pre-apply / post-apply commands declared per migration
- **`pg-flux explain`** — natural-language description of what a migration will do
- **Editor integration** — LSP-style completions for `schema/*.sql` files
- **VS Code extension** — syntax highlight for `-- @renamed`, inline drift indicators

## Out of scope (forever)

These exist on the roadmap explicitly to **not** happen. Save the GitHub issues:

| Not happening | Why |
|---|---|
| Other database engines (MySQL, SQLite, MariaDB, SQL Server) | The whole value of pg-flux is PG-grade accuracy. Cross-DB tools exist (Atlas, Liquibase) — they're worse for any one DB but better at portability. Different product. |
| Query generation (sqlc-style) | sqlc does this well. pg-flux generates *types*, sqlc generates *query glue*. They compose. |
| Schema visual designer / GUI | pg-flux is a CLI for the SQL you write by hand. |
| Hosted SaaS | Run pg-flux against your own database. |
| Configuration through Go / TS code instead of YAML | YAML is fine. Config-as-code adds a moving part for no gain. |
| Auto-applied migrations on startup | Production migration tooling should be explicit and observable, not magic. |
| Schema validation against an external authority (DBT, Atlas Cloud, etc.) | The schema files in your repo ARE the authority. |

## How priorities are set

1. **Bug fixes** beat features. Always.
2. **Coverage gaps in PG 14-18 features** beat new features. The "100% PG 14-18" goal is the project's spine.
3. **Things that show up in the 130-step matrix as DRIFT** are P0.
4. **Things 3+ unrelated users request in issues** move up the queue.
5. **Things one user requests for their specific stack** stay where they are unless they're general-purpose.

## Want something on this list?

Open a [Discussion](https://github.com/nexg/pg-flux/discussions) — not an Issue. Issues are for bugs and concrete feature proposals; Discussions are for "should pg-flux do X?" conversations where the answer might be no.
