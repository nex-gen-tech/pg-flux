# Changelog

All notable changes to pg-flux are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Nothing yet.

## [0.1.5] — 2026-05-25

### Fixed

- **Silent error output** (`cmd/pg-flux`): commands with `SilenceErrors:true` (`drift`, `verify`, `plan`, `apply`, `gen`) now print `Error: <message>` to stderr before exiting — previously any non-sentinel error (bad DSN, missing schema dir, connection refused) produced zero output and a silent exit 1.
- **Python codegen: identity column annotation** (`pkg/codegen`): identity columns (`GENERATED ALWAYS AS IDENTITY`) now correctly annotate `# server-computed` instead of incorrectly falling through to `# has DB default`.
- **Python codegen: DB-default comment** (`pkg/codegen`): `NOT NULL` columns with a non-server `DEFAULT` now annotate `# has DB default` in the base model so the `Optional[T] = None` pattern is no longer unexplained.
- **`pull` empty output** (`cmd/pg-flux`): "Wrote 0 object(s) to " → "No undeclared objects found — nothing to pull." (and correctly shows the output path when objects are written).
- **`inspect` conflict warning** (`cmd/pg-flux`): now prints a note when writing to `schema/tables/` so users know to check for duplicate-table conflicts with existing schema files.

### Improved

- **`gen --lang` flag** (`cmd/pg-flux`): description now lists all four supported languages: `go,ts,python,rust` (was `go,ts`).
- **Global flags declutter** (`cmd/pg-flux`): 7 advanced / rarely-used flags hidden from `--help` (`--shadow-semantic`, `--shadow-equivalence`, `--validate-plpgsql`, `--validate-sql`, `--append-validate-not-valid`, `--set-not-null-reltuple-hint`, `--log-format`) — still functional when passed explicitly; reduces noise for new users.
- **Next-step hints** (`cmd/pg-flux`): `migrate generate` prints `Next: pg-flux migrate apply`; `migrate apply` prints `Next: pg-flux gen (refresh generated types)` after applying one or more migrations.
- **`gen` summary message** (`pkg/codegen`): `[go] wrote 1, skipped 0 (already up to date)` → `[go] wrote 1 file(s)` — cleaner output.

### Docs

- **`codegen-go.md`** (new): comprehensive Go codegen reference — all config options, full PG→Go type mapping, struct tags (sqlx/gorm/bun/ent), enums with Scan/Value, ORM integration examples, generated file structure.
- **`codegen-ts.md`** (new): comprehensive TypeScript codegen reference — all 11 options, branded IDs, null_style/enum_style modes, Zod validators, insert/update helpers, json_shapes, generated file structure.
- **9 existing doc pages updated**: `cli-overview.md` (exit code 5), `cli-migrate.md` (--dry-run, 30s advisory lock), `cli-gen.md` (python/rust languages), `codegen.md` (4-language table), `configuration.md` (Python/Rust examples, config key validation), `codegen-python.md` (frozen→from_attributes fix, ORM always-on), `installation.md` (Windows binaries, v0.1.5), `troubleshooting.md` (advisory lock retry, config key warning), `footer.tsx` (Python/Rust/Examples links).

## [0.1.4] — 2026-05-24

### Added

- **`--dry-run` flag for `migrate generate`** (`cmd/pg-flux`): `pg-flux migrate generate --dry-run` prints the generated SQL to stdout without writing any files — useful for previewing changes in CI or before committing.
- **Documented exit codes** (`cmd/pg-flux`): exit codes are now stable, documented in `--help`, and enforced: `0`=OK, `1`=error, `2`=drift detected, `3`=stale codegen, `4`=undeclared objects, `5`=hazard blocked.
- **Config key typo warnings** (`cmd/pg-flux`): unknown keys in `pgflux.yml` now print a warning with a Levenshtein-distance suggestion (e.g. `warning: unknown config key "migraitons" — did you mean "migrations"?`).
- **Python codegen: singularized model names** (`pkg/codegen`): table/view/composite class names are now singularized — `users` → `User`, `todo_tags` → `TodoTag` — matching Go, TypeScript, and Rust conventions.
- **Python codegen: `UserCreate` / `UserUpdate` helpers** (`pkg/codegen`): every table class gains two companion models. `{Name}Create` excludes identity, generated, and server-default columns; required fields have no default. `{Name}Update` wraps all writable columns as `Optional[T] = None` for partial patch payloads.
- **Python codegen: ORM config** (`pkg/codegen`): all generated `BaseModel` classes include `model_config = ConfigDict(from_attributes=True)` for SQLAlchemy / SQLModel compatibility; `ConfigDict` is auto-imported from pydantic when needed.
- **`codegen-python.md` docs** (`apps/web/content/docs`): comprehensive Python codegen reference — quick start, PG→Python type mapping, nullable handling, enums, views, composite types, domains, functions/TypedDict, UserCreate/UserUpdate, ORM compatibility, type overrides, and generated file structure.
- **`codegen-rust.md` docs** (`apps/web/content/docs`): comprehensive Rust codegen reference — quick start, Cargo.toml snippet, PG→Rust type mapping, `Option<T>`, enums with `sqlx::Type`, views, composite types, domains as newtypes, functions/procedures, module structure, type overrides.
- **`examples.md` docs** (`apps/web/content/docs`): index page covering all five examples (fastapi-todo, express-bookmarks, go-events, go-shop, rust-hrm) with feature highlights and quick-start commands.

