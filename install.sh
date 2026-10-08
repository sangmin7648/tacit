#!/bin/sh
set -e

REPO="sangmin7648/tacit"
INSTALL_DIR="$HOME/.local/bin"
APP_DIR="$HOME/Applications"
APP_ID="io.github.sangmin7648.tacit"

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
[ "$PLATFORM" = "darwin-arm64" ] || error "tacit is available for Apple Silicon Macs only"

# ── Refuse while tacit runs ─────────────────────────────
#
# Replacing the binary under a running daemon loses its macOS permissions the
# next time it restarts capture, so stop everything first.

if pgrep -xq Tacit; then
  error "Tacit is running. Quit it from its menu-bar icon, then run this again."
fi
PIDFILE="$HOME/.tacit/tacit.pid"
if [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null; then
  error "tacit is listening (PID $(cat "$PIDFILE")). Stop it with 'tacit stop', then run this again."
fi

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

ARCHIVE="Tacit-${VERSION}-${PLATFORM}.zip"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

info "Downloading $URL"
curl -fSL -o "$TMPDIR/$ARCHIVE" "$URL" ||
  error "Download failed: $URL (releases from before the Mac app have no $ARCHIVE)"
ditto -x -k "$TMPDIR/$ARCHIVE" "$TMPDIR"
[ -d "$TMPDIR/Tacit.app" ] || error "$ARCHIVE does not contain Tacit.app"

# ── Install ─────────────────────────────────────────────
#
# Tacit.app carries the CLI (Contents/Helpers/tacit), and $INSTALL_DIR/tacit
# is a link to it, so the terminal and the menu-bar app always run the same
# build. The app is ad-hoc signed, not notarized: fetched with curl it carries
# no quarantine flag, so Gatekeeper does not check it.

mkdir -p "$APP_DIR" "$INSTALL_DIR"
UPDATING=0
[ -d "$APP_DIR/Tacit.app" ] && UPDATING=1
rm -rf "$APP_DIR/Tacit.app"
ditto "$TMPDIR/Tacit.app" "$APP_DIR/Tacit.app"
xattr -dr com.apple.quarantine "$APP_DIR/Tacit.app" 2>/dev/null || true

ln -sfn "$APP_DIR/Tacit.app/Contents/Helpers/tacit" "$INSTALL_DIR/tacit"
# Before the app, the CLI was a standalone binary with this framework beside
# it. Keep it while `make install`'s tacit-dev still uses it.
[ -e "$INSTALL_DIR/tacit-dev" ] || rm -rf "$INSTALL_DIR/ten_vad.framework"

if [ "$UPDATING" = 1 ]; then
  # macOS ties an ad-hoc signed app's microphone grant to that exact build, so
  # the old grant no longer applies — yet System Settings still shows it on,
  # and the new build hears nothing. Clear it so Tacit can ask again.
  tccutil reset Microphone "$APP_ID" >/dev/null 2>&1 || true
  warn "Updated: Tacit will ask for the microphone again."
fi
info "Installed $APP_DIR/Tacit.app and linked $INSTALL_DIR/tacit to it"

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

info "Done! Open the menu-bar app with: open ~/Applications/Tacit.app"
info "Or use the CLI: tacit setup, then tacit listen"
