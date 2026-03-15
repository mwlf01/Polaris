package windowsupdate

import (
	"fmt"

	"golang.org/x/sys/windows/registry"

	"polaris/internal/config"
	"polaris/internal/ui"
)

const (
	// Registry paths for Windows Update policy settings.
	wuPolicyKey = `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`
	auPolicyKey = `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`
)

// auOptionsMap maps user-friendly auto_update values to the AUOptions registry value.
// See https://learn.microsoft.com/en-us/windows/deployment/update/waas-wu-settings
var auOptionsMap = map[string]uint32{
	"notify":        2, // Notify before download
	"download_only": 3, // Auto download, notify before install
	"auto":          4, // Auto download and schedule install
}

// Apply writes the desired Windows Update settings to the registry.
// Returns the number of settings changed.
func Apply(wu *config.WindowsUpdate) (int, error) {
	if wu == nil {
		return 0, nil
	}

	changed := 0

	if wu.AutoUpdate != "" {
		c, err := applyAutoUpdate(wu.AutoUpdate)
		if err != nil {
			return 0, err
		}
		changed += c
	}

	if wu.ActiveHours != nil {
		c, err := applyActiveHours(wu.ActiveHours.Start, wu.ActiveHours.End)
		if err != nil {
			return 0, err
		}
		changed += c
	}

	if wu.DeferFeatureUpdatesDays != nil {
		c, err := setDWORDIfChanged(wuPolicyKey, "DeferFeatureUpdatesPeriodInDays", uint32(*wu.DeferFeatureUpdatesDays))
		if err != nil {
			return 0, fmt.Errorf("setting defer_feature_updates_days: %w", err)
		}
		changed += c
	}

	if wu.DeferQualityUpdatesDays != nil {
		c, err := setDWORDIfChanged(wuPolicyKey, "DeferQualityUpdatesPeriodInDays", uint32(*wu.DeferQualityUpdatesDays))
		if err != nil {
			return 0, fmt.Errorf("setting defer_quality_updates_days: %w", err)
		}
		changed += c
	}

	if wu.NoAutoRestart != nil {
		val := uint32(0)
		if *wu.NoAutoRestart {
			val = 1
		}
		c, err := setDWORDIfChanged(auPolicyKey, "NoAutoRebootWithLoggedOnUsers", val)
		if err != nil {
			return 0, fmt.Errorf("setting no_auto_restart: %w", err)
		}
		changed += c
	}

	if wu.MicrosoftProductUpdates != nil {
		val := uint32(0)
		if *wu.MicrosoftProductUpdates {
			val = 1
		}
		c, err := setDWORDIfChanged(auPolicyKey, "AllowMUUpdateService", val)
		if err != nil {
			return 0, fmt.Errorf("setting microsoft_product_updates: %w", err)
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

// applyAutoUpdate sets the NoAutoUpdate and AUOptions registry values.
func applyAutoUpdate(mode string) (int, error) {
	changed := 0

	if mode == "disabled" {
		c, err := setDWORDIfChanged(auPolicyKey, "NoAutoUpdate", 1)
		if err != nil {
			return 0, fmt.Errorf("setting auto_update to disabled: %w", err)
		}
		return c, nil
	}

	// Enable auto-update first.
	c1, err := setDWORDIfChanged(auPolicyKey, "NoAutoUpdate", 0)
	if err != nil {
		return 0, fmt.Errorf("enabling auto_update: %w", err)
	}
	changed += c1

	auOption, ok := auOptionsMap[mode]
	if !ok {
		return 0, fmt.Errorf("unknown auto_update mode %q", mode)
	}

	c2, err := setDWORDIfChanged(auPolicyKey, "AUOptions", auOption)
	if err != nil {
		return 0, fmt.Errorf("setting AUOptions: %w", err)
	}
	changed += c2

	return changed, nil
}

// applyActiveHours sets the ActiveHoursStart/End and enables the policy.
func applyActiveHours(start, end int) (int, error) {
	changed := 0

	c1, err := setDWORDIfChanged(wuPolicyKey, "SetActiveHours", 1)
	if err != nil {
		return 0, fmt.Errorf("enabling active hours: %w", err)
	}
	changed += c1

	c2, err := setDWORDIfChanged(wuPolicyKey, "ActiveHoursStart", uint32(start))
	if err != nil {
		return 0, fmt.Errorf("setting active_hours.start: %w", err)
	}
	changed += c2

	c3, err := setDWORDIfChanged(wuPolicyKey, "ActiveHoursEnd", uint32(end))
	if err != nil {
		return 0, fmt.Errorf("setting active_hours.end: %w", err)
	}
	changed += c3

	return changed, nil
}

// setDWORDIfChanged writes a DWORD value to the registry only if it differs
// from the current value. Returns 1 if a change was made, 0 otherwise.
func setDWORDIfChanged(keyPath, valueName string, desired uint32) (int, error) {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return 0, fmt.Errorf("opening registry key %q: %w", keyPath, err)
	}
	defer key.Close()

	// Read current value; if it matches, skip the write.
	current, _, err := key.GetIntegerValue(valueName)
	if err == nil && uint32(current) == desired {
		return 0, nil
	}

	if err := key.SetDWordValue(valueName, desired); err != nil {
		return 0, fmt.Errorf("writing %q to %q: %w", valueName, keyPath, err)
	}

	return 1, nil
}
