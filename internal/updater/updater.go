package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"polaris/internal/config"
)

// Manifest is the remote update descriptor served as JSON.
// It contains a single version and a map of per-platform binaries
// keyed by "os/arch" (e.g. "windows/amd64", "linux/arm64").
//
//	{
//	  "version": "1.2.3",
//	  "binaries": {
//	    "windows/amd64": { "url": "https://…/polaris.exe", "sha256": "abc…" },
//	    "linux/amd64":   { "url": "https://…/polaris",     "sha256": "def…" }
//	  }
//	}
type Manifest struct {
	Version  string                 `json:"version"`
	Binaries map[string]BinaryEntry `json:"binaries"`
}

// BinaryEntry describes a single downloadable binary for one OS/arch.
type BinaryEntry struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// updateState tracks an in-progress update so the new binary can
// finalize on success or rollback on failure.
type updateState struct {
	PreviousVersion string    `json:"previous_version"`
	NewVersion      string    `json:"new_version"`
	BackupPath      string    `json:"backup_path"`
	Timestamp       time.Time `json:"timestamp"`
	Attempts        int       `json:"attempts"`
}

const (
	stateFile       = "update.json"
	maxAttempts     = 3
	watchdogTimeout = 10 * time.Minute

	// maxManifestSize limits the update manifest to 1 MB.
	maxManifestSize = 1 << 20
	// maxBinarySize limits binary downloads to 500 MB.
	maxBinarySize = 500 << 20
)

// ──────────────────────────────────────────────────────────────────────
// Startup rollback guard
// ──────────────────────────────────────────────────────────────────────

// CheckPendingUpdate must be called early on startup. If a previous
// update left a state file, it either finalizes the update (success) or
// rolls back to the backup (failure). Returns true if a rollback was
// performed and the caller should exit so the SCM can restart with the
// restored binary.
func CheckPendingUpdate(currentVersion string) (rolledBack bool) {
	sp, err := statePath()
	if err != nil {
		return false
	}

	data, err := os.ReadFile(sp)
	if err != nil {
		return false // no pending update
	}

	var state updateState
	if err := json.Unmarshal(data, &state); err != nil {
		os.Remove(sp)
		return false
	}

	// Watchdog: if state is too old, something went very wrong → rollback.
	if time.Since(state.Timestamp) > watchdogTimeout {
		log.Printf("[Polaris] update watchdog triggered (%s old), rolling back",
			time.Since(state.Timestamp).Round(time.Second))
		return rollback(&state, sp)
	}

	// Increment attempt counter immediately (persisted before any work).
	state.Attempts++
	writeState(sp, &state)

	if state.Attempts > maxAttempts {
		log.Printf("[Polaris] update failed after %d start attempts, rolling back", maxAttempts)
		return rollback(&state, sp)
	}

	// If we reach here, the new binary started successfully.
	finalize(&state, sp)
	return false
}

// rollback restores the backup binary and removes the state file.
// Returns true so the caller knows to exit (SCM will restart with the
// old binary).
func rollback(state *updateState, sp string) bool {
	exePath, exeErr := os.Executable()
	if exeErr != nil {
		log.Printf("[Polaris] rollback: cannot determine exe path: %v", exeErr)
		os.Remove(sp)
		return false
	}

	if _, err := os.Stat(state.BackupPath); err != nil {
		log.Printf("[Polaris] rollback: backup %q not found, giving up", state.BackupPath)
		os.Remove(sp)
		return false
	}

	// Move current (broken) binary out of the way.
	failedPath := exePath + ".failed"
	os.Remove(failedPath)
	if err := os.Rename(exePath, failedPath); err != nil {
		log.Printf("[Polaris] rollback: cannot rename current binary: %v", err)
		os.Remove(sp)
		return false
	}

	// Restore backup.
	if err := os.Rename(state.BackupPath, exePath); err != nil {
		// Critical: try to undo the rename so we at least have something.
		os.Rename(failedPath, exePath)
		log.Printf("[Polaris] rollback: cannot restore backup: %v", err)
		os.Remove(sp)
		return false
	}

	os.Remove(sp)
	os.Remove(failedPath)
	log.Printf("[Polaris] rollback to %s complete — restarting", state.PreviousVersion)
	return true
}

// finalize removes the backup and state file after a successful update.
func finalize(state *updateState, sp string) {
	os.Remove(state.BackupPath)
	os.Remove(sp)
	log.Printf("[Polaris] update finalized: %s → %s", state.PreviousVersion, state.NewVersion)
}

