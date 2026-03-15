# Configuration

Polaris reads a YAML file and reconciles the desired state with the system. When started without arguments, Polaris automatically looks for a `config.yaml` next to the executable. If none is found, an error is shown.

```powershell
# Simplest usage — finds config.yaml next to the exe automatically
.\polaris.exe

# Equivalent explicit subcommand
.\polaris.exe apply

# Explicit path
.\polaris.exe apply --config C:\Polaris\my-config.yaml
```

## Structure

```yaml
packages:
  - name: Display Name
    id: Package-ID
    version: "1.0.0"       # optional
    source: winget          # "winget", "msstore", or "choco"
    state: present          # "present" or "absent"
```

## Validation

Polaris validates the configuration before execution. Missing required fields or invalid values result in an error message before any changes are made.

---

## Includes

Configuration files can include other YAML files — both **local paths** and **HTTP(S) URLs**. This allows you to split and reuse configurations modularly. Included files can themselves include further files (recursively). Circular references are detected and rejected with an error message.

```yaml
includes:
  - packages.yaml
  - security/defender.yaml
  - ../shared/registry.yaml
  - https://example.com/polaris/base-packages.yaml
```

### Path Resolution

Local paths are resolved relative to the directory of the including file. Absolute paths are also supported.

### Remote Includes (URL)

Includes starting with `http://` or `https://` are fetched from the network. Polaris caches remote files locally (in `%LOCALAPPDATA%\Polaris\cache\`) and uses **HTTP ETags** to avoid unnecessary downloads:

- **First fetch** — The file is downloaded and cached along with its ETag.
- **Subsequent fetches** — Polaris sends an `If-None-Match` header. If the server responds with `304 Not Modified`, the cached version is used without re-downloading.
- **Offline mode** — If the server is unreachable, the previously cached version is used automatically. If no cached version exists, the include is skipped with an error.
- **Cache cleanup** — When a URL is removed from `includes`, its cached file is automatically deleted on the next run.

This is ideal for centrally managed configurations (e.g. a shared company baseline hosted on a web server or Git raw URL).

### Authentication

Remote includes can use **Basic Auth** or **mTLS** (mutual TLS with client certificates from the Windows certificate store). Use the object form instead of a plain string:

#### Basic Auth

```yaml
includes:
  - url: https://config.example.com/packages.yaml
    auth:
      type: basic
      username: "polaris"
      password: "secret"
```

#### mTLS (Client Certificate)

```yaml
includes:
  - url: https://secure.example.com/policies.yaml
    auth:
      type: mtls
      subject: "CN=polaris-client"   # certificate subject in Windows cert store
      store: "My"                     # store name (default: "My")
```

The certificate is loaded from the **Windows Certificate Store** at runtime. The private key never leaves the store — Polaris signs TLS handshakes via the Windows CNG API (NCrypt).

#### Auth Propagation

When `propagate_auth: true` is set on an include entry, the same authentication is automatically used for all nested URL includes from that file:

```yaml
includes:
  - url: https://config.example.com/main.yaml
    auth:
      type: basic
      username: "polaris"
      password: "secret"
    propagate_auth: true   # nested includes in main.yaml inherit this auth
```

If a nested include specifies its own `auth`, it takes precedence over the propagated auth.

---

## Compatibility

Polaris can optionally verify that the YAML configuration is compatible with the running version, operating system, architecture, and Windows version. All fields are optional. The `os`, `arch`, and `windows_version` fields accept a single value or a list.

```yaml
compatibility:
  min_version: "1.0.0"
  max_version: "2.0.0"
  os: windows                   # single value
  arch:                          # or as a list
    - amd64
    - arm64
  windows_version:
    - "11"                       # all Windows 11 versions
    - "10 22H2"                  # only Windows 10 22H2
```

### Fields

| Field             | Required | Description |
|-------------------|----------|-------------|
| `min_version`     | No       | Minimum Polaris version (semver, e.g. `"1.0.0"`) |
| `max_version`     | No       | Maximum Polaris version (semver, e.g. `"2.0.0"`) |
| `os`              | No       | Allowed operating systems (e.g. `windows`) |
| `arch`            | No       | Allowed architectures (e.g. `amd64`, `arm64`) |
| `windows_version` | No       | Allowed Windows versions (prefix matching, see below) |

### Windows Versions

`windows_version` uses prefix matching. The detected version includes the major version and the display version (e.g. `11 24H2`). This allows both broad and exact constraints:

| YAML value       | Matches                                         |
|-------------------|-------------------------------------------------|
| `"11"`            | All Windows 11 versions (11 24H2, 11 25H2, …)  |
| `"11 24H2"`       | Only Windows 11 24H2                            |
| `"10"`            | All Windows 10 versions (10 21H2, 10 22H2, …)  |
| `"10 22H2"`       | Only Windows 10 22H2                            |
| `"Server 2022"`   | Windows Server 2022                             |

> **Note:** If no fields are specified, no checks are performed. Development builds (`dev`) skip version checks.

### Per-File Compatibility

Each included file can optionally have its own `compatibility` section. If an included file is not compatible with the running system, it is **automatically skipped** — its entries are not loaded. A `[SKIP]` message with the file name and reason is shown in the output.

```yaml
includes:
  - win10-packages.yaml     # compatibility: windows_version: "10"
  - win11-packages.yaml     # compatibility: windows_version: "11"
  - server-settings.yaml    # compatibility: windows_version: "Server 2022"
```

Only the files matching the current system are actually loaded.

---

## Merge Behavior

| Type | Behavior |
|------|----------|
| Lists (`packages`, `registry`, `users`, `group_policy`) | Entries are appended |
| `windows_update`, `windows_defender` | Deep merge: parent fields take precedence per field, unset fields are filled from child files. Defender exclusions are combined. |
| `compatibility` | Each file is checked independently; for merge, first definition wins |

### Example

**main.yaml:**
```yaml
includes:
  - packages.yaml
  - settings.yaml

compatibility:
  os: windows
```

**packages.yaml:**
```yaml
compatibility:
  windows_version: "11"

packages:
  - name: Firefox
    id: Mozilla.Firefox
    source: winget
    state: present
```

**settings.yaml:**
```yaml
includes:
  - defender.yaml       # can include further files

windows_update:
  auto_update: notify
```

---

Previous: [Installation](installation.md) · Next: [Packages](packages.md)
