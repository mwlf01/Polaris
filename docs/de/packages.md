# Paketverwaltung

Polaris unterstützt vier Paketquellen. Die Paketmanager werden vor der Verarbeitung automatisch aktualisiert.

## Konfiguration

```yaml
packages:
  - name: Anzeigename
    id: Paket-ID
    version: "1.0.0"       # optional
    source: winget          # "winget", "msstore", "choco" oder "appx"
    state: present          # "present" oder "absent"
```

### Felder

| Feld      | Pflicht | Beschreibung |
|-----------|---------|--------------|
| `name`    | Nein    | Lesbarer Name (wird in der Ausgabe angezeigt) |
| `id`      | Ja      | Paket-ID der jeweiligen Quelle |
| `version` | Nein    | Bestimmte Version, die installiert werden soll |
| `source`  | Ja      | Paketquelle: `winget`, `msstore`, `choco` oder `appx` |
| `state`   | Ja      | Gewünschter Zustand: `present` oder `absent` |

---

## Paketquellen

### WinGet (`source: winget`)

Standard-Paketquelle für Windows. Paket-IDs findest du mit:

```powershell
winget search <Suchbegriff>
```

Beispiel:

```yaml
- name: Mozilla Firefox
  id: Mozilla.Firefox
  source: winget
  state: present
```

### Microsoft Store (`source: msstore`)

Für Apps aus dem Microsoft Store. Die Produkt-ID findest du in der URL der Store-Seite:

```
https://apps.microsoft.com/detail/9MSPC6MP8FM4
                                   ^^^^^^^^^^^^^
```

Oder per Suche:

```powershell
winget search --source msstore <Suchbegriff>
```

Beispiel:

```yaml
- name: Microsoft Whiteboard
  id: 9MSPC6MP8FM4
  source: msstore
  state: present
```

### AppX (`source: appx`)

Für vorinstallierte Windows-Apps (z.B. Feedback Hub, Clipchamp, Microsoft News), die über `Get-AppxPackage` / `Remove-AppxPackage` verwaltet werden. Diese Apps lassen sich nicht zuverlässig über WinGet mit MS Store Produkt-IDs entfernen.

Die AppX-Paket-ID findest du mit:

```powershell
Get-AppxPackage -Name *Suchbegriff*
```

Beispiel:

```yaml
- name: Feedback Hub
  id: Microsoft.WindowsFeedbackHub
  source: appx
  state: absent

- name: Clipchamp
  id: Clipchamp.Clipchamp
  source: appx
  state: absent

- name: Microsoft News
  id: Microsoft.BingNews
  source: appx
  state: absent
```

> **Hinweis:** Die `appx`-Quelle unterstützt nur das Entfernen von Paketen (`state: absent`). Zum Installieren von Store-Apps verwende `source: msstore`. Beim Entfernen wird auch die Bereitstellung (Provisioning) aufgehoben, sodass die App nicht für neue Benutzerprofile zurückkehrt.

### Chocolatey (`source: choco`)

Chocolatey wird bei Bedarf automatisch über WinGet installiert und aktuell gehalten. Paket-IDs findest du mit `choco search <Suchbegriff>` oder auf [community.chocolatey.org/packages](https://community.chocolatey.org/packages).

Beispiel:

```yaml
- name: Git
  id: git
  source: choco
  state: present
```

---

## Verhalten

### Zustand `present`

- Paket **nicht installiert** → wird installiert
- Paket **bereits installiert** → wird auf die neueste Version aktualisiert (oder auf die in `version` angegebene Version)

### Zustand `absent`

- Paket **installiert** → wird deinstalliert
- Paket **nicht installiert** → keine Aktion

### Versionspinning

Wird `version` angegeben, stellt Polaris sicher, dass genau diese Version installiert ist. Ohne `version` wird immer die neueste Version verwendet.

```yaml
- name: Node.js LTS
  id: OpenJS.NodeJS.LTS
  source: winget
  version: "20.10.0"
  state: present
```

---

Zurück: [Konfiguration](configuration.md) · Weiter: [Systemmodule](modules.md)
