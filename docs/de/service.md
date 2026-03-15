# Windows-Dienst

Polaris kann sich selbst als Windows-Dienst registrieren, der unter **NT AUTHORITY\SYSTEM** läuft und die Konfiguration periodisch anwendet.

## Dienstverwaltung

```powershell
# Dienst installieren (automatischer Start, läuft als SYSTEM)
polaris service install

# Dienststatus prüfen
polaris service status

# Dienst entfernen
polaris service uninstall
```

Nach der Installation startet der Dienst automatisch beim Systemstart. Er liest `config.yaml` neben der ausführbaren Datei und wendet sie im konfigurierten `schedule.interval` erneut an.

Der Dienst enthält automatische Wiederherstellung: Bei einem Absturz startet Windows ihn nach 10/30/60 Sekunden neu.

---

## Zeitplan

Der optionale Abschnitt `schedule` steuert das Intervall, in dem Polaris die Konfiguration erneut anwendet, wenn es als Windows-Dienst läuft. Das Intervall verwendet das Go-Duration-Format.

```yaml
schedule:
  interval: "30m"    # alle 30 Minuten erneut anwenden
```

Unterstützte Formate: `"15m"`, `"30m"`, `"1h"`, `"6h"`, `"24h"`, etc. Wenn nicht angegeben, ist der Standard **15 Minuten**.

Die speziellen Werte `"once"`, `"0"` oder `"off"` bewirken, dass Polaris die Konfiguration nur einmal beim Dienststart anwendet und sich dann beendet — nützlich für einmalige Provisionierung.

Der Zeitplan ist nur im Dienst-Modus relevant — bei manueller Ausführung (`polaris` oder `polaris apply`) wird die Konfiguration einmal angewendet und der Prozess beendet sich. Änderungen am Intervall werden beim nächsten Zyklus übernommen, ohne den Dienst neu starten zu müssen.

---

## Ausführung unter SYSTEM

### Was automatisch funktioniert

- **Registry**, **Windows Update**, **Windows Defender**, **Benutzer** und **Gruppenrichtlinien** verwenden System-APIs und funktionieren ohne Anpassungen.
- **Chocolatey** wird global installiert und ist für SYSTEM verfügbar.

### WinGet unter SYSTEM

WinGet ist eine MSIX-App, deren PATH-Alias nur für interaktive Benutzer verfügbar ist. Polaris löst die `winget.exe`-Binärdatei automatisch in `C:\Program Files\WindowsApps\` auf, wenn der PATH-Alias nicht verfügbar ist. Falls WinGet gar nicht installiert ist, verwendet Polaris `Add-AppxProvisionedPackage` für eine systemweite Installation (anstelle des benutzerspezifischen `Add-AppxPackage`).

### Microsoft Store-Apps unter SYSTEM

Store-Apps (`source: msstore`) benötigen einen Benutzerkontext und können nicht direkt als SYSTEM installiert werden. Polaris erkennt dies automatisch und führt den winget-Befehl in der Sitzung des aktuell angemeldeten Benutzers aus (unsichtbar, kein Fenster). Der angemeldete Benutzer benötigt keine Administratorrechte und sieht weder Fenster noch Eingabeaufforderungen. Falls kein Benutzer angemeldet ist, schlägt die Operation mit einer aussagekräftigen Fehlermeldung fehl.

Es ist keine zusätzliche Konfiguration erforderlich — Polaris erkennt den SYSTEM-Kontext und passt sich automatisch an.

---

## Alternative: Geplante Aufgabe

Falls du eine geplante Aufgabe statt einem Dienst bevorzugst:

```powershell
$action  = New-ScheduledTaskAction -Execute "C:\Polaris\polaris.exe"
$trigger = New-ScheduledTaskTrigger -Daily -At 06:00
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName "Polaris" -Action $action -Trigger $trigger -Principal $principal
```

---

Zurück: [Systemmodule](modules.md) · Weiter: [Selbst-Update](self-update.md)
