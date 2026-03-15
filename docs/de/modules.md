# Systemmodule

Polaris kann verschiedene Windows-Systemeinstellungen verwalten. Alle Module sind optional — füge nur die Sektionen ein, die du benötigst.

---

## Windows Update

Windows Update-Einstellungen konfigurieren. Alle Felder sind optional — nur angegebene Einstellungen werden gesetzt.

```yaml
windows_update:
  auto_update: notify
  active_hours:
    start: 8
    end: 17
  defer_feature_updates_days: 30
  defer_quality_updates_days: 7
  no_auto_restart: true
  microsoft_product_updates: true
```

### Einstellungen

| Feld | Werte | Beschreibung |
|------|-------|--------------|
| `auto_update` | `disabled`, `notify`, `download_only`, `auto` | Steuert das automatische Update-Verhalten |
| `active_hours.start` | `0`–`23` | Beginn der aktiven Stunden (kein Neustart) |
| `active_hours.end` | `0`–`23` | Ende der aktiven Stunden |
| `defer_feature_updates_days` | `0`–`365` | Feature-Updates um X Tage verzögern |
| `defer_quality_updates_days` | `0`–`30` | Qualitäts-Updates um X Tage verzögern |
| `no_auto_restart` | `true` / `false` | Automatischen Neustart bei angemeldeten Benutzern verhindern |
| `microsoft_product_updates` | `true` / `false` | Updates für andere Microsoft-Produkte (z.B. Office, Edge) über Windows Update erhalten |

### Auto-Update Modi

| Modus | Beschreibung |
|-------|--------------|
| `disabled` | Automatische Updates deaktiviert |
| `notify` | Vor dem Download benachrichtigen |
| `download_only` | Automatisch herunterladen, vor Installation benachrichtigen |
| `auto` | Automatisch herunterladen und installieren |

> **Hinweis:** Windows Update-Einstellungen erfordern Administratorrechte.

---

## Windows Defender

Microsoft Defender Antivirus konfigurieren. Alle Felder sind optional.

```yaml
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
```

### Einstellungen

| Feld | Werte | Beschreibung |
|------|-------|--------------|
| `real_time_protection` | `true` / `false` | Echtzeitschutz aktivieren/deaktivieren |
| `cloud_protection` | `true` / `false` | Cloudbasierter Schutz |
| `sample_submission` | `true` / `false` | Automatische Übermittlung von Beispieldateien |
| `pua_protection` | `true` / `false` | Schutz vor potenziell unerwünschten Apps |

### Ausschlüsse

Ausschlüsse sind additiv — Polaris stellt sicher, dass die gelisteten Einträge vorhanden sind, entfernt aber keine bestehenden Ausschlüsse.

| Feld | Beschreibung |
|------|--------------|
| `exclusions.paths` | Ordner oder Dateien, die nicht gescannt werden |
| `exclusions.extensions` | Dateiendungen (ohne Punkt) |
| `exclusions.processes` | Prozessnamen |

### Scan-Zeitplan

| Feld | Werte | Beschreibung |
|------|-------|--------------|
| `scan_schedule.day` | `everyday`, `monday`–`sunday` | Tag des geplanten Scans |
| `scan_schedule.time` | `HH:MM` (24h) | Uhrzeit des Scans |

> **Hinweis:** Windows Defender-Einstellungen erfordern Administratorrechte.

---

## Benutzer

Lokale Windows-Benutzerkonten anlegen, konfigurieren oder löschen.

### Benutzer anlegen

```yaml
users:
  - name: DevUser
    full_name: "Developer Account"
    description: "Lokales Entwicklungskonto"
    password: "SecureP@ss123"
    groups:
      - Administrators
      - "Remote Desktop Users"
    password_never_expires: true
    account_disabled: false
    state: present
```

### Benutzer löschen

```yaml
users:
  - name: OldUser
    state: absent
```

### Felder

