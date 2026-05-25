---
title: Installation
group: Getting started
order: 2
description: Get pg-flux on your machine — binary download, Go install, Docker, or build from source. Full Windows, macOS, and Linux coverage.
---

Pick the install path that matches your platform and taste. All of them give you the same `pg-flux` binary.

---

## macOS / Linux — quick install (recommended)

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

```bash
$ pg-flux version
pg-flux v0.1.6
```

> [!TIP]
> Pin to a specific version: `curl -sSfL https://... | PGFLUX_VERSION=v0.1.6 sh`
> Override the install dir: `... | PGFLUX_BIN_DIR=$HOME/.local/bin sh`

---

## Windows

### Option A — PowerShell one-liner (recommended)

Run in **PowerShell 5+** or **Windows Terminal**:

```powershell
# Installs to $HOME\.local\bin and adds it to your user PATH
$InstallDir = "$env:USERPROFILE\.local\bin"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$Release = (Invoke-RestMethod "https://api.github.com/repos/nex-gen-tech/pg-flux/releases/latest").tag_name
$Arch = if ([System.Environment]::Is64BitOperatingSystem -and $env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$Zip = "pg-flux-windows-$Arch.zip"
$Url = "https://github.com/nex-gen-tech/pg-flux/releases/latest/download/$Zip"

Invoke-WebRequest -Uri $Url -OutFile "$env:TEMP\$Zip"
Expand-Archive -Path "$env:TEMP\$Zip" -DestinationPath "$env:TEMP\pg-flux-extracted" -Force
Copy-Item "$env:TEMP\pg-flux-extracted\pg-flux.exe" -Destination "$InstallDir\pg-flux.exe" -Force

# Add to user PATH (permanent, no admin required)
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    $env:PATH += ";$InstallDir"
}

pg-flux version
```

> [!NOTE]
> No admin rights required. The binary installs to your user profile and PATH is updated only for your account. Restart your terminal after install to pick up the PATH change in new windows.

### Option B — Winget (coming soon)

```powershell
winget install nex-gen-tech.pg-flux
```

