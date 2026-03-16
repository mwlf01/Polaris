# Package Management

Polaris supports four package sources. Package managers are automatically updated before processing.

## Configuration

```yaml
packages:
  - name: Display Name
    id: Package-ID
    version: "1.0.0"       # optional
    source: winget          # "winget", "msstore", "choco", or "appx"
    state: present          # "present" or "absent"
```

### Fields

| Field     | Required | Description |
|-----------|----------|-------------|
| `name`    | No       | Human-readable name (shown in output) |
| `id`      | Yes      | Package ID for the respective source |
| `version` | No       | Specific version to install |
| `source`  | Yes      | Package source: `winget`, `msstore`, `choco`, or `appx` |
| `state`   | Yes      | Desired state: `present` or `absent` |

---

## Package Sources

### WinGet (`source: winget`)

Default package source for Windows. Find package IDs with:

```powershell
winget search <search term>
```

Example:

```yaml
- name: Mozilla Firefox
  id: Mozilla.Firefox
  source: winget
  state: present
```

### Microsoft Store (`source: msstore`)

For apps from the Microsoft Store. Find the product ID in the Store page URL:

```
https://apps.microsoft.com/detail/9MSPC6MP8FM4
                                   ^^^^^^^^^^^^^
```

Or search:

```powershell
winget search --source msstore <search term>
```

Example:

```yaml
- name: Microsoft Whiteboard
  id: 9MSPC6MP8FM4
  source: msstore
  state: present
```

### AppX (`source: appx`)

For pre-installed Windows apps (e.g. Feedback Hub, Clipchamp, Microsoft News) that are managed via `Get-AppxPackage` / `Remove-AppxPackage`. These apps cannot be reliably removed by WinGet using MS Store product IDs.

Find AppX package names with:

```powershell
Get-AppxPackage -Name *keyword*
```

Example:

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

> **Note:** The `appx` source only supports removing packages (`state: absent`). To install Store apps, use `source: msstore`. When removing, provisioning is also removed so the app does not return for new user profiles.

### Chocolatey (`source: choco`)

Chocolatey is automatically installed via WinGet and kept up to date when needed. Find package IDs with `choco search <search term>` or at [community.chocolatey.org/packages](https://community.chocolatey.org/packages).

Example:

```yaml
- name: Git
  id: git
  source: choco
  state: present
```

---

## Behavior

### State `present`

- Package **not installed** → will be installed
- Package **already installed** → will be upgraded to the latest version (or to the version specified in `version`)

### State `absent`

- Package **installed** → will be uninstalled
- Package **not installed** → no action

### Version Pinning

When `version` is specified, Polaris ensures that exact version is installed. Without `version`, the latest version is always used.

```yaml
- name: Node.js LTS
  id: OpenJS.NodeJS.LTS
  source: winget
  version: "20.10.0"
  state: present
```

---

Previous: [Configuration](configuration.md) · Next: [System Modules](modules.md)
