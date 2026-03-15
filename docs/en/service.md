# Windows Service

Polaris can register itself as a Windows service that runs under **NT AUTHORITY\SYSTEM** and periodically applies the configuration.

## Service Management

```powershell
# Install the service (automatic start, runs as SYSTEM)
polaris service install

# Check service status
polaris service status

# Remove the service
polaris service uninstall
```

After installation, the service starts automatically on boot. It reads `config.yaml` next to the executable and re-applies at the configured `schedule.interval`.

The service includes automatic recovery: if it crashes, Windows restarts it after 10/30/60 seconds.

---

## Schedule

The optional `schedule` section controls the interval at which Polaris re-applies the configuration when running as a Windows service. The interval uses Go duration format.

```yaml
schedule:
  interval: "30m"    # re-apply every 30 minutes
```

Supported formats: `"15m"`, `"30m"`, `"1h"`, `"6h"`, `"24h"`, etc. If omitted, the default is **15 minutes**.

Special values `"once"`, `"0"`, or `"off"` make Polaris apply the configuration once at service startup and then stop — useful for one-time provisioning.

The schedule is only relevant in service mode — when running manually (`polaris` or `polaris apply`), the configuration is applied once and the process exits. Changes to the interval are picked up on the next cycle without restarting the service.

---

## Running under SYSTEM

### What works automatically

- **Registry**, **Windows Update**, **Windows Defender**, **Users**, and **Group Policy** modules use system-level APIs and work without any changes.
- **Chocolatey** installs globally and is available to SYSTEM.

### WinGet under SYSTEM

WinGet is an MSIX app whose PATH alias is only available to interactive users. Polaris automatically resolves the `winget.exe` binary inside `C:\Program Files\WindowsApps\` when the PATH alias is not available. If WinGet is not installed at all, Polaris uses `Add-AppxProvisionedPackage` to install it system-wide (instead of the per-user `Add-AppxPackage`).

### Microsoft Store apps under SYSTEM

Store apps (`source: msstore`) require a user context and cannot be installed directly as SYSTEM. Polaris automatically detects this and launches the winget command in the session of the currently logged-in user (invisible, no window). The logged-in user does not need administrator privileges and will not see any windows or prompts. If no user is logged in, the operation fails with a descriptive error.

No additional configuration is required — Polaris detects the SYSTEM context and adapts automatically.

---

## Alternative: Scheduled Task

If you prefer a scheduled task over a service:

```powershell
$action  = New-ScheduledTaskAction -Execute "C:\Polaris\polaris.exe"
$trigger = New-ScheduledTaskTrigger -Daily -At 06:00
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
Register-ScheduledTask -TaskName "Polaris" -Action $action -Trigger $trigger -Principal $principal
```

---

Previous: [System Modules](modules.md) · Next: [Self-Update](self-update.md)
