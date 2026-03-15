# Installation

## Prerequisites

- **Windows 10/11** or Windows Server 2019+
- **WinGet** (Windows Package Manager) — automatically installed if not present
  - WinGet is pre-installed on Windows 11
  - On older systems, Polaris installs WinGet automatically on first run

## Download

Download the latest release from the [GitHub Releases](https://github.com/mwlf01/Polaris/releases) page.

## Build from Source

Prerequisite: [Go](https://go.dev/dl/) 1.25 or newer.

```powershell
git clone https://github.com/mwlf01/Polaris.git
cd polaris
go build -o bin/polaris.exe ./cmd/polaris
```

## Verification

```powershell
.\bin\polaris.exe version
```

---

Next: [Configuration](configuration.md)
