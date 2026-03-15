package sysutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	wingetOnce sync.Once
	wingetPath string
	wingetErr  error
)

// IsSystemUser returns true when the current process is running as
// NT AUTHORITY\SYSTEM (e.g. as a Windows service).
func IsSystemUser() bool {
	// SYSTEM's profile directory is %SystemRoot%\system32\config\systemprofile.
	// The simplest heuristic: check USERPROFILE or USERNAME.
	user := os.Getenv("USERNAME")
	if strings.EqualFold(user, "SYSTEM") {
		return true
	}
	profile := os.Getenv("USERPROFILE")
	return strings.HasSuffix(strings.ToLower(profile), `\systemprofile`)
}

// ResolveWingetPath returns the absolute path to winget.exe.
// It searches PATH first and then the well-known WindowsApps location
// used by the MSIX-packaged App Installer. The result is cached.
func ResolveWingetPath() (string, error) {
	wingetOnce.Do(func() {
		wingetPath, wingetErr = resolveWinget()
	})
	return wingetPath, wingetErr
}

// ResetWingetCache clears the cached winget path so that the next call
// to ResolveWingetPath re-evaluates. Used after installing winget.
func ResetWingetCache() {
	wingetOnce = sync.Once{}
	wingetPath = ""
	wingetErr = nil
}

func resolveWinget() (string, error) {
	// 1. Try PATH (works for interactive users).
	if p, err := exec.LookPath("winget.exe"); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath("winget"); err == nil {
		return p, nil
	}

	// 2. Search the WindowsApps directory (works for SYSTEM).
	return searchWindowsApps()
}

// searchWindowsApps globs for winget.exe inside the DesktopAppInstaller
// MSIX package folder and returns the newest version found.
func searchWindowsApps() (string, error) {
	programFiles := os.Getenv("ProgramFiles")
	if programFiles == "" {
		programFiles = `C:\Program Files`
	}

	pattern := filepath.Join(programFiles, "WindowsApps",
		"Microsoft.DesktopAppInstaller_*_x64__8wekyb3d8bbwe", "winget.exe")

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		// Also try without architecture qualifier (arm64 systems).
		pattern = filepath.Join(programFiles, "WindowsApps",
			"Microsoft.DesktopAppInstaller_*_*__8wekyb3d8bbwe", "winget.exe")
		matches, _ = filepath.Glob(pattern)
	}

	if len(matches) == 0 {
		return "", exec.ErrNotFound
	}

	// Sort descending to pick the latest version.
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	return matches[0], nil
}
