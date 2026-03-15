# Installation

## Voraussetzungen

- **Windows 10/11** oder Windows Server 2019+
- **WinGet** (Windows Package Manager) — wird automatisch installiert, falls nicht vorhanden
  - WinGet ist in Windows 11 vorinstalliert
  - Auf älteren Systemen installiert Polaris WinGet automatisch beim ersten Start

## Herunterladen

Lade die neueste Version von der [GitHub Releases](https://github.com/mwlf01/Polaris/releases) Seite herunter.

## Aus Quellcode bauen

Voraussetzung: [Go](https://go.dev/dl/) 1.25 oder neuer.

```powershell
git clone https://github.com/mwlf01/Polaris.git
cd polaris
go build -o bin/polaris.exe ./cmd/polaris
```

## Überprüfung

```powershell
.\bin\polaris.exe version
```

---

Weiter: [Konfiguration](configuration.md)
