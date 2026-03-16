package appx

import (
	"fmt"
	"os/exec"
	"strings"

	"polaris/internal/config"
	"polaris/internal/provider"
)

// Provider implements provider.PackageProvider for AppX/MSIX packages
// using PowerShell's Get-AppxPackage and Remove-AppxPackage cmdlets.
// This is the correct way to manage pre-provisioned Windows apps
// (e.g. Feedback Hub, Clipchamp, Microsoft News) that winget cannot
// reliably detect or remove via MS Store product IDs.
//
// Package IDs are AppX package names such as "Microsoft.WindowsFeedbackHub"
// or wildcard patterns like "Microsoft.BingNews*".
type Provider struct{}

// New creates a new AppX Provider.
func New() *Provider {
	return &Provider{}
}

// Name returns "appx".
func (p *Provider) Name() string {
	return "appx"
}

// IsInstalled checks whether an AppX package is installed for any user.
func (p *Provider) IsInstalled(pkg config.Package) (bool, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Get-AppxPackage -AllUsers -Name '%s' | Measure-Object | Select-Object -ExpandProperty Count`,
			sanitizePSString(pkg.ID)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("checking AppX package %q: %w\nOutput: %s", pkg.ID, err, string(output))
	}
	count := strings.TrimSpace(string(output))
	return count != "" && count != "0", nil
}

// Install is not supported for AppX packages. Use source "msstore" or
// "winget" to install packages. The appx provider only supports
// checking installed state and removing packages.
func (p *Provider) Install(pkg config.Package) error {
	return fmt.Errorf("appx provider does not support installing packages — use source \"msstore\" or \"winget\" instead")
}

// Uninstall removes an AppX package for all users and removes its
// provisioning so it does not get reinstalled for new user profiles.
func (p *Provider) Uninstall(pkg config.Package) error {
	escapedID := sanitizePSString(pkg.ID)

	// Remove the package for all users.
	removeCmd := fmt.Sprintf(
		`Get-AppxPackage -AllUsers -Name '%s' | Remove-AppxPackage -AllUsers -ErrorAction Stop`,
		escapedID)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", removeCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(output))
		if outStr == "" {
			return provider.ErrNotInstalled
		}
		return fmt.Errorf("removing AppX package %q: %w\nOutput: %s", pkg.ID, err, outStr)
	}

	// Also remove provisioning so the app doesn't come back for new users.
	deprovisionCmd := fmt.Sprintf(
		`Get-AppxProvisionedPackage -Online | Where-Object { $_.DisplayName -like '%s' } | Remove-AppxProvisionedPackage -Online -ErrorAction SilentlyContinue`,
		escapedID)
	cmd = exec.Command("powershell", "-NoProfile", "-Command", deprovisionCmd)
	_ = cmd.Run() // best-effort, don't fail if deprovisioning fails

	return nil
}

// Upgrade is not supported for AppX packages.
func (p *Provider) Upgrade(pkg config.Package) error {
	return provider.ErrNoUpdateAvailable
}

// SelfUpdate is a no-op for the AppX provider.
func (p *Provider) SelfUpdate() error {
	return provider.ErrNoUpdateAvailable
}

// sanitizePSString makes a string safe for embedding in a PowerShell
// single-quoted literal by doubling any single quotes.
func sanitizePSString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
