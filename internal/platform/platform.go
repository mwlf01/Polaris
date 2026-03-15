package platform

import (
	"fmt"
	"runtime"
)

// Platform provides an abstraction over OS-specific operations.
// Future implementations for Linux and macOS can be added here.
type Platform interface {
	// Name returns the platform identifier (e.g. "windows", "linux", "darwin").
	Name() string

	// OSVersion returns a human-friendly OS version string.
	// On Windows this returns "10", "11", "Server 2019", "Server 2022", etc.
	OSVersion() string

	// EnsurePackageManager checks whether the required package manager is installed.
	// If it is not present, it attempts to install it automatically.
	EnsurePackageManager() error

	// WingetPath returns the resolved absolute path to winget.exe.
	// Only valid after a successful call to EnsurePackageManager.
	WingetPath() string
}

// Detect returns the Platform implementation for the current operating system.
func Detect() (Platform, error) {
	switch runtime.GOOS {
	case "windows":
		return &Windows{}, nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
