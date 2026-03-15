# Polaris – Documentation (English)

Polaris is a lightweight configuration management agent for Windows, written in Go. It ensures your system matches a desired state defined in YAML.

## Quick Start

```powershell
# Build from source
go build -o bin/polaris.exe ./cmd/polaris

# Apply a configuration
.\bin\polaris.exe apply --config config.yaml
```

## Documentation

| # | Topic | Description |
|---|-------|-------------|
| 1 | [Installation](installation.md) | Prerequisites, download, build from source |
| 2 | [Configuration](configuration.md) | Config structure, includes, authentication, compatibility, merge behavior |
| 3 | [Packages](packages.md) | WinGet, Microsoft Store, Chocolatey — install, upgrade, remove |
| 4 | [System Modules](modules.md) | Windows Update, Defender, Users, Group Policy, Registry |
| 5 | [Windows Service](service.md) | Service management, schedule, SYSTEM user details |
| 6 | [Self-Update](self-update.md) | Automatic updates, manifest format, rollback |
| 7 | [Troubleshooting](troubleshooting.md) | Common issues, output indicators, full example |

## License

MIT – see [LICENSE](../../LICENSE).
