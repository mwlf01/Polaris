# Polaris

[![Release](https://img.shields.io/github/v/release/mwlf01/Polaris?style=flat-square)](https://github.com/mwlf01/Polaris/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/github/go-mod/go-version/mwlf01/Polaris?style=flat-square)](go.mod)

[English](#english) | [Deutsch](#deutsch)

---

## English

**Polaris** is a lightweight configuration management agent for Windows, written in Go. It ensures your system matches a desired state defined in YAML — similar to Ansible Pull or PowerShell DSC.

### Features

- **Packages** — Install, upgrade, and remove software via WinGet, Microsoft Store, and Chocolatey
- **Windows Update** — Configure auto-update mode, active hours, deferral policies
- **Windows Defender** — Manage real-time protection, cloud protection, exclusions, scan schedules
- **Registry** — Create, update, and delete registry keys and values
- **Group Policy** — Apply local group policy settings via LGPO.exe
- **Local Users** — Create, update, and remove local user accounts
- **Includes** — Split configuration across multiple YAML files or HTTP(S) URLs with caching, Basic Auth, and mTLS
- **Compatibility** — Per-file version, OS, architecture, and Windows version constraints
- **Self-Update** — Automatic self-update with SHA-256 verification, atomic swap, and rollback
- **Windows Service** — Run as a Windows service with configurable interval
- **Idempotent** — Only applies changes when the current state differs from the desired state

### Quick Start

Download the latest release from the [Releases](https://github.com/mwlf01/Polaris/releases) page, or build from source:

```powershell
git clone https://github.com/mwlf01/Polaris.git
cd Polaris
go build -o bin/polaris.exe ./cmd/polaris
.\bin\polaris.exe apply --config config.yaml
```

### Documentation

| Topic | Link |
|-------|------|
| Installation | [docs/en/installation.md](docs/en/installation.md) |
| Configuration & Includes | [docs/en/configuration.md](docs/en/configuration.md) |
| Package Management | [docs/en/packages.md](docs/en/packages.md) |
| System Modules | [docs/en/modules.md](docs/en/modules.md) |
| Windows Service | [docs/en/service.md](docs/en/service.md) |
| Self-Update | [docs/en/self-update.md](docs/en/self-update.md) |
| Troubleshooting | [docs/en/troubleshooting.md](docs/en/troubleshooting.md) |

---

## Deutsch

**Polaris** ist ein leichtgewichtiger Configuration-Management-Agent für Windows, geschrieben in Go. Er stellt sicher, dass dein System dem gewünschten Zustand entspricht, der in einer YAML-Datei definiert ist — vergleichbar mit Ansible Pull oder PowerShell DSC.

### Funktionen

- **Pakete** — Software installieren, aktualisieren und entfernen via WinGet, Microsoft Store und Chocolatey
- **Windows Update** — Auto-Update-Modus, aktive Stunden und Verzögerungsrichtlinien konfigurieren
- **Windows Defender** — Echtzeitschutz, Cloud-Schutz, Ausschlüsse und Scan-Zeitpläne verwalten
- **Registry** — Registry-Schlüssel und -Werte erstellen, ändern und löschen
- **Gruppenrichtlinien** — Lokale Gruppenrichtlinien über LGPO.exe anwenden
- **Lokale Benutzer** — Lokale Benutzerkonten anlegen, aktualisieren und entfernen
- **Includes** — Konfiguration auf mehrere YAML-Dateien oder HTTP(S)-URLs aufteilen, mit Caching, Basic Auth und mTLS
- **Kompatibilität** — Versions-, OS-, Architektur- und Windows-Versionseinschränkungen pro Datei
- **Selbst-Update** — Automatisches Self-Update mit SHA-256-Verifizierung, atomarem Tausch und Rollback
- **Windows-Dienst** — Als Windows-Dienst mit konfigurierbarem Intervall ausführen
- **Idempotent** — Änderungen nur bei Abweichung vom gewünschten Zustand

### Schnellstart

Lade die neueste Version von den [Releases](https://github.com/mwlf01/Polaris/releases) herunter, oder baue aus Quellcode:

```powershell
git clone https://github.com/mwlf01/Polaris.git
cd Polaris
go build -o bin/polaris.exe ./cmd/polaris
.\bin\polaris.exe apply --config config.yaml
```

### Dokumentation

| Thema | Link |
|-------|------|
| Installation | [docs/de/installation.md](docs/de/installation.md) |
| Konfiguration & Includes | [docs/de/configuration.md](docs/de/configuration.md) |
| Paketverwaltung | [docs/de/packages.md](docs/de/packages.md) |
| Systemmodule | [docs/de/modules.md](docs/de/modules.md) |
| Windows-Dienst | [docs/de/service.md](docs/de/service.md) |
| Selbst-Update | [docs/de/self-update.md](docs/de/self-update.md) |
| Fehlerbehebung | [docs/de/troubleshooting.md](docs/de/troubleshooting.md) |

---

## License / Lizenz

[MIT](LICENSE)
