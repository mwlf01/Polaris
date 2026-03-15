# Selbst-Update

Polaris kann sich automatisch aktualisieren. Die `update`-Sektion verweist auf ein **JSON-Manifest**, das die neueste Version beschreibt:

```yaml
update:
  url: "https://releases.example.com/polaris/update.json"
```

---

## Update-Manifest

Das Manifest ist eine einzelne JSON-Datei, die **alle Plattformen und Architekturen** abdeckt. Polaris wählt automatisch den Eintrag passend zu seinem `runtime.GOOS/runtime.GOARCH`:

```json
{
  "version": "1.2.3",
  "binaries": {
    "windows/amd64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-windows-amd64.exe",
      "sha256": "a1b2c3d4..."
    },
    "windows/arm64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-windows-arm64.exe",
      "sha256": "e5f6a7b8..."
    },
    "linux/amd64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-linux-amd64",
      "sha256": "c9d0e1f2..."
    },
    "linux/arm64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-linux-arm64",
      "sha256": "a3b4c5d6..."
    },
    "darwin/amd64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-darwin-amd64",
      "sha256": "e7f8a9b0..."
    },
    "darwin/arm64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-darwin-arm64",
      "sha256": "c1d2e3f4..."
    }
  }
}
```

| Feld               | Beschreibung |
|--------------------|--------------|
| `version`          | Die neue Versionszeichenkette (wird mit der laufenden Version verglichen) |
| `binaries`         | Map von `os/arch` → Binäreintrag |
| `binaries.*.url`   | Download-URL für die Binärdatei |
| `binaries.*.sha256` | SHA-256-Prüfsumme zur Integritätsverifizierung |

Das Schlüsselformat verwendet Go's `runtime.GOOS` und `runtime.GOARCH` Werte (z.B. `windows/amd64`, `linux/arm64`, `darwin/arm64`). Neue Plattformen oder Architekturen können jederzeit hinzugefügt werden — Polaris überspringt das Update einfach, wenn kein passender Eintrag vorhanden ist.

---

## Authentifizierung

Die Update-URL unterstützt die gleichen `auth`-Optionen wie Includes (Basic Auth und mTLS):

```yaml
update:
  url: "https://secure.example.com/polaris/update.json"
  auth:
    type: basic
    username: "polaris"
    password: "secret"
```

---

## Funktionsweise

1. **Prüfung** — Am Ende jedes Dienst-Zyklus (oder via `polaris update`) ruft Polaris das Manifest ab und vergleicht die Version.
2. **Download & Verifizierung** — Die neue Binärdatei wird heruntergeladen und ihre SHA-256-Prüfsumme verifiziert.
3. **Atomarer Tausch** — Die aktuelle Binärdatei wird in `.bak` (Backup) umbenannt und die neue an ihre Stelle gesetzt.
4. **Dienst-Neustart** — Im Dienst-Modus beendet sich der Prozess mit einem Fehlercode, wodurch der SCM-Recovery-Mechanismus den Dienst mit der neuen Binärdatei neu startet.
5. **Rollback-Schutz** — Beim Start validiert sich die neue Binärdatei selbst. Wenn sie nach 3 Versuchen nicht startet oder die Update-Zustandsdatei älter als 10 Minuten ist, wird das Backup automatisch wiederhergestellt.

---

## Rollback

Der Update-Prozess ist für **Zero-Touch-Deployments** konzipiert, bei denen kein manueller Eingriff möglich ist:

- Die vorherige Binärdatei wird immer als `.bak`-Datei aufbewahrt, bis das Update als erfolgreich bestätigt ist.
- Ein Startup-Guard zählt, wie oft die neue Binärdatei gestartet wurde. Nach 3 fehlgeschlagenen Starts wird das Backup automatisch wiederhergestellt.
- Ein Watchdog-Timer löst ein automatisches Rollback aus, wenn die Update-Zustandsdatei älter als 10 Minuten ist.
- Wenn die neue Binärdatei abstürzt, bevor sie den Rollback-Check erreicht, startet der SCM-Recovery sie neu — und der Rollback-Guard greift beim nächsten Versuch.

---

## Manuelles Update

```powershell
polaris update
```

Dies lädt die Konfiguration, prüft das Manifest und wendet das Update an, falls verfügbar.

---

Zurück: [Windows-Dienst](service.md) · Weiter: [Fehlerbehebung](troubleshooting.md)
