# Multi-PG mutation matrix

This directory exercises pg-flux's diff/apply/drift loop against PG 14, 15, 16, 17, 18
through a sequence of 26 incremental schema mutations. Each step changes one feature
group; the harness expects every step to apply cleanly and leave `pg-flux drift`
returning **No drift** on every supported PG version.

## Prerequisites

5 Docker containers running on the local host (one per supported PG major):

```
docker run -d --name pgflux-test-pg14 -e POSTGRES_USER=pgflux -e POSTGRES_PASSWORD=pgflux -e POSTGRES_DB=pgflux -p 5441:5432 postgres:14
docker run -d --name pgflux-test-pg15 -e POSTGRES_USER=pgflux -e POSTGRES_PASSWORD=pgflux -e POSTGRES_DB=pgflux -p 5442:5432 postgres:15
docker run -d --name pgflux-test-pg16 -e POSTGRES_USER=pgflux -e POSTGRES_PASSWORD=pgflux -e POSTGRES_DB=pgflux -p 5443:5432 postgres:16
docker run -d --name pgflux-test     -e POSTGRES_USER=pgflux -e POSTGRES_PASSWORD=pgflux -e POSTGRES_DB=pgflux -p 5440:5432 postgres:17
docker run -d --name pgflux-test-pg18 -e POSTGRES_USER=pgflux -e POSTGRES_PASSWORD=pgflux -e POSTGRES_DB=pgflux -p 5444:5432 postgres:18
```

Build the binary first: `go build -o pg-flux ./cmd/pg-flux/`.

## Run

```sh
./test/matrix/run.sh
```

Summary is printed at the end; full per-step log lands at `/tmp/full_matrix_report.txt`.
Target outcome: **130 OK / 0 APPLY-FAIL / 0 GEN-FAIL / 0 DRIFT**.

## Steps

| #  | Mutation                                                       |
|----|----------------------------------------------------------------|
| 01 | Baseline (table + fn + trigger + RLS policy + view + sequence + enum + domain) |
| 02 | ADD COLUMN with DEFAULT                                        |
| 03 | RENAME COLUMN via `-- @renamed from=` hint                      |
| 04 | ALTER COLUMN TYPE (text → varchar(N), preserves DEFAULT)        |
| 05 | ADD CHECK constraint (auto-NOT VALID + VALIDATE)                |
| 06 | Function metadata: STABLE / PARALLEL SAFE / COST                |
| 07 | ALTER POLICY in-place USING clause                              |
| 08 | ALTER SEQUENCE params (START / INCREMENT)                       |
| 09 | ALTER TYPE ADD VALUE (enum)                                     |
| 10 | CREATE INDEX with INCLUDE + WHERE predicate                     |
| 11 | COMMENT ON table / column / index                               |
| 12 | GRANT to roles + PUBLIC                                         |
| 13 | SET NOT NULL on existing column                                 |
| 14 | DROP COLUMN (DATA_LOSS hazard)                                  |
| 15 | View body change (`= 'x'` → `IN ('x','y')`)                     |
| 16 | Trigger redefine (timing + WHEN clause)                         |
| 17 | ALTER DEFAULT PRIVILEGES                                        |
| 18 | REVOKE one priv (set-diff)                                      |
| 19 | Partitioned table + partition CREATE                            |
| 20 | IDENTITY columns (GENERATED ALWAYS / BY DEFAULT AS IDENTITY)    |
| 21 | GENERATED STORED column                                         |
| 22 | UNLOGGED table + WITH (fillfactor=70) reloptions                |
| 23 | EVENT TRIGGER ON ddl_command_end                                |
| 24 | Composite TYPE (CREATE)                                         |
| 25 | Composite TYPE ALTER ATTRIBUTE (add/widen)                      |
| 26 | ALTER OWNER TO                                                  |
