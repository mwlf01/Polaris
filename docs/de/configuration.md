# Konfiguration

Polaris liest eine YAML-Datei ein und gleicht den gewünschten Zustand mit dem System ab. Wird Polaris ohne Argumente gestartet, sucht es automatisch nach einer `config.yaml` neben der ausführbaren Datei. Wird dort keine gefunden, erscheint eine Fehlermeldung.

```powershell
# Einfachste Verwendung — findet config.yaml neben der exe automatisch
.\polaris.exe

# Äquivalenter expliziter Unterbefehl
.\polaris.exe apply

# Expliziter Pfad
.\polaris.exe apply --config C:\Polaris\meine-config.yaml
```

## Aufbau

```yaml
packages:
  - name: Anzeigename
    id: Paket-ID
    version: "1.0.0"       # optional
    source: winget          # "winget", "msstore" oder "choco"
    state: present          # "present" oder "absent"
```

## Validierung

Polaris prüft die Konfiguration vor der Ausführung. Fehlende Pflichtfelder oder ungültige Werte führen zu einer Fehlermeldung, bevor Änderungen vorgenommen werden.

---

## Includes

Konfigurationsdateien können andere YAML-Dateien einbinden — sowohl **lokale Pfade** als auch **HTTP(S)-URLs**. So lassen sich Konfigurationen modular aufteilen und wiederverwenden. Eingebundene Dateien können ihrerseits weitere Dateien einbinden (rekursiv). Zirkuläre Referenzen werden erkannt und mit einer Fehlermeldung abgelehnt.

```yaml
includes:
  - packages.yaml
  - security/defender.yaml
  - ../shared/registry.yaml
  - https://example.com/polaris/base-packages.yaml
```

### Pfadauflösung

Lokale Pfade werden relativ zum Verzeichnis der einbindenden Datei aufgelöst. Absolute Pfade sind ebenfalls möglich.

### Remote-Includes (URL)

