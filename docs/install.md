# Install

## From GitHub Releases

Download the appropriate binary for your platform from the [Releases page](https://github.com/CameronBrooks11/super-productivity-local-gobridge/releases).

### Platform Matrix

| Platform | Archive | Binary |
|----------|---------|--------|
| Linux x86_64 | `sp-local-bridge_linux_amd64.tar.gz` | `sp-local-bridge` |
| Linux arm64 | `sp-local-bridge_linux_arm64.tar.gz` | `sp-local-bridge` |
| macOS arm64 (Apple Silicon) | `sp-local-bridge_darwin_arm64.tar.gz` | `sp-local-bridge` |
| macOS x86_64 (Intel) | `sp-local-bridge_darwin_amd64.tar.gz` | `sp-local-bridge` |
| Windows x86_64 | `sp-local-bridge_windows_amd64.zip` | `sp-local-bridge.exe` |

### Linux / macOS

```sh
# Download and extract (example: Linux x86_64)
curl -Lo sp-local-bridge.tar.gz \
  https://github.com/CameronBrooks11/super-productivity-local-gobridge/releases/latest/download/sp-local-bridge_linux_amd64.tar.gz
tar xzf sp-local-bridge.tar.gz
chmod +x sp-local-bridge

# Move to PATH
sudo mv sp-local-bridge /usr/local/bin/
```

### Windows

1. Download the `.zip` for your architecture.
2. Extract `sp-local-bridge.exe`.
3. Move it to a directory in your `PATH`, or use the full path in host configs.

## From Source

Requires Go 1.23+:

```sh
go install github.com/CameronBrooks11/super-productivity-local-gobridge/cmd/sp-local-bridge@latest
```

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

1. Remove the binary:
   ```sh
   sudo rm /usr/local/bin/sp-local-bridge
   ```

2. Optionally remove host configs:
   ```sh
   sp-local-bridge configure --remove claude-desktop
   sp-local-bridge configure --remove vscode-copilot
   sp-local-bridge configure --remove codex
   ```

   If you already removed the binary, manually edit or delete the host config files. Backup files (`.bak`) are created alongside configs during `configure` and can be used to restore the original state.
