package grouppolicy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"polaris/internal/config"
	"polaris/internal/sysutil"
	"polaris/internal/ui"
)

const lgpoWingetID = "Microsoft.SecurityComplianceToolkit.LGPO"

// lgpoTypeMap maps config type names to LGPO text format type prefixes.
var lgpoTypeMap = map[string]string{
	"string":        "SZ",
	"expand_string": "EXPAND_SZ",
	"dword":         "DWORD",
	"qword":         "QWORD",
	"multi_string":  "MULTI_SZ",
}

// hiveMap maps scope to the registry hive used for reading current values.
var hiveMap = map[string]registry.Key{
	"computer": registry.LOCAL_MACHINE,
	"user":     registry.CURRENT_USER,
}

// Apply processes all group policy entries via LGPO.exe.
// Returns the number of settings changed.
func Apply(entries []config.GroupPolicyEntry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}

	// Determine which entries actually need changes.
	needsChange := make([]bool, len(entries))
	changeCount := 0
	for i, entry := range entries {
		needsChange[i], _ = entryNeedsChange(entry)
		if needsChange[i] {
			changeCount++
		}
	}

	if changeCount == 0 {
		ui.Status("OK", "already configured")
		return 0, nil
	}

	// Ensure LGPO.exe is available.
	lgpoPath, err := ensureLGPO()
	if err != nil {
		return 0, fmt.Errorf("ensuring LGPO is available: %w", err)
	}

	// Generate LGPO text file with only the entries that need changes.
	textContent := generateLGPOText(entries, needsChange)

	// Write to a temp file.
	tmpFile, err := os.CreateTemp("", "polaris-gpo-*.txt")
	if err != nil {
		return 0, fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(textContent); err != nil {
		tmpFile.Close()
		return 0, fmt.Errorf("writing LGPO text: %w", err)
	}
	tmpFile.Close()

	// Apply via LGPO.exe.
	cmd := exec.Command(lgpoPath, "/t", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("LGPO.exe failed: %w\nOutput: %s", err, string(output))
	}

	ui.Statusf("DONE", "%d setting(s) changed", changeCount)
	return changeCount, nil
}

// entryNeedsChange checks if a group policy entry differs from current state
// by reading the corresponding registry value under the policy key.
func entryNeedsChange(entry config.GroupPolicyEntry) (bool, error) {
	hive, ok := hiveMap[entry.Scope]
	if !ok {
		return true, nil
	}

	key, err := registry.OpenKey(hive, entry.Path, registry.QUERY_VALUE)
	if err != nil {
		// Key doesn't exist.
		return entry.State == "present", nil
	}
	defer key.Close()

	if entry.State == "absent" {
		if entry.Name == "" {
			// Want to delete key — it exists, so change needed.
			return true, nil
		}
		// Check if value exists.
		_, _, err := key.GetValue(entry.Name, nil)
		if err != nil {
			// Value doesn't exist — already absent.
			return false, nil
		}
		return true, nil
	}

	// State is "present" — check current value.
	switch entry.Type {
	case "dword", "qword":
		val, _, err := key.GetIntegerValue(entry.Name)
		if err != nil {
			return true, nil
		}
		desired, err := toUint64(entry.Value)
		if err != nil {
			return true, nil
		}
		return val != desired, nil

	case "string", "expand_string":
		val, _, err := key.GetStringValue(entry.Name)
		if err != nil {
			return true, nil
		}
		desired, ok := entry.Value.(string)
		if !ok {
			return true, nil
		}
		return val != desired, nil

	case "multi_string":
		val, _, err := key.GetStringsValue(entry.Name)
		if err != nil {
			return true, nil
		}
		desired, err := toStringSlice(entry.Value)
		if err != nil {
			return true, nil
		}
		return !stringSlicesEqual(val, desired), nil
	}

	return true, nil
}

