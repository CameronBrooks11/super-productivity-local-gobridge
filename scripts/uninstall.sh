#!/usr/bin/env bash
# Uninstall sp-local-bridge and its multicall aliases.
# Supported platforms: Linux, macOS.
#
# Options:
#   --remove-configs   Remove host config entries without prompting
#   --keep-configs     Skip host config removal without prompting
#   REMOVE_CONFIGS=1   Environment variable equivalent to --remove-configs
set -euo pipefail

BINARY="sp-local-bridge"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
ALIASES=(sp-local-bridge-mcp sp-local-bridge-doctor sp-local-bridge-print-config sp-local-bridge-configure)

# Parse flags
REMOVE_CONFIGS="${REMOVE_CONFIGS:-}"
KEEP_CONFIGS=""
for arg in "$@"; do
  case "$arg" in
    --remove-configs) REMOVE_CONFIGS=1 ;;
    --keep-configs) KEEP_CONFIGS=1 ;;
  esac
done

TARGET="${INSTALL_DIR}/${BINARY}"

if [[ ! -f "$TARGET" ]]; then
  echo "${BINARY} not found at ${TARGET}"
  echo "If installed elsewhere, set INSTALL_DIR: INSTALL_DIR=/path/to/bin ./uninstall.sh"
  exit 0
fi

# Determine whether to remove host configs
do_remove_configs=""
if [[ -n "$REMOVE_CONFIGS" ]]; then
  do_remove_configs=1
elif [[ -z "$KEEP_CONFIGS" ]]; then
  # Prompt only if stdin is a terminal; default to no otherwise
  if [ -t 0 ]; then
    echo "Remove host config entries before uninstalling? (y/N)"
    read -r REPLY || REPLY=""
    if [[ "$REPLY" =~ ^[Yy]$ ]]; then
      do_remove_configs=1
    fi
  fi
fi

if [[ -n "$do_remove_configs" ]]; then
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
