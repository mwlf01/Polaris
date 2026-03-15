package users

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"polaris/internal/config"
	"polaris/internal/ui"
)

// wellKnownGroups maps common English group names to their well-known SIDs.
// This allows configs to use English names on any localized Windows.
var wellKnownGroups = map[string]string{
	"administrators":                  "S-1-5-32-544",
	"users":                           "S-1-5-32-545",
	"guests":                          "S-1-5-32-546",
	"power users":                     "S-1-5-32-547",
	"backup operators":                "S-1-5-32-551",
	"remote desktop users":            "S-1-5-32-555",
	"network configuration operators": "S-1-5-32-556",
	"performance monitor users":       "S-1-5-32-558",
	"performance log users":           "S-1-5-32-559",
	"distributed com users":           "S-1-5-32-562",
	"iis_iusrs":                       "S-1-5-32-568",
	"cryptographic operators":         "S-1-5-32-569",
	"event log readers":               "S-1-5-32-573",
	"hyper-v administrators":          "S-1-5-32-578",
	"remote management users":         "S-1-5-32-580",
	"device owners":                   "S-1-5-32-583",
}

// localUser holds the subset of Get-LocalUser output we care about.
type localUser struct {
	Name                 string `json:"Name"`
	FullName             string `json:"FullName"`
	Description          string `json:"Description"`
	Enabled              bool   `json:"Enabled"`
	PasswordNeverExpires bool   // resolved separately
}

