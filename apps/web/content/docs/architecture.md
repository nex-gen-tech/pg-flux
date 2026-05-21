---
title: How it works
group: Getting started
order: 3
description: The pipeline, the data model, and the decisions behind them.
---

You're about to point this tool at your production database. Reasonable question: what does it actually do?

Short answer: **parse the SQL you wrote, ask Postgres what it has, diff the two, emit the smallest set of DDL that makes them match, and refuse to run anything dangerous unless you explicitly say so.**

Long answer below.

## The pipeline

Every pg-flux command — `migrate generate`, `dump`, `verify`, `gen`, `drift` — flows through the same six stages:

```text
                   ┌─────────────────────────────────────────────────────┐
                   │                                                     │
   schema/*.sql ──►│ parse  ──►  desired SchemaState                     │
                   │                            ┐                        │
                   │                            ▼                        │
                   │                          diff  ──►  []Change ──►   │  ──►  output
                   │                            ▲                  ▲    │      (migration .sql,
   live PG    ────►│ inspect ──►  live SchemaState                  │    │       generated code,
                   │                                                │    │       drift report, ...)
                   │                                            DAG sort  │
                   │                                                     │
                   └─────────────────────────────────────────────────────┘
```

Five named components do the work:

| Component | Lives in | Job |
|---|---|---|
| **Parser** | `pkg/src/` | Read the SQL files in your `schema/` and produce a `SchemaState` |
| **Inspector** | `pkg/inspector/` | Query PostgreSQL's `pg_catalog` and produce a `SchemaState` |
| **Schema model** | `pkg/schema/` | A typed Go representation of *everything* in a database. The thing both sides converge to. |
| **Differ** | `pkg/differ/` | Compare two `SchemaState`s and emit a list of `Change` operations |
| **DAG sort** | `pkg/dag/` | Order the changes so PostgreSQL accepts them (drop foreign keys before parent tables, create types before columns that use them) |
| **Emitter** | various | Turn the ordered changes into the requested output — migration SQL, generated code, structured diff |

Everything else (`pkg/migrate`, `pkg/codegen`, `pkg/dump`, `pkg/exec`) is built on top of these five.

## The model: `SchemaState`

The center of gravity is `pkg/schema/model.go`. It's a big plain-old-struct that holds maps of every catalog kind:

```go
type SchemaState struct {
  Tables          map[string]*Table
  Indexes         map[string]*Index
  Views           map[string]*View
  Functions       map[string]*Function
  Triggers        map[string]*Trigger
  Policies        map[string]*Policy
  Sequences       map[string]*Sequence
  Extensions      map[string]*Extension
  EnumValues      map[string][]string
  Domains         map[string]*Domain
  CompositeTypes  map[string]*CompositeType
  RangeTypes      map[string]*RangeType
  EventTriggers   map[string]*EventTrigger
  Statistics      map[string]*Statistics
  ForeignServers  map[string]*ForeignServer
  ForeignTables   map[string]*ForeignTable
  DefaultPrivileges []*DefaultPrivilege
  // ... and a few more
}
```

Both the parser and the inspector produce one of these. The differ takes two. **The whole tool is built around the symmetry: source and live are the same shape, so they're comparable.**

If you want to understand what pg-flux can or can't represent, look at this file. If a field exists on `Table`, the differ can detect changes to it. If a kind isn't in `SchemaState`, pg-flux doesn't manage it.

## Stage 1: parse

