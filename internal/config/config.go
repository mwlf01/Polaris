package config

import (
	"gopkg.in/yaml.v3"
)

// FlexibleStringList can be unmarshalled from either a single YAML string or a list of strings.
// This allows writing both `os: windows` and `os: [windows, linux]` in the config.
type FlexibleStringList []string

func (f *FlexibleStringList) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		*f = []string{value.Value}
		return nil
	}
	var list []string
	if err := value.Decode(&list); err != nil {
		return err
	}
	*f = list
	return nil
}

// Schedule defines how often Polaris re-applies the configuration when
// running as a Windows service. The Interval string is parsed by
// time.ParseDuration (e.g. "30m", "1h", "6h").
type Schedule struct {
	Interval string `yaml:"interval"`
}

// UpdateConfig defines the self-update settings. Polaris checks the
// manifest URL periodically and updates itself when a newer version is
// available. The binary is replaced atomically with a rollback guard.
type UpdateConfig struct {
	URL  string      `yaml:"url"`            // URL to the JSON update manifest
	Auth *AuthConfig `yaml:"auth,omitempty"` // optional basic or mTLS auth
}

// Config represents the top-level desired state configuration.
type Config struct {
	Includes        []IncludeEntry     `yaml:"includes,omitempty"`
	Compatibility   *Compatibility     `yaml:"compatibility,omitempty"`
	Schedule        *Schedule          `yaml:"schedule,omitempty"`
	Update          *UpdateConfig      `yaml:"update,omitempty"`
	Packages        []Package          `yaml:"packages"`
	WindowsUpdate   *WindowsUpdate     `yaml:"windows_update,omitempty"`
	WindowsDefender *WindowsDefender   `yaml:"windows_defender,omitempty"`
	Registry        []RegistryEntry    `yaml:"registry,omitempty"`
	Users           []User             `yaml:"users,omitempty"`
	GroupPolicy     []GroupPolicyEntry `yaml:"group_policy,omitempty"`

	// Skipped counts the number of included files that were skipped
	// (missing, broken, or incompatible). Populated by the loader.
	Skipped int `yaml:"-"`

	// compatibilities collects Compatibility entries from all files (including
	// included ones) together with their source file path. Populated by the
	// loader during recursive include resolution.
	compatibilities []FileCompatibility
}

// FileCompatibility pairs a Compatibility section with the file it came from.
type FileCompatibility struct {
	File   string
	Compat *Compatibility
}

// Compatibilities returns all per-file compatibility entries collected during loading.
func (c *Config) Compatibilities() []FileCompatibility {
	return c.compatibilities
}

// AddCompatibility appends a per-file compatibility entry. Used by the loader.
func (c *Config) AddCompatibility(file string, compat *Compatibility) {
	if compat != nil {
		c.compatibilities = append(c.compatibilities, FileCompatibility{File: file, Compat: compat})
	}
}

// LoadContext carries runtime information used during config loading to evaluate
// per-file compatibility. Included files whose compatibility does not match are
// silently skipped.
type LoadContext struct {
	Version        string // running Polaris version (e.g. "1.0.0" or "dev")
	OS             string // runtime.GOOS
	Arch           string // runtime.GOARCH
	WindowsVersion string // e.g. "11 24H2"
}

// Compatibility defines optional constraints for Polaris version, OS, and architecture.
// All fields are optional. Only specified fields are checked.
type Compatibility struct {
	MinVersion     string             `yaml:"min_version,omitempty"`     // minimum Polaris version (semver)
	MaxVersion     string             `yaml:"max_version,omitempty"`     // maximum Polaris version (semver)
	OS             FlexibleStringList `yaml:"os,omitempty"`              // allowed OS values (e.g. "windows")
	Arch           FlexibleStringList `yaml:"arch,omitempty"`            // allowed architectures (e.g. "amd64", "arm64")
	WindowsVersion FlexibleStringList `yaml:"windows_version,omitempty"` // allowed Windows versions (e.g. "10", "11", "Server 2022")
}

// GroupPolicyEntry describes a single local group policy setting.
// Applied via LGPO.exe (auto-installed from winget if missing).
type GroupPolicyEntry struct {
	Scope string      `yaml:"scope"` // "computer" or "user"
	Path  string      `yaml:"path"`
	Name  string      `yaml:"name,omitempty"`
	Type  string      `yaml:"type,omitempty"` // string, expand_string, multi_string, dword, qword
	Value interface{} `yaml:"value,omitempty"`
	State string      `yaml:"state"` // "present" or "absent"
}

