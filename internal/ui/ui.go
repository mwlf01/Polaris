package ui

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const lineWidth = 50

// Banner prints the Polaris startup header.
func Banner(version string) {
	fmt.Println()
	fmt.Printf("  Polaris %s\n", version)
	fmt.Println("  Configuration Management Agent")
	Separator()
}

// Separator prints a full-width horizontal rule.
func Separator() {
	fmt.Println(strings.Repeat("\u2500", lineWidth))
}

// Section prints a titled section separator, e.g. "── Packages ──────────".
func Section(title string) {
	fmt.Println()
	prefix := "\u2500\u2500 " + title + " "
	// Calculate remaining fill accounting for Unicode display width.
	fill := lineWidth - displayWidth(prefix)
	if fill < 3 {
		fill = 3
	}
	fmt.Println(prefix + strings.Repeat("\u2500", fill))
}

// Info prints an aligned key-value line, e.g. "  Config ......... path".
func Info(key, value string) {
	dots := 16 - len(key)
	if dots < 2 {
		dots = 2
	}
	fmt.Printf("  %s %s %s\n", key, strings.Repeat(".", dots), value)
}

// Status prints a tagged status line with consistent alignment.
//
//	[OK]       message
//	[INSTALL]  message
func Status(tag, msg string) {
	label := "[" + tag + "]"
	fmt.Printf("  %-10s %s\n", label, msg)
}

// Statusf is like Status but with printf-style formatting for the message.
func Statusf(tag, format string, args ...interface{}) {
	Status(tag, fmt.Sprintf(format, args...))
}

// Summary prints the final summary block.
func Summary(ok, changed, skipped, failed, total int) {
	fmt.Println()
	Separator()
	fmt.Printf("  %d ok \u00b7 %d changed \u00b7 %d skipped \u00b7 %d failed \u00b7 %d total\n", ok, changed, skipped, failed, total)
	Separator()
}

// SetupHelp customizes the cobra help output to a minimal, clean format.
func SetupHelp(rootCmd *cobra.Command, version string) {
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		Banner(version)
		fmt.Println()
		fmt.Println("  polaris                  Apply config.yaml")
		fmt.Println("  polaris apply -c <file>  Apply a specific file")
		fmt.Println("  polaris update            Check for updates and apply")
		fmt.Println("  polaris service install   Install as Windows service")
		fmt.Println("  polaris service uninstall Remove Windows service")
		fmt.Println("  polaris service status    Show service status")
		fmt.Println("  polaris version           Show version")
		fmt.Println()
	})
}

// displayWidth returns the approximate display width of a string,
// counting ASCII chars as 1 and common Unicode box-drawing chars as 1.
func displayWidth(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
