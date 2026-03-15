package winget

import (
	"fmt"
	"os/exec"

	"polaris/internal/config"
	"polaris/internal/provider"
	"polaris/internal/sysutil"
)

// winget exit codes
const (
	exitNoAppsFound         = 0x8a150014 // no installed package found matching input criteria
	exitUpdateNotApplicable = 0x8a15002b // package already installed, no update available
)

// Provider implements provider.PackageProvider using the Windows Package Manager (winget).
type Provider struct {
	exePath string
}

// New creates a new winget Provider. wingetPath must be the resolved absolute
// path to winget.exe (use sysutil.ResolveWingetPath or platform.WingetPath).
func New(wingetPath string) *Provider {
	return &Provider{exePath: wingetPath}
}

// Name returns "winget".
func (p *Provider) Name() string {
	return "winget"
}

// run executes winget with the given arguments. When asUser is true and
// the process runs as SYSTEM, the command is launched in the logged-in
// user's session (invisible, CREATE_NO_WINDOW) so that Microsoft Store
// packages can be installed.
func (p *Provider) run(asUser bool, args ...string) (output []byte, exitCode int, err error) {
	if asUser && sysutil.IsSystemUser() {
		// Use plain "winget.exe" so the user's own PATH resolves it.
		// The SYSTEM-resolved path (WindowsApps) is not accessible to
		// regular users.
		return sysutil.RunAsLoggedInUserWithExitCode("winget.exe", args...)
	}
	cmd := exec.Command(p.exePath, args...)
	output, err = cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return output, exitErr.ExitCode(), err
	}
	return output, 0, err
}

// IsInstalled checks whether a package is currently installed via winget.
func (p *Provider) IsInstalled(pkg config.Package) (bool, error) {
	args := []string{"list", "--id", pkg.ID, "--accept-source-agreements"}
	if pkg.Source != "msstore" {
		args = append(args, "--exact")
	}

	output, _, err := p.run(pkg.Source == "msstore", args...)
	if err != nil {
		// winget returns a non-zero exit code when the package is not found,
		// regardless of system language. Any exit error means "not installed".
		return false, nil
	}
	_ = output
	return true, nil
}

// Install installs a package via winget.
func (p *Provider) Install(pkg config.Package) error {
	args := []string{"install", "--id", pkg.ID, "--accept-source-agreements", "--accept-package-agreements"}

	if pkg.Source == "msstore" {
		args = append(args, "--source", "msstore")
	} else {
		args = append(args, "--exact", "--silent")
	}

	if pkg.Version != "" {
		args = append(args, "--version", pkg.Version)
	}

	output, code, err := p.run(pkg.Source == "msstore", args...)
	if err != nil {
		if code == exitUpdateNotApplicable {
			return provider.ErrAlreadyInstalled
		}
		return fmt.Errorf("winget install failed for %q: %w\nOutput: %s", pkg.ID, err, string(output))
	}
	return nil
}

// Upgrade upgrades a package to the latest version or to a specific version via winget.
func (p *Provider) Upgrade(pkg config.Package) error {
	args := []string{"upgrade", "--id", pkg.ID, "--accept-source-agreements", "--accept-package-agreements"}

	if pkg.Source == "msstore" {
		args = append(args, "--source", "msstore")
	} else {
		args = append(args, "--exact", "--silent")
	}

	if pkg.Version != "" {
		args = append(args, "--version", pkg.Version)
	}

	output, code, err := p.run(pkg.Source == "msstore", args...)
	if err != nil {
		if code == exitUpdateNotApplicable {
			return provider.ErrNoUpdateAvailable
		}
		return fmt.Errorf("winget upgrade failed for %q: %w\nOutput: %s", pkg.ID, err, string(output))
	}
	return nil
}

// SelfUpdate updates winget (App Installer) itself.
func (p *Provider) SelfUpdate() error {
	args := []string{"upgrade", "--id", "Microsoft.DesktopAppInstaller",
		"--source", "winget", "--silent",
		"--accept-source-agreements", "--accept-package-agreements"}

	output, code, err := p.run(false, args...)
	if err != nil {
		switch code {
		case exitUpdateNotApplicable, exitNoAppsFound:
			return provider.ErrNoUpdateAvailable
		}
		return fmt.Errorf("winget self-update failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// Uninstall removes a package via winget.
func (p *Provider) Uninstall(pkg config.Package) error {
	args := []string{"uninstall", "--id", pkg.ID, "--accept-source-agreements"}
	if pkg.Source == "msstore" {
		args = append(args, "--source", "msstore")
	} else {
		args = append(args, "--exact", "--silent")
	}

	output, code, err := p.run(pkg.Source == "msstore", args...)
	if err != nil {
		if code == exitNoAppsFound {
			return provider.ErrNotInstalled
		}
		return fmt.Errorf("winget uninstall failed for %q: %w\nOutput: %s", pkg.ID, err, string(output))
	}
	return nil
}
