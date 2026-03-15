# Polaris – Dokumentation (Deutsch)

Polaris ist ein leichtgewichtiger Configuration-Management-Agent für Windows, geschrieben in Go. Er stellt sicher, dass dein System dem gewünschten Zustand entspricht, der in einer YAML-Datei definiert ist.

## Schnellstart

```powershell
# Aus Quellcode bauen
go build -o bin/polaris.exe ./cmd/polaris

# Konfiguration anwenden
.\bin\polaris.exe apply --config config.yaml
```

## Dokumentation

| # | Thema | Beschreibung |
|---|-------|--------------|
| 1 | [Installation](installation.md) | Voraussetzungen, Download, aus Quellcode bauen |
| 2 | [Konfiguration](configuration.md) | Konfigurationsaufbau, Includes, Authentifizierung, Kompatibilität, Merge-Verhalten |
| 3 | [Paketverwaltung](packages.md) | WinGet, Microsoft Store, Chocolatey — installieren, aktualisieren, entfernen |
| 4 | [Systemmodule](modules.md) | Windows Update, Defender, Benutzer, Gruppenrichtlinien, Registry |
| 5 | [Windows-Dienst](service.md) | Dienstverwaltung, Zeitplan, SYSTEM-Benutzer |
| 6 | [Selbst-Update](self-update.md) | Automatische Updates, Manifest-Format, Rollback |
| 7 | [Fehlerbehebung](troubleshooting.md) | Häufige Probleme, Ausgabe-Indikatoren, vollständiges Beispiel |

## Lizenz

MIT – siehe [LICENSE](../../LICENSE).
