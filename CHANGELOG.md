# Changelog

All notable changes to pg-flux are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Nothing yet.

## [0.1.0] — 2026-05-21

The first public release. The point of v0.1 is: "this works against real PG 14-18 schemas, and we have tests to prove it." The point of v1.0 will be: "we promise not to break you."

### Added

#### Declarative migrations
- **Parser** built on `pg_query_go/v6` (PG17-based libpg_query). Covers ~95% of PostgreSQL DDL across versions 14-18, with version gates for PG15+ NULLS NOT DISTINCT, PG17+ WITHOUT OVERLAPS / PERIOD FK, PG18+ virtual generated columns / NOT ENFORCED / named NOT NULL ... NOT VALID.
- **Inspector** reads `pg_catalog` and builds a structured `SchemaState`. Captures tables, columns (with storage, compression, collation, identity, generated), all constraint kinds, indexes, views, materialized views, sequences, functions (with metadata + structured signatures), procedures, triggers (with enable state), policies, enums (with values + ordering), domains, composite types, range types, extensions, event triggers, statistics, foreign servers/tables, default privileges, comments, owners, grants.
- **Differ** emits minimum DDL with structural understanding (not text-diff). Auto-rewrites ADD CHECK / ADD FK to `NOT VALID + VALIDATE` for safe large-table application. Splits view dependency rebuilds. Detects column renames via `-- @renamed` hints.
- **`pg-flux migrate generate`** — diff source vs live, write a timestamped `.sql` file with embedded baseline hash.
- **`pg-flux migrate apply`** — apply pending files in transactions with session advisory lock, CONCURRENTLY statements run autocommit, blocking hazards refuse without explicit opt-in.
- **`pg-flux migrate status / repair / baseline`** — production lifecycle commands.
- **`pg-flux drift`** / **`pg-flux verify`** — symmetric and asymmetric drift detection (`--strict` for CI).
- **`pg-flux dump`** — extract live schema to source files, round-trip clean (enforced by integration test).
- **`pg-flux pull`** — capture undeclared live objects to a quarantine file; never modifies user-edited source.
- **Hazard system** with `DATA_LOSS`, `COLUMN_TYPE_CHANGE`, `CONSTRAINT_SCAN`, `MASS_DROP`, `RLS_GAP`, `STAGED_SET_NOT_NULL`, `VALIDATE_CONSTRAINT_SCAN`. Blocking by default; opt-in via `--allow-hazards`.
- **Baseline-hash drift check** — every generated migration embeds a sha256 of live state; apply refuses if live drifted, with `--force-after-drift` override.
- **Mass-drop guard** — refuses to apply if >25% of live objects would die; tunable via `--mass-drop-threshold`, bypass with `--allow-mass-drop`.

#### Codegen
- **`pg-flux gen`** for Go and TypeScript.
- **Every PG object with a row shape gets a type**: tables, views (with inferred column types), enums, composite types, domains, functions (param + result), procedures (params).
- **Configurable per output** (every option both YAML and CLI):
  - `column_case` snake / camel / pascal
  - `bigint_as` bigint / number / string (TS)
  - `date_as` Date / string / temporal (TS)
  - `null_style` union / undefined / optional (TS)
  - `enum_style` union / const-object / ts-enum (TS)
  - `branded_ids` (TS)
  - `insert_update_helpers` (TS)
  - `validators: zod` (TS)
  - `orm_tags` sqlx / gorm / bun / ent (Go)
  - `omitempty` nullable / defaults / all (Go)
  - `readonly` identity / generated / defaults / all
  - `functions: true` (opt-in)
  - `json_shapes` per-column TS shape overrides
  - `include_tables` / `exclude_tables` / `exclude_schemas` filtering
  - `type_overrides` per PG type
- **`pg-flux gen init`** scaffolds `.pg-flux-codegen.yml` with every option documented inline.
- **`pg-flux gen --check`** — CI gate that exits 1 if generated files are stale.
- **Comment hints**: `pg-flux: gotype=... tstype=...` per-column overrides with balanced-brace tokenization (so `tstype={ source: string; ip?: string }` works without quoting).
- **Custom type resolution**: column references to user-defined enums / composites / domains resolve to the generated type name (not a fallback `string`).

#### Project infrastructure
- **Bun-workspace monorepo**: `apps/cli` (Go) + `apps/web` (docs site).
- **Documentation site** at `apps/web/` — Bun + React + Tailwind v4 + shadcn primitives, Postgres-themed, dark mode default, ⌘K search, themed scrollbars, animated macOS terminal hero.
- **CI workflows**: unit tests on every PR, full PG 14-18 × 26 mutation matrix on push to main + nightly cron.
- **Structured logging** via `pkg/obs` (stdlib `log/slog`) with `--log-format=json` and `--verbose`.
- **130/130 mutation matrix** across PG 14-18.

### Known limitations (filed in [ROADMAP.md](./ROADMAP.md))

- ALTER OWNER doesn't yet cover SCHEMA (requires schema-level modeling).
- Function signature codegen skips aggregate / window functions (no app-call shape).
- View column inference works for everything `pg_attribute` reports; expression complexity is not the limit.
- TypeScript watch mode for `pg-flux gen` not built yet — pair with `entr` or `chokidar`.

### Acknowledgments

- [pganalyze/pg_query_go](https://github.com/pganalyze/pg_query_go) — the libpg_query binding that lets us parse PostgreSQL with PostgreSQL's own grammar.
- [shikijs/shiki](https://github.com/shikijs/shiki) — code highlighting that doesn't make code blocks look like 2008.
- [shadcn-ui/ui](https://github.com/shadcn-ui/ui) — the component primitives behind the docs site.
- The PostgreSQL team for building the database we're trying to help you manage.

[Unreleased]: https://github.com/nexg/pg-flux/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/nexg/pg-flux/releases/tag/v0.1.0
