package registry

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"

	"polaris/internal/config"
	"polaris/internal/ui"
)

// hiveMap maps short hive names to registry.Key constants.
var hiveMap = map[string]registry.Key{
	"HKLM": registry.LOCAL_MACHINE,
	"HKCU": registry.CURRENT_USER,
	"HKCR": registry.CLASSES_ROOT,
	"HKU":  registry.USERS,
}

// Apply processes all registry entries and ensures they match the desired state.
// Returns the number of changes made.
func Apply(entries []config.RegistryEntry) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}

	changed := 0
	for i, entry := range entries {
		c, err := applyEntry(entry)
		if err != nil {
			ui.Statusf("ERROR", "registry[%d] %s: %v", i, formatEntry(entry), err)
			return 0, err
		}
		changed += c
	}

	if changed > 0 {
		ui.Statusf("DONE", "%d change(s) applied", changed)
	} else {
		ui.Status("OK", "already configured")
	}

	return changed, nil
}

// formatEntry returns a human-readable description of a registry entry.
func formatEntry(entry config.RegistryEntry) string {
	if entry.Name != "" {
		return fmt.Sprintf("%s\\%s", entry.Path, entry.Name)
	}
	return entry.Path
}

// parseHiveAndSubkey splits a path like "HKLM\SOFTWARE\Foo" into the hive key
// and the subkey string "SOFTWARE\Foo".
func parseHiveAndSubkey(path string) (registry.Key, string, error) {
	parts := strings.SplitN(path, `\`, 2)
	hive, ok := hiveMap[strings.ToUpper(parts[0])]
	if !ok {
		return 0, "", fmt.Errorf("unknown hive %q", parts[0])
	}
	return hive, parts[1], nil
}

// applyEntry applies a single registry entry. Returns 1 if a change was made.
func applyEntry(entry config.RegistryEntry) (int, error) {
	hive, subkey, err := parseHiveAndSubkey(entry.Path)
	if err != nil {
		return 0, err
	}

	switch entry.State {
	case "present":
		return applyPresent(hive, subkey, entry)
	case "absent":
		return applyAbsent(hive, subkey, entry)
	}
	return 0, nil
}

// applyPresent ensures a registry value exists with the desired type and data.
func applyPresent(hive registry.Key, subkey string, entry config.RegistryEntry) (int, error) {
	key, _, err := registry.CreateKey(hive, subkey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return 0, fmt.Errorf("creating/opening key %q: %w", entry.Path, err)
	}
	defer key.Close()

	switch entry.Type {
	case "string":
		return setStringValue(key, entry.Name, registry.SZ, entry.Value)
	case "expand_string":
		return setStringValue(key, entry.Name, registry.EXPAND_SZ, entry.Value)
	case "multi_string":
		return setMultiStringValue(key, entry)
	case "dword":
		return setDWORDValue(key, entry)
	case "qword":
		return setQWORDValue(key, entry)
	case "binary":
		return setBinaryValue(key, entry)
	default:
		return 0, fmt.Errorf("unsupported type %q", entry.Type)
	}
}

// applyAbsent ensures a registry value or key does not exist.
func applyAbsent(hive registry.Key, subkey string, entry config.RegistryEntry) (int, error) {
	if entry.Name != "" {
		// Delete a specific value.
		key, err := registry.OpenKey(hive, subkey, registry.SET_VALUE|registry.QUERY_VALUE)
		if err != nil {
			// Key doesn't exist — value is already absent.
			return 0, nil
		}
		defer key.Close()

		if err := key.DeleteValue(entry.Name); err != nil {
			// Value doesn't exist — already absent.
			return 0, nil
		}
		return 1, nil
	}

	// Delete the entire key (and all its subkeys).
	if err := registry.DeleteKey(hive, subkey); err != nil {
		// Key doesn't exist — already absent.
		return 0, nil
	}
	return 1, nil
}

// setStringValue writes a REG_SZ or REG_EXPAND_SZ value if it differs from current.
func setStringValue(key registry.Key, name string, valType uint32, value interface{}) (int, error) {
	desired, ok := value.(string)
	if !ok {
		return 0, fmt.Errorf("value for string type must be a string, got %T", value)
	}

	current, typ, err := key.GetStringValue(name)
	if err == nil && typ == valType && current == desired {
		return 0, nil
	}

	if valType == registry.EXPAND_SZ {
		if err := key.SetExpandStringValue(name, desired); err != nil {
			return 0, fmt.Errorf("writing expand_string %q: %w", name, err)
		}
	} else {
		if err := key.SetStringValue(name, desired); err != nil {
			return 0, fmt.Errorf("writing string %q: %w", name, err)
		}
	}
	return 1, nil
}

// setMultiStringValue writes a REG_MULTI_SZ value if it differs from current.
func setMultiStringValue(key registry.Key, entry config.RegistryEntry) (int, error) {
	desired, err := toStringSlice(entry.Value)
	if err != nil {
		return 0, fmt.Errorf("value for multi_string must be a list of strings: %w", err)
	}

	current, _, getErr := key.GetStringsValue(entry.Name)
	if getErr == nil && stringSlicesEqual(current, desired) {
		return 0, nil
	}

	if err := key.SetStringsValue(entry.Name, desired); err != nil {
		return 0, fmt.Errorf("writing multi_string %q: %w", entry.Name, err)
	}
	return 1, nil
}

// setDWORDValue writes a REG_DWORD value if it differs from current.
func setDWORDValue(key registry.Key, entry config.RegistryEntry) (int, error) {
	desired, err := toUint32(entry.Value)
	if err != nil {
		return 0, fmt.Errorf("value for dword must be an integer: %w", err)
	}

	current, _, getErr := key.GetIntegerValue(entry.Name)
	if getErr == nil && uint32(current) == desired {
		return 0, nil
	}

	if err := key.SetDWordValue(entry.Name, desired); err != nil {
		return 0, fmt.Errorf("writing dword %q: %w", entry.Name, err)
	}
	return 1, nil
}

// setQWORDValue writes a REG_QWORD value if it differs from current.
func setQWORDValue(key registry.Key, entry config.RegistryEntry) (int, error) {
	desired, err := toUint64(entry.Value)
	if err != nil {
		return 0, fmt.Errorf("value for qword must be an integer: %w", err)
	}

	current, _, getErr := key.GetIntegerValue(entry.Name)
	if getErr == nil && current == desired {
		return 0, nil
	}

	if err := key.SetQWordValue(entry.Name, desired); err != nil {
		return 0, fmt.Errorf("writing qword %q: %w", entry.Name, err)
	}
	return 1, nil
}

// setBinaryValue writes a REG_BINARY value if it differs from current.
func setBinaryValue(key registry.Key, entry config.RegistryEntry) (int, error) {
	hexStr, ok := entry.Value.(string)
	if !ok {
		return 0, fmt.Errorf("value for binary must be a hex string, got %T", entry.Value)
	}

	desired, err := hex.DecodeString(hexStr)
	if err != nil {
		return 0, fmt.Errorf("invalid hex string for binary value: %w", err)
	}

	current, _, getErr := key.GetBinaryValue(entry.Name)
	if getErr == nil && bytes.Equal(current, desired) {
		return 0, nil
	}

	if err := key.SetBinaryValue(entry.Name, desired); err != nil {
		return 0, fmt.Errorf("writing binary %q: %w", entry.Name, err)
	}
	return 1, nil
}

// --- Helper functions ---

// toUint32 converts a YAML-parsed value (usually int or float64) to uint32.
// Returns an error if the value is negative or exceeds the uint32 range.
func toUint32(v interface{}) (uint32, error) {
	switch n := v.(type) {
	case int:
		if n < 0 || n > 0xFFFFFFFF {
			return 0, fmt.Errorf("value %d out of uint32 range", n)
		}
		return uint32(n), nil
	case int64:
		if n < 0 || n > 0xFFFFFFFF {
			return 0, fmt.Errorf("value %d out of uint32 range", n)
		}
		return uint32(n), nil
	case float64:
		if n < 0 || n > 0xFFFFFFFF {
			return 0, fmt.Errorf("value %f out of uint32 range", n)
		}
		return uint32(n), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint32", v)
	}
}

// toUint64 converts a YAML-parsed value to uint64.
// Returns an error if the value is negative.
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

// toStringSlice converts a YAML-parsed value ([]interface{}) to []string.
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

// stringSlicesEqual checks whether two string slices are identical.
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
