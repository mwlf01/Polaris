# Self-Update

Polaris can update itself automatically. The `update` section points to a **JSON manifest** that describes the latest version:

```yaml
update:
  url: "https://releases.example.com/polaris/update.json"
```

---

## Update Manifest

The manifest is a single JSON file that covers **all platforms and architectures**. Polaris automatically selects the entry matching its `runtime.GOOS/runtime.GOARCH`:

```json
{
  "version": "1.2.3",
  "binaries": {
    "windows/amd64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-windows-amd64.exe",
      "sha256": "a1b2c3d4..."
    },
    "windows/arm64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-windows-arm64.exe",
      "sha256": "e5f6a7b8..."
    },
    "linux/amd64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-linux-amd64",
      "sha256": "c9d0e1f2..."
    },
    "linux/arm64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-linux-arm64",
      "sha256": "a3b4c5d6..."
    },
    "darwin/amd64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-darwin-amd64",
      "sha256": "e7f8a9b0..."
    },
    "darwin/arm64": {
      "url": "https://releases.example.com/polaris/polaris-1.2.3-darwin-arm64",
      "sha256": "c1d2e3f4..."
    }
  }
}
```

| Field              | Description |
|--------------------|-------------|
| `version`          | The new version string (compared against the running version) |
| `binaries`         | Map of `os/arch` → binary entry |
| `binaries.*.url`   | Download URL for the binary |
| `binaries.*.sha256` | SHA-256 checksum for integrity verification |

The key format uses Go's `runtime.GOOS` and `runtime.GOARCH` values (e.g. `windows/amd64`, `linux/arm64`, `darwin/arm64`). New platforms or architectures can be added at any time — Polaris simply skips the update if no matching entry exists.

---

## Authentication

The update URL supports the same `auth` options as includes (Basic Auth and mTLS):

```yaml
update:
  url: "https://secure.example.com/polaris/update.json"
  auth:
    type: basic
    username: "polaris"
    password: "secret"
```

---

## How It Works

1. **Check** — At the end of every service cycle (or via `polaris update`), Polaris fetches the manifest and compares the version.
2. **Download & Verify** — The new binary is downloaded and its SHA-256 checksum is verified.
3. **Atomic Swap** — The current binary is renamed to `.bak` (backup), and the new binary is placed in its position.
4. **Service Restart** — In service mode, the process exits with a non-zero code, triggering the SCM recovery mechanism to restart with the new binary.
5. **Rollback Guard** — On startup, the new binary validates itself. If it fails to start after 3 attempts or the update state is older than 10 minutes, the backup is automatically restored.

---

## Rollback

The update process is designed for **zero-touch deployments** where no manual intervention is possible:

- The previous binary is always kept as a `.bak` file until the update is confirmed successful.
- A startup guard tracks how many times the new binary has been started. After 3 failed starts, the backup is automatically restored.
- A watchdog timer triggers an automatic rollback if the update state file is older than 10 minutes.
- If the new binary crashes before even reaching the rollback check, the SCM recovery restarts it — and the rollback guard catches it on the next attempt.

---

## Manual Update

```powershell
polaris update
```

This loads the config, checks the manifest, and applies the update if available.

---

Previous: [Windows Service](service.md) · Next: [Troubleshooting](troubleshooting.md)
