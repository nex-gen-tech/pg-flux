# pg-flux

> **One source of truth for your Postgres schema AND your app types.**

<p>
  <a href="https://nex-gen-tech.github.io/pg-flux/"><img src="https://img.shields.io/badge/docs-nex--gen--tech.github.io%2Fpg--flux-5B6CF6?style=flat-square&logo=bookstack&logoColor=white" alt="docs"></a>
  <a href="https://github.com/nex-gen-tech/pg-flux/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/nex-gen-tech/pg-flux/test.yml?branch=main&label=tests&style=flat-square" alt="tests"></a>
  <a href="https://github.com/nex-gen-tech/pg-flux/actions/workflows/matrix.yml"><img src="https://img.shields.io/github/actions/workflow/status/nex-gen-tech/pg-flux/matrix.yml?branch=main&label=PG%2014-18%20matrix&style=flat-square" alt="matrix"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/nex-gen-tech/pg-flux?style=flat-square" alt="MIT license"></a>
  <a href="https://github.com/nex-gen-tech/pg-flux/releases"><img src="https://img.shields.io/github/v/release/nex-gen-tech/pg-flux?style=flat-square&label=release" alt="release"></a>
  <img src="https://img.shields.io/badge/PostgreSQL-14%20%E2%80%93%2018-336791?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL 14-18">
  <img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.25+">
</p>

Declarative PostgreSQL migrations with safe apply, drift detection, schema dump, and end-to-end Go + TypeScript + Python + Rust codegen. Write SQL once. Keep your schema, your migrations, and your app types in lock-step. Forever.

