# pg_query / SQL parser limitations

pg-flux uses [pg_query_go](https://github.com/pganalyze/pg_query_go) (libpg_query) for parse, deparse, and fingerprints. Behavior follows upstream postgres parser support in the linked library version.

## Standard SQL and newer PostgreSQL features

- Some **SQL:20xx** column constraint forms (for example certain `ENFORCED` / `NOT ENFORCED` spellings) may **fail to parse** or differ from server behavior until libpg_query tracks the same grammar as the target server.
- When `--validate-sql` is enabled, a failed parse surfaces as a load error with line context.
- Workarounds: express the same constraint in a form PostgreSQL and pg_query both accept, or load the object from a **live** database via `inspect` and maintain SQL that round-trips.

## Upstream tracking

- Parser gaps are resolved by **bumping pg_query_go** / libpg_query in `go.mod` when releases add the needed grammar.
- For project-wide backlog items, see `docs/prd/08-implementation-roadmap.md`.
