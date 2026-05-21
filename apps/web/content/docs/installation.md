---
title: Installation
group: Getting started
order: 2
description: Get pg-flux on your machine — Go install, binary, Docker, or build from source.
---

Pick the install path that matches your taste. All of them give you the same `pg-flux` binary; the differences are just how you get there.

## Go install (most direct)

If you have Go 1.25+ already, this is the shortest path:

```bash
go install github.com/nexg/pg-flux/cmd/pg-flux@latest
```

The binary lands in `$GOBIN` (or `$GOPATH/bin` if `GOBIN` isn't set). Make sure that directory is on your `PATH`:

```bash
# add to your shell rc
export PATH="$PATH:$(go env GOBIN):$(go env GOPATH)/bin"
```

Verify:

```bash
$ pg-flux version
pg-flux v0.1.0
```

> [!TIP]
> To pin to a specific version: `go install github.com/nexg/pg-flux/cmd/pg-flux@v0.1.0`.
> Replace `latest` with the tag you want.

## Binary release

For machines that don't have Go, grab a pre-built binary from [GitHub Releases](https://github.com/nexg/pg-flux/releases).

```bash
# Linux x86_64
curl -sSL https://github.com/nexg/pg-flux/releases/latest/download/pg-flux-linux-amd64.tar.gz \
  | tar xz
sudo mv pg-flux /usr/local/bin/
pg-flux version

# macOS Apple Silicon
curl -sSL https://github.com/nexg/pg-flux/releases/latest/download/pg-flux-darwin-arm64.tar.gz \
  | tar xz
sudo mv pg-flux /usr/local/bin/

# macOS Intel
curl -sSL https://github.com/nexg/pg-flux/releases/latest/download/pg-flux-darwin-amd64.tar.gz \
  | tar xz
sudo mv pg-flux /usr/local/bin/

# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/nexg/pg-flux/releases/latest/download/pg-flux-windows-amd64.zip" -OutFile pg-flux.zip
Expand-Archive pg-flux.zip -DestinationPath C:\bin
```

Binaries are statically linked and have no external dependencies beyond a libc.

## From source

If you want to build from source — for development, custom patches, or to verify the binary you're using matches HEAD:

```bash
git clone https://github.com/nexg/pg-flux.git
cd pg-flux/apps/cli
go build -o pg-flux ./cmd/pg-flux
./pg-flux version
```

The build takes ~30 seconds on a modern machine. No CGO required (with one exception — see below).

> [!NOTE]
> The `pg_query_go/v6` dependency includes a libpg_query C library that
> needs CGO at build time. Make sure your environment has `gcc` or `clang`
> installed. On most systems this is automatic; on slim Docker images
> you may need to add `build-essential`.

## Docker (coming in v0.2)

A container image is on the roadmap. For now, install via Go in your container:

```dockerfile
FROM golang:1.25 AS pgflux
RUN go install github.com/nexg/pg-flux/cmd/pg-flux@latest

FROM debian:stable-slim
COPY --from=pgflux /go/bin/pg-flux /usr/local/bin/
ENTRYPOINT ["pg-flux"]
```

## Verify

```bash
pg-flux version
pg-flux --help
```

`--help` prints the full command tree and every global flag. If you ever forget a flag, this is faster than the docs.

## PostgreSQL requirements

pg-flux supports PostgreSQL **14 through 18**. Anything older isn't tested or supported.

| PG version | Status | Notable features pg-flux uses |
|---|---|---|
| **14** | Supported | LZ4 compression, multirange types, named NOT VALID — basic baseline |
| **15** | Supported | NULLS NOT DISTINCT, security_invoker views, MERGE (DML, not directly used) |
| **16** | Supported | Inline STORAGE in CREATE TABLE, MAINTAIN privilege |
| **17** | Supported | WITHOUT OVERLAPS / PERIOD FK, event-trigger reindex events |
| **18** | Supported | Virtual generated columns, NOT ENFORCED constraints, named NOT NULL ... NOT VALID |
| **13 and earlier** | NOT supported | Missing features pg-flux relies on |

pg-flux detects the server version on connect and gates version-specific syntax. Trying to use a PG18 feature against PG14 fails at `migrate generate` time with a clear error, not at apply.

## Optional: shadow database

Some commands (`migrate apply --shadow-dsn`) accept a disposable database for pre-flight validation. Spin one up with Docker:

```bash
docker run -d --name pgflux-shadow \
  -e POSTGRES_USER=shadow \
  -e POSTGRES_PASSWORD=shadow \
  -p 5433:5432 \
  postgres:17
```

Then in your environment:

```bash
export PGFLUX_SHADOW_DSN="postgres://shadow:shadow@localhost:5433/shadow?sslmode=disable"
```

pg-flux will use this for shadow-validation when `--shadow-dsn` is passed (or the env var is set, depending on the command).

## Optional: development tooling

For working on pg-flux itself (not just using it):

| Tool | Purpose |
|---|---|
| **Docker** | Spin up PG 14, 15, 16, 17, 18 containers for the matrix harness |
| **`psql`** | Required by the matrix harness |
| **Bun 1.3+** | Build the docs site |
| **`entr`** | Optional, for auto-rebuild on file changes |

The full dev setup is documented in [CONTRIBUTING.md](https://github.com/nexg/pg-flux/blob/main/CONTRIBUTING.md).

## What's next?

- [Quick start →](/docs/quick-start.html) — the 5-minute version of "actually using pg-flux"
- [Configuration →](/docs/configuration.html) — what goes in `.pg-flux.yml`
- [Privileges →](/docs/privileges.html) — what DB permissions pg-flux needs