// User describes a local Windows user account and its desired state.
type User struct {
	Name                 string   `yaml:"name"`
	FullName             string   `yaml:"full_name,omitempty"`
	Description          string   `yaml:"description,omitempty"`
	Password             string   `yaml:"password,omitempty"`
	Groups               []string `yaml:"groups,omitempty"`
	PasswordNeverExpires *bool    `yaml:"password_never_expires,omitempty"`
	AccountDisabled      *bool    `yaml:"account_disabled,omitempty"`
	State                string   `yaml:"state"` // "present" or "absent"
}

// WindowsDefender describes the desired Microsoft Defender Antivirus settings.
type WindowsDefender struct {
	RealTimeProtection *bool               `yaml:"real_time_protection,omitempty"`
	CloudProtection    *bool               `yaml:"cloud_protection,omitempty"`
	SampleSubmission   *bool               `yaml:"sample_submission,omitempty"`
	PUAProtection      *bool               `yaml:"pua_protection,omitempty"`
	Exclusions         *DefenderExclusions `yaml:"exclusions,omitempty"`
	ScanSchedule       *ScanSchedule       `yaml:"scan_schedule,omitempty"`
}

// DefenderExclusions defines paths, extensions, and processes to exclude from scanning.
// Exclusions are additive — Polaris ensures listed items are present but does not
// remove existing exclusions that are not in the list.
type DefenderExclusions struct {
	Paths      []string `yaml:"paths,omitempty"`
	Extensions []string `yaml:"extensions,omitempty"`
	Processes  []string `yaml:"processes,omitempty"`
}

// ScanSchedule defines when Defender runs its scheduled scan.
type ScanSchedule struct {
	Day  string `yaml:"day"`  // monday-sunday or everyday
	Time string `yaml:"time"` // HH:MM (24h)
}

// RegistryEntry describes a single registry value or key and its desired state.
// When State is "absent" and Name is empty, the entire key at Path is deleted.
// When State is "absent" and Name is set, only that value is deleted.
type RegistryEntry struct {
	Path  string      `yaml:"path"`
	Name  string      `yaml:"name,omitempty"`
	Type  string      `yaml:"type,omitempty"` // string, expand_string, multi_string, dword, qword, binary
	Value interface{} `yaml:"value,omitempty"`
	State string      `yaml:"state"` // "present" or "absent"
}

// WindowsUpdate describes the desired Windows Update policy settings.
type WindowsUpdate struct {
	AutoUpdate              string       `yaml:"auto_update,omitempty"`
	ActiveHours             *ActiveHours `yaml:"active_hours,omitempty"`
	DeferFeatureUpdatesDays *int         `yaml:"defer_feature_updates_days,omitempty"`
	DeferQualityUpdatesDays *int         `yaml:"defer_quality_updates_days,omitempty"`
	NoAutoRestart           *bool        `yaml:"no_auto_restart,omitempty"`
	MicrosoftProductUpdates *bool        `yaml:"microsoft_product_updates,omitempty"`
}

// ActiveHours defines the time window during which Windows should not restart.
type ActiveHours struct {
	Start int `yaml:"start"`
	End   int `yaml:"end"`
}

// IncludeEntry represents a single include directive. It can be either a
// plain string (backward compatible) or an object with URL, auth, and
// propagate_auth fields.
type IncludeEntry struct {
	URL           string      `yaml:"url"`
	Auth          *AuthConfig `yaml:"auth,omitempty"`
	PropagateAuth bool        `yaml:"propagate_auth,omitempty"`
}

// UnmarshalYAML allows includes to be either plain strings or objects.
func (e *IncludeEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		e.URL = value.Value
		return nil
	}
	type raw IncludeEntry
	return value.Decode((*raw)(e))
}

// AuthConfig defines authentication for remote includes.
type AuthConfig struct {
	Type     string `yaml:"type"`               // "basic" or "mtls"
	Username string `yaml:"username,omitempty"` // basic auth
	Password string `yaml:"password,omitempty"` // basic auth
	Subject  string `yaml:"subject,omitempty"`  // mtls: certificate subject in Windows cert store
	Store    string `yaml:"store,omitempty"`    // mtls: cert store name (default: "My")
}

// Package describes a single software package and its desired state.
type Package struct {
	Name    string `yaml:"name"`
	ID      string `yaml:"id"`
	Version string `yaml:"version,omitempty"`
	Source  string `yaml:"source,omitempty"` // "winget", "msstore", or "choco"
	State   string `yaml:"state"`            // "present" or "absent"
}
