# Contributing to pg-flux

So you want to help build the thing. Welcome. This page is the operating manual: how the codebase is laid out, how to run the tests, what the review bar is, and what's deliberately out of scope.

## Ground rules

- **Open an issue before a big PR.** Nobody enjoys closing a 2,000-line PR because the design went sideways. If your change touches more than ~200 LOC or affects the public CLI surface, sketch it in an issue first.
- **The matrix has to stay green.** 130/130 across PostgreSQL 14 – 18 is the project's North Star. If your change can break it, write a regression test before fixing the bug.
- **Tests come with the code, not after.** PRs that say "I'll add tests later" don't get merged.
- **Don't widen the API on a whim.** Public types in `pkg/` are commitments. If a function makes sense as an internal helper, leave it unexported.

## Repository layout

```text
pg-flux/
├── apps/
│   ├── cli/        ← the Go CLI — schema parser, inspector, differ,
│   │               migrator, dump/verify/pull, codegen pipeline
│   └── web/        ← the docs site + landing page
│                     (Bun + React + Tailwind + shadcn primitives)
├── .github/        ← CI workflows + issue/PR templates
├── README.md       ← project overview
├── CHANGELOG.md    ← release notes
├── ROADMAP.md      ← what's coming, what's deliberately out of scope
└── package.json    ← root Bun workspace
```

Most contributions land in `apps/cli/`. Docs PRs land in `apps/web/content/docs/`.

## Setup

You need:

- **Go 1.25+** for the CLI
- **Bun 1.3+** for the docs site
- **Docker** for the integration tests (spins up PG 14 – 18 containers via the matrix harness)
- **PostgreSQL client tools** (`psql`) for the matrix harness

```bash
git clone https://github.com/nexg/pg-flux.git
cd pg-flux

# build the CLI
bun run build:cli

# run unit tests
bun run test:cli

# run the docs site locally
bun run dev:web      # → http://localhost:3000 (rebuild on each request)
bun run build:web    # static build → apps/web/dist/
bun run preview      # serve the static build
```

## The dev loop

For most CLI work:

```bash
cd apps/cli
go test ./...                              # unit tests (fast, no DB)
go test -tags=integration ./pkg/codegen/   # against the live test container
```

The test container is `pgflux-test` on port `5440`. Start it once:

```bash
docker run -d --name pgflux-test \
  -e POSTGRES_USER=pgflux -e POSTGRES_PASSWORD=pgflux \
  -p 5440:5432 postgres:17
```

For PG14-18 matrix testing locally, spin up additional containers on ports 5441 (14), 5442 (15), 5443 (16), 5444 (18). The harness handles the rest:

```bash
BIN=$(pwd)/pg-flux bash test/matrix/run.sh
```

## What a good PR looks like

| Element | Why it matters |
|---|---|
| One change per PR | Reviewable in an hour, revertable in a minute |
| Unit test for the bug being fixed | Locks the bug shut |
| Updated docs (if behavior changed) | Future-you will thank past-you |
| Commit message explains *why*, not just *what* | The diff explains what |
| `go vet ./...` + `go test ./...` green | Don't break trunk |
| Matrix still 130/130 if your change touches differ / inspector / codegen | Required |

Commit messages follow [conventional commits](https://www.conventionalcommits.org/):

```text
feat(codegen): add VARIADIC parameter support for Go emitter
fix(differ): correct view dependency ordering when matview references function
docs: add CI integration recipe for GitLab CI
test(matrix): add fixture for partition ATTACH/DETACH
```

## Review process

1. PR opened → CI runs (unit tests on every push, matrix on push to main)
2. Maintainer reviews within a few days for routine fixes; longer for design changes
3. Squash-merge once approved + green
4. `CHANGELOG.md` updated at release time, not per-PR

## What we don't accept

- **Refactors without tests** that prove nothing broke
- **Cosmetic-only changes** (renaming variables, formatting tweaks) — bundle these with real work
- **Features that widen the CLI surface without an issue discussion** — every new flag is a forever commitment
- **Dependencies on services other than PostgreSQL** — pg-flux runs against PG, full stop
- **Breaking changes without a deprecation cycle** — even pre-1.0

## Out of scope

- **Other databases.** pg-flux is PostgreSQL-only by design. Adding MySQL support would mean abandoning every PG-specific feature (RLS, partition syntax, generated columns, NOT VALID + VALIDATE, `pg_query`). Use Atlas or Liquibase for cross-DB.
- **Query builder generation.** That's [sqlc](https://sqlc.dev/)'s job and it does it well. pg-flux generates *types*, not query glue. They compose.
- **Schema designers / GUI tools.** pg-flux is a CLI for the SQL you write by hand.
- **Hosted service.** Run pg-flux against your own database.

## Security issues

If you find a security vulnerability — anything that bypasses hazard gates, escalates DB privileges, or exposes credentials — see [SECURITY.md](./SECURITY.md). Do NOT open a public issue.

## Code of conduct

Be kind, be specific, assume good faith. Full text in [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md).

## License

By contributing, you agree your contributions will be licensed under the [MIT License](./LICENSE).
