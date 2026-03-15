package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"polaris/internal/ui"

	"gopkg.in/yaml.v3"
)

// semver represents a parsed semantic version (major.minor.patch).
type semver struct {
	Major, Minor, Patch int
}

// parseSemver parses a version string like "1.2.3" or "v1.2.3" into a semver.
func parseSemver(s string) (semver, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("invalid semver %q: expected major.minor.patch", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return semver{}, fmt.Errorf("invalid semver %q: %w", s, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return semver{}, fmt.Errorf("invalid semver %q: %w", s, err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return semver{}, fmt.Errorf("invalid semver %q: %w", s, err)
	}
	return semver{Major: major, Minor: minor, Patch: patch}, nil
}

// compare returns -1, 0, or 1 if v is less than, equal to, or greater than other.
func (v semver) compare(other semver) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// Loader is the interface for loading a Config from any source.
// This allows future implementations (e.g. HTTP) to be added easily.
type Loader interface {
	Load() (*Config, error)
}

// FileLoader loads configuration from a local YAML file.
// URL-based includes are fetched via an internal cache with ETag support.
type FileLoader struct {
	Path    string
	Context LoadContext
	cache   *URLCache
}

// NewFileLoader creates a new FileLoader for the given path and runtime context.
// The context is used to evaluate per-file compatibility during include resolution.
func NewFileLoader(path string, ctx LoadContext) *FileLoader {
	return &FileLoader{Path: path, Context: ctx}
}

// Load reads and parses the YAML file into a Config struct.
// If the config contains an `includes` list, the referenced files are loaded
// recursively and merged into a single Config. Circular references are detected
// and rejected. Include paths are resolved relative to the including file.
func (f *FileLoader) Load() (*Config, error) {
	absPath, err := filepath.Abs(f.Path)
	if err != nil {
		return nil, fmt.Errorf("resolving path %q: %w", f.Path, err)
	}

	// Create URL cache for remote includes.
	cache, err := newURLCache()
	if err != nil {
		return nil, fmt.Errorf("initializing URL cache: %w", err)
	}
	f.cache = cache

	visited := make(map[string]bool)
	cfg, err := loadRecursive(absPath, visited, f.Context, f.cache, nil, true)
	if err != nil {
		return nil, err
	}

	// Remove cached files that are no longer referenced.
	f.cache.Cleanup()

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// loadRecursive loads a single YAML file, then recursively loads and merges
// all files referenced in its `includes` list. visited tracks absolute paths
// to detect circular references. propagatedAuth is the auth config inherited
// from a parent include with propagate_auth: true. isRoot is true for the
// top-level file (which must not be skipped even if incompatible).
func loadRecursive(absPath string, visited map[string]bool, ctx LoadContext, cache *URLCache, propagatedAuth *AuthConfig, isRoot bool) (*Config, error) {
	if visited[absPath] {
		return nil, fmt.Errorf("circular include detected: %q", absPath)
	}
	visited[absPath] = true

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", absPath, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", absPath, err)
	}

	// For included (non-root) files, check compatibility and skip if not matching.
	if !isRoot && cfg.Compatibility != nil {
		if err := CheckCompatibility(cfg.Compatibility, ctx.Version, ctx.OS, ctx.Arch, ctx.WindowsVersion); err != nil {
			ui.Statusf("SKIP", "%s \u2013 %v", filepath.Base(absPath), err)
			skipped := &Config{}
			skipped.Skipped = 1
			return skipped, nil
		}
	}

	// Record this file's compatibility (if any).
	cfg.AddCompatibility(absPath, cfg.Compatibility)

	// Resolve and load includes.
	baseDir := filepath.Dir(absPath)
	for _, inc := range cfg.Includes {
		var incAbs string

		// Determine effective auth for this include.
		auth := inc.Auth
		if auth == nil {
			auth = propagatedAuth
		}

		// Determine what auth to propagate to this include's children.
		var childAuth *AuthConfig
		if inc.Auth != nil && inc.PropagateAuth {
			childAuth = inc.Auth
		} else if inc.Auth == nil {
			childAuth = propagatedAuth
		}

		if isURL(inc.URL) {
			// Remote include: fetch via cache (ETag / offline fallback).
			cachedPath, err := cache.Fetch(inc.URL, auth)
			if err != nil {
				ui.Statusf("SKIP", "%s \u2013 %v", truncateURL(inc.URL), err)
				cfg.Skipped++
				continue
			}
			incAbs = cachedPath
		} else {
			// Local include: resolve relative to the including file.
			incPath := inc.URL
			if !filepath.IsAbs(incPath) {
				incPath = filepath.Join(baseDir, incPath)
			}
			resolved, err := filepath.Abs(incPath)
			if err != nil {
				return nil, fmt.Errorf("resolving include path %q (from %q): %w", inc.URL, absPath, err)
			}
			incAbs = resolved
		}

		child, err := loadRecursive(incAbs, visited, ctx, cache, childAuth, false)
		if err != nil {
			displayName := inc.URL
			if isURL(inc.URL) {
				displayName = truncateURL(inc.URL)
			} else {
				displayName = filepath.Base(inc.URL)
			}
			ui.Statusf("SKIP", "%s \u2013 %v", displayName, err)
			cfg.Skipped++
			continue
		}

		mergeConfig(&cfg, child)
	}

	// Clear includes after processing (not relevant for execution).
	cfg.Includes = nil

	return &cfg, nil
}

// mergeConfig merges child into parent.
// Lists (packages, registry, users, group_policy) are appended.
// Singleton objects are deep-merged (parent fields take precedence per field).
// Per-file compatibility entries are always propagated.
func mergeConfig(parent, child *Config) {
	// Compatibility: first definition wins.
	if parent.Compatibility == nil && child.Compatibility != nil {
		parent.Compatibility = child.Compatibility
	}

	// Schedule: first definition wins.
	if parent.Schedule == nil && child.Schedule != nil {
		parent.Schedule = child.Schedule
	}

	// Update: first definition wins.
	if parent.Update == nil && child.Update != nil {
		parent.Update = child.Update
	}

	// Deep-merge singleton objects so settings can be split across files.
	parent.WindowsUpdate = mergeWindowsUpdate(parent.WindowsUpdate, child.WindowsUpdate)
	parent.WindowsDefender = mergeWindowsDefender(parent.WindowsDefender, child.WindowsDefender)

	// Lists: append child entries.
	parent.Packages = append(parent.Packages, child.Packages...)
	parent.Registry = append(parent.Registry, child.Registry...)
	parent.Users = append(parent.Users, child.Users...)
	parent.GroupPolicy = append(parent.GroupPolicy, child.GroupPolicy...)

	// Propagate skipped count from child.
	parent.Skipped += child.Skipped

	// Propagate all per-file compatibility entries from child.
	for _, fc := range child.Compatibilities() {
		parent.AddCompatibility(fc.File, fc.Compat)
	}
}

// mergeWindowsUpdate deep-merges two WindowsUpdate configs.
// Parent fields take precedence; child fills unset fields.
func mergeWindowsUpdate(parent, child *WindowsUpdate) *WindowsUpdate {
	if parent == nil {
		return child
	}
	if child == nil {
		return parent
	}
	if parent.AutoUpdate == "" {
		parent.AutoUpdate = child.AutoUpdate
	}
	if parent.ActiveHours == nil {
		parent.ActiveHours = child.ActiveHours
	}
	if parent.DeferFeatureUpdatesDays == nil {
		parent.DeferFeatureUpdatesDays = child.DeferFeatureUpdatesDays
	}
	if parent.DeferQualityUpdatesDays == nil {
		parent.DeferQualityUpdatesDays = child.DeferQualityUpdatesDays
	}
	if parent.NoAutoRestart == nil {
		parent.NoAutoRestart = child.NoAutoRestart
	}
	if parent.MicrosoftProductUpdates == nil {
		parent.MicrosoftProductUpdates = child.MicrosoftProductUpdates
	}
	return parent
}

// mergeWindowsDefender deep-merges two WindowsDefender configs.
// Parent fields take precedence; child fills unset fields.
// Exclusion lists are combined (appended).
func mergeWindowsDefender(parent, child *WindowsDefender) *WindowsDefender {
	if parent == nil {
		return child
	}
	if child == nil {
		return parent
	}
	if parent.RealTimeProtection == nil {
		parent.RealTimeProtection = child.RealTimeProtection
	}
	if parent.CloudProtection == nil {
		parent.CloudProtection = child.CloudProtection
	}
	if parent.SampleSubmission == nil {
		parent.SampleSubmission = child.SampleSubmission
	}
	if parent.PUAProtection == nil {
		parent.PUAProtection = child.PUAProtection
	}
	if parent.ScanSchedule == nil {
		parent.ScanSchedule = child.ScanSchedule
	}
	// Exclusions: merge lists from both.
	if parent.Exclusions == nil {
		parent.Exclusions = child.Exclusions
	} else if child.Exclusions != nil {
		parent.Exclusions.Paths = append(parent.Exclusions.Paths, child.Exclusions.Paths...)
		parent.Exclusions.Extensions = append(parent.Exclusions.Extensions, child.Exclusions.Extensions...)
		parent.Exclusions.Processes = append(parent.Exclusions.Processes, child.Exclusions.Processes...)
	}
	return parent
}

// validate checks the Config for logical errors.
func validate(cfg *Config) error {
	// Validate all per-file compatibility entries (not just the merged top-level one).
	for _, fc := range cfg.Compatibilities() {
		if err := validateCompatibility(fc.Compat); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(fc.File), err)
		}
	}

	for i := range cfg.Packages {
		pkg := &cfg.Packages[i]
		if pkg.ID == "" {
			return fmt.Errorf("package at index %d: 'id' is required", i)
		}
		if pkg.State == "" {
			return fmt.Errorf("package %q: 'state' is required", pkg.ID)
		}
		if pkg.State != "present" && pkg.State != "absent" {
			return fmt.Errorf("package %q: 'state' must be 'present' or 'absent', got %q", pkg.ID, pkg.State)
		}
		if pkg.Source == "" {
			return fmt.Errorf("package %q: 'source' is required (winget, msstore, or choco)", pkg.ID)
		}
		if pkg.Source != "winget" && pkg.Source != "msstore" && pkg.Source != "choco" {
			return fmt.Errorf("package %q: 'source' must be 'winget', 'msstore', or 'choco', got %q", pkg.ID, pkg.Source)
		}
	}

	if err := validateWindowsUpdate(cfg.WindowsUpdate); err != nil {
		return err
	}

	if err := validateRegistry(cfg.Registry); err != nil {
		return err
	}

	if err := validateDefender(cfg.WindowsDefender); err != nil {
		return err
	}

	if err := validateUsers(cfg.Users); err != nil {
		return err
	}

	if err := validateGroupPolicy(cfg.GroupPolicy); err != nil {
		return err
	}

	return nil
}