The Winget manifest is in progress. Watch the [GitHub Releases](https://github.com/nex-gen-tech/pg-flux/releases) page for updates.

### Option C — Manual download

1. Go to [GitHub Releases](https://github.com/nex-gen-tech/pg-flux/releases/latest).
2. Download `pg-flux-windows-amd64.zip` (Intel/AMD) or `pg-flux-windows-arm64.zip` (ARM/Snapdragon).
3. Extract the zip — it contains a single `pg-flux.exe`.
4. Move `pg-flux.exe` somewhere on your `PATH`, for example:
   - `%USERPROFILE%\.local\bin\` (user-local, no admin)
   - `C:\Program Files\pg-flux\` (system-wide, requires admin)
5. If the destination isn't on your `PATH` yet, add it:

```powershell
# User PATH — no admin required
$dir = "$env:USERPROFILE\.local\bin"
$cur = [Environment]::GetEnvironmentVariable("PATH", "User")
[Environment]::SetEnvironmentVariable("PATH", "$cur;$dir", "User")
```

### Verify the checksum (optional but recommended)

```powershell
$SumsUrl = "https://github.com/nex-gen-tech/pg-flux/releases/latest/download/SHA256SUMS"
Invoke-WebRequest -Uri $SumsUrl -OutFile "$env:TEMP\SHA256SUMS"

$Expected = (Get-Content "$env:TEMP\SHA256SUMS" | Select-String "windows-amd64").ToString().Split(" ")[0]
$Actual   = (Get-FileHash "$env:USERPROFILE\.local\bin\pg-flux.exe" -Algorithm SHA256).Hash.ToLower()

if ($Expected -eq $Actual) { Write-Host "Checksum OK" } else { Write-Warning "Checksum mismatch!" }
```

### WSL2 (easiest for developers)

If you already use WSL2 (Windows Subsystem for Linux), the macOS/Linux install path works unchanged inside your WSL terminal:

```bash
curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh | sh
```

WSL2 gives you the exact same experience as Linux — including running PostgreSQL natively in the WSL environment. Most Windows developers working with pg-flux use WSL2.

> [!TIP]
> PostgreSQL in WSL2: `sudo apt install postgresql` then connect with `postgres://localhost/yourdb`.
> The pg-flux binary installed in WSL2 is separate from the Windows binary — pick one or the other.

---

## Go install (all platforms)

If you already have Go 1.22+ installed:

```bash
go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest
```

The binary lands in `$GOBIN`. Make sure that directory is on your `PATH`:

**macOS / Linux:**
```bash
# add to ~/.bashrc or ~/.zshrc
export PATH="$PATH:$(go env GOPATH)/bin"
```

**Windows (PowerShell):**
```powershell
$GoBin = go env GOPATH | Join-Path -ChildPath "bin"
$cur = [Environment]::GetEnvironmentVariable("PATH", "User")
[Environment]::SetEnvironmentVariable("PATH", "$cur;$GoBin", "User")
```

Pin to a specific version with `@v0.1.6` instead of `@latest`.

---

## From source

For development, custom patches, or to verify the binary matches HEAD:

**macOS / Linux:**
```bash
git clone https://github.com/nex-gen-tech/pg-flux.git
cd pg-flux/apps/cli
go build -o pg-flux ./cmd/pg-flux
./pg-flux version
```

**Windows (PowerShell):**
```powershell
git clone https://github.com/nex-gen-tech/pg-flux.git
cd pg-flux\apps\cli
go build -o pg-flux.exe .\cmd\pg-flux
.\pg-flux.exe version
```

> [!NOTE]
> **CGO is required.** pg-flux links against `libpg_query` for SQL parsing.
>
> | Platform | Toolchain |
> |---|---|
> | macOS | Xcode Command Line Tools (`xcode-select --install`) |
> | Linux | `gcc` (`apt install build-essential` or equivalent) |
> | Windows | MSYS2 + MinGW-w64 (see below) or LLVM/Clang |
>
> **Windows toolchain setup:**
> 1. Install [MSYS2](https://www.msys2.org/)
> 2. In the MSYS2 shell: `pacman -S mingw-w64-x86_64-gcc`
> 3. Add `C:\msys64\mingw64\bin` to your `PATH`
> 4. In a new PowerShell: `go build -o pg-flux.exe .\cmd\pg-flux`
>
> Pre-built binaries from GitHub Releases do **not** require CGO or any C toolchain to run — only building from source does.

---

## Docker (coming in v0.2)

A container image is on the roadmap. For now, install via Go in your container:

```dockerfile
FROM golang:1.25 AS pgflux
RUN go install github.com/nex-gen-tech/pg-flux/cmd/pg-flux@latest

FROM debian:stable-slim
COPY --from=pgflux /go/bin/pg-flux /usr/local/bin/
ENTRYPOINT ["pg-flux"]
```

---

## Verify installation

```bash
pg-flux version
pg-flux --help
```

`--help` prints the full command tree. If you ever forget a flag, this is faster than the docs.

---

## Updating pg-flux

Once installed, you can update pg-flux from within itself:

```bash
# Update to the latest release
pg-flux update

# Pin to a specific version
pg-flux update --version v0.1.6
```

What `pg-flux update` does:

1. Fetches the latest (or requested) release tag from GitHub.
2. Downloads the matching binary for your OS and architecture.
3. Verifies the SHA-256 checksum against the release's `SHA256SUMS` file.
4. Atomically replaces the running binary in-place — no separate download step needed.

```bash
$ pg-flux update
Checking for updates...
Updating pg-flux v0.1.6 → v0.1.6
Fetching checksums...
Downloading pg-flux-darwin-arm64.tar.gz...
Downloaded 6.2 MB
Checksum verified
Installing to /usr/local/bin/pg-flux...
pg-flux updated to v0.1.6
```

> [!NOTE]
> If pg-flux is installed in a system directory (e.g. `/usr/local/bin`), you may need `sudo pg-flux update`.
> For user-local installs (`~/.local/bin`) no elevated permissions are needed.

---

## PostgreSQL requirements

pg-flux supports PostgreSQL **14 through 18**. Anything older isn't tested or supported.

| PG version | Status | Notable features pg-flux uses |
|---|---|---|
| **14** | Supported | LZ4 compression, multirange types |
| **15** | Supported | NULLS NOT DISTINCT, security_invoker views |
| **16** | Supported | Inline STORAGE in CREATE TABLE |
| **17** | Supported | WITHOUT OVERLAPS / PERIOD FK |
| **18** | Supported | Virtual generated columns, NOT ENFORCED constraints |
| **13 and earlier** | NOT supported | — |

pg-flux detects the server version on connect and gates version-specific syntax automatically.

---

## Optional: shadow database

Some commands (`migrate apply --shadow-dsn`) accept a disposable database for pre-flight validation:

```bash
docker run -d --name pgflux-shadow \
  -e POSTGRES_USER=shadow \
  -e POSTGRES_PASSWORD=shadow \
  -p 5433:5432 \
  postgres:17
```

```bash
export PGFLUX_SHADOW_DSN="postgres://shadow:shadow@localhost:5433/shadow?sslmode=disable"
```

**Windows (PowerShell):**
```powershell
$env:PGFLUX_SHADOW_DSN = "postgres://shadow:shadow@localhost:5433/shadow?sslmode=disable"
```

---

## Optional: development tooling

For working on pg-flux itself:

| Tool | Purpose |
|---|---|
| **Docker** | Spin up PG 14–18 containers for the matrix harness |
| **`psql`** | Required by the matrix harness |
| **Bun 1.3+** | Build the docs site |
| **MSYS2 + MinGW-w64** | Required to build from source on Windows |

The full dev setup is documented in [CONTRIBUTING.md](https://github.com/nex-gen-tech/pg-flux/blob/main/CONTRIBUTING.md).

---

## What's next?

- [Quick start →](/docs/quick-start.html) — the 5-minute version of actually using pg-flux
- [Configuration →](/docs/configuration.html) — what goes in `.pg-flux.yml`
