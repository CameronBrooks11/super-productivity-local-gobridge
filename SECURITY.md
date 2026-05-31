# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | Yes |
| Older releases | No |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email the maintainer or use GitHub's private vulnerability reporting
3. Include steps to reproduce and potential impact

## Scope

This bridge operates locally only:
- Connects to `127.0.0.1:3876` (Super Productivity desktop app)
- MCP server communicates via stdio (no network exposure)
- No credentials or secrets are stored
- No delete operations are exposed

See [docs/security.md](docs/security.md) for the full threat model.
