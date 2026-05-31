#!/usr/bin/env bash
# Uninstall sp-local-bridge.
set -euo pipefail

BINARY="sp-local-bridge"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

TARGET="${INSTALL_DIR}/${BINARY}"

if [[ ! -f "$TARGET" ]]; then
  echo "${BINARY} not found at ${TARGET}"
  exit 0
fi

if [[ -w "$INSTALL_DIR" ]]; then
  rm -f "$TARGET"
else
  sudo rm -f "$TARGET"
fi

echo "Removed ${TARGET}"
