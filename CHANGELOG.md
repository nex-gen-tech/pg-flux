# Changelog

All notable changes to pg-flux are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Rust codegen** (`pkg/codegen`): `pg-flux gen --lang rust` generates a complete set of Rust source files backed by `sqlx` and `serde`:
  - `tables.rs` — `pub struct` per table with `#[derive(Debug, Clone, sqlx::FromRow, Serialize, Deserialize)]`; nullable columns use `Option<T>`; identity/generated columns marked readonly.
  - `enums.rs` — `pub enum` per PG enum with `#[derive(… sqlx::Type)]`, `#[sqlx(type_name = "…")]` and per-variant `#[sqlx(rename)]` / `#[serde(rename)]` annotations — handles hyphens and any PG value format.
  - `views.rs` — read-only `sqlx::FromRow` structs for views and materialized views; all columns `Option<T>`.
  - `types.rs` — composite types as plain `Serialize`/`Deserialize` structs; domains as newtype structs (`pub struct EmailAddress(pub String)`).
  - `functions.rs` (opt-in via `--functions`) — `*Params`, `*Result`, and scalar `type *Row = …` for user-defined functions and procedures.
  - `mod.rs` — barrel file re-exporting all emitted modules.
  - Fully-qualified type paths (`uuid::Uuid`, `chrono::DateTime<chrono::Utc>`, `serde_json::Value`) — no extra `use` declarations required in generated files.
  - Type overrides via `type_overrides` in config (e.g. map `numeric` → `rust_decimal::Decimal`).

- **Python codegen parity** (`pkg/codegen`): `pg-flux gen --lang python` now matches Go/TypeScript coverage:
  - **Views** → read-only `BaseModel` classes with docstring and all columns `Optional[T]`.
  - **Composite types** → nested `BaseModel` classes (attributes as plain fields).
  - **Domains** → `NewType` wrappers (e.g. `EmailAddress = NewType("EmailAddress", str)`).
  - **Functions** (opt-in via `--functions`) → `TypedDict` subclasses for params (`total=False`) and results; scalar return type aliases.

## [0.1.1] — 2026-05-22

Bug-fix release. All six correctness gaps discovered during the go-shop end-to-end build are resolved. Every example (fastapi-todo, express-bookmarks, go-events, go-shop) now exits 0 on both `drift` and `verify` with no workarounds.

### Fixed

- **B1 — FK to partitioned table generates ghost constraints** (`pkg/inspector`): PostgreSQL auto-creates per-partition FK clones (`conparentid != 0`). The inspector now filters these out with `AND c.conparentid = 0`, so they no longer appear as undeclared constraints in `drift` output.
- **B2 — Stored procedure re-emitted on every `migrate generate`** (`pkg/differ`): `pg_get_functiondef()` returns `IN param_name type` but source files omit the `IN` mode keyword. Added `reParamModeIn` regex to strip `IN ` prefix before fingerprinting procedure headers.
- **B3 — `CREATE SCHEMA` re-emitted on every drift/generate run** (`pkg/schema`, `pkg/inspector`, `pkg/differ`): pg-flux now tracks `CREATE SCHEMA` as a first-class object. The inspector reads live schemas from `pg_namespace`; the differ suppresses `CREATE SCHEMA IF NOT EXISTS` emission when the schema already exists.
- **B4 — Inline unnamed column CHECK constraints silently dropped** (`pkg/src`): The source parser now captures inline column-level `CHECK` constraints and auto-generates names following PostgreSQL's `<table>_<col>_check` convention.
- **B5 — `GRANT … TO PUBLIC` never emitted in migrations** (`pkg/src`, `pkg/differ`): `grants.sql` sorts alphabetically before table/view schema files. When processed on first pass, grant targets didn't exist yet and grants were silently discarded. Fixed with a `PendingGrants` second-pass in `LoadDesiredState`, matching the existing `PendingRLS`/`PendingAlterPolicy` patterns.
- **B6 — Enum cast in partial index predicate causes perpetual drift** (`pkg/differ`): PostgreSQL stores `WHERE status = 'active'::product_status` while source has `WHERE status = 'active'`. Added `stripUserDefinedCasts()` that removes `::user_defined_type` from index predicates before comparing, preserving numeric/binary built-in type casts.

### Added

- **Python codegen** (`pkg/codegen`): `pg-flux gen --lang python` generates `models.py` with Pydantic v2 `BaseModel` classes and `str, Enum` enums. Nullable columns emit `Optional[T] = None`; generated columns emit `Optional[T] = None  # server-computed`.
- **`pg-flux migrate rehash`** (`pkg/migrate`): Accepts a manually edited migration file by writing a SHA-256 content-hash into the `pg-flux-baseline-hash` header. Subsequent `migrate apply` recognises the content-hash and skips the live-DB drift check.
- **`pg-flux init` no-overwrite** (`cmd`): `init` now skips writing sample schema files that already exist and prints `skipped schema/users.sql (already exists)`.
- **Codegen singularizer exception list** (`pkg/codegen`): 24-word exception list prevents the singularizer mangling common English words ending in `-us`, `-ss`, `-as`, `-is` (e.g. `event_status` no longer becomes `EventStatu`).
- **go-events example** (`examples/go-events`): Go + chi multi-tenant event management — `GENERATED ALWAYS AS IDENTITY`, deferrable FK, materialized view, `text[]` GIN index, 15 migrations.
- **go-shop example** (`examples/go-shop`): Comprehensive Go + chi e-commerce app — 2 schemas, 3-partition partitioned table, EXCLUDE constraint, BRIN/INCLUDE indexes, SECURITY DEFINER, stored procedure, RLS, domains, composite type, 58-statement clean migration.
- **GitHub Pages docs site** (`apps/web`): Live at [nex-gen-tech.github.io/pg-flux](https://nex-gen-tech.github.io/pg-flux/). Auto-deploys on push to `main`.
- **Examples CI** (`.github/workflows`): All four examples replay end-to-end (apply → drift → verify) as regression gates in CI.

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

[Unreleased]: https://github.com/nex-gen-tech/pg-flux/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/nex-gen-tech/pg-flux/releases/tag/v0.1.0