The parser is `pkg/src/`. It runs your `schema/**/*.sql` files through [`pg_query_go`](https://github.com/pganalyze/pg_query_go) — a Go binding for libpg_query, which is PostgreSQL's actual parser, extracted into a library. We don't roll our own SQL parser. We let PostgreSQL parse PostgreSQL.

The output is an AST. We walk the AST, recognize each statement kind (`CREATE TABLE`, `CREATE TYPE`, `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`, ...), and populate the corresponding fields on `SchemaState`.

> [!NOTE]
> A second pass resolves cross-file forward references. If file `a.sql` does `ALTER POLICY` and file `b.sql` does the `CREATE POLICY`, we stash the ALTER in `PendingAlterPolicy` during file 1 and apply it after all files are loaded.

PG 14 – 18 syntax all parses cleanly because libpg_query is built from the PG17 grammar and is backward compatible with everything earlier.

## Stage 2: inspect

The inspector is `pkg/inspector/`. It connects to your live PG and runs SQL against `pg_catalog`:

- `pg_class` for tables, views, matviews, indexes
- `pg_attribute` for columns
- `pg_constraint` for PK/UNIQUE/CHECK/FK/EXCLUDE
- `pg_proc` for functions and procedures
- `pg_trigger` for triggers
- `pg_policy` for RLS policies
- `pg_type` + `pg_enum` for enums, domains, composites, ranges
- `pg_namespace`, `pg_description`, `pg_default_acl`, `pg_event_trigger`, `pg_statistic_ext`, ... etc.

Roughly 25 queries total per `inspect()` call. They're parallelized where possible (errgroup) and complete in well under a second on schemas with a few hundred tables.

> [!TIP]
> Want to see what the inspector sees? Run `pg-flux inspect` — it dumps the full
> live state as CREATE-style SQL to stdout. Useful for debugging what
> pg-flux *thinks* your database looks like vs what it actually has.

## Stage 3: diff

The differ (`pkg/differ/`) is the most surface-area-dense package in the project. It takes two `SchemaState`s — typically `desired` (from source) and `live` (from inspector) — and produces a list of `Change` operations.

For each kind of object the differ asks three questions:

1. **In desired, missing from live?** Emit a CREATE.
2. **In live, missing from desired?** Emit a DROP (with `MASS_DROP` guard).
3. **In both?** Compare every field. Emit the minimum ALTER necessary.

For tables, "compare every field" expands into a lot. Column type changes, default changes, NOT NULL toggles, IDENTITY changes, storage changes, compression changes, COLLATE changes, generated-column expressions, RLS toggles, ownership, comments, grants — all individually diffed and individually emitted.

### Fingerprints

Some objects have a body that's complex enough that field-by-field comparison doesn't make sense — view definitions, trigger bodies, function bodies. For these we use a **fingerprint**: parse the source through `pg_query.Deparse`, normalize the result (lowercase keywords, collapse whitespace, strip type casts that `pg_get_viewdef` adds but source omits, etc.), and compare the normalized strings.

Fingerprints are why this works:

```sql
CREATE OR REPLACE VIEW active_users AS
  SELECT id, email FROM users WHERE deleted_at IS NULL;
```

vs what PG stores after parsing:

```sql
SELECT users.id, users.email FROM public.users WHERE (users.deleted_at IS NULL);
```

…match. Even though they're textually different, they parse and deparse to the same canonical form.

### Hazards

The differ also classifies every change it emits. A `DROP TABLE` carries a `DataLoss` hazard. An `ALTER COLUMN ... TYPE` carries `ColumnTypeChange`. An `ADD CONSTRAINT CHECK ... ` without NOT VALID carries `ConstraintScan`.

By default, blocking hazards cause `apply` to refuse without an explicit `--allow-hazards` opt-in. See [Hazards](/docs/hazards.html) for the full taxonomy.

## Stage 4: DAG sort

The differ produces changes in object-kind order: all tables first, then all indexes, then all views, etc. That's not what PostgreSQL wants. PostgreSQL wants:

- Types created before columns that use them
- Foreign keys dropped before parent tables
- Views dropped before columns they reference are altered
- Triggers dropped before the functions they call are replaced

`pkg/dag/` does this. Every `Change` has a base priority (CREATE_TYPE < CREATE_TABLE < CREATE_INDEX < CREATE_VIEW < ...) and explicit dependencies (this CREATE INDEX needs that CREATE TABLE first; this DROP VIEW needs that ALTER COLUMN TYPE second).

We run Kahn's algorithm for topological sort. If a cycle exists — a function references a view that references the function — we error out at this stage rather than try to apply a doomed migration.

> [!IMPORTANT]
> The DAG also handles `RENAME` operations specially. A column rename means
> any constraint, index, or view that references the column must be diffed
> against the *new* name in source, not the old. We build a rename map
> early and thread it through the rest of the diff.

## Stage 5: emit

Once changes are sorted, the emitter turns them into the output format you asked for:

- **`migrate generate`** → an idiomatic `.sql` file with `BEGIN; ... COMMIT;` for the transactional batch, then any `CONCURRENTLY` statements after the commit, then a baseline-hash header
- **`dump`** → per-kind .sql files mirroring how a developer would have written them by hand
- **`drift` / `verify`** → human or JSON diff reports
- **`gen`** → Go structs / TypeScript interfaces (separate pipeline; see [Codegen](/docs/codegen.html))

## Why this shape?

A few decisions you might second-guess. Here's the reasoning.

### Why declarative, not migration-files-first?

Migration tools like goose and golang-migrate ask you to write SQL up + SQL down pairs. That's two sources of truth — the up files and the cumulative state they imply.

pg-flux has one source of truth: the current state of your schema in `schema/`. Migrations are emitted FROM that, not maintained alongside it.

The trade-off: you lose explicit "down" migrations. We think that's fine — production rollbacks happen by writing a forward migration that undoes the previous one, not by running a literal "down". The "down" file in most projects is a lie anyway, edited last six months ago and untested.

### Why round-trip clean dumps?

A dump-and-reload pipeline that produces drift is worse than no dump. Operators don't trust it.

We invest heavily in round-trip cleanliness: the dump output, when re-parsed by `pkg/src/` and diffed against the same live DB, must produce zero pending changes. This is enforced by a build-tag=integration test on every PR. It catches the subtle cases — quoted identifiers, multi-word types, `pg_catalog.` schema prefixes, identity sequences masquerading as freestanding sequences.

### Why use libpg_query instead of writing our own parser?

Postgres's grammar is enormous and constantly evolves. Maintaining a separate parser would mean perpetually chasing PG syntax additions. libpg_query is generated directly from the Postgres source, so when PG 18 ships a new feature, we get the parser update for free.

### Why a 26-step matrix across PG 14 – 18?

Because the surface area is real. The 26-step matrix exercises every feature category sequentially against five PostgreSQL versions, producing 130 (5 × 26) test cases per CI run. Every regression we've ever caught in the differ was caught by this matrix, not by unit tests.

### Why advisory locks for `migrate apply`?

You don't want two CI jobs racing to apply the same migration to the same database. The advisory lock is keyed by `host:port/db` — so two applies against the same database serialize, while applies against different databases proceed in parallel.

### Why baseline-hash drift checking?

The window between `migrate generate` and `migrate apply` is where things go sideways. Someone manually changes prod. A scheduled job creates a temp table. Another tool runs DDL. By the time apply runs, the migration was generated against state X but the DB is now state Y.

The baseline hash is sha256 of the live `SchemaState` at generate time. Apply re-computes it and refuses if it doesn't match. You can override with `--force-after-drift`, but you'll see exactly which hashes differ.

## Where to go from here

- [Schema authoring](/docs/schema-authoring.html) — how to actually structure the SQL files
- [Hazards](/docs/hazards.html) — what blocking hazards look like and how to opt in
- [Drift recovery](/docs/drift.html) — what to do when drift happens
- [Codegen architecture](/docs/codegen.html) — the parallel pipeline that produces Go + TS types