| Feld | Pflicht | Beschreibung |
|------|---------|--------------|
| `name` | Ja | Benutzername |
| `full_name` | Nein | Anzeigename |
| `description` | Nein | Beschreibung des Kontos |
| `password` | Nein | Passwort (wird bei jedem Lauf gesetzt, wenn angegeben) |
| `groups` | Nein | Lokale Gruppen, denen der Benutzer hinzugefügt wird |
| `password_never_expires` | Nein | Passwort läuft nie ab |
| `account_disabled` | Nein | Konto deaktiviert |
| `state` | Ja | `present` (anlegen/aktualisieren) oder `absent` (löschen) |

> **Hinweis:** Gruppenmitgliedschaften sind additiv — Polaris fügt den Benutzer den gelisteten Gruppen hinzu, entfernt aber keine bestehenden Mitgliedschaften. Passwörter werden im Klartext in der YAML-Datei gespeichert. Stellen Sie sicher, dass die Konfigurationsdatei angemessen geschützt ist.

---

## Gruppenrichtlinien

Lokale Gruppenrichtlinien über LGPO.exe (Local Group Policy Object Utility) setzen. LGPO.exe wird bei Bedarf automatisch über winget installiert.

### Richtlinie setzen

```yaml
group_policy:
  - scope: computer
    path: SOFTWARE\Policies\Microsoft\Windows\Windows Error Reporting
    name: Disabled
    type: dword
    value: 1
    state: present
```

### Richtlinie löschen

```yaml
group_policy:
  - scope: computer
    path: SOFTWARE\Policies\Microsoft\Edge
    name: OldPolicy
    state: absent
```

### Felder

| Feld    | Pflicht | Beschreibung |
|---------|---------|--------------|
| `scope` | Ja      | `computer` oder `user` |
| `path`  | Ja      | Registry-Pfad der Richtlinie (ohne Hive) |
| `name`  | Bei `present` | Name des Richtlinienwerts |
| `type`  | Bei `present` | Typ: `string`, `expand_string`, `multi_string`, `dword`, `qword` |
| `value` | Bei `present` | Gewünschter Wert |
| `state` | Ja      | `present` (setzen) oder `absent` (löschen) |

> **Hinweis:** LGPO.exe (`Microsoft.SecurityComplianceToolkit.LGPO`) wird automatisch per winget installiert, wenn es nicht auf dem System vorhanden ist. Gruppenrichtlinien erfordern Administratorrechte.

---

## Registry

Beliebige Windows-Registry-Einträge erstellen, ändern oder löschen.

### Wert erstellen oder ändern

```yaml
registry:
  - path: HKLM\SOFTWARE\MyCompany\MyApp
    name: AppSetting
    type: string
    value: "enabled"
    state: present
```

### Wert oder Key löschen

```yaml
registry:
  # Einzelnen Wert löschen
  - path: HKLM\SOFTWARE\MyCompany\MyApp
    name: OldSetting
    state: absent

  # Gesamten Key löschen
  - path: HKLM\SOFTWARE\MyCompany\OldApp
    state: absent
```

### Felder

| Feld    | Pflicht | Beschreibung |
|---------|---------|--------------|
| `path`  | Ja      | Registry-Pfad mit Hive (`HKLM`, `HKCU`, `HKCR`, `HKU`) |
| `name`  | Bei `present` | Name des Werts |
| `type`  | Bei `present` | Typ: `string`, `expand_string`, `multi_string`, `dword`, `qword`, `binary` |
| `value` | Bei `present` | Gewünschter Wert |
| `state` | Ja      | `present` (erstellen/ändern) oder `absent` (löschen) |

### Werttypen

| Typ | YAML-Wert | Beispiel |
|-----|-----------|---------|
| `string` | Text | `"hello"` |
| `expand_string` | Text mit Umgebungsvariablen | `"%SystemRoot%\\temp"` |
| `multi_string` | Liste von Texten | `["eins", "zwei"]` |
| `dword` | Ganzzahl (32-Bit) | `42` |
| `qword` | Ganzzahl (64-Bit) | `100000` |
| `binary` | Hex-String | `"48656c6c6f"` |

> **Hinweis:** Registry-Änderungen unter `HKLM` erfordern Administratorrechte.

---

Zurück: [Paketverwaltung](packages.md) · Weiter: [Windows-Dienst](service.md)
