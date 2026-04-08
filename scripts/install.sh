#!/usr/bin/env sh
# barq-witness installer
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/YASSERRMD/barq-witness/main/scripts/install.sh | sh
#
# Options (environment variables):
#   BARQ_VERSION   - Release tag to install (default: v0.1.0)
#   BARQ_DIR       - Installation directory (default: /usr/local/bin)

set -e

VERSION="${BARQ_VERSION:-v0.1.0}"
INSTALL_DIR="${BARQ_DIR:-/usr/local/bin}"
REPO="YASSERRMD/barq-witness"

# Detect OS and architecture.
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

case "$OS" in
  linux|darwin) ;;
  *)
    echo "Unsupported OS: $OS" >&2
    exit 1
    ;;
esac

BINARY="barq-witness-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"
DEST="${INSTALL_DIR}/barq-witness"

echo "Installing barq-witness ${VERSION} (${OS}/${ARCH}) to ${DEST} ..."

# Download.
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$DEST" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$DEST" "$URL"
else
  echo "curl or wget is required to install barq-witness." >&2
  exit 1
fi

chmod +x "$DEST"

echo "barq-witness installed successfully."
echo ""
"$DEST" version
echo ""
echo "Run 'barq-witness init' inside a git repository to get started."
