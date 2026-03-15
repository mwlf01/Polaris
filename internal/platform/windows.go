package platform

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"

	"polaris/internal/sysutil"
	"polaris/internal/ui"
)

// Windows implements the Platform interface for Microsoft Windows.
type Windows struct {
	wingetPath string
}

// Name returns "windows".
func (w *Windows) Name() string {
	return "windows"
}

// OSVersion returns a human-friendly Windows version string such as
// "11 24H2", "10 22H2", "Server 2022", or "Server 2025".
// Client editions include the display version (e.g. "24H2") when available.
func (w *Windows) OSVersion() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	productName, _, _ := key.GetStringValue("ProductName")

	// Server editions: extract "Server YYYY" from ProductName.
	if strings.Contains(productName, "Server") {
		re := regexp.MustCompile(`Server\s+(\d{4})`)
		if m := re.FindStringSubmatch(productName); len(m) == 2 {
			return "Server " + m[1]
		}
		return "Server"
	}

	// Client editions: distinguish 10 vs 11 by build number.
	major := "10"
	buildStr, _, _ := key.GetStringValue("CurrentBuildNumber")
	if build, err := strconv.Atoi(buildStr); err == nil && build >= 22000 {
		major = "11"
	}

	// Append display version (e.g. "24H2", "22H2") when available.
	displayVersion, _, err := key.GetStringValue("DisplayVersion")
	if err == nil && displayVersion != "" {
		return major + " " + displayVersion
	}
	return major
}

// WingetPath returns the resolved absolute path to winget.exe.
func (w *Windows) WingetPath() string {
	return w.wingetPath
}

// EnsurePackageManager resolves the path to winget.exe. If winget is not
// present it attempts an automatic install (per-user for interactive sessions,
// provisioned/system-wide when running as SYSTEM).
func (w *Windows) EnsurePackageManager() error {
	// Try to resolve an existing winget.exe.
	if p, err := sysutil.ResolveWingetPath(); err == nil {
		w.wingetPath = p
		return nil
	}

	ui.Status("INSTALL", "winget \u2013 not found, installing automatically...")

	// Install VCLibs dependency (required by winget on some systems).
	vcLibsScript := `Add-AppxPackage -Path "https://aka.ms/Microsoft.VCLibs.x64.14.00.Desktop.appx" -ErrorAction SilentlyContinue`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", vcLibsScript)
	cmd.CombinedOutput() // best-effort, may already be present

	if sysutil.IsSystemUser() {
		// Running as SYSTEM — use Add-AppxProvisionedPackage for a machine-wide install.
		installScript := strings.Join([]string{
			`$ProgressPreference = 'SilentlyContinue'`,
			`$releases = Invoke-RestMethod -Uri "https://api.github.com/repos/microsoft/winget-cli/releases/latest"`,
			`$url = ($releases.assets | Where-Object { $_.name -match '\.msixbundle$' }).browser_download_url`,
			`$lic = ($releases.assets | Where-Object { $_.name -match '_License.*\.xml$' }).browser_download_url`,
			`$out = Join-Path $env:TEMP "WinGet.msixbundle"`,
			`$licOut = Join-Path $env:TEMP "WinGet_License.xml"`,
			`Invoke-WebRequest -Uri $url -OutFile $out`,
			`Invoke-WebRequest -Uri $lic -OutFile $licOut`,
			`Add-AppxProvisionedPackage -Online -PackagePath $out -LicensePath $licOut -ErrorAction Stop`,
			`Remove-Item $out, $licOut -Force -ErrorAction SilentlyContinue`,
		}, "; ")

		cmd = exec.Command("powershell", "-NoProfile", "-Command", installScript)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install winget (SYSTEM): %w\nOutput: %s", err, string(output))
		}
	} else {
		// Interactive user — per-user install via Add-AppxPackage.
		installScript := strings.Join([]string{
			`$ProgressPreference = 'SilentlyContinue'`,
			`$releases = Invoke-RestMethod -Uri "https://api.github.com/repos/microsoft/winget-cli/releases/latest"`,
			`$url = ($releases.assets | Where-Object { $_.name -match '\.msixbundle$' }).browser_download_url`,
			`$out = Join-Path $env:TEMP "Microsoft.DesktopAppInstaller.msixbundle"`,
			`Invoke-WebRequest -Uri $url -OutFile $out`,
			`Add-AppxPackage -Path $out`,
			`Remove-Item $out -Force -ErrorAction SilentlyContinue`,
		}, "; ")

		cmd = exec.Command("powershell", "-NoProfile", "-Command", installScript)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install winget: %w\nOutput: %s", err, string(output))
		}
	}

	// Re-resolve after installation.
	sysutil.ResetWingetCache()
	p, err := sysutil.ResolveWingetPath()
	if err != nil {
		return fmt.Errorf("winget still not found after installation — please install manually: https://aka.ms/getwinget")
	}
	w.wingetPath = p

	ui.Status("DONE", "winget \u2013 installed successfully")
	return nil
}
