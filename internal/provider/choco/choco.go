package choco

import (
	"fmt"
	"os/exec"
	"strings"

	"polaris/internal/config"
	"polaris/internal/provider"
)

// Provider implements provider.PackageProvider using Chocolatey.
type Provider struct{}

// New creates a new Chocolatey Provider.
func New() *Provider {
	return &Provider{}
}

// Name returns "choco".
func (p *Provider) Name() string {
	return "choco"
}

// findInstalledName returns the actual installed package name for a given ID.
// Chocolatey metapackages (e.g. "git") install a ".install" or ".portable"
// variant (e.g. "git.install"). The metapackage itself may not appear in the
// local list, so we also check for those variants.
// Returns an empty string if the package is not installed.
func (p *Provider) findInstalledName(id string) (string, error) {
	cmd := exec.Command("choco", "list", "--limit-output")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("choco list failed for %q: %w\nOutput: %s", id, err, string(output))
	}

	lowerID := strings.ToLower(id)
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "|", 2)
		if len(parts) < 1 {
			continue
		}
		name := strings.ToLower(parts[0])
		if name == lowerID || name == lowerID+".install" || name == lowerID+".portable" {
			return parts[0], nil
		}
	}
	return "", nil
}

// IsInstalled checks whether a package is currently installed via Chocolatey.
func (p *Provider) IsInstalled(pkg config.Package) (bool, error) {
	name, err := p.findInstalledName(pkg.ID)
	if err != nil {
		return false, err
	}
	return name != "", nil
}

// Install installs a package via Chocolatey.
func (p *Provider) Install(pkg config.Package) error {
	args := []string{"install", pkg.ID, "-y", "--no-progress"}

	if pkg.Version != "" {
		args = append(args, "--version", pkg.Version)
	}

	cmd := exec.Command("choco", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("choco install failed for %q: %w\nOutput: %s", pkg.ID, err, string(output))
	}
	return nil
}

// Upgrade upgrades a package to the latest version or to a specific version via Chocolatey.
func (p *Provider) Upgrade(pkg config.Package) error {
	args := []string{"upgrade", pkg.ID, "-y", "--no-progress"}

	if pkg.Version != "" {
		args = append(args, "--version", pkg.Version)
	}

	cmd := exec.Command("choco", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("choco upgrade failed for %q: %w\nOutput: %s", pkg.ID, err, string(output))
	}

	// Chocolatey exits 0 even when no upgrade is available.
	// Check output for the "already installed" indicator.
	out := string(output)
	if strings.Contains(out, "already installed") || strings.Contains(out, "is the latest version") {
		return provider.ErrNoUpdateAvailable
	}

	return nil
}

// SelfUpdate updates Chocolatey itself.
func (p *Provider) SelfUpdate() error {
	cmd := exec.Command("choco", "upgrade", "chocolatey", "-y", "--no-progress")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("choco self-update failed: %w\nOutput: %s", err, string(output))
	}

	out := string(output)
	if strings.Contains(out, "already installed") || strings.Contains(out, "is the latest version") {
		return provider.ErrNoUpdateAvailable
	}

	return nil
}

// Uninstall removes a package via Chocolatey.
// If the exact ID is a metapackage (e.g. "git") that is installed as a variant
// (e.g. "git.install"), we uninstall the actual installed name instead.
func (p *Provider) Uninstall(pkg config.Package) error {
	target := pkg.ID
	if name, err := p.findInstalledName(pkg.ID); err == nil && name != "" {
		target = name
	}

	args := []string{"uninstall", target, "-y", "--no-progress", "--remove-dependencies"}

	cmd := exec.Command("choco", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("choco uninstall failed for %q: %w\nOutput: %s", pkg.ID, err, string(output))
	}
	return nil
}
