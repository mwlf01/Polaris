# Fehlerbehebung

## Häufige Probleme

### „winget not available"

Polaris versucht WinGet automatisch zu installieren. Falls die automatische Installation fehlschlägt:
- Installiere den [App-Installer](https://www.microsoft.com/store/productId/9NBLGGH4NNS1) aus dem Microsoft Store oder lade WinGet von [GitHub](https://aka.ms/getwinget) herunter
- Starte die Konsole nach der Installation neu

### Paket wird nicht gefunden

Die `id` muss exakt mit der Paket-ID der jeweiligen Quelle übereinstimmen.
- **WinGet**: Groß-/Kleinschreibung beachten (z.B. `Mozilla.Firefox`)
- **Chocolatey**: In der Regel kleingeschrieben (z.B. `git`)

---

## Ausgabe

```
  Polaris v1.0.0
  Configuration Management Agent
──────────────────────────────────────────────────

  Config ............. C:\Polaris\config.yaml
  Platform ........... Windows 11 24H2 (amd64)

── Package Managers ──────────────────────────────
  [OK]       winget – already up to date
  [OK]       choco – already up to date

── Windows Update ────────────────────────────────
  [OK]       already configured

── Packages (3) ──────────────────────────────────
  [OK]       Mozilla Firefox (Mozilla.Firefox) – up to date
  [INSTALL]  Git (git) – installing...
  [DONE]     Git (git) – installed

──────────────────────────────────────────────────
  1 ok · 1 changed · 0 skipped · 0 failed · 2 total
──────────────────────────────────────────────────
```

### Indikatoren

| Symbol      | Bedeutung |
|-------------|-----------|
| `[OK]`      | Bereits im gewünschten Zustand |
| `[INSTALL]` | Wird installiert |
| `[REMOVE]`  | Wird deinstalliert |
| `[DONE]`    | Aktion erfolgreich |
| `[SKIP]`    | Inkompatible Datei übersprungen |
| `[ERROR]`   | Aktion fehlgeschlagen |

---

## Vollständiges Beispiel

```yaml
compatibility:
  min_version: "1.0.0"
  os: windows
  arch:
    - amd64
    - arm64
  windows_version:
    - "10"
    - "11"

schedule:
  interval: "30m"

update:
  url: "https://releases.example.com/polaris/update.json"

windows_update:
  auto_update: notify
  active_hours:
    start: 8
    end: 17
  defer_feature_updates_days: 30
  defer_quality_updates_days: 7
  no_auto_restart: true
  microsoft_product_updates: true

windows_defender:
  real_time_protection: true
  cloud_protection: true
  sample_submission: true
  pua_protection: true
  exclusions:
    paths:
      - "C:\\Dev"
    extensions:
      - "log"
    processes:
      - "devenv.exe"
  scan_schedule:
    day: wednesday
    time: "02:00"

users:
  - name: DevUser
    full_name: "Developer Account"
    password: "SecureP@ss123"
    groups:
      - Administrators
    password_never_expires: true
    state: present

group_policy:
  - scope: computer
    path: SOFTWARE\Policies\Microsoft\Windows\Windows Error Reporting
    name: Disabled
    type: dword
    value: 1
    state: present

registry:
  - path: HKLM\SOFTWARE\MyCompany\MyApp
    name: AppSetting
    type: string
    value: "enabled"
    state: present

packages:
  - name: Mozilla Firefox
    id: Mozilla.Firefox
    source: winget
    state: present

  - name: 7-Zip
    id: 7zip.7zip
    source: winget
    state: present

  - name: Microsoft Whiteboard
    id: 9MSPC6MP8FM4
    source: msstore
    state: present

  - name: Git
    id: git
    source: choco
    state: present

  - name: Unerwünschte App
    id: Publisher.UnwantedApp
    source: winget
    state: absent
```

---

Zurück: [Selbst-Update](self-update.md) · [Zur Übersicht](README.md)
