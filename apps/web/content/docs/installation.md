---
title: Installation
group: Getting started
order: 2
---

# Installation

## Go install

Requires Go 1.25+:

```bash
go install github.com/nexg/pg-flux/cmd/pg-flux@latest
```

The binary lands in `$GOBIN` (or `$GOPATH/bin`). Ensure that directory is on your `PATH`:

```bash
export PATH="$PATH:$(go env GOBIN || go env GOPATH)/bin"
```

## Binary release

Pre-built binaries for Linux, macOS, and Windows are on the [releases page](https://github.com/nexg/pg-flux/releases). Extract and put `pg-flux` somewhere on your `PATH`.

```bash
curl -sSL https://github.com/nexg/pg-flux/releases/latest/download/pg-flux-linux-amd64.tar.gz | tar xz
sudo mv pg-flux /usr/local/bin/
pg-flux version
```

## From source

```bash
git clone https://github.com/nexg/pg-flux.git
cd pg-flux/apps/cli
go build -o pg-flux ./cmd/pg-flux
./pg-flux version
```

## Verify

```bash
pg-flux version
pg-flux --help
```

`--help` prints the full command tree and every global flag.

## PostgreSQL requirements

| Version | Status |
|---|---|
| 14 | Fully supported |
| 15 | Fully supported (NULLS NOT DISTINCT, security_invoker views) |
| 16 | Fully supported (inline STORAGE in CREATE TABLE) |
| 17 | Fully supported (WITHOUT OVERLAPS, MAINTAIN privilege gated) |
| 18 | Fully supported (virtual generated, named NOT NULL NOT VALID, NOT ENFORCED) |
| 13 and earlier | Not supported |

pg-flux detects the server version on every connect and gates version-specific syntax accordingly. Trying to apply a PG18-only feature against a PG14 server fails loudly at `migrate generate` time, not at apply.

## Optional: shadow database

Some commands accept `--shadow-dsn` to validate migrations against a disposable DB before applying to the real one. Spin one up with Docker:

```bash
docker run -d --name pgflux-shadow \
  -e POSTGRES_USER=shadow -e POSTGRES_PASSWORD=shadow \
  -p 5433:5432 postgres:17
```

Then pass `--shadow-dsn=postgres://shadow:shadow@localhost:5433/shadow?sslmode=disable` to migration commands.

## What's next?

- [Quick start →](/docs/quick-start.html)
- [Configuration →](/docs/configuration.html)
