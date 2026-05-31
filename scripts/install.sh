#!/usr/bin/env bash
# Install sp-local-bridge from GitHub releases.
set -euo pipefail

REPO="CameronBrooks11/super-productivity-local-gobridge"
BINARY="sp-local-bridge"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
VERSION="${VERSION:-$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')}"
if [[ -z "$VERSION" ]]; then
  echo "Failed to determine latest version"
  exit 1
fi

echo "Installing ${BINARY} v${VERSION} (${OS}/${ARCH})..."

# Download
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -sL "$URL" -o "${TMP}/${ARCHIVE}"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

# Install
if [[ -w "$INSTALL_DIR" ]]; then
  install -m 755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  sudo install -m 755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
"${INSTALL_DIR}/${BINARY}" --version
