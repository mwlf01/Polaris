package service

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"polaris/internal/config"
	"polaris/internal/executor"
	"polaris/internal/platform"
	"polaris/internal/ui"
	"polaris/internal/updater"
)

const (
	serviceName        = "Polaris"
	serviceDisplayName = "Polaris Configuration Management Agent"
	serviceDescription = "Ensures the system matches a desired state defined in YAML."
	defaultInterval    = 15 * time.Minute
)

// polarisService implements svc.Handler.
type polarisService struct {
	version string
}

// Execute is the main service loop called by the Windows SCM.
func (s *polarisService) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}

	// Resolve config path (next to executable).
	cfgPath, err := resolveServiceConfig()
	if err != nil {
		log.Printf("[Polaris] config resolution failed: %v", err)
		status <- svc.Status{State: svc.StopPending}
		return true, 1
	}

	// Run first apply immediately, then determine interval.
	result := s.runCycle(cfgPath)

	// Self-update was applied → exit with error code so SCM recovery
	// restarts the service with the new binary.
	if result.needsRestart {
		log.Printf("[Polaris] restarting for self-update")
		status <- svc.Status{State: svc.StopPending}
		return false, 1
	}

	// interval == 0 means one-shot mode: apply once and stop.
	if result.interval == 0 {
		log.Printf("[Polaris] one-shot mode, stopping after first apply")
		status <- svc.Status{State: svc.StopPending}
		return false, 0
	}

	interval := result.interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	status <- svc.Status{State: svc.Running, Accepts: accepted}
	log.Printf("[Polaris] service started, interval %s", interval)

	for {
		select {
		case <-ticker.C:
			res := s.runCycle(cfgPath)

			if res.needsRestart {
				log.Printf("[Polaris] restarting for self-update")
				status <- svc.Status{State: svc.StopPending}
				return false, 1
			}

			// interval == 0 means switched to one-shot: stop now.
			if res.interval == 0 {
				log.Printf("[Polaris] interval set to once, stopping")
				status <- svc.Status{State: svc.StopPending}
				return false, 0
			}
			if res.interval != interval {
				ticker.Stop()
				interval = res.interval
				ticker = time.NewTicker(interval)
				log.Printf("[Polaris] interval changed to %s", interval)
			}

		case c := <-r:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				log.Printf("[Polaris] service stopping")
				status <- svc.Status{State: svc.StopPending}
				return false, 0
			case svc.Interrogate:
				status <- c.CurrentStatus
			}
		}
	}
}

// cycleResult carries the outcome of a single apply cycle.
type cycleResult struct {
	interval     time.Duration
	needsRestart bool // true when a self-update was applied
}

// runCycle loads the config, applies the desired state, and checks for
// self-updates. Returns the next interval and whether a restart is needed.
func (s *polarisService) runCycle(cfgPath string) cycleResult {
	log.Printf("[Polaris] applying configuration: %s", cfgPath)

	plat, err := platform.Detect()
	if err != nil {
		log.Printf("[Polaris] platform detection failed: %v", err)
		return cycleResult{interval: defaultInterval}
	}

	ctx := config.LoadContext{
		Version:        s.version,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		WindowsVersion: plat.OSVersion(),
	}

	loader := config.NewFileLoader(cfgPath, ctx)
	cfg, err := loader.Load()
	if err != nil {
		log.Printf("[Polaris] config load failed: %v", err)
		return cycleResult{interval: defaultInterval}
	}

	// Determine interval from config.
	// "once" or "0" → 0 (one-shot), empty/missing → defaultInterval.
	interval := defaultInterval
	if cfg.Schedule != nil && cfg.Schedule.Interval != "" {
		switch cfg.Schedule.Interval {
		case "once", "0", "off":
			interval = 0
		default:
			if d, err := time.ParseDuration(cfg.Schedule.Interval); err == nil && d > 0 {
				interval = d
			} else {
				log.Printf("[Polaris] invalid schedule interval %q, using default %s", cfg.Schedule.Interval, defaultInterval)
			}
		}
	}

	exec := executor.New(s.version)
	if err := exec.Apply(cfg); err != nil {
		log.Printf("[Polaris] apply finished with errors: %v", err)
	} else {
		log.Printf("[Polaris] apply finished successfully")
	}

	// Check for self-update after applying config.
	needsRestart := false
	if cfg.Update != nil && cfg.Update.URL != "" {
		restart, err := updater.CheckAndUpdate(s.version, cfg.Update)
		if err != nil {
			log.Printf("[Polaris] update check failed: %v", err)
		} else if restart {
			needsRestart = true
		}
	}

	return cycleResult{interval: interval, needsRestart: needsRestart}
}

// Run enters the Windows service dispatcher. Should be called from main()
// when svc.IsWindowsService() returns true.
func Run(version string) error {
	return svc.Run(serviceName, &polarisService{version: version})
}

// Install registers Polaris as a Windows service.
func Install() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager: %w", err)
	}
	defer m.Disconnect()

	// Check if already installed.
	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %q is already installed", serviceName)
	}

	s, err = m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName: serviceDisplayName,
		Description: serviceDescription,
		StartType:   mgr.StartAutomatic,
	})
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}
	defer s.Close()

	// Configure recovery: restart after 10 seconds on failure.
	err = s.SetRecoveryActions([]mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 10 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
	}, 86400) // reset failure count after 24h
	if err != nil {
		log.Printf("[Polaris] warning: could not set recovery actions: %v", err)
	}

	ui.Status("DONE", fmt.Sprintf("service %q installed (automatic start)", serviceName))
	return nil
}

// Uninstall removes the Polaris Windows service.
func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %q not found: %w", serviceName, err)
	}
	defer s.Close()

	// Stop the service if running.
	s.Control(svc.Stop)
	time.Sleep(2 * time.Second)

	if err := s.Delete(); err != nil {
		return fmt.Errorf("deleting service: %w", err)
	}

	ui.Status("DONE", fmt.Sprintf("service %q uninstalled", serviceName))
	return nil
}

// Status prints the current state of the Polaris service.
func Status() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		ui.Status("INFO", fmt.Sprintf("service %q is not installed", serviceName))
		return nil
	}
	defer s.Close()

	st, err := s.Query()
	if err != nil {
		return fmt.Errorf("querying service status: %w", err)
	}

	stateStr := "unknown"
	switch st.State {
	case svc.Stopped:
		stateStr = "stopped"
	case svc.StartPending:
		stateStr = "starting"
	case svc.StopPending:
		stateStr = "stopping"
	case svc.Running:
		stateStr = "running"
	case svc.ContinuePending:
		stateStr = "resuming"
	case svc.PausePending:
		stateStr = "pausing"
	case svc.Paused:
		stateStr = "paused"
	}

	ui.Status("INFO", fmt.Sprintf("service %q is %s", serviceName, stateStr))
	return nil
}

func resolveServiceConfig() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}
	candidate := filepath.Join(filepath.Dir(exePath), "config.yaml")
	if _, err := os.Stat(candidate); err != nil {
		return "", fmt.Errorf("config.yaml not found next to %s", exePath)
	}
	return candidate, nil
}
