package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows/svc"

	"polaris/internal/config"
	"polaris/internal/executor"
	"polaris/internal/platform"
	"polaris/internal/service"
	"polaris/internal/ui"
	"polaris/internal/updater"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	// Rollback guard: if a previous update left a state file and the
	// new binary is failing, restore the backup and exit so the SCM
	// restarts with the old binary.
	if updater.CheckPendingUpdate(version) {
		os.Exit(1) // non-zero triggers SCM recovery → restarts with restored binary
	}

	// When launched by the Windows SCM, enter service mode immediately.
	isSvc, err := svc.IsWindowsService()
	if err == nil && isSvc {
		if err := service.Run(version); err != nil {
			log.Fatalf("[Polaris] service failed: %v", err)
		}
		return
	}

	rootCmd := &cobra.Command{
		Use:   "polaris",
		Short: "Polaris – Configuration Management Agent",
		Long:  "Polaris is a lightweight configuration management agent that ensures your system matches a desired state defined in YAML.\n\nWhen called without a subcommand, Polaris looks for config.yaml next to the executable and applies it.",
		RunE:  runDefault,
	}

	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the desired state configuration",
		Long:  "Reads the configuration YAML and applies the desired state to the local system.",
		RunE:  runApply,
	}

	applyCmd.Flags().StringP("config", "c", "", "Path to the configuration YAML file")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of Polaris",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Polaris %s\n", version)
		},
	}

	serviceCmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the Polaris Windows service",
	}

	serviceInstallCmd := &cobra.Command{
		Use:   "install",
		Short: "Install Polaris as a Windows service",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Banner(version)
			return service.Install()
		},
	}

	serviceUninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the Polaris Windows service",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Banner(version)
			return service.Uninstall()
		},
	}

	serviceStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show the status of the Polaris Windows service",
		RunE: func(cmd *cobra.Command, args []string) error {
			ui.Banner(version)
			return service.Status()
		},
	}

	serviceCmd.AddCommand(serviceInstallCmd, serviceUninstallCmd, serviceStatusCmd)

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Check for updates and apply if available",
		RunE:  runUpdate,
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	rootCmd.AddCommand(applyCmd, versionCmd, serviceCmd, updateCmd)

	ui.SetupHelp(rootCmd, version)

	if err := rootCmd.Execute(); err != nil {
		rootCmd.Help()
		ui.Status("ERROR", err.Error())
		fmt.Println()
		os.Exit(1)
	}
}

// resolveConfigPath determines the config file path.
// Priority: 1) explicit flag  2) config.yaml next to the executable.
func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	// Look next to the executable.
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine executable path: %w", err)
	}
	candidate := filepath.Join(filepath.Dir(exePath), "config.yaml")
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("no config.yaml found next to %s", exePath)
	}
	return candidate, nil
}

// runDefault is called when polaris.exe is started without a subcommand.
func runDefault(cmd *cobra.Command, args []string) error {
	cfgPath, err := resolveConfigPath("")
	if err != nil {
		cmd.Help()
		ui.Status("ERROR", "config.yaml not found next to polaris.exe")
		fmt.Println()
		return nil
	}
	return applyConfig(cfgPath)
}

// runApply is called for the explicit "apply" subcommand.
func runApply(cmd *cobra.Command, args []string) error {
	explicit, _ := cmd.Flags().GetString("config")
	cfgPath, err := resolveConfigPath(explicit)
	if err != nil {
		return err
	}
	return applyConfig(cfgPath)
}

// runUpdate checks for updates and applies them if available.
func runUpdate(cmd *cobra.Command, args []string) error {
	cfgPath, err := resolveConfigPath("")
	if err != nil {
		return err
	}

	ui.Banner(version)

	plat, err := platform.Detect()
	if err != nil {
		return err
	}

	ctx := config.LoadContext{
		Version:        version,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		WindowsVersion: plat.OSVersion(),
	}

	loader := config.NewFileLoader(cfgPath, ctx)
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if cfg.Update == nil || cfg.Update.URL == "" {
		ui.Status("OK", "no update URL configured")
		return nil
	}

	restarted, err := updater.CheckAndUpdate(version, cfg.Update)
	if err != nil {
		return fmt.Errorf("update check failed: %w", err)
	}
	if restarted {
		ui.Status("DONE", "update applied — please restart polaris")
	} else {
		ui.Status("OK", fmt.Sprintf("already up to date (%s)", version))
	}
	return nil
}

// applyConfig loads and applies the configuration from the given path.
func applyConfig(cfgPath string) error {
	ui.Banner(version)

	// Detect platform early so the loader can evaluate per-file compatibility.
	plat, err := platform.Detect()
	if err != nil {
		return err
	}

	ui.Info("Config", cfgPath)
	ui.Info("Platform", fmt.Sprintf("Windows %s (%s)", plat.OSVersion(), runtime.GOARCH))

	ctx := config.LoadContext{
		Version:        version,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		WindowsVersion: plat.OSVersion(),
	}

	loader := config.NewFileLoader(cfgPath, ctx)
	cfg, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	exec := executor.New(version)
	return exec.Apply(cfg)
}
