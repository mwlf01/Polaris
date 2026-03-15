# Paketverwaltung

Polaris unterstützt drei Paketquellen. Die Paketmanager werden vor der Verarbeitung automatisch aktualisiert.

## Konfiguration

```yaml
packages:
  - name: Anzeigename
    id: Paket-ID
    version: "1.0.0"       # optional
    source: winget          # "winget", "msstore" oder "choco"
    state: present          # "present" oder "absent"
```

### Felder

| Feld      | Pflicht | Beschreibung |
|-----------|---------|--------------|
| `name`    | Nein    | Lesbarer Name (wird in der Ausgabe angezeigt) |
| `id`      | Ja      | Paket-ID der jeweiligen Quelle |
| `version` | Nein    | Bestimmte Version, die installiert werden soll |
| `source`  | Ja      | Paketquelle: `winget`, `msstore` oder `choco` |
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
