package defender

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"polaris/internal/config"
	"polaris/internal/ui"
)

// scanDayMap maps user-friendly day names to the Set-MpPreference ScanScheduleDay values.
// https://learn.microsoft.com/en-us/powershell/module/defender/set-mppreference
var scanDayMap = map[string]int{
	"everyday":  0,
	"sunday":    1,
	"monday":    2,
	"tuesday":   3,
	"wednesday": 4,
	"thursday":  5,
	"friday":    6,
	"saturday":  7,
}

// mpPrefs holds the subset of Get-MpPreference output we care about.
// ScanScheduleTime is interface{} because PowerShell serializes TimeSpan
// as an object with Hours/Minutes/Seconds fields, not as a plain string.
type mpPrefs struct {
	DisableRealtimeMonitoring bool        `json:"DisableRealtimeMonitoring"`
	MAPSReporting             int         `json:"MAPSReporting"`
	SubmitSamplesConsent      int         `json:"SubmitSamplesConsent"`
	PUAProtection             int         `json:"PUAProtection"`
	ScanScheduleDay           int         `json:"ScanScheduleDay"`
	ScanScheduleTime          interface{} `json:"ScanScheduleTime"`
	ExclusionPath             []string    `json:"ExclusionPath"`
	ExclusionExtension        []string    `json:"ExclusionExtension"`
	ExclusionProcess          []string    `json:"ExclusionProcess"`
}

// Apply configures Windows Defender according to the desired state.
func Apply(def *config.WindowsDefender) (int, error) {
	if def == nil {
		return 0, nil
	}

	current, err := getCurrentPrefs()
	if err != nil {
		return 0, fmt.Errorf("reading Defender preferences: %w", err)
	}

	changed := 0

	if def.RealTimeProtection != nil {
		// DisableRealtimeMonitoring is the inverse of our config value.
		desired := !*def.RealTimeProtection
		if current.DisableRealtimeMonitoring != desired {
			if err := setPreference("DisableRealtimeMonitoring", boolToPSString(desired)); err != nil {
				return 0, err
			}
			changed++
		}
	}

	if def.CloudProtection != nil {
		// MAPSReporting: 0=disabled, 2=advanced (enabled).
		desired := 0
		if *def.CloudProtection {
			desired = 2
		}
		if current.MAPSReporting != desired {
			if err := setPreference("MAPSReporting", fmt.Sprintf("%d", desired)); err != nil {
				return 0, err
			}
			changed++
		}
	}

	if def.SampleSubmission != nil {
		// SubmitSamplesConsent: 0=always prompt, 1=send safe samples, 2=never send, 3=send all.
		desired := 2
		if *def.SampleSubmission {
			desired = 1
		}
		if current.SubmitSamplesConsent != desired {
			if err := setPreference("SubmitSamplesConsent", fmt.Sprintf("%d", desired)); err != nil {
				return 0, err
			}
			changed++
		}
	}

	if def.PUAProtection != nil {
		// PUAProtection: 0=disabled, 1=enabled.
		desired := 0
		if *def.PUAProtection {
			desired = 1
		}
		if current.PUAProtection != desired {
			if err := setPreference("PUAProtection", fmt.Sprintf("%d", desired)); err != nil {
				return 0, err
			}
			changed++
		}
	}

	if def.ScanSchedule != nil {
		c, err := applyScanSchedule(current, def.ScanSchedule)
		if err != nil {
			return 0, err
		}
		changed += c
	}

	if def.Exclusions != nil {
		c, err := applyExclusions(current, def.Exclusions)
		if err != nil {
			return 0, err
		}
		changed += c
	}

	if changed > 0 {
		ui.Statusf("DONE", "%d setting(s) changed", changed)
	} else {
		ui.Status("OK", "already configured")
	}

	return changed, nil
}

