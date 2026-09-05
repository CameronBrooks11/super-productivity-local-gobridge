#!/usr/bin/env bash
# Install sp-local-bridge from GitHub releases, or from a local checkout.
# Supported platforms: Linux, macOS. For Windows, download manually from Releases.
set -euo pipefail

REPO="CameronBrooks11/super-productivity-local-gobridge"
BINARY="sp-local-bridge"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# --from-source builds the checkout this script lives in, instead of downloading
# a release. Without it, running this script from a checkout still installs the
# published release, which silently ignores any commits made since the last tag.
FROM_SOURCE="${SP_FROM_SOURCE:-0}"
for arg in "$@"; do
  case "$arg" in
    --from-source) FROM_SOURCE=1 ;;
    -h|--help)
      echo "Usage: install.sh [--from-source]"
      echo
      echo "  --from-source   Build and install from this checkout rather than"
      echo "                  downloading the latest published release."
      echo
      echo "Environment: INSTALL_DIR, VERSION, SKIP_CHECKSUM, SP_FROM_SOURCE"
      exit 0
      ;;
    *) echo "Error: unknown option '$arg' (try --help)" >&2; exit 2 ;;
  esac
done

# Multicall aliases — the single binary handles all via subcommands,
# but symlinks allow direct invocation for MCP hosts.
ALIASES=(sp-local-bridge-mcp sp-local-bridge-doctor sp-local-bridge-print-config sp-local-bridge-configure)

# Detect OS and arch (SP_TEST_OS allows testing without network calls)
OS="${SP_TEST_OS:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
case "$OS" in
  linux|darwin) ;;
  *) echo "Error: install.sh supports Linux and macOS only. For Windows, download manually from Releases." >&2; exit 1 ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Error: Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if [[ "$FROM_SOURCE" == "1" ]]; then
  # Build the checkout containing this script.
  REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
  if [[ ! -f "$REPO_ROOT/go.mod" ]]; then
    echo "Error: --from-source needs a checkout; no go.mod at ${REPO_ROOT}." >&2
    exit 1
  fi
  command -v go >/dev/null 2>&1 || {
    echo "Error: --from-source needs the Go toolchain, which was not found." >&2
    echo "Install Go, or drop --from-source to fetch a prebuilt release." >&2
    exit 1
  }

  # Same version stamping as the Makefile, so `--version` reflects the commit
  # actually built rather than the tag it descends from.
  VERSION="$(git -C "$REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo "dev")"
  COMMIT="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")"
  DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  PKG="github.com/${REPO}/internal/version"

  echo "Building ${BINARY} ${VERSION} from ${REPO_ROOT}..."
  ( cd "$REPO_ROOT" && go build \
      -ldflags "-s -w -X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.Date=${DATE}" \
      -o "${TMP}/${BINARY}" ./cmd/sp-local-bridge )
else

# Normalize version (strip leading 'v' if present)
if [[ -n "${VERSION:-}" ]]; then
  VERSION="${VERSION#v}"
else
  # Get latest stable version from GitHub API; fall back to newest pre-release
  VERSION="$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')" || true
  if [[ -z "$VERSION" ]]; then
    VERSION="$(curl -sL "https://api.github.com/repos/${REPO}/releases" \
      | grep '"tag_name"' | head -1 | sed -E 's/.*"v([^"]+)".*/\1/')" || true
  fi
  if [[ -z "$VERSION" ]]; then
    echo "Error: Failed to determine latest version from GitHub API." >&2
    echo "Set VERSION=x.y.z explicitly or check your network." >&2
    exit 1
  fi
fi

echo "Installing ${BINARY} v${VERSION} (${OS}/${ARCH}) to ${INSTALL_DIR}..."

# Determine archive format (GoReleaser uses .zip for Windows, .tar.gz otherwise)
# Note: Windows is rejected by the OS check above; this branch exists for
# completeness if Windows support is added to the script in the future.
if [[ "$OS" == "windows" ]]; then
  ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.zip"
else
  ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
fi
CHECKSUMS="checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"

echo "Downloading ${ARCHIVE}..."
curl -fSL "${BASE_URL}/${ARCHIVE}" -o "${TMP}/${ARCHIVE}"
echo "Downloading checksums..."
curl -fSL "${BASE_URL}/${CHECKSUMS}" -o "${TMP}/${CHECKSUMS}"

# Verify checksum
echo "Verifying checksum..."
EXPECTED="$(grep "${ARCHIVE}" "${TMP}/${CHECKSUMS}" | awk '{print $1}')"
if [[ -z "$EXPECTED" ]]; then
  if [[ "${SKIP_CHECKSUM:-}" == "1" ]]; then
    echo "Warning: No checksum found, skipping (SKIP_CHECKSUM=1)." >&2
  else
    echo "Error: No checksum found for ${ARCHIVE} in checksums.txt" >&2
    echo "Set SKIP_CHECKSUM=1 to bypass (not recommended)." >&2
    exit 1
  fi
else
  # Use sha256sum (Linux) or shasum (macOS) for checksum
  if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL="$(sha256sum "${TMP}/${ARCHIVE}" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    ACTUAL="$(shasum -a 256 "${TMP}/${ARCHIVE}" | awk '{print $1}')"
  else
    echo "Error: Neither sha256sum nor shasum found. Cannot verify checksum." >&2
    exit 1
  fi
  if [[ "$ACTUAL" != "$EXPECTED" ]]; then
    echo "Error: Checksum mismatch!" >&2
    echo "  Expected: ${EXPECTED}" >&2
    echo "  Actual:   ${ACTUAL}" >&2
    echo "The downloaded file may be corrupted. Aborting." >&2
    exit 1
  fi
  echo "Checksum OK."
fi

# Extract
if [[ "$ARCHIVE" == *.zip ]]; then
  unzip -q "${TMP}/${ARCHIVE}" -d "$TMP"
else
  tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"
fi

fi

# Validate the binary runs (both paths converge here).
if ! "${TMP}/${BINARY}" --version >/dev/null 2>&1; then
  echo "Error: Binary failed to execute. Aborting." >&2
  exit 1
fi

# Create install directory if needed
mkdir -p "$INSTALL_DIR"

# Install binary
install -m 755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"

# Create multicall symlinks
for alias in "${ALIASES[@]}"; do
  ln -sf "${BINARY}" "${INSTALL_DIR}/${alias}"
done

echo ""
echo "✓ Installed ${BINARY} v${VERSION#v} to ${INSTALL_DIR}/${BINARY}"
echo "  Aliases: ${ALIASES[*]}"
echo ""

# Check PATH
if ! echo "$PATH" | tr ':' '\n' | grep -qx "$INSTALL_DIR"; then
  echo "Warning: ${INSTALL_DIR} is not in your PATH." >&2
  echo "Add this to your shell profile:" >&2
  echo "  export PATH=\"${INSTALL_DIR}:\$PATH\"" >&2
  echo ""
fi

# Next steps
echo "Next steps:"
echo "  1. sp-local-bridge doctor        — verify SP app connection"
echo "  2. sp-local-bridge configure <host> — configure an MCP host"
echo "     Supported hosts: claude-code, claude-desktop, vscode-copilot, codex"
