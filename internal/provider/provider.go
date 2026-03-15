package provider

import (
	"errors"

	"polaris/internal/config"
)

// ErrAlreadyInstalled is returned by Install when the package is already installed
// and no update is available. This can happen with msstore packages where IsInstalled
// cannot reliably detect the installed state.
var ErrAlreadyInstalled = errors.New("package is already installed")

// ErrNotInstalled is returned by Uninstall when the package is not installed.
// This can happen with msstore packages where IsInstalled cannot reliably detect the state.
var ErrNotInstalled = errors.New("package is not installed")

// ErrNoUpdateAvailable is returned by Upgrade when the package is already at the latest
// (or the specified) version.
var ErrNoUpdateAvailable = errors.New("no update available")

// Result holds the outcome of a single package operation.
type Result struct {
	Package config.Package
	Changed bool
	Error   error
}

// PackageProvider is the interface that all package management backends must implement.
// Adding support for a new package manager only requires a new implementation of
// this interface.
type PackageProvider interface {
	// Name returns the provider identifier (e.g. "winget", "choco").
	Name() string

	// IsInstalled checks whether a package is currently installed.
	IsInstalled(pkg config.Package) (bool, error)

	// Install installs a package.
	Install(pkg config.Package) error

	// Uninstall removes a package.
	Uninstall(pkg config.Package) error

	// Upgrade upgrades a package to the latest version or to a specific version.
	Upgrade(pkg config.Package) error

	// SelfUpdate updates the package manager itself.
	SelfUpdate() error
}