// Apply processes all user entries and ensures they match the desired state.
// Returns the number of changes made.
func Apply(entries []config.User) (int, error) {
	if len(entries) == 0 {
		return 0, nil
	}

	changed := 0
	for _, entry := range entries {
		c, err := applyUser(entry)
		if err != nil {
			ui.Statusf("ERROR", "User %q: %v", entry.Name, err)
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

// applyUser applies a single user entry. Returns the number of changes made.
func applyUser(entry config.User) (int, error) {
	exists, err := userExists(entry.Name)
	if err != nil {
		return 0, err
	}

	switch entry.State {
	case "present":
		return applyPresent(entry, exists)
	case "absent":
		return applyAbsent(entry, exists)
	}
	return 0, nil
}

// applyPresent ensures a user exists with the desired properties.
func applyPresent(entry config.User, exists bool) (int, error) {
	changed := 0

	if !exists {
		if err := createUser(entry); err != nil {
			return 0, err
		}
		changed++
	} else {
		c, err := updateUser(entry)
		if err != nil {
			return 0, err
		}
		changed += c
	}

	// Ensure group memberships.
	c, err := ensureGroups(entry.Name, entry.Groups)
	if err != nil {
		return changed, err
	}
	changed += c

	return changed, nil
}

// applyAbsent ensures a user does not exist.
func applyAbsent(entry config.User, exists bool) (int, error) {
	if !exists {
		return 0, nil
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Remove-LocalUser -Name '%s'`, sanitizePSString(entry.Name)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("removing user %q: %w\nOutput: %s", entry.Name, err, string(output))
	}
	return 1, nil
}

// userExists checks whether a local user account exists.
func userExists(name string) (bool, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Get-LocalUser -Name '%s' -ErrorAction SilentlyContinue | ConvertTo-Json -Depth 1`, sanitizePSString(name)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil
	}
	trimmed := strings.TrimSpace(string(output))
	return trimmed != "" && trimmed != "null", nil
}

// createUser creates a new local user account.
func createUser(entry config.User) error {
	// Build the PowerShell command.
	var parts []string
	parts = append(parts, fmt.Sprintf(`$pw = ConvertTo-SecureString '%s' -AsPlainText -Force`, sanitizePSString(entry.Password)))

	newCmd := fmt.Sprintf(`New-LocalUser -Name '%s' -Password $pw`, sanitizePSString(entry.Name))
	if entry.FullName != "" {
		newCmd += fmt.Sprintf(` -FullName '%s'`, sanitizePSString(entry.FullName))
	}
	if entry.Description != "" {
		newCmd += fmt.Sprintf(` -Description '%s'`, sanitizePSString(entry.Description))
	}
	if entry.PasswordNeverExpires != nil && *entry.PasswordNeverExpires {
		newCmd += " -PasswordNeverExpires"
	}
	if entry.AccountDisabled != nil && *entry.AccountDisabled {
		newCmd += " -Disabled"
	}
	parts = append(parts, newCmd)

	script := strings.Join(parts, "; ")
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating user %q: %w\nOutput: %s", entry.Name, err, string(output))
	}
	return nil
}

// updateUser updates an existing user's properties if they differ.
func updateUser(entry config.User) (int, error) {
	current, err := getUser(entry.Name)
	if err != nil {
		return 0, err
	}

	var setCmds []string

	if entry.FullName != "" && current.FullName != entry.FullName {
		setCmds = append(setCmds, fmt.Sprintf(`-FullName "%s"`, entry.FullName))
	}
	if entry.Description != "" && current.Description != entry.Description {
		setCmds = append(setCmds, fmt.Sprintf(`-Description "%s"`, entry.Description))
	}
	if entry.AccountDisabled != nil {
		// Enabled is the inverse of AccountDisabled.
		desiredEnabled := !*entry.AccountDisabled
		if current.Enabled != desiredEnabled {
			if desiredEnabled {
				setCmds = append(setCmds, "-Disabled:$false")
			} else {
				setCmds = append(setCmds, "-Disabled")
			}
		}
	}

	changed := 0

	// Handle password change — always set if specified (we can't read the current one).
	if entry.Password != "" {
		pwCmd := fmt.Sprintf(`$pw = ConvertTo-SecureString '%s' -AsPlainText -Force; Set-LocalUser -Name '%s' -Password $pw`,
			sanitizePSString(entry.Password), sanitizePSString(entry.Name))
		cmd := exec.Command("powershell", "-NoProfile", "-Command", pwCmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return 0, fmt.Errorf("setting password for %q: %w\nOutput: %s", entry.Name, err, string(output))
		}
		// Password is always re-set since we cannot read the current one.
		// This means it counts as a change on every run when password is specified.
		changed++
	}

	// Handle PasswordNeverExpires separately via Set-LocalUser.
	if entry.PasswordNeverExpires != nil {
		currentPNE, err := getPasswordNeverExpires(entry.Name)
		if err == nil && currentPNE != *entry.PasswordNeverExpires {
			val := "$false"
			if *entry.PasswordNeverExpires {
				val = "$true"
			}
			setCmds = append(setCmds, fmt.Sprintf("-PasswordNeverExpires %s", val))
		}
	}

	if len(setCmds) > 0 {
		script := fmt.Sprintf(`Set-LocalUser -Name '%s' %s`, sanitizePSString(entry.Name), strings.Join(setCmds, " "))
		cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return 0, fmt.Errorf("updating user %q: %w\nOutput: %s", entry.Name, err, string(output))
		}
		changed++
	}

	return changed, nil
}

// getUser retrieves a local user's properties.
func getUser(name string) (*localUser, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Get-LocalUser -Name '%s' | ConvertTo-Json -Depth 1`, sanitizePSString(name)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("getting user %q: %w\nOutput: %s", name, err, string(output))
	}

	var user localUser
	if err := json.Unmarshal(output, &user); err != nil {
		return nil, fmt.Errorf("parsing user %q JSON: %w", name, err)
	}
	return &user, nil
}

// getPasswordNeverExpires checks the PasswordNeverExpires property.
func getPasswordNeverExpires(name string) (bool, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`(Get-LocalUser -Name '%s').PasswordNeverExpires`, sanitizePSString(name)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "True", nil
}

// ensureGroups ensures the user is a member of all listed groups. Additive only.
// Group names are resolved via SID for well-known groups to support localized Windows.
func ensureGroups(username string, groups []string) (int, error) {
	changed := 0
	for _, group := range groups {
		resolved, err := resolveGroupName(group)
		if err != nil {
			// Fall back to the original name if resolution fails.
			resolved = group
		}
		member, err := isMemberOf(username, resolved)
		if err != nil {
			return changed, err
		}
		if !member {
			if err := addToGroup(username, resolved); err != nil {
				return changed, err
			}
			changed++
		}
	}
	return changed, nil
}

// resolveGroupName translates a well-known English group name to the local
// group name on this system by looking up its SID. If the name is not a
// known alias, it is returned unchanged.
func resolveGroupName(name string) (string, error) {
	sid, ok := wellKnownGroups[strings.ToLower(name)]
	if !ok {
		return name, nil
	}

	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`(New-Object System.Security.Principal.SecurityIdentifier("%s")).Translate([System.Security.Principal.NTAccount]).Value.Split('\')[-1]`, sid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolving SID %s: %w", sid, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// isMemberOf checks whether a user is a member of a local group.
func isMemberOf(username, group string) (bool, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Get-LocalGroupMember -Group '%s' -ErrorAction SilentlyContinue | Where-Object { $_.Name -like '*\%s' } | Measure-Object | Select-Object -ExpandProperty Count`, sanitizePSString(group), sanitizePSString(username)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Group may not exist — treat as not a member.
		return false, nil
	}
	return strings.TrimSpace(string(output)) != "0", nil
}

// addToGroup adds a user to a local group.
func addToGroup(username, group string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		fmt.Sprintf(`Add-LocalGroupMember -Group '%s' -Member '%s'`, sanitizePSString(group), sanitizePSString(username)))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adding %q to group %q: %w\nOutput: %s", username, group, err, string(output))
	}
	return nil
}

// sanitizePSString makes a string safe for embedding in a PowerShell
// single-quoted literal by doubling any single quotes. Single-quoted
// strings in PowerShell do not interpret any escape sequences or
// variable references, making them immune to injection.
func sanitizePSString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
