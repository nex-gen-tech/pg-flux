---
title: Installation
group: Getting started
order: 2
description: Get pg-flux on your machine — Go install, binary, Docker, or build from source.
---

Pick the install path that matches your taste. All of them give you the same `pg-flux` binary; the differences are just how you get there.

## Quick install (recommended)

One command. No Go required.

```bash
curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh | sh
```

What it does:

1. Detects your OS (macOS or Linux) and arch (amd64 or arm64).
2. Resolves the latest release tag from GitHub.
3. Downloads the matching binary tarball.
4. Verifies the SHA-256 against the release's `SHA256SUMS` file.
5. Extracts and installs to `/usr/local/bin/pg-flux` (or `~/.local/bin` if `/usr/local/bin` isn't writable).
6. Runs `pg-flux version` to confirm.

Verify:

```bash
$ pg-flux version
pg-flux v0.1.0
```

> [!TIP]
> Pin to a specific version: `curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh | PGFLUX_VERSION=v0.1.0 sh`.
> Override the install directory: `... | PGFLUX_BIN_DIR=$HOME/.local/bin sh`.

## Manual binary download

If you'd rather not pipe a script into your shell, grab the binary directly from [GitHub Releases](https://github.com/nex-gen-tech/pg-flux/releases).

```bash
# macOS Apple Silicon
curl -sSfL -o pg-flux.tar.gz https://github.com/nex-gen-tech/pg-flux/releases/latest/download/pg-flux-darwin-arm64.tar.gz
tar -xzf pg-flux.tar.gz
sudo mv pg-flux /usr/local/bin/
pg-flux version

# macOS Intel
curl -sSfL -o pg-flux.tar.gz https://github.com/nex-gen-tech/pg-flux/releases/latest/download/pg-flux-darwin-amd64.tar.gz
tar -xzf pg-flux.tar.gz
sudo mv pg-flux /usr/local/bin/

# Linux x86_64
curl -sSfL -o pg-flux.tar.gz https://github.com/nex-gen-tech/pg-flux/releases/latest/download/pg-flux-linux-amd64.tar.gz
tar -xzf pg-flux.tar.gz
sudo mv pg-flux /usr/local/bin/

# Linux ARM64
curl -sSfL -o pg-flux.tar.gz https://github.com/nex-gen-tech/pg-flux/releases/latest/download/pg-flux-linux-arm64.tar.gz
tar -xzf pg-flux.tar.gz
sudo mv pg-flux /usr/local/bin/
```

Always verify the checksum if you download manually:

```bash
curl -sSfL -o SHA256SUMS https://github.com/nex-gen-tech/pg-flux/releases/latest/download/SHA256SUMS
shasum -a 256 -c SHA256SUMS --ignore-missing
```

> [!NOTE]
> Supported platforms are `darwin-arm64`, `darwin-amd64`, `linux-amd64`, and `linux-arm64`.
> Windows isn't shipped because the `libpg_query` C library doesn't build cleanly on it yet.

## Go install

If you already have Go 1.25+ and would rather build the binary yourself:

```bash
go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
```

The binary lands in `$GOBIN` (or `$GOPATH/bin` if `GOBIN` isn't set). Make sure that directory is on your `PATH`:

```bash
# add to your shell rc
export PATH="$PATH:$(go env GOBIN):$(go env GOPATH)/bin"
```

Pin to a specific tag with `@v0.1.0` instead of `@latest`.

## From source

If you want to build from source — for development, custom patches, or to verify the binary you're using matches HEAD:

```bash
git clone https://github.com/nex-gen-tech/pg-flux.git
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
RUN go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest

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

The full dev setup is documented in [CONTRIBUTING.md](https://github.com/nex-gen-tech/pg-flux/blob/main/CONTRIBUTING.md).

## What's next?

- [Quick start →](/docs/quick-start.html) — the 5-minute version of "actually using pg-flux"
- [Configuration →](/docs/configuration.html) — what goes in `.pg-flux.yml`
- [Privileges →](/docs/privileges.html) — what DB permissions pg-flux needs