// ──────────────────────────────────────────────────────────────────────
// Update check & apply
// ──────────────────────────────────────────────────────────────────────

// CheckAndUpdate fetches the update manifest and applies the update if
// a newer version is available. Returns true if the binary was replaced
// and the caller should trigger a restart (exit with non-zero code so
// the SCM recovery mechanism kicks in).
func CheckAndUpdate(currentVersion string, updateCfg *config.UpdateConfig) (needsRestart bool, err error) {
	if updateCfg == nil || updateCfg.URL == "" {
		return false, nil
	}

	manifest, err := fetchManifest(updateCfg.URL, updateCfg.Auth)
	if err != nil {
		return false, fmt.Errorf("fetching update manifest: %w", err)
	}

	if manifest.Version == "" {
		return false, nil
	}
	// Normalize version strings for comparison (strip leading "v").
	if normalizeVersion(manifest.Version) == normalizeVersion(currentVersion) {
		return false, nil
	}

	// Resolve the binary entry for the current platform.
	platformKey := runtime.GOOS + "/" + runtime.GOARCH
	bin, ok := manifest.Binaries[platformKey]
	if !ok {
		return false, fmt.Errorf("no binary in manifest for platform %s", platformKey)
	}

	log.Printf("[Polaris] update available: %s → %s (%s)", currentVersion, manifest.Version, platformKey)

	exePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("cannot determine executable path: %w", err)
	}

	newPath := exePath + ".new"
	backupPath := exePath + ".bak"

	// Download new binary (using same auth as the manifest if configured).
	if err := downloadFile(bin.URL, newPath, updateCfg.Auth); err != nil {
		return false, fmt.Errorf("downloading update: %w", err)
	}

	// Verify SHA-256.
	if err := verifyChecksum(newPath, bin.SHA256); err != nil {
		os.Remove(newPath)
		return false, fmt.Errorf("checksum verification failed: %w", err)
	}

	// Backup current binary.
	os.Remove(backupPath)
	if err := os.Rename(exePath, backupPath); err != nil {
		os.Remove(newPath)
		return false, fmt.Errorf("backing up current binary: %w", err)
	}

	// Place new binary.
	if err := os.Rename(newPath, exePath); err != nil {
		// Critical: restore backup immediately.
		os.Rename(backupPath, exePath)
		return false, fmt.Errorf("placing new binary: %w", err)
	}

	// Write update state for the rollback guard.
	sp, spErr := statePath()
	if spErr != nil {
		log.Printf("[Polaris] warning: cannot write update state: %v", spErr)
	}
	writeState(sp, &updateState{
		PreviousVersion: currentVersion,
		NewVersion:      manifest.Version,
		BackupPath:      backupPath,
		Timestamp:       time.Now(),
		Attempts:        0,
	})

	log.Printf("[Polaris] update %s → %s applied, restart required", currentVersion, manifest.Version)
	return true, nil
}

// ──────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────

func exeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}
	return filepath.Dir(exe), nil
}

func statePath() (string, error) {
	dir, err := exeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFile), nil
}

func writeState(path string, state *updateState) {
	if path == "" {
		return
	}
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.WriteFile(path, data, 0600)
}

// normalizeVersion strips a leading "v" prefix for comparison.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func fetchManifest(url string, auth *config.AuthConfig) (*Manifest, error) {
	client, cleanup, err := config.BuildAuthClient(auth)
	if err != nil {
		return nil, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if auth != nil && auth.Type == "basic" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Limit manifest size to prevent abuse.
	limited := io.LimitReader(resp.Body, maxManifestSize)
	var m Manifest
	if err := json.NewDecoder(limited).Decode(&m); err != nil {
		return nil, fmt.Errorf("decoding manifest: %w", err)
	}

	if m.Version == "" {
		return nil, fmt.Errorf("manifest has no version")
	}
	if len(m.Binaries) == 0 {
		return nil, fmt.Errorf("manifest has no binaries")
	}

	return &m, nil
}

func downloadFile(url, dest string, auth *config.AuthConfig) error {
	client, cleanup, err := config.BuildAuthClient(auth)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if auth != nil && auth.Type == "basic" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}

	// Limit download size to prevent disk exhaustion.
	_, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxBinarySize))

	// Sync to disk before closing to catch write errors.
	if copyErr == nil {
		copyErr = f.Sync()
	}

	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(dest)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(dest)
		return closeErr
	}

	return nil
}

func verifyChecksum(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("sha256 mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}