### Fixed

- **SQL injection in enum rename/add DDL** (`pkg/differ`): enum values containing single quotes (e.g. `O'Brien`) are now properly escaped before being interpolated into `ALTER TYPE … RENAME VALUE` and `ALTER TYPE … ADD VALUE` statements.
- **Advisory lock retry with timeout and guidance** (`pkg/exec`): `migrate apply` now retries `pg_try_advisory_lock` for up to 30 seconds (1-second intervals) before failing; the error message includes the exact `SELECT pg_advisory_unlock(…)` statement for manual recovery.
- **DSN credential redaction in error messages** (`pkg/db`): passwords in PostgreSQL DSNs are now masked (`***`) in all error messages — no credentials appear in logs or terminal output.

### CI/CD

- **golangci-lint** added to CI (`test.yml`); configured via `.golangci.yml` (errcheck, govet, staticcheck, gosimple, ineffassign, unused, gofmt).
- **Matrix tests now run on PRs** (previously only on `push`).
- **All GitHub Actions pinned to full commit SHAs** for supply-chain security.
- **Windows binaries** (`windows-amd64`, `windows-arm64`) added to the release matrix; packaged as `.zip`.
- **Code coverage** uploaded as a CI artifact (`coverage.out`).

## [0.1.3] — 2026-05-24

### Added

- **Benchmark suite and competitor comparison** (`docs/benchmarks.md`, `README.md`): real measurements (darwin-arm64, PostgreSQL 17, 73-object schema) comparing pg-flux against Atlas, Flyway, Liquibase, golang-migrate, goose, Alembic, and Prisma Migrate across five dimensions: CLI cold-start latency, schema drift speed, migration apply speed, codegen speed, and binary size. Methodology and reproduction instructions documented in `docs/benchmarks.md`.
- **Feature matrix** (`README.md`): 18-row × 8-tool comparison table covering declarative schema, drift detection, hazard guards, NOT VALID rewrite, native PG parser, PG 14–18 version gating, advisory lock, bidirectional dump, app codegen (Go/TS/Python/Rust), binary footprint, and more.

## [0.1.2] — 2026-05-22

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

- **`rust-hrm` example** (`examples/rust-hrm`): multi-tenant HR management app (Actix-web 4 + sqlx 0.8 + pg-flux Rust codegen). The most comprehensive example to date — covers every feature in the four existing examples plus:
  - `daterange` type: `positions.valid_during`, `leave_requests.during`
  - `tstzrange` type: `shifts.during`
  - `pg_trgm` extension + GIN trigram index on a generated column (`full_name gin_trgm_ops`) for fast fuzzy name search
  - Window function `rank() OVER (PARTITION BY …)` inside a materialized view (`department_stats`)
  - Two EXCLUDE constraints in the same schema (`positions_no_overlap` on daterange, `shifts_no_overlap` on tstzrange)
  - Self-referential table with depth tracking (`departments.parent_id`)
  - Deferrable FK on leave approvals
  - Full pg-flux Rust codegen output committed to `gen/` (tables · enums · views · types · functions · mod.rs)
  - Added to CI examples job; passes `migrate apply → drift → verify` cleanly.

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

[Unreleased]: https://github.com/nex-gen-tech/pg-flux/compare/v0.1.3...HEAD
[0.1.3]: https://github.com/nex-gen-tech/pg-flux/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/nex-gen-tech/pg-flux/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/nex-gen-tech/pg-flux/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/nex-gen-tech/pg-flux/releases/tag/v0.1.0
