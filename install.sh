#!/usr/bin/env sh
#
# pg-flux installer
#
#   curl -sSfL https://raw.githubusercontent.com/nexg/pg-flux/main/install.sh | sh
#
# Environment variables:
#   PGFLUX_VERSION  Tag to install (default: latest release).
#   PGFLUX_BIN_DIR  Where to put the binary (default: /usr/local/bin, or
#                   $HOME/.local/bin if /usr/local/bin isn't writable).
#   PGFLUX_NO_SUDO  If set, skip sudo even if the target dir isn't writable.
#

set -eu

REPO="nexg/pg-flux"
BIN_NAME="pg-flux"

red()    { printf "\033[31m%s\033[0m\n" "$1"; }
green()  { printf "\033[32m%s\033[0m\n" "$1"; }
blue()   { printf "\033[34m%s\033[0m\n" "$1"; }
muted()  { printf "\033[2m%s\033[0m\n" "$1"; }
err()    { red "error: $1" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || err "missing required tool: $1"
}

need curl
need tar
need uname
need mktemp

# ---------- OS / arch detection ----------------------------------------------

UNAME_OS="$(uname -s)"
UNAME_ARCH="$(uname -m)"

case "$UNAME_OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)      err "unsupported OS: $UNAME_OS (pg-flux ships binaries for macOS and Linux)" ;;
esac

case "$UNAME_ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) err "unsupported architecture: $UNAME_ARCH (supported: amd64, arm64)" ;;
esac

ASSET="pg-flux-${OS}-${ARCH}.tar.gz"

# ---------- resolve version --------------------------------------------------

VERSION="${PGFLUX_VERSION:-}"
if [ -z "$VERSION" ]; then
  blue ">> resolving latest release"
  VERSION="$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" \
    | awk -F'"' '/"tag_name"/ {print $4; exit}')" || true
  if [ -z "$VERSION" ]; then
    err "could not resolve latest release for ${REPO}. Set PGFLUX_VERSION=vX.Y.Z to install a specific tag."
  fi
fi

muted "   version: $VERSION"
muted "   target:  $OS/$ARCH"

# ---------- pick install dir -------------------------------------------------

BIN_DIR="${PGFLUX_BIN_DIR:-}"
if [ -z "$BIN_DIR" ]; then
  if [ -w "/usr/local/bin" ] || [ -z "${PGFLUX_NO_SUDO:-}" ]; then
    BIN_DIR="/usr/local/bin"
  else
    BIN_DIR="${HOME}/.local/bin"
    mkdir -p "$BIN_DIR"
  fi
fi

muted "   install: $BIN_DIR/$BIN_NAME"

# Need sudo?
SUDO=""
if [ ! -w "$BIN_DIR" ]; then
  if [ -n "${PGFLUX_NO_SUDO:-}" ]; then
    err "$BIN_DIR is not writable and PGFLUX_NO_SUDO is set. Set PGFLUX_BIN_DIR=\$HOME/.local/bin and re-run."
  fi
  if command -v sudo >/dev/null 2>&1; then
    SUDO="sudo"
    muted "   (using sudo to write to $BIN_DIR)"
  else
    err "$BIN_DIR is not writable and sudo isn't available. Set PGFLUX_BIN_DIR=\$HOME/.local/bin and re-run."
  fi
fi

# ---------- download + checksum ----------------------------------------------

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

blue ">> downloading $ASSET"
curl -sSfL -o "$TMP/$ASSET" "${BASE_URL}/${ASSET}" \
  || err "could not download ${BASE_URL}/${ASSET}"

blue ">> verifying checksum"
curl -sSfL -o "$TMP/SHA256SUMS" "${BASE_URL}/SHA256SUMS" \
  || err "could not download checksums file"

EXPECTED="$(awk -v a="$ASSET" '$2 == a {print $1}' "$TMP/SHA256SUMS")"
if [ -z "$EXPECTED" ]; then
  err "no checksum entry for $ASSET in SHA256SUMS"
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "$TMP/$ASSET" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "$TMP/$ASSET" | awk '{print $1}')"
else
  err "neither sha256sum nor shasum is available"
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  err "checksum mismatch for $ASSET (expected $EXPECTED, got $ACTUAL)"
fi
muted "   ok ($ACTUAL)"

# ---------- extract + install ------------------------------------------------

blue ">> installing"
( cd "$TMP" && tar -xzf "$ASSET" )

if [ ! -f "$TMP/$BIN_NAME" ]; then
  err "archive did not contain expected binary '$BIN_NAME'"
fi

chmod +x "$TMP/$BIN_NAME"
$SUDO mv "$TMP/$BIN_NAME" "$BIN_DIR/$BIN_NAME"

# ---------- verify -----------------------------------------------------------

if ! command -v "$BIN_NAME" >/dev/null 2>&1; then
  red "Installed to $BIN_DIR/$BIN_NAME but it isn't on your PATH."
  muted "Add this to your shell rc:"
  muted "  export PATH=\"$BIN_DIR:\$PATH\""
  exit 0
fi

INSTALLED="$($BIN_NAME version 2>/dev/null || true)"
green ">> done"
muted "   $INSTALLED"
muted "   try: pg-flux --help"
