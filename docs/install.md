# Install

## Recommended: Install Script (Linux / macOS)

The install script downloads the latest release, verifies the SHA256 checksum, and installs to `~/.local/bin`:

```sh
curl -sSL https://raw.githubusercontent.com/CameronBrooks11/super-productivity-local-gobridge/main/scripts/install.sh | bash
```

The script auto-detects your OS and architecture. It fails closed if the checksum does not match.

## Manual Download from GitHub Releases

Download the appropriate archive for your platform from the [Releases page](https://github.com/CameronBrooks11/super-productivity-local-gobridge/releases).

### Platform Matrix

Release archives are versioned. Replace `VERSION` with the release version (e.g. `0.1.0`):

| Platform | Archive |
|----------|---------|
| Linux x86_64 | `sp-local-bridge_VERSION_linux_amd64.tar.gz` |
| Linux arm64 | `sp-local-bridge_VERSION_linux_arm64.tar.gz` |
| macOS arm64 (Apple Silicon) | `sp-local-bridge_VERSION_darwin_arm64.tar.gz` |
| macOS x86_64 (Intel) | `sp-local-bridge_VERSION_darwin_amd64.tar.gz` |
| Windows x86_64 | `sp-local-bridge_VERSION_windows_amd64.zip` |
| Windows arm64 | `sp-local-bridge_VERSION_windows_arm64.zip` |

### Linux / macOS

```sh
# Set the version you want to install
VERSION="0.1.0"

# Download archive and checksums
curl -LO "https://github.com/CameronBrooks11/super-productivity-local-gobridge/releases/download/v${VERSION}/sp-local-bridge_${VERSION}_linux_amd64.tar.gz"
curl -LO "https://github.com/CameronBrooks11/super-productivity-local-gobridge/releases/download/v${VERSION}/checksums.txt"

# Verify checksum
grep "sp-local-bridge_${VERSION}_linux_amd64.tar.gz" checksums.txt | sha256sum -c -

# Extract and install
tar xzf "sp-local-bridge_${VERSION}_linux_amd64.tar.gz"
chmod +x sp-local-bridge
sudo mv sp-local-bridge /usr/local/bin/
```

### Windows

1. Download the `.zip` for your architecture from the Releases page.
2. Download `checksums.txt` from the same release.
3. Verify the checksum matches (PowerShell: `Get-FileHash` or `certutil -hashfile`).
4. Extract `sp-local-bridge.exe`.
5. Move it to a directory in your `PATH`, or use the full path in host configs.

## From Source

Requires Go 1.23+:

```sh
go install github.com/CameronBrooks11/super-productivity-local-gobridge/cmd/sp-local-bridge@latest
```

::: info
`go install` builds without release metadata, so `sp-local-bridge --version` will show `dev`. For versioned builds, use release artifacts or clone and build with `make build`.
:::

Or clone and build:

```sh
git clone https://github.com/CameronBrooks11/super-productivity-local-gobridge.git
cd super-productivity-local-gobridge
make build
# Binary: ./sp-local-bridge
```

## Verify Installation

```sh
sp-local-bridge --version
sp-local-bridge doctor
```

## Upgrade

Download the new release binary and replace the existing one. Host configs do not need to change unless the binary path moves.

## Uninstall

1. Remove host configs first (while the binary is still available):
   ```sh
   sp-local-bridge configure --remove claude-desktop
   sp-local-bridge configure --remove vscode-copilot
   sp-local-bridge configure --remove codex
   ```

2. Remove the binary:
   ```sh
   sudo rm /usr/local/bin/sp-local-bridge
   ```

   Backup files (`.bak`) are created alongside host configs during `configure` and can be used to restore the original state if needed.
