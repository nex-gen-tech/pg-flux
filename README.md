# pg-flux

> Declarative PostgreSQL schema migration + bidirectional codegen. One source of truth for the database AND the application types.

A Go CLI that:

- **Manages your schema declaratively** — write SQL once, diff against the live DB, generate a migration, apply it safely.
- **Generates application types** (Go + TypeScript) for every table, enum, composite type, domain, view, function, and procedure — keeping app code in lock-step with the DB after every migration.
- **Round-trips through your existing DB** — `pg-flux dump` extracts a complete pg-flux schema from an existing database so adoption is one command.

## Monorepo layout

| Path | What's inside |
|---|---|
| **[`apps/cli`](./apps/cli/)** | The pg-flux Go CLI: schema parser, inspector, differ, migrator, dump/verify/pull, and the codegen pipeline. All Go code lives here. |
| **[`apps/web`](./apps/web/)** | The documentation site + landing page. Pure Bun fullstack (Bun.serve + TSX + markdown). Static-builds to `apps/web/dist/`. |

## Quick links

- **Install & quick-start:** [`apps/cli/docs/production/01-installation.md`](./apps/cli/docs/production/) — also rendered at the docs site
- **CLI reference:** [`apps/cli/docs/production/04-cli-reference.md`](./apps/cli/docs/production/)
- **Codegen:** [`apps/web/content/docs/codegen.md`](./apps/web/content/docs/) (also surfaced on the site)
- **Contributing:** see `apps/cli/README` for the matrix harness + integration tests

## Developing

Both apps build independently. Workspace scripts at the root for convenience:

```bash
bun run build:cli    # go build -o pg-flux ./cmd/pg-flux
bun run test:cli     # go test ./... in apps/cli
bun run build:web    # bun run build in apps/web (renders the docs site to dist/)
bun run dev:web      # local docs site preview on http://localhost:3000
bun run build        # both
```

The Go CLI has no dependency on Bun; the docs site has no dependency on Go. Each app's CI / test surface is documented in its own README.

## Versions

- **Go:** 1.25+ (see `apps/cli/go.mod`)
- **Bun:** 1.3+ (see root `package.json`)
- **PostgreSQL:** 14, 15, 16, 17, 18 — full feature coverage matrix runs in CI

## License

See `LICENSE` at repo root.
