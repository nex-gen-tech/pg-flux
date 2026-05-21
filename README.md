# pg-flux

> **One source of truth for your Postgres schema AND your app types.**

<p>
  <a href="https://github.com/nex-gen-tech/pg-flux/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/nex-gen-tech/pg-flux/test.yml?branch=main&label=tests&style=flat-square" alt="tests"></a>
  <a href="https://github.com/nex-gen-tech/pg-flux/actions/workflows/matrix.yml"><img src="https://img.shields.io/github/actions/workflow/status/nex-gen-tech/pg-flux/matrix.yml?branch=main&label=PG%2014-18%20matrix&style=flat-square" alt="matrix"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/nex-gen-tech/pg-flux?style=flat-square" alt="MIT license"></a>
  <a href="https://github.com/nex-gen-tech/pg-flux/releases"><img src="https://img.shields.io/github/v/release/nex-gen-tech/pg-flux?style=flat-square&label=release" alt="release"></a>
  <img src="https://img.shields.io/badge/PostgreSQL-14%20%E2%80%93%2018-336791?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL 14-18">
  <img src="https://img.shields.io/badge/Go-1.25%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.25+">
</p>

Declarative PostgreSQL migrations with safe apply, drift detection, schema dump, and end-to-end Go + TypeScript codegen. Write SQL once. Keep your schema, your migrations, and your app types in lock-step. Forever.

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
| **Codegen** | Go + TypeScript types for every catalog object with a row shape. Tables, enums, composite types, domains, views (with inferred column types), function params + results, procedure params. |
| **Configurable codegen** | Branded IDs, zod validators, ORM tag flavors (sqlx/gorm/bun/ent), camelCase/snake/pascal, `bigint as number\|string\|bigint`, dates as `Date\|string\|temporal`, optional vs nullable, Insert/Update helper types. |
| **PG 14–18 coverage** | NULLS NOT DISTINCT, virtual generated columns, NOT ENFORCED, named NOT NULL ... NOT VALID, security_invoker views — version-gated and emitted only against supporting servers. Verified in a 26-step × 5-PG-version matrix in CI. |
| **CI-friendly** | `--check`, `--strict`, `--dry-run`, structured JSON logging, deterministic output (gen produces byte-identical files for the same schema). |
| **Built on `pg_query`** | The parser is PostgreSQL's own grammar via libpg_query. We don't roll our own SQL parser; we let PG parse PG. |

## Status

**v0.1 — production-ready against PostgreSQL 14–18.** The 130/130 PG-version × mutation matrix runs in CI nightly and on every merge to `main`. The differ, inspector, and codegen pipelines are stable. The CLI surface is stable; new flags are additive.

We have **not** committed to a 1.0 yet. The public Go API in `pkg/` is subject to refactoring up to 1.0. The CLI surface is more stable than the library surface.

See [ROADMAP.md](./ROADMAP.md) for what's planned and what's deliberately out of scope.

## Install

The fastest path — no Go required:

```bash
curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh | sh
```

That detects your OS + arch (macOS and Linux, amd64 + arm64), downloads the right binary from the latest [GitHub Release](https://github.com/nex-gen-tech/pg-flux/releases), verifies the SHA-256 checksum, and drops `pg-flux` into `/usr/local/bin` (or `~/.local/bin` if that isn't writable).

Other paths:

| Path | Command |
|---|---|
| **curl \| sh** | `curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh \| sh` |
| Manual binary | Download from [GitHub Releases](https://github.com/nex-gen-tech/pg-flux/releases), `tar -xzf`, move to `/usr/local/bin` |
| Pin a version | `curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh \| PGFLUX_VERSION=v0.1.0 sh` |
| Go install | `go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest` (requires Go 1.25+) |
| Build from source | `git clone … && cd pg-flux/apps/cli && go build -o pg-flux ./cmd/pg-flux` |
| Docker (coming soon) | `docker run --rm -v $(pwd):/app pgflux/cli:0.1 migrate apply` |

Binaries are statically linked against libc only — no other runtime dependencies. Supported platforms: `darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`.

## Documentation

Full docs at the project site:

- **[Quick start](./apps/web/content/docs/quick-start.md)** — 5 minutes from zero to working setup
- **[Architecture](./apps/web/content/docs/architecture.md)** — what's happening under the hood
- **[Migration recipes](./apps/web/content/docs/recipes.md)** — rename a column, add NOT NULL on a huge table, drop a FK safely, and more
- **[Codegen](./apps/web/content/docs/codegen.md)** — every emit option for Go + TS
- **[CLI reference](./apps/web/content/docs/cli-overview.md)** — every command and flag
- **[Troubleshooting](./apps/web/content/docs/troubleshooting.md)** — when things go wrong
- **[CI/CD integration](./apps/web/content/docs/ci-cd.md)** — GitHub Actions / GitLab / CircleCI examples

To run the docs site locally:

```bash
bun run dev:web      # http://localhost:3000
```

## Repository structure

This is a Bun-workspace monorepo:

| Path | What's there |
|---|---|
| [`apps/cli`](./apps/cli/) | The Go CLI. All Go code lives here — schema parser, inspector, differ, migrator, dump/verify/pull, codegen pipeline, integration tests, matrix harness. |
| [`apps/web`](./apps/web/) | The documentation site + landing page. Pure Bun + React + Tailwind v4 + shadcn primitives. Static build deploys anywhere. |

## Contributing

We'd love your help. Start here:

- 📖 [**CONTRIBUTING.md**](./CONTRIBUTING.md) — dev loop, test instructions, PR review bar
- 🗺️ [**ROADMAP.md**](./ROADMAP.md) — what's planned, what's out of scope
- 🔐 [**SECURITY.md**](./SECURITY.md) — responsible disclosure if you find a vulnerability
- 🤝 [**CODE_OF_CONDUCT.md**](./CODE_OF_CONDUCT.md) — be kind, be specific, assume good faith
- 📝 [**CHANGELOG.md**](./CHANGELOG.md) — what landed when

Bug reports, feature ideas, and "this docs page is wrong" PRs are all welcome.

## Acknowledgments

- [`pg_query_go`](https://github.com/pganalyze/pg_query_go) — libpg_query binding, the reason our parser is PG-grade
- [`shikijs/shiki`](https://github.com/shikijs/shiki) — code highlighting in the docs site
- [`shadcn-ui/ui`](https://github.com/shadcn-ui/ui) — UI primitives behind the docs
- The PostgreSQL team — the database we're trying to help you manage

## License

[MIT](./LICENSE). Use it, fork it, ship it.