// generateLGPOText creates the LGPO text format for the given entries.
// Only entries where needsChange[i] is true are included.
func generateLGPOText(entries []config.GroupPolicyEntry, needsChange []bool) string {
	var sb strings.Builder

	for i, entry := range entries {
		if !needsChange[i] {
			continue
		}

		scope := "Computer"
		if entry.Scope == "user" {
			scope = "User"
		}

		sb.WriteString(scope)
		sb.WriteString("\n")
		sb.WriteString(entry.Path)
		sb.WriteString("\n")

		if entry.State == "absent" {
			if entry.Name == "" {
				// Delete all values in the key.
				sb.WriteString("*\n")
				sb.WriteString("DELETEALLVALUES\n")
			} else {
				sb.WriteString(entry.Name)
				sb.WriteString("\n")
				sb.WriteString("DELETE\n")
			}
		} else {
			sb.WriteString(entry.Name)
			sb.WriteString("\n")
			sb.WriteString(formatLGPOValue(entry))
			sb.WriteString("\n")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatLGPOValue formats the value for LGPO text format.
func formatLGPOValue(entry config.GroupPolicyEntry) string {
	prefix := lgpoTypeMap[entry.Type]

	switch entry.Type {
	case "dword", "qword":
		v, _ := toUint64(entry.Value)
		return fmt.Sprintf("%s:%d", prefix, v)
	case "string", "expand_string":
		s, _ := entry.Value.(string)
		return fmt.Sprintf("%s:%s", prefix, s)
	case "multi_string":
		items, _ := toStringSlice(entry.Value)
		// LGPO uses \0 as separator for MULTI_SZ.
		return fmt.Sprintf("%s:%s", prefix, strings.Join(items, "\\0"))
	}
	return ""
}

// ensureLGPO checks if LGPO.exe is on PATH or in well-known locations.
// If not found, it installs it via winget and returns the full path.
// Works correctly under both interactive users and SYSTEM.
func ensureLGPO() (string, error) {
	// Check if already on PATH.
	path, err := exec.LookPath("lgpo.exe")
	if err == nil {
		return path, nil
	}

	// Check common install location.
	programFiles := os.Getenv("ProgramFiles")
	lgpoDir := filepath.Join(programFiles, "LGPO")
	lgpoExe := filepath.Join(lgpoDir, "LGPO.exe")
	if _, err := os.Stat(lgpoExe); err == nil {
		return lgpoExe, nil
	}

	// Check winget symlinks directory. Under SYSTEM %LOCALAPPDATA% points to
	// the system profile, so we also check the ProgramFiles-based location
	// that winget uses for machine-scope installs.
	for _, base := range lgpoSearchBases() {
		candidate := filepath.Join(base, "Microsoft", "WinGet", "Links", "LGPO.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Not found — install via winget.
	wingetExe, err := sysutil.ResolveWingetPath()
	if err != nil {
		return "", fmt.Errorf("cannot install LGPO: winget not found")
	}

	ui.Status("INSTALL", "LGPO \u2013 installing via winget...")
	cmd := exec.Command(wingetExe, "install", "--id", lgpoWingetID,
		"--exact", "--silent", "--accept-source-agreements", "--accept-package-agreements")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("installing LGPO via winget: %w\nOutput: %s", err, string(output))
	}

	// Try to find it again after install.
	path, err = exec.LookPath("lgpo.exe")
	if err == nil {
		return path, nil
	}
	if _, err := os.Stat(lgpoExe); err == nil {
		return lgpoExe, nil
	}
	for _, base := range lgpoSearchBases() {
		candidate := filepath.Join(base, "Microsoft", "WinGet", "Links", "LGPO.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("LGPO.exe not found after installation")
}

// lgpoSearchBases returns directories to search for winget symlinks.
// Under SYSTEM the normal %LOCALAPPDATA% is the system profile which
// is unlikely to contain user-installed symlinks, so we also check
// the ProgramFiles-based location.
func lgpoSearchBases() []string {
	bases := []string{}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		bases = append(bases, la)
	}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		bases = append(bases, pf)
	}
	return bases
}

// --- Helpers ---

func toUint64(v interface{}) (uint64, error) {
	switch n := v.(type) {
	case int:
		if n < 0 {
			return 0, fmt.Errorf("value %d is negative, cannot convert to uint64", n)
		}
		return uint64(n), nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("value %d is negative, cannot convert to uint64", n)
		}
		return uint64(n), nil
	case float64:
		if n < 0 {
			return 0, fmt.Errorf("value %f is negative, cannot convert to uint64", n)
		}
		return uint64(n), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", v)
	}
}

func toStringSlice(v interface{}) ([]string, error) {
	slice, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected a list, got %T", v)
	}
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("list item must be a string, got %T", item)
		}
		result = append(result, s)
	}
	return result, nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