// getCurrentPrefs reads the current Defender preferences via PowerShell.
func getCurrentPrefs() (*mpPrefs, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"Get-MpPreference | ConvertTo-Json -Depth 1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("Get-MpPreference failed: %w\nOutput: %s", err, string(output))
	}

	var prefs mpPrefs
	if err := json.Unmarshal(output, &prefs); err != nil {
		return nil, fmt.Errorf("parsing MpPreference JSON: %w", err)
	}
	return &prefs, nil
}

// setPreference calls Set-MpPreference with a single parameter.
func setPreference(name, value string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf("Set-MpPreference -%s %s", name, value))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Set-MpPreference -%s %s failed: %w\nOutput: %s", name, value, err, string(output))
	}
	return nil
}

// applyScanSchedule sets the scan day and time if they differ from current.
func applyScanSchedule(current *mpPrefs, schedule *config.ScanSchedule) (int, error) {
	changed := 0

	if schedule.Day != "" {
		desired := scanDayMap[strings.ToLower(schedule.Day)]
		if current.ScanScheduleDay != desired {
			if err := setPreference("ScanScheduleDay", fmt.Sprintf("%d", desired)); err != nil {
				return 0, err
			}
			changed++
		}
	}

	if schedule.Time != "" {
		// PowerShell expects the time as a timespan string, e.g. "02:00:00".
		desiredTS := schedule.Time + ":00"
		// Current value from JSON is a TimeSpan object or string.
		currentTime := timespanToHHMMSS(current.ScanScheduleTime)
		if currentTime != desiredTS {
			if err := setPreference("ScanScheduleTime", desiredTS); err != nil {
				return 0, err
			}
			changed++
		}
	}

	return changed, nil
}

// applyExclusions ensures all listed exclusions are present. Additive only —
// existing exclusions not in the list are left untouched.
func applyExclusions(current *mpPrefs, excl *config.DefenderExclusions) (int, error) {
	changed := 0

	for _, p := range excl.Paths {
		if !containsCaseInsensitive(current.ExclusionPath, p) {
			if err := addExclusion("ExclusionPath", p); err != nil {
				return 0, err
			}
			changed++
		}
	}

	for _, ext := range excl.Extensions {
		if !containsCaseInsensitive(current.ExclusionExtension, ext) {
			if err := addExclusion("ExclusionExtension", ext); err != nil {
				return 0, err
			}
			changed++
		}
	}

	for _, proc := range excl.Processes {
		if !containsCaseInsensitive(current.ExclusionProcess, proc) {
			if err := addExclusion("ExclusionProcess", proc); err != nil {
				return 0, err
			}
			changed++
		}
	}

	return changed, nil
}

// addExclusion adds a single exclusion via Add-MpPreference.
func addExclusion(paramName, value string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Add-MpPreference -%s '%s'`, paramName, sanitizePSString(value)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Add-MpPreference -%s %q failed: %w\nOutput: %s", paramName, value, err, string(output))
	}
	return nil
}

// --- Helpers ---

func boolToPSString(b bool) string {
	if b {
		return "$true"
	}
	return "$false"
}

// sanitizePSString makes a string safe for embedding in a PowerShell
// single-quoted literal by doubling any single quotes.
func sanitizePSString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// containsCaseInsensitive checks if a string slice contains a value (case-insensitive).
func containsCaseInsensitive(slice []string, val string) bool {
	lower := strings.ToLower(val)
	for _, s := range slice {
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return false
}

// timespanToHHMMSS converts a PowerShell TimeSpan value (serialized as a JSON
// object with Hours, Minutes, Seconds fields) to an "HH:MM:SS" string.
func timespanToHHMMSS(v interface{}) string {
	switch t := v.(type) {
	case string:
		// Already a string — normalize.
		t = strings.TrimSpace(t)
		if len(t) >= 8 && t[2] == ':' && t[5] == ':' {
			return t[:8]
		}
		return t
	case map[string]interface{}:
		// TimeSpan object from PowerShell JSON.
		h := toInt(t["Hours"])
		m := toInt(t["Minutes"])
		s := toInt(t["Seconds"])
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	default:
		return "00:00:00"
	}
}

// toInt converts a JSON number (float64) to int.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