Includes die mit `http://` oder `https://` beginnen, werden aus dem Netzwerk geladen. Polaris speichert Remote-Dateien lokal im Cache (`%LOCALAPPDATA%\Polaris\cache\`) und nutzt **HTTP ETags** um unnötige Downloads zu vermeiden:

- **Erster Abruf** — Die Datei wird heruntergeladen und zusammen mit ihrem ETag zwischengespeichert.
- **Folgende Abrufe** — Polaris sendet einen `If-None-Match`-Header. Antwortet der Server mit `304 Not Modified`, wird die gecachte Version ohne erneuten Download verwendet.
- **Offline-Modus** — Ist der Server nicht erreichbar, wird automatisch die zuvor gecachte Version verwendet. Existiert keine gecachte Version, wird das Include mit einem Fehler übersprungen.
- **Cache-Bereinigung** — Wird eine URL aus `includes` entfernt, wird die zugehörige Cache-Datei beim nächsten Lauf automatisch gelöscht.

Dies ist ideal für zentral verwaltete Konfigurationen (z.B. eine gemeinsame Firmen-Baseline auf einem Webserver oder einer Git-Raw-URL).

### Authentifizierung

Remote-Includes unterstützen **Basic Auth** und **mTLS** (gegenseitiges TLS mit Client-Zertifikaten aus dem Windows-Zertifikatspeicher). Verwende die Objekt-Form statt eines einfachen Strings:

#### Basic Auth

```yaml
includes:
  - url: https://config.example.com/packages.yaml
    auth:
      type: basic
      username: "polaris"
      password: "secret"
```

#### mTLS (Client-Zertifikat)

```yaml
includes:
  - url: https://secure.example.com/policies.yaml
    auth:
      type: mtls
      subject: "CN=polaris-client"   # Zertifikat-Subject im Windows-Zertifikatspeicher
      store: "My"                     # Speichername (Standard: "My")
```

Das Zertifikat wird zur Laufzeit aus dem **Windows-Zertifikatspeicher** geladen. Der private Schlüssel verlässt den Speicher nie — Polaris signiert TLS-Handshakes über die Windows CNG API (NCrypt).

#### Auth-Vererbung

Wenn `propagate_auth: true` bei einem Include gesetzt ist, wird die gleiche Authentifizierung automatisch für alle verschachtelten URL-Includes aus dieser Datei verwendet:

```yaml
includes:
  - url: https://config.example.com/main.yaml
    auth:
      type: basic
      username: "polaris"
      password: "secret"
    propagate_auth: true   # verschachtelte Includes in main.yaml erben diese Auth
```

Wenn ein verschachteltes Include eine eigene `auth`-Sektion hat, hat diese Vorrang vor der vererbten Auth.

---

## Kompatibilität

Polaris kann optional prüfen, ob die YAML-Konfiguration mit der laufenden Version, dem Betriebssystem, der Architektur und der Windows-Version kompatibel ist. Alle Felder sind optional. Die Felder `os`, `arch` und `windows_version` akzeptieren einen einzelnen Wert oder eine Liste.

```yaml
compatibility:
  min_version: "1.0.0"
  max_version: "2.0.0"
  os: windows                   # einzelner Wert
  arch:                          # oder als Liste
    - amd64
    - arm64
  windows_version:
    - "11"                       # alle Windows 11 Versionen
    - "10 22H2"                  # nur Windows 10 22H2
```

### Felder

| Feld              | Pflicht | Beschreibung |
|-------------------|---------|--------------|
| `min_version`     | Nein    | Minimale Polaris-Version (Semver, z.B. `"1.0.0"`) |
| `max_version`     | Nein    | Maximale Polaris-Version (Semver, z.B. `"2.0.0"`) |
| `os`              | Nein    | Erlaubte Betriebssysteme (z.B. `windows`) |
| `arch`            | Nein    | Erlaubte Architekturen (z.B. `amd64`, `arm64`) |
| `windows_version` | Nein    | Erlaubte Windows-Versionen (Prefix-Matching, siehe unten) |

### Windows-Versionen

`windows_version` verwendet Prefix-Matching. Die erkannte Version enthält die Hauptversion und die Display-Version (z.B. `11 24H2`). So lassen sich breite und exakte Einschränkungen kombinieren:

| Wert in YAML    | Matcht                                        |
|-----------------|-----------------------------------------------|
| `"11"`          | Alle Windows 11 Versionen (11 24H2, 11 25H2, …) |
| `"11 24H2"`     | Nur Windows 11 24H2                           |
| `"10"`          | Alle Windows 10 Versionen (10 21H2, 10 22H2, …) |
| `"10 22H2"`     | Nur Windows 10 22H2                           |
| `"Server 2022"` | Windows Server 2022                           |

> **Hinweis:** Wenn kein Feld angegeben wird, werden keine Prüfungen durchgeführt. Entwicklungs-Builds (`dev`) überspringen Versionsprüfungen.

### Eigene Kompatibilität pro Datei

Jede eingebundene Datei kann optional eine eigene `compatibility`-Sektion haben. Ist eine eingebundene Datei nicht mit dem laufenden System kompatibel, wird sie **automatisch übersprungen** — ihre Einträge werden nicht geladen. In der Ausgabe erscheint eine `[SKIP]`-Meldung mit dem Dateinamen und dem Grund.

```yaml
includes:
  - win10-packages.yaml     # compatibility: windows_version: "10"
  - win11-packages.yaml     # compatibility: windows_version: "11"
  - server-settings.yaml    # compatibility: windows_version: "Server 2022"
```

Nur die zur Laufzeit passenden Dateien werden tatsächlich geladen.

---

## Merge-Verhalten

| Typ | Verhalten |
|-----|-----------|
| Listen (`packages`, `registry`, `users`, `group_policy`) | Einträge werden angehängt |
| `windows_update`, `windows_defender` | Deep-Merge: gesetzte Felder der Eltern-Datei haben Vorrang, fehlende Felder werden aus Kind-Dateien ergänzt. Defender-Exclusions werden kombiniert. |
| `compatibility` | Jede Datei wird einzeln geprüft; für Merge gilt erste Definition |

### Beispiel

**main.yaml:**
```yaml
includes:
  - packages.yaml
  - settings.yaml

compatibility:
  os: windows
```

**packages.yaml:**
```yaml
compatibility:
  windows_version: "11"

packages:
  - name: Firefox
    id: Mozilla.Firefox
    source: winget
    state: present
```

**settings.yaml:**
```yaml
includes:
  - defender.yaml       # kann weitere Dateien einbinden

windows_update:
  auto_update: notify
```

---

Zurück: [Installation](installation.md) · Weiter: [Paketverwaltung](packages.md)