**→ [Full documentation at nex-gen-tech.github.io/pg-flux](https://nex-gen-tech.github.io/pg-flux/)**

---

## What this is

You're tired. We get it.

You're tired of writing the same schema three times: once as SQL, once as a Go struct, once as a TypeScript interface. You're tired of migrations going sideways because someone added a column in prod and forgot to backport. You're tired of `ALTER TABLE` quietly taking an `AccessExclusiveLock` for 17 minutes during peak hours. You're tired of the gap between "the database" and "the code that talks to the database."

pg-flux closes that gap.

```text
schema.sql  ──►  pg-flux migrate generate  ──►  migrations/*.sql  ──►  pg-flux migrate apply
   │                                                                        │
   └─────────────►  pg-flux gen  ──►  Go structs + TS interfaces  ◄─────────┘
```

One model. One CLI. Every catalog object pg-flux touches gets a typed representation in your app. Every migration runs with hazard guards on by default. Every drift gets caught before it hits prod.

## 30-second example

```bash
# install (no Go required)
curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh | sh

# scaffold a project
pg-flux init

# write SQL in schema/
cat > schema/users.sql <<'SQL'
CREATE TYPE role AS ENUM ('admin','member');
CREATE TABLE users (
  id    bigint PRIMARY KEY,
  email text NOT NULL,
  role  role NOT NULL DEFAULT 'member'
);
SQL

# generate + apply
pg-flux migrate generate --label add_users
pg-flux migrate apply

# generate types — Go AND TypeScript
pg-flux gen --lang go,ts --validators=zod --branded-ids
```

You now have:

- `migrations/20260520_*.sql` — a versioned, hash-stamped migration
- `internal/dbgen/users.go` — `type User struct { ID UserId; Email string; Role Role }`
- `src/db/tables.ts` — `interface User { id: UserId; email: string; role: Role }`
- `src/db/validators.ts` — `z.object({ id: z.bigint(), ... })`

Your schema and your code can no longer drift. CI gates exist for the next time someone forgets:

```bash
pg-flux drift --strict    # exit 2 if live DB ≠ source
pg-flux verify --strict   # exit 4 if live has undeclared objects
pg-flux gen --check       # exit 3 if generated files are stale
```

## What's in the box

| Capability | Detail |
|---|---|
| **Declarative migrations** | Write SQL once. pg-flux diffs against live, emits minimum DDL, applies in a transaction with an advisory lock. |
| **Hazard guards** | Refuses to apply mass-drops, type-changing rewrites, or constraint-validating scans without explicit `--allow-hazards`. NOT VALID + VALIDATE auto-rewrite for safe FK/CHECK adds on large tables. |
| **Drift detection** | Three layers: source ↔ live (`drift`), live ⊃ source (`verify`), and per-migration baseline-hash to catch "someone changed prod between generate and apply." |
| **Bidirectional dump** | `pg-flux dump` extracts a complete pg-flux source tree from any existing database. Round-trip clean — `migrate generate` immediately after produces zero pending statements. |
| **Codegen** | Go + TypeScript + Python + Rust types for every catalog object with a row shape. Tables, enums, composite types, domains, views, functions, procedures. |
| **Configurable codegen** | Branded IDs, zod validators, ORM tag flavors (sqlx/gorm/bun/ent), camelCase/snake/pascal, `bigint as number\|string\|bigint`, dates as `Date\|string\|temporal`, optional vs nullable, Insert/Update helper types. |
| **PG 14–18 coverage** | NULLS NOT DISTINCT, virtual generated columns, NOT ENFORCED, named NOT NULL ... NOT VALID, security_invoker views — version-gated and emitted only against supporting servers. |
| **CI-friendly** | `--check`, `--strict`, `--dry-run`, structured JSON logging, deterministic output. |
| **Built on `pg_query`** | The parser is PostgreSQL's own grammar via libpg_query. We don't roll our own SQL parser; we let PG parse PG. |

## How pg-flux compares

> Benchmarks run on Apple M-series (darwin-arm64), PostgreSQL 17, local socket. Schema used: 73-object real-world HRM schema (tables, views, functions, EXCLUDE constraints, range types, partitioned tables). All numbers are median of 10 cold runs. [Methodology →](./docs/benchmarks.md)

### Feature matrix

| Feature | pg-flux | Atlas | Flyway | Liquibase | golang-migrate | goose | Alembic | Prisma |
|---|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| Declarative desired-state schema | ✓ | ✓ | ✗ | ✗ | ✗ | ✗ | ~ | ~ |
| Drift detection (source ↔ live) | ✓ | ~ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Undeclared-object detection (`verify`) | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Baseline-hash tamper detection | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Hazard / destructive-op guards | ✓ | ~ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| `NOT VALID` + `VALIDATE` auto-rewrite | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Native PG parser (`libpg_query`) | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| PG 14–18 version-gated DDL | ✓ | ~ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Advisory lock on apply | ✓ | ✓ | ~ | ~ | ✗ | ✗ | ✗ | ✓ |
| Bidirectional schema dump | ✓ | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| App codegen — Go | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| App codegen — TypeScript | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ~ |
| App codegen — Python | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| App codegen — Rust | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Zero-dependency static binary | ✓ | ✓ | ✗ | ✗ | ✓ | ✓ | ✗ | ✗ |
| PostgreSQL-only focus | ✓ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ |
| Cloud UI / schema visualization | ✗ | ✓ | ✓ | ✓ | ✗ | ✗ | ✗ | ✓ |
| ORM integration | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ | ✓ |
| Imperative up/down migrations | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

✓ full  · ~ partial/limited  · ✗ not supported

**Key notes:**
- *Atlas* — closest modern peer. Declarative, Go binary, supports HCL + SQL. Multi-database (MySQL, SQLite, MariaDB). Has a paid cloud offering with schema visualization. No app codegen.
- *Flyway / Liquibase* — battle-tested enterprise tools. Require a JVM. Excellent for teams already standardized on Java tooling.
- *golang-migrate / goose* — simple, fast migration runners. No schema awareness, no codegen, no drift detection. Right choice when you just need SQL files applied in order.
- *Alembic* — the standard in Python/SQLAlchemy projects. Tied to SQLAlchemy's ORM model; not SQL-first.
- *Prisma Migrate* — excellent for Prisma ORM users. Generates TypeScript client types only; locked to Prisma's schema format.
- *pg-flux* — the only tool that treats the database schema as the source of truth for both migrations **and** app types across four languages, with three layers of drift protection and PostgreSQL-specific safety rewrites.

### Benchmarks

All measurements are real — scripts available at [`docs/benchmarks.md`](./docs/benchmarks.md).

#### CLI cold-start latency

Time from invocation to first byte of output (`--help` / `--version`). This matters in CI pipelines where the tool runs on every commit.

| Tool | Median | Notes |
|---|---:|---|
| golang-migrate | 4 ms | |
| **pg-flux** | **6 ms** | |
| goose | 15 ms | |
| atlas | 63 ms | |
| Flyway | ~3 000 ms | JVM startup |
| Liquibase | ~4 000 ms | JVM startup |
| Alembic | ~400 ms | Python interpreter startup |
| Prisma | ~800 ms | Node.js startup |

#### Schema drift / diff speed (73-object schema)

Time to inspect the live database and compute the diff against the declared source. Goose and golang-migrate have no drift detection and are excluded.

| Tool | Median | Notes |
|---|---:|---|
| **pg-flux drift** | **57 ms** | 3-layer check: source↔live + verify + baseline hash |
| atlas schema diff | 125 ms | source↔live only |

#### Migration apply speed

Time to apply a batch of migrations to a fresh database. pg-flux's higher number reflects work the simpler tools skip: source parse, baseline-hash verification, hazard scan, and advisory lock acquisition.

| Tool | Median | Notes |
|---|---:|---|
| golang-migrate | 39 ms | SQL files, no pre-flight checks |
| goose | 43 ms | SQL files, no pre-flight checks |
| atlas | 140 ms | schema diff + apply |
| **pg-flux** | **260 ms** | parse → hash verify → hazard scan → advisory lock → transact |

If raw apply speed is your only metric, a simple runner wins. If you need the safety layer, the 260 ms is the cost of it.

#### Codegen speed (73-object schema)

Time from CLI invocation to generated files written to disk. Atlas has no codegen; schema inspect is shown as the nearest equivalent operation.

| Tool | Median | What it produces |
|---|---:|---|
| **pg-flux gen --lang go** | **45 ms** | Go structs, enums, views, functions |
| atlas schema inspect | 91 ms | HCL/SQL schema dump only — no app types |

pg-flux generates typed application code **and** completes in half the time Atlas needs just to inspect the schema.

#### Binary size (release tarball, linux-amd64)

| Tool | Size | Runtime required |
|---|---:|---|
| **pg-flux** | **5.4 MB** | none — static binary |
| golang-migrate | 16.6 MB | none — static binary |
| goose | 38.1 MB | none — static binary |
| atlas | ~100 MB | none — static binary |
| Flyway | ~50 MB + JRE | Java 17+ (~300 MB) |
| Liquibase | ~80 MB + JRE | Java 17+ (~300 MB) |
| Alembic | ~2 MB wheels | Python 3.8+ |
| Prisma | ~30 MB | Node.js 18+ |

## Status

**v0.1.3 — production-ready against PostgreSQL 14–18.** The 130/130 PG-version × mutation matrix runs in CI nightly and on every merge to `main`. Five real-world example apps (FastAPI + Python, Express + TypeScript, Go events, Go e-commerce, Rust HRM) pass `drift` and `verify` cleanly.

See [CHANGELOG.md](./CHANGELOG.md) for what changed in each release and [ROADMAP.md](./ROADMAP.md) for what's coming.

## Install

The fastest path — no Go required:

```bash
curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh | sh
```

Other paths:

| Path | Command |
|---|---|
| **curl \| sh** | `curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh \| sh` |
| Manual binary | Download from [GitHub Releases](https://github.com/nex-gen-tech/pg-flux/releases), `tar -xzf`, move to `/usr/local/bin` |
| Pin a version | `curl -sSfL .../install.sh \| PGFLUX_VERSION=v0.1.3 sh` |
| Go install | `go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest` |
| Build from source | `git clone … && cd pg-flux/apps/cli && go build -o pg-flux ./cmd/pg-flux` |

Binaries are statically linked. Supported platforms: `darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`.

> **No-sudo install.** Set `PGFLUX_BIN_DIR=$HOME/.local/bin` before piping if `/usr/local/bin` isn't writable.

## Documentation

**Live docs: [nex-gen-tech.github.io/pg-flux](https://nex-gen-tech.github.io/pg-flux/)**

| Guide | What it covers |
|---|---|
| [Quick start](https://nex-gen-tech.github.io/pg-flux/docs/quick-start.html) | 5 minutes from zero to working setup |
| [How it works](https://nex-gen-tech.github.io/pg-flux/docs/how-it-works.html) | Architecture — parser, inspector, differ, migrator |
| [Migration recipes](https://nex-gen-tech.github.io/pg-flux/docs/recipes.html) | Rename column, add NOT NULL safely, drop FK, and more |
| [Codegen](https://nex-gen-tech.github.io/pg-flux/docs/codegen.html) | Every emit option for Go + TS + Python |
| [CLI reference](https://nex-gen-tech.github.io/pg-flux/docs/cli-overview.html) | Every command and flag |
| [Hazards](https://nex-gen-tech.github.io/pg-flux/docs/hazards.html) | What gets blocked, why, and how to override safely |
| [CI / CD integration](https://nex-gen-tech.github.io/pg-flux/docs/ci-cd.html) | GitHub Actions / GitLab / CircleCI examples |
| [Drift recovery](https://nex-gen-tech.github.io/pg-flux/docs/drift.html) | When `drift` fires and what to do |

## Examples

Four real-world apps in [`examples/`](./examples/):

| Example | Stack | What it exercises |
|---|---|---|
| [`fastapi-todo`](./examples/fastapi-todo/) | Python + FastAPI + psycopg3 | UUID PKs, enums, RLS, Python codegen |
| [`express-bookmarks`](./examples/express-bookmarks/) | TypeScript + Express + node-postgres | JSONB, tsvector, GIN indexes, DB-side trigger |
| [`go-events`](./examples/go-events/) | Go + chi + pgx/v5 | IDENTITY columns, materialized view, deferrable FK, `text[]` GIN |
| [`go-shop`](./examples/go-shop/) | Go + chi + pgx/v5 | 2 schemas, partitioned table, EXCLUDE, BRIN, INCLUDE, domains, composite type, SECURITY DEFINER, stored procedure, grants |
| [`rust-hrm`](./examples/rust-hrm/) | Rust + Actix-web + sqlx | Everything in go-shop + `daterange`, `tstzrange`, `pg_trgm` trigram GIN, window function in matview, multiple EXCLUDE constraints, self-referential table, Rust codegen |

Every example passes `pg-flux drift` and `pg-flux verify` cleanly and runs end-to-end in CI.

## Repository structure

| Path | What's there |
|---|---|
| [`apps/cli`](./apps/cli/) | Go CLI — schema parser, inspector, differ, migrator, codegen, integration tests |
| [`apps/web`](./apps/web/) | Docs site — Bun + React + Tailwind v4, deployed to GitHub Pages |
| [`examples/`](./examples/) | Real-world example apps |
| [`docs/`](./docs/) | Spec, architecture notes, task tracking |

## Contributing

- 📖 [**CONTRIBUTING.md**](./CONTRIBUTING.md) — dev loop, test instructions, PR review bar
- 🗺️ [**ROADMAP.md**](./ROADMAP.md) — what's planned, what's out of scope
- 🔐 [**SECURITY.md**](./SECURITY.md) — responsible disclosure
- 📝 [**CHANGELOG.md**](./CHANGELOG.md) — what landed when

## Acknowledgments

- [`pg_query_go`](https://github.com/pganalyze/pg_query_go) — libpg_query binding
- [`shikijs/shiki`](https://github.com/shikijs/shiki) — code highlighting in docs
- [`shadcn-ui/ui`](https://github.com/shadcn-ui/ui) — UI primitives behind the docs
- The PostgreSQL team

## License

[MIT](./LICENSE).