// validateCompatibility checks the Compatibility section for logical errors.
// It only validates the format of version strings. The actual runtime check
// (comparing against the running Polaris version, OS, and arch) is done by
// CheckCompatibility.
func validateCompatibility(c *Compatibility) error {
	if c == nil {
		return nil
	}
	if c.MinVersion != "" {
		if _, err := parseSemver(c.MinVersion); err != nil {
			return fmt.Errorf("compatibility.min_version: %w", err)
		}
	}
	if c.MaxVersion != "" {
		if _, err := parseSemver(c.MaxVersion); err != nil {
			return fmt.Errorf("compatibility.max_version: %w", err)
		}
	}
	if c.MinVersion != "" && c.MaxVersion != "" {
		minV, _ := parseSemver(c.MinVersion)
		maxV, _ := parseSemver(c.MaxVersion)
		if minV.compare(maxV) > 0 {
			return fmt.Errorf("compatibility: min_version %q must not be greater than max_version %q", c.MinVersion, c.MaxVersion)
		}
	}
	return nil
}

// CheckCompatibility verifies that the running Polaris version, OS, architecture, and
// Windows version satisfy the constraints defined in the config.
// A version of "dev" always passes version checks.
func CheckCompatibility(c *Compatibility, version, currentOS, currentArch, windowsVersion string) error {
	if c == nil {
		return nil
	}

	// Version checks (skip for dev builds).
	if version != "dev" && version != "" {
		runV, err := parseSemver(version)
		if err != nil {
			// If the running version can't be parsed, skip version checks.
			goto platformCheck
		}
		if c.MinVersion != "" {
			minV, _ := parseSemver(c.MinVersion)
			if runV.compare(minV) < 0 {
				return fmt.Errorf("this config requires Polaris >= %s, but running %s", c.MinVersion, version)
			}
		}
		if c.MaxVersion != "" {
			maxV, _ := parseSemver(c.MaxVersion)
			if runV.compare(maxV) > 0 {
				return fmt.Errorf("this config requires Polaris <= %s, but running %s", c.MaxVersion, version)
			}
		}
	}

platformCheck:
	if len(c.OS) > 0 && !containsFold(c.OS, currentOS) {
		return fmt.Errorf("this config requires OS %v, but running on %q", []string(c.OS), currentOS)
	}
	if len(c.Arch) > 0 && !containsFold(c.Arch, currentArch) {
		return fmt.Errorf("this config requires architecture %v, but running on %q", []string(c.Arch), currentArch)
	}
	if len(c.WindowsVersion) > 0 && windowsVersion != "" && !containsPrefixFold(c.WindowsVersion, windowsVersion) {
		return fmt.Errorf("this config requires Windows version %v, but running %q", []string(c.WindowsVersion), windowsVersion)
	}
	return nil
}

