package executor

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"polaris/internal/config"
	"polaris/internal/defender"
	"polaris/internal/grouppolicy"
	"polaris/internal/platform"
	"polaris/internal/provider"
	"polaris/internal/provider/appx"
	"polaris/internal/provider/choco"
	"polaris/internal/provider/winget"
	"polaris/internal/registry"
	"polaris/internal/ui"
	"polaris/internal/users"
	"polaris/internal/windowsupdate"
)

// Executor orchestrates the application of the desired state.
type Executor struct {
	version string
	winget  provider.PackageProvider
	choco   provider.PackageProvider
	appx    provider.PackageProvider
}

// New creates a new Executor. version is the running Polaris version (e.g. "1.0.0" or "dev").
func New(version string) *Executor {
	return &Executor{version: version}
}

// Apply reads the desired state from cfg and ensures the system matches it.
func (e *Executor) Apply(cfg *config.Config) error {
	plat, err := platform.Detect()
	if err != nil {
		return err
	}

	// Check root file compatibility (included files are already checked at load time).
	if err := config.CheckCompatibility(cfg.Compatibility, e.version, runtime.GOOS, runtime.GOARCH, plat.OSVersion()); err != nil {
		return fmt.Errorf("compatibility check failed: %w", err)
	}

	if err := plat.EnsurePackageManager(); err != nil {
		return fmt.Errorf("winget not available: %w", err)
	}

	e.winget = winget.New(plat.WingetPath())
	e.appx = appx.New()
	needsChoco := e.needsProvider(cfg, "choco")

	var totalOK, totalChanged, totalFailed int

	// Self-update package managers.
	ui.Section("Package Managers")
	e.selfUpdate("winget", e.winget)

	if needsChoco {
		if err := e.ensureChoco(); err != nil {
			return err
		}
		e.choco = choco.New()
		e.selfUpdate("choco", e.choco)
	}

	// Apply Windows Update settings.
	if cfg.WindowsUpdate != nil {
		ui.Section("Windows Update")
		c, err := windowsupdate.Apply(cfg.WindowsUpdate)
		if err != nil {
			ui.Statusf("ERROR", "Windows Update: %v", err)
			totalFailed++
		} else if c > 0 {
			totalChanged++
		} else {
			totalOK++
		}
	}

	// Apply registry entries.
	if len(cfg.Registry) > 0 {
		ui.Section("Registry")
		c, err := registry.Apply(cfg.Registry)
		if err != nil {
			ui.Statusf("ERROR", "Registry: %v", err)
			totalFailed++
		} else if c > 0 {
			totalChanged++
		} else {
			totalOK++
		}
	}

	// Apply Windows Defender settings.
	if cfg.WindowsDefender != nil {
		ui.Section("Windows Defender")
		c, err := defender.Apply(cfg.WindowsDefender)
		if err != nil {
			ui.Statusf("ERROR", "Windows Defender: %v", err)
			totalFailed++
		} else if c > 0 {
			totalChanged++
		} else {
			totalOK++
		}
	}

	// Apply user accounts.
	if len(cfg.Users) > 0 {
		ui.Section("Users")
		c, err := users.Apply(cfg.Users)
		if err != nil {
			ui.Statusf("ERROR", "Users: %v", err)
			totalFailed++
		} else if c > 0 {
			totalChanged++
		} else {
			totalOK++
		}
	}

	// Apply group policy settings.
	if len(cfg.GroupPolicy) > 0 {
		ui.Section("Group Policy")
		c, err := grouppolicy.Apply(cfg.GroupPolicy)
		if err != nil {
			ui.Statusf("ERROR", "Group Policy: %v", err)
			totalFailed++
		} else if c > 0 {
			totalChanged++
		} else {
			totalOK++
		}
	}

	// Apply packages.
	ui.Section(fmt.Sprintf("Packages (%d)", len(cfg.Packages)))
	results := e.applyPackages(cfg.Packages)

	// Aggregate package results into totals.
	for _, r := range results {
		switch {
		case r.Error != nil:
			totalFailed++
		case r.Changed:
			totalChanged++
		default:
			totalOK++
		}
	}

	totalSkipped := cfg.Skipped
	total := totalOK + totalChanged + totalSkipped + totalFailed
	ui.Summary(totalOK, totalChanged, totalSkipped, totalFailed, total)

	if totalFailed > 0 {
		return fmt.Errorf("%d operation(s) failed", totalFailed)
	}
	return nil
}

// needsProvider checks whether any package in the config uses the given source.
func (e *Executor) needsProvider(cfg *config.Config, source string) bool {
	for _, pkg := range cfg.Packages {
		if pkg.Source == source {
			return true
		}
	}
	return false
}

// selfUpdate updates a package manager and prints the result.
func (e *Executor) selfUpdate(name string, prov provider.PackageProvider) {
	if err := prov.SelfUpdate(); err != nil {
		if errors.Is(err, provider.ErrNoUpdateAvailable) {
			ui.Statusf("OK", "%s \u2013 already up to date", name)
		} else {
			ui.Statusf("ERROR", "%s \u2013 update failed: %v", name, err)
		}
	} else {
		ui.Statusf("DONE", "%s \u2013 updated successfully", name)
	}
}

