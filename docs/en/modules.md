# System Modules

Polaris can manage several Windows system settings. All modules are optional — only include the sections you need.

---

## Windows Update

Configure Windows Update settings. All fields are optional — only specified settings will be applied.

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

### Settings

| Field | Values | Description |
|-------|--------|-------------|
| `auto_update` | `disabled`, `notify`, `download_only`, `auto` | Controls automatic update behavior |
| `active_hours.start` | `0`–`23` | Start of active hours (no restart) |
| `active_hours.end` | `0`–`23` | End of active hours |
| `defer_feature_updates_days` | `0`–`365` | Defer feature updates by X days |
| `defer_quality_updates_days` | `0`–`30` | Defer quality updates by X days |
| `no_auto_restart` | `true` / `false` | Prevent automatic restart when users are logged in |
| `microsoft_product_updates` | `true` / `false` | Receive updates for other Microsoft products (e.g. Office, Edge) via Windows Update |

### Auto-Update Modes

| Mode | Description |
|------|-------------|
| `disabled` | Automatic updates disabled |
| `notify` | Notify before download |
| `download_only` | Auto download, notify before install |
| `auto` | Auto download and install |

> **Note:** Windows Update settings require administrator privileges.

---

## Windows Defender

Configure Microsoft Defender Antivirus. All fields are optional.

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

### Settings

| Field | Values | Description |
|-------|--------|-------------|
| `real_time_protection` | `true` / `false` | Enable/disable real-time protection |
| `cloud_protection` | `true` / `false` | Cloud-delivered protection |
| `sample_submission` | `true` / `false` | Automatic sample submission |
| `pua_protection` | `true` / `false` | Block potentially unwanted applications |

### Exclusions

Exclusions are additive — Polaris ensures listed items are present but does not remove existing exclusions not in the list.

| Field | Description |
|-------|-------------|
| `exclusions.paths` | Folders or files to exclude from scanning |
| `exclusions.extensions` | File extensions (without dot) |
| `exclusions.processes` | Process names |

### Scan Schedule

| Field | Values | Description |
|-------|--------|-------------|
| `scan_schedule.day` | `everyday`, `monday`–`sunday` | Day of the scheduled scan |
| `scan_schedule.time` | `HH:MM` (24h) | Time of the scan |

> **Note:** Windows Defender settings require administrator privileges.

---

## Users

Create, configure, or delete local Windows user accounts.

### Create a User

```yaml
users:
  - name: DevUser
    full_name: "Developer Account"
    description: "Local development account"
    password: "SecureP@ss123"
    groups:
      - Administrators
      - "Remote Desktop Users"
    password_never_expires: true
    account_disabled: false
    state: present
```

### Delete a User

```yaml
users:
  - name: OldUser
    state: absent
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Username |
| `full_name` | No | Display name |
| `description` | No | Account description |
| `password` | No | Password (set on every run if specified) |
| `groups` | No | Local groups to add the user to |
| `password_never_expires` | No | Password never expires |
| `account_disabled` | No | Disable the account |
| `state` | Yes | `present` (create/update) or `absent` (delete) |

> **Note:** Group memberships are additive — Polaris adds the user to listed groups but does not remove existing memberships. Passwords are stored in plain text in the YAML file. Make sure the configuration file is properly secured.

---

## Group Policy

Set local group policies via LGPO.exe (Local Group Policy Object Utility). LGPO.exe is automatically installed from winget if not present on the system.

### Set a Policy

```yaml
group_policy:
  - scope: computer
    path: SOFTWARE\Policies\Microsoft\Windows\Windows Error Reporting
    name: Disabled
    type: dword
    value: 1
    state: present
```

### Delete a Policy

```yaml
group_policy:
  - scope: computer
    path: SOFTWARE\Policies\Microsoft\Edge
    name: OldPolicy
    state: absent
```

### Fields

| Field   | Required | Description |
|---------|----------|-------------|
| `scope` | Yes      | `computer` or `user` |
| `path`  | Yes      | Registry path of the policy (without hive) |
| `name`  | For `present` | Policy value name |
| `type`  | For `present` | Type: `string`, `expand_string`, `multi_string`, `dword`, `qword` |
| `value` | For `present` | Desired value |
| `state` | Yes      | `present` (set) or `absent` (delete) |

> **Note:** LGPO.exe (`Microsoft.SecurityComplianceToolkit.LGPO`) is automatically installed via winget if not found on the system. Group policies require administrator privileges.

---

## Registry

Create, modify, or delete arbitrary Windows Registry entries.

### Create or Update a Value

```yaml
registry:
  - path: HKLM\SOFTWARE\MyCompany\MyApp
    name: AppSetting
    type: string
    value: "enabled"
    state: present
```

### Delete a Value or Key

```yaml
registry:
  # Delete a specific value
  - path: HKLM\SOFTWARE\MyCompany\MyApp
    name: OldSetting
    state: absent

  # Delete an entire key
  - path: HKLM\SOFTWARE\MyCompany\OldApp
    state: absent
```

### Fields

| Field   | Required | Description |
|---------|----------|-------------|
| `path`  | Yes      | Registry path with hive (`HKLM`, `HKCU`, `HKCR`, `HKU`) |
| `name`  | For `present` | Value name |
| `type`  | For `present` | Type: `string`, `expand_string`, `multi_string`, `dword`, `qword`, `binary` |
| `value` | For `present` | Desired value |
| `state` | Yes      | `present` (create/update) or `absent` (delete) |

### Value Types

| Type | YAML Value | Example |
|------|-----------|---------|
| `string` | Text | `"hello"` |
| `expand_string` | Text with environment variables | `"%SystemRoot%\\temp"` |
| `multi_string` | List of strings | `["one", "two"]` |
| `dword` | Integer (32-bit) | `42` |
| `qword` | Integer (64-bit) | `100000` |
| `binary` | Hex string | `"48656c6c6f"` |

> **Note:** Registry changes under `HKLM` require administrator privileges.

---

Previous: [Packages](packages.md) · Next: [Windows Service](service.md)
