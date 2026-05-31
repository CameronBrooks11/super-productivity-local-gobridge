#!/usr/bin/env bash
# Uninstall sp-local-bridge and its multicall aliases.
# Supported platforms: Linux, macOS.
set -euo pipefail

BINARY="sp-local-bridge"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
ALIASES=(sp-local-bridge-mcp sp-local-bridge-doctor sp-local-bridge-print-config sp-local-bridge-configure)

TARGET="${INSTALL_DIR}/${BINARY}"

if [[ ! -f "$TARGET" ]]; then
  echo "${BINARY} not found at ${TARGET}"
  echo "If installed elsewhere, set INSTALL_DIR: INSTALL_DIR=/path/to/bin ./uninstall.sh"
  exit 0
fi

# Offer host config cleanup BEFORE removing the binary
echo "Remove host config entries before uninstalling? (y/N)"
read -r REPLY
if [[ "$REPLY" =~ ^[Yy]$ ]]; then
  for host in claude-desktop vscode-copilot codex; do
    "${TARGET}" configure --remove "$host" 2>/dev/null || true
  done
  echo ""
fi

# Remove binary
rm -f "$TARGET"
echo "Removed ${TARGET}"

# Remove aliases
for alias in "${ALIASES[@]}"; do
  link="${INSTALL_DIR}/${alias}"
  if [[ -L "$link" || -f "$link" ]]; then
    rm -f "$link"
    echo "Removed ${link}"
  fi
done

echo ""
echo "✓ Uninstalled sp-local-bridge from ${INSTALL_DIR}"
