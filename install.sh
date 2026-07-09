#!/bin/sh
set -e

REPO="sangmin7648/tacit"
INSTALL_DIR="$HOME/.local/bin"

# ── Helpers ─────────────────────────────────────────────

info()  { printf "\033[1;34m==>\033[0m %s\n" "$1"; }
warn()  { printf "\033[1;33m==>\033[0m %s\n" "$1"; }
error() { printf "\033[1;31m==>\033[0m %s\n" "$1" >&2; exit 1; }

# ── Detect platform ─────────────────────────────────────

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) error "Unsupported architecture: $ARCH" ;;
esac

PLATFORM="${OS}-${ARCH}"
info "Detected platform: $PLATFORM"

# ── Resolve version ─────────────────────────────────────

VERSION="${TACIT_VERSION:-latest}"
if [ "$VERSION" = "latest" ]; then
  # Resolve the latest tag from the github.com /releases/latest redirect
  # (302 -> /releases/tag/vX.Y.Z) rather than the api.github.com REST API.
  # The REST API is rate-limited to 60 requests/hour per IP for unauthenticated
  # callers and frequently returns 403 behind shared/NAT IPs, which blocked
  # `tacit update`. The web redirect has no such limit. Pin with TACIT_VERSION
  # to skip resolution entirely.
  VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
    "https://github.com/${REPO}/releases/latest" | sed -n 's#.*/releases/tag/##p')
  [ -n "$VERSION" ] || error "Failed to resolve latest version. Set TACIT_VERSION=vX.Y.Z to pin a version."
fi
info "Installing tacit $VERSION"

# ── Download ────────────────────────────────────────────

ARCHIVE="tacit-${VERSION}-${PLATFORM}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

info "Downloading $URL"
curl -fSL -o "$TMPDIR/$ARCHIVE" "$URL" || error "Download failed. Check version/platform: $URL"
tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR"

# ── Install ─────────────────────────────────────────────

mkdir -p "$INSTALL_DIR"

cp "$TMPDIR/tacit" "$INSTALL_DIR/tacit"
chmod +x "$INSTALL_DIR/tacit"

# Bundle ten_vad.framework (macOS)
if [ "$OS" = "darwin" ] && [ -d "$TMPDIR/ten_vad.framework" ]; then
  rm -rf "$INSTALL_DIR/ten_vad.framework"
  cp -R "$TMPDIR/ten_vad.framework" "$INSTALL_DIR/ten_vad.framework"
fi

# Remove macOS quarantine and apply ad-hoc signature (Gatekeeper)
if [ "$OS" = "darwin" ]; then
  xattr -dr com.apple.quarantine "$INSTALL_DIR/tacit" 2>/dev/null || true
  if [ -d "$INSTALL_DIR/ten_vad.framework" ]; then
    xattr -dr com.apple.quarantine "$INSTALL_DIR/ten_vad.framework" 2>/dev/null || true
  fi
  # Ad-hoc sign to satisfy macOS Sequoia (15+) Gatekeeper even without notarization
  codesign --force --deep --sign - "$INSTALL_DIR/tacit" 2>/dev/null || true
  if [ -d "$INSTALL_DIR/ten_vad.framework" ]; then
    codesign --force --deep --sign - "$INSTALL_DIR/ten_vad.framework" 2>/dev/null || true
  fi
fi

# ── PATH check ──────────────────────────────────────────

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    warn "$INSTALL_DIR is not in your PATH."
    echo ""
    echo "  Add this to your shell profile (~/.zshrc or ~/.bashrc):"
    echo ""
    echo "    export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
    ;;
esac

info "Done! Run 'tacit --help' to get started."