// ensureChoco installs Chocolatey via winget if it is not already available.
func (e *Executor) ensureChoco() error {
	if _, err := exec.LookPath("choco"); err == nil {
		return nil
	}

	ui.Status("INSTALL", "Chocolatey \u2013 installing via winget...")
	chocoPkg := config.Package{ID: "Chocolatey.Chocolatey", Source: "winget"}
	if err := e.winget.Install(chocoPkg); err != nil {
		if errors.Is(err, provider.ErrAlreadyInstalled) {
			ui.Status("OK", "Chocolatey \u2013 already installed")
			return nil
		}
		ui.Statusf("ERROR", "Chocolatey \u2013 %v", err)
		return fmt.Errorf("failed to install Chocolatey: %w", err)
	}
	ui.Status("DONE", "Chocolatey \u2013 installed successfully")
	return nil
}

// providerFor returns the appropriate PackageProvider for a package source.
func (e *Executor) providerFor(pkg config.Package) provider.PackageProvider {
	switch pkg.Source {
	case "choco":
		return e.choco
	case "appx":
		return e.appx
	default:
		return e.winget
	}
}

// applyPackages iterates over all desired packages and ensures their state.
func (e *Executor) applyPackages(packages []config.Package) []provider.Result {
	results := make([]provider.Result, 0, len(packages))

	for _, pkg := range packages {
		prov := e.providerFor(pkg)
		result := e.applyPackage(prov, pkg)
		results = append(results, result)
	}

	return results
}

// applyPackage ensures a single package matches its desired state.
func (e *Executor) applyPackage(prov provider.PackageProvider, pkg config.Package) provider.Result {
	result := provider.Result{Package: pkg}

	// For msstore packages, IsInstalled is unreliable because winget uses
	// different internal IDs for installed store apps. We skip the check
	// and always attempt the operation, then interpret the result.
	if pkg.Source == "msstore" {
		return e.applyPackageDirect(prov, pkg)
	}

	installed, err := prov.IsInstalled(pkg)
	if err != nil {
		result.Error = err
		ui.Statusf("ERROR", "%s (%s) \u2013 %v", pkg.Name, pkg.ID, err)
		return result
	}

	pkgLabel := fmt.Sprintf("%s (%s)", pkg.Name, pkg.ID)

	switch pkg.State {
	case "present":
		if installed {
			if err := prov.Upgrade(pkg); err != nil {
				if errors.Is(err, provider.ErrNoUpdateAvailable) {
					ui.Statusf("OK", "%s \u2013 up to date", pkgLabel)
				} else {
					result.Error = err
					ui.Statusf("ERROR", "%s \u2013 %v", pkgLabel, err)
				}
			} else {
				result.Changed = true
				ui.Statusf("DONE", "%s \u2013 upgraded", pkgLabel)
			}
		} else {
			ui.Statusf("INSTALL", "%s \u2013 installing...", pkgLabel)
			if err := prov.Install(pkg); err != nil {
				if errors.Is(err, provider.ErrAlreadyInstalled) {
					ui.Statusf("OK", "%s \u2013 already installed", pkgLabel)
				} else {
					result.Error = err
					ui.Statusf("ERROR", "%s \u2013 %v", pkgLabel, err)
				}
			} else {
				result.Changed = true
				ui.Statusf("DONE", "%s \u2013 installed", pkgLabel)
			}
		}
	case "absent":
		if !installed {
			ui.Statusf("OK", "%s \u2013 already absent", pkgLabel)
		} else {
			ui.Statusf("REMOVE", "%s \u2013 removing...", pkgLabel)
			if err := prov.Uninstall(pkg); err != nil {
				result.Error = err
				ui.Statusf("ERROR", "%s \u2013 %v", pkgLabel, err)
			} else {
				result.Changed = true
				ui.Statusf("DONE", "%s \u2013 removed", pkgLabel)
			}
		}
	}

	return result
}

// applyPackageDirect attempts the operation without checking IsInstalled first.
// Used for msstore packages where IsInstalled is unreliable.
func (e *Executor) applyPackageDirect(prov provider.PackageProvider, pkg config.Package) provider.Result {
	result := provider.Result{Package: pkg}
	pkgLabel := fmt.Sprintf("%s (%s)", pkg.Name, pkg.ID)

	switch pkg.State {
	case "present":
		ui.Statusf("INSTALL", "%s \u2013 installing...", pkgLabel)
		if err := prov.Install(pkg); err != nil {
			if errors.Is(err, provider.ErrAlreadyInstalled) {
				if err := prov.Upgrade(pkg); err != nil {
					if errors.Is(err, provider.ErrNoUpdateAvailable) {
						ui.Statusf("OK", "%s \u2013 up to date", pkgLabel)
					} else {
						result.Error = err
						ui.Statusf("ERROR", "%s \u2013 %v", pkgLabel, err)
					}
				} else {
					result.Changed = true
					ui.Statusf("DONE", "%s \u2013 upgraded", pkgLabel)
				}
			} else {
				result.Error = err
				ui.Statusf("ERROR", "%s \u2013 %v", pkgLabel, err)
			}
		} else {
			result.Changed = true
			ui.Statusf("DONE", "%s \u2013 installed", pkgLabel)
		}
	case "absent":
		ui.Statusf("REMOVE", "%s \u2013 removing...", pkgLabel)
		if err := prov.Uninstall(pkg); err != nil {
			if errors.Is(err, provider.ErrNotInstalled) {
				ui.Statusf("OK", "%s \u2013 already absent", pkgLabel)
			} else {
				result.Error = err
				ui.Statusf("ERROR", "%s \u2013 %v", pkgLabel, err)
			}
		} else {
			result.Changed = true
			ui.Statusf("DONE", "%s \u2013 removed", pkgLabel)
		}
	}

	return result
}
