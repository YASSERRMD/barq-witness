#!/usr/bin/env sh
# barq-witness installer for macOS and Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/yasserrmd/barq-witness/main/install.sh | sh
# Or with a specific version: curl -fsSL ... | BARQ_VERSION=v1.1.1 sh
# Or to a custom dir:        curl -fsSL ... | BARQ_INSTALL_DIR=~/.local/bin sh

set -e

REPO="yasserrmd/barq-witness"
BINARY="barq-witness"
INSTALL_DIR="${BARQ_INSTALL_DIR:-/usr/local/bin}"
VERSION="${BARQ_VERSION:-}"

# Colors (only when stdout is a terminal)
if [ -t 1 ]; then
  GREEN="\033[32m"; YELLOW="\033[33m"; RED="\033[31m"; RESET="\033[0m"; BOLD="\033[1m"
else
  GREEN=""; YELLOW=""; RED=""; RESET=""; BOLD=""
fi

info()  { printf "${GREEN}[barq-witness]${RESET} %s\n" "$*"; }
warn()  { printf "${YELLOW}[barq-witness]${RESET} %s\n" "$*"; }
error() { printf "${RED}[barq-witness] ERROR:${RESET} %s\n" "$*" >&2; exit 1; }

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS="linux"  ;;
  Darwin*) OS="darwin" ;;
  *)       error "Unsupported OS: $OS. Please download manually from https://github.com/${REPO}/releases" ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) error "Unsupported architecture: $ARCH. Please download manually from https://github.com/${REPO}/releases" ;;
esac

info "Detected: ${OS}/${ARCH}"

# Resolve latest version if not set
if [ -z "$VERSION" ]; then
  info "Fetching latest release version..."
  if command -v curl > /dev/null 2>&1; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  elif command -v wget > /dev/null 2>&1; then
    VERSION="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
  else
    error "Neither curl nor wget found. Please install one of them and retry."
  fi
  [ -z "$VERSION" ] && error "Could not determine latest version. Set BARQ_VERSION manually."
fi

info "Installing barq-witness ${VERSION}..."

# Construct download URL
ASSET_NAME="${BINARY}-${OS}-${ARCH}"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"

# Download
TMP_FILE="$(mktemp)"
trap 'rm -f "$TMP_FILE"' EXIT

info "Downloading ${DOWNLOAD_URL}"
if command -v curl > /dev/null 2>&1; then
  curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE" || error "Download failed. Check that ${VERSION} exists at https://github.com/${REPO}/releases"
elif command -v wget > /dev/null 2>&1; then
  wget -qO "$TMP_FILE" "$DOWNLOAD_URL" || error "Download failed. Check that ${VERSION} exists at https://github.com/${REPO}/releases"
fi

chmod +x "$TMP_FILE"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY}"
else
  warn "Writing to ${INSTALL_DIR} requires sudo..."
  sudo mv "$TMP_FILE" "${INSTALL_DIR}/${BINARY}"
fi

# Verify
if command -v barq-witness > /dev/null 2>&1; then
  INSTALLED_VERSION="$(barq-witness version 2>/dev/null || true)"
  info "Installation complete: ${INSTALLED_VERSION}"
else
  info "Installed to ${INSTALL_DIR}/${BINARY}"
  warn "Make sure ${INSTALL_DIR} is in your PATH."
  info "  Add to your shell profile: export PATH=\"\$PATH:${INSTALL_DIR}\""
fi

printf "\n${BOLD}Quick start:${RESET}\n"
printf "  cd your-project\n"
printf "  barq-witness init\n"
printf "  barq-witness report\n\n"
printf "Docs: https://github.com/${REPO}\n\n"