// containsFold checks whether the list contains the value (case-insensitive).
func containsFold(list []string, value string) bool {
	for _, item := range list {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

// containsPrefixFold checks whether any item in the list is a case-insensitive
// prefix of value. This allows broad matches like "11" to match "11 24H2",
// while exact entries like "11 24H2" still work.
func containsPrefixFold(list []string, value string) bool {
	lowerValue := strings.ToLower(value)
	for _, item := range list {
		if strings.HasPrefix(lowerValue, strings.ToLower(item)) {
			return true
		}
	}
	return false
}

// validateGroupPolicy checks GroupPolicyEntry items for logical errors.
func validateGroupPolicy(entries []GroupPolicyEntry) error {
	validTypes := map[string]bool{
		"string": true, "expand_string": true, "multi_string": true,
		"dword": true, "qword": true,
	}

	for i, entry := range entries {
		if entry.Scope == "" {
			return fmt.Errorf("group_policy[%d]: 'scope' is required", i)
		}
		if entry.Scope != "computer" && entry.Scope != "user" {
			return fmt.Errorf("group_policy[%d]: 'scope' must be 'computer' or 'user', got %q", i, entry.Scope)
		}
		if entry.Path == "" {
			return fmt.Errorf("group_policy[%d]: 'path' is required", i)
		}
		if entry.State == "" {
			return fmt.Errorf("group_policy[%d]: 'state' is required", i)
		}
		if entry.State != "present" && entry.State != "absent" {
			return fmt.Errorf("group_policy[%d]: 'state' must be 'present' or 'absent', got %q", i, entry.State)
		}
		if entry.State == "present" {
			if entry.Name == "" {
				return fmt.Errorf("group_policy[%d]: 'name' is required when state is 'present'", i)
			}
			if entry.Type == "" {
				return fmt.Errorf("group_policy[%d]: 'type' is required when state is 'present'", i)
			}
			if !validTypes[entry.Type] {
				return fmt.Errorf("group_policy[%d]: 'type' must be string, expand_string, multi_string, dword, or qword, got %q", i, entry.Type)
			}
		}
	}
	return nil
}

// validateUsers checks User entries for logical errors.
func validateUsers(users []User) error {
	for i, u := range users {
		if u.Name == "" {
			return fmt.Errorf("users[%d]: 'name' is required", i)
		}
		if u.State == "" {
			return fmt.Errorf("users[%d] %q: 'state' is required", i, u.Name)
		}
		if u.State != "present" && u.State != "absent" {
			return fmt.Errorf("users[%d] %q: 'state' must be 'present' or 'absent', got %q", i, u.Name, u.State)
		}
	}
	return nil
}

// validateDefender checks the WindowsDefender config for logical errors.
func validateDefender(def *WindowsDefender) error {
	if def == nil {
		return nil
	}

	if def.ScanSchedule != nil {
		validDays := map[string]bool{
			"everyday": true, "monday": true, "tuesday": true, "wednesday": true,
			"thursday": true, "friday": true, "saturday": true, "sunday": true,
		}
		if def.ScanSchedule.Day != "" && !validDays[strings.ToLower(def.ScanSchedule.Day)] {
			return fmt.Errorf("windows_defender.scan_schedule.day must be 'everyday' or a weekday, got %q", def.ScanSchedule.Day)
		}

		if def.ScanSchedule.Time != "" {
			parts := strings.Split(def.ScanSchedule.Time, ":")
			if len(parts) != 2 {
				return fmt.Errorf("windows_defender.scan_schedule.time must be in HH:MM format, got %q", def.ScanSchedule.Time)
			}
			var h, m int
			if _, err := fmt.Sscanf(def.ScanSchedule.Time, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
				return fmt.Errorf("windows_defender.scan_schedule.time must be a valid HH:MM time, got %q", def.ScanSchedule.Time)
			}
		}
	}

	return nil
}

// validateRegistry checks RegistryEntry items for logical errors.
func validateRegistry(entries []RegistryEntry) error {
	validHives := map[string]bool{
		"HKLM": true, "HKCU": true, "HKCR": true, "HKU": true,
	}
	validTypes := map[string]bool{
		"string": true, "expand_string": true, "multi_string": true,
		"dword": true, "qword": true, "binary": true,
	}

	for i, entry := range entries {
		if entry.Path == "" {
			return fmt.Errorf("registry[%d]: 'path' is required", i)
		}

		// Validate that the path starts with a known hive.
		parts := strings.SplitN(entry.Path, `\`, 2)
		if !validHives[strings.ToUpper(parts[0])] {
			return fmt.Errorf("registry[%d]: path must start with HKLM, HKCU, HKCR, or HKU, got %q", i, parts[0])
		}
		if len(parts) < 2 || parts[1] == "" {
			return fmt.Errorf("registry[%d]: path must include a subkey after the hive", i)
		}

		if entry.State == "" {
			return fmt.Errorf("registry[%d]: 'state' is required", i)
		}
		if entry.State != "present" && entry.State != "absent" {
			return fmt.Errorf("registry[%d]: 'state' must be 'present' or 'absent', got %q", i, entry.State)
		}

		if entry.State == "present" {
			if entry.Name == "" {
				return fmt.Errorf("registry[%d]: 'name' is required when state is 'present'", i)
			}
			if entry.Type == "" {
				return fmt.Errorf("registry[%d]: 'type' is required when state is 'present'", i)
			}
			if !validTypes[entry.Type] {
				return fmt.Errorf("registry[%d]: 'type' must be string, expand_string, multi_string, dword, qword, or binary, got %q", i, entry.Type)
			}
		}
	}
	return nil
}

// validateWindowsUpdate checks the WindowsUpdate config for logical errors.
func validateWindowsUpdate(wu *WindowsUpdate) error {
	if wu == nil {
		return nil
	}

	validModes := map[string]bool{
		"disabled": true, "notify": true, "download_only": true, "auto": true,
	}
	if wu.AutoUpdate != "" && !validModes[wu.AutoUpdate] {
		return fmt.Errorf("windows_update.auto_update must be 'disabled', 'notify', 'download_only', or 'auto', got %q", wu.AutoUpdate)
	}

	if wu.ActiveHours != nil {
		if wu.ActiveHours.Start < 0 || wu.ActiveHours.Start > 23 {
			return fmt.Errorf("windows_update.active_hours.start must be 0-23, got %d", wu.ActiveHours.Start)
		}
		if wu.ActiveHours.End < 0 || wu.ActiveHours.End > 23 {
			return fmt.Errorf("windows_update.active_hours.end must be 0-23, got %d", wu.ActiveHours.End)
		}
		if wu.ActiveHours.Start == wu.ActiveHours.End {
			return fmt.Errorf("windows_update.active_hours.start and end must differ")
		}
	}

	if wu.DeferFeatureUpdatesDays != nil {
		d := *wu.DeferFeatureUpdatesDays
		if d < 0 || d > 365 {
			return fmt.Errorf("windows_update.defer_feature_updates_days must be 0-365, got %d", d)
		}
	}

	if wu.DeferQualityUpdatesDays != nil {
		d := *wu.DeferQualityUpdatesDays
		if d < 0 || d > 30 {
			return fmt.Errorf("windows_update.defer_quality_updates_days must be 0-30, got %d", d)
		}
	}

	return nil
}
