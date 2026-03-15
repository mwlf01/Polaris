# Troubleshooting

## Common Issues

### "winget not available"

Polaris attempts to install WinGet automatically. If the automatic installation fails:
- Install the [App Installer](https://www.microsoft.com/store/productId/9NBLGGH4NNS1) from the Microsoft Store or download WinGet from [GitHub](https://aka.ms/getwinget)
- Restart your terminal after installation

### Package Not Found

The `id` must exactly match the package ID for the respective source.
- **WinGet**: Case-sensitive (e.g. `Mozilla.Firefox`)
- **Chocolatey**: Usually lowercase (e.g. `git`)

---

## Output

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

### Indicators

| Indicator   | Meaning |
|-------------|---------|
| `[OK]`      | Already in desired state |
| `[INSTALL]` | Being installed |
| `[REMOVE]`  | Being removed |
| `[DONE]`    | Action successful |
| `[SKIP]`    | Incompatible file skipped |
| `[ERROR]`   | Action failed |

---

## Full Example

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

  - name: Unwanted App
    id: Publisher.UnwantedApp
    source: winget
    state: absent
```

---

Previous: [Self-Update](self-update.md) · [Back to overview](README.md)
