package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"provisioner/pkg/environment"
	"provisioner/pkg/logging"
	"provisioner/pkg/opentofu"
)

type Scheduler struct {
	environments    []environment.Environment
	state           *State
	client          opentofu.TofuClient
	statePath       string
	stopChan        chan bool
	lastConfigCheck time.Time
	configDir       string
	quietMode       bool
}

func New() *Scheduler {
	configDir := getConfigDir()
	stateDir := getStateDir()

	return &Scheduler{
		statePath: filepath.Join(stateDir, "scheduler.json"),
		stopChan:  make(chan bool),
		configDir: configDir,
	}
}

func NewWithClient(client opentofu.TofuClient) *Scheduler {
	configDir := getConfigDir()
	stateDir := getStateDir()

	return &Scheduler{
		client:    client,
		statePath: filepath.Join(stateDir, "scheduler.json"),
		stopChan:  make(chan bool),
		configDir: configDir,
	}
}

// NewQuiet creates a new scheduler for CLI operations (suppresses verbose loading output)
func NewQuiet() *Scheduler {
	configDir := getConfigDir()
	stateDir := getStateDir()

	return &Scheduler{
		statePath: filepath.Join(stateDir, "scheduler.json"),
		stopChan:  make(chan bool),
		configDir: configDir,
		quietMode: true,
	}
}

func (s *Scheduler) LoadEnvironments() error {
	environmentsDir := filepath.Join(s.configDir, "environments")

	environments, err := environment.LoadEnvironments(environmentsDir)
	if err != nil {
		return fmt.Errorf("failed to load environments: %w", err)
	}

	s.environments = environments
	s.lastConfigCheck = time.Now()

	enabledCount := 0
	for _, env := range s.environments {
		if env.Config.Enabled {
			enabledCount++
		}
	}

	if !s.quietMode {
		logging.LogSystemd("Loaded %d environments (%d enabled, %d disabled)", len(s.environments), enabledCount, len(s.environments)-enabledCount)

		for _, env := range s.environments {
			status := "disabled"
			if env.Config.Enabled {
				status = "enabled"
			}

			deploySchedules, _ := env.Config.GetDeploySchedules()
			destroySchedules, _ := env.Config.GetDestroySchedules()

			logging.LogSystemd("Environment: %s (%s) - deploy: %s, destroy: %s",
				env.Name,
				status,
				formatSchedules(deploySchedules),
				formatSchedules(destroySchedules))
		}
	}

	return nil
}

func (s *Scheduler) LoadState() error {
	state, err := LoadState(s.statePath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	s.state = state
	if !s.quietMode {
		logging.LogSystemd("State loaded with %d environment records", len(s.state.Environments))
	}
	return nil
}

func (s *Scheduler) SaveState() error {
	if s.state == nil {
		return fmt.Errorf("no state to save")
	}

	return s.state.SaveState(s.statePath)
}

func (s *Scheduler) Start() {
	logging.LogSystemd("Starting scheduler loop...")

	// Initialize OpenTofu client if not provided
	if s.client == nil {
		client, err := opentofu.New()
		if err != nil {
			logging.LogSystemd("Failed to initialize OpenTofu client: %v", err)
			return
		}
		s.client = client
	}

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkSchedules()
		case <-s.stopChan:
			logging.LogSystemd("Scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopChan)
}

func (s *Scheduler) checkSchedules() {
	now := time.Now()

	// Check for configuration changes every 30 seconds
	if now.Sub(s.lastConfigCheck) > 30*time.Second {
		if s.hasConfigChanged() {
			logging.LogSystemd("Configuration changes detected, reloading environments...")
			if err := s.LoadEnvironments(); err != nil {
				logging.LogSystemd("Error reloading environments: %v", err)
			}
		} else {
			s.lastConfigCheck = now
		}
	}

	for _, env := range s.environments {
		// Only check schedules for enabled environments
		if env.Config.Enabled {
			s.checkEnvironmentSchedules(env, now)
		}
	}

	// Save state after checking all schedules
	if err := s.SaveState(); err != nil {
		logging.LogSystemd("Error saving state: %v", err)
	}
}

func (s *Scheduler) checkEnvironmentSchedules(env environment.Environment, now time.Time) {
	envState := s.state.GetEnvironmentState(env.Name)

	// Skip if environment is currently being deployed or destroyed
	if envState.Status == StatusDeploying || envState.Status == StatusDestroying {
		logging.LogEnvironment(env.Name, "Environment is busy (%s), skipping", envState.Status)
		return
	}

	// Check deploy schedules
	deploySchedules, err := env.Config.GetDeploySchedules()
	if err != nil {
		logging.LogEnvironment(env.Name, "Invalid deploy schedule: %v", err)
	} else if s.ShouldRunDeploySchedule(deploySchedules, now, envState) {
		logging.LogEnvironment(env.Name, "Triggering deployment")
		go s.deployEnvironment(env)
	}

	// Check destroy schedules
	destroySchedules, err := env.Config.GetDestroySchedules()
	if err != nil {
		logging.LogEnvironment(env.Name, "Invalid destroy schedule: %v", err)
	} else if len(destroySchedules) == 0 {
		// Permanent deployment - no destroy schedules (destroy_schedule: false)
		// Log only in verbose mode to avoid spam
	} else if s.ShouldRunDestroySchedule(destroySchedules, now, envState) {
		logging.LogEnvironment(env.Name, "Triggering destruction")
		go s.destroyEnvironment(env)
	}
}

// ShouldRunDeploySchedule checks if environment should be deployed based on schedule and current state
func (s *Scheduler) ShouldRunDeploySchedule(schedules []string, now time.Time, envState *EnvironmentState) bool {
	// Don't deploy if already deployed
	if envState.Status == StatusDeployed {
		return false
	}

	// Don't retry deployment if in failed state (wait for config change)
	if envState.Status == StatusDeployFailed {
		return false
	}

	// Check if any deploy schedule has passed today and we haven't deployed since then
	for _, scheduleStr := range schedules {
		schedule, err := ParseCron(scheduleStr)
		if err != nil {
			logging.LogSystemd("Failed to parse deploy schedule '%s': %v", scheduleStr, err)
			continue
		}

		// Find the most recent time this schedule should have run today
		lastScheduledTime := s.getLastScheduledTimeToday(schedule, now)
		if lastScheduledTime == nil {
			continue // No scheduled time today
		}

		// Check if we should deploy:
		// 1. The scheduled time has passed
		// 2. We haven't deployed since that scheduled time
		if now.After(*lastScheduledTime) {
			if envState.LastDeployed == nil || envState.LastDeployed.Before(*lastScheduledTime) {
				// Note: We don't log here since this will be logged in checkEnvironmentSchedules
				return true
			}
		}
	}
	return false
}

// ShouldRunDestroySchedule checks if environment should be destroyed based on schedule and current state
func (s *Scheduler) ShouldRunDestroySchedule(schedules []string, now time.Time, envState *EnvironmentState) bool {
	// Don't destroy if already destroyed
	if envState.Status == StatusDestroyed {
		return false
	}

	// Don't retry destruction if in failed state (wait for config change)
	if envState.Status == StatusDestroyFailed {
		return false
	}

	// Check if any destroy schedule has passed today and we haven't destroyed since then
	for _, scheduleStr := range schedules {
		schedule, err := ParseCron(scheduleStr)
		if err != nil {
			logging.LogSystemd("Failed to parse destroy schedule '%s': %v", scheduleStr, err)
			continue
		}

		// Find the most recent time this schedule should have run today
		lastScheduledTime := s.getLastScheduledTimeToday(schedule, now)
		if lastScheduledTime == nil {
			continue // No scheduled time today
		}

		// Check if we should destroy:
		// 1. The scheduled time has passed
		// 2. We haven't destroyed since that scheduled time
		if now.After(*lastScheduledTime) {
			if envState.LastDestroyed == nil || envState.LastDestroyed.Before(*lastScheduledTime) {
				// Note: We don't log here since this will be logged in checkEnvironmentSchedules
				return true
			}
		}
	}
	return false
}

// getLastScheduledTimeToday finds the most recent time today that matches the CRON schedule
func (s *Scheduler) getLastScheduledTimeToday(schedule *CronSchedule, now time.Time) *time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Check each minute of today to find the most recent match
	var lastMatch *time.Time
	for minute := 0; minute < 24*60; minute++ {
		checkTime := today.Add(time.Duration(minute) * time.Minute)
		if checkTime.After(now) {
			break // Don't check future times
		}
		if schedule.ShouldRun(checkTime) {
			lastMatch = &checkTime
		}
	}

	return lastMatch
}

// shouldRunAnySchedule checks if any of the provided schedules should run at the given time (legacy exact match)
func (s *Scheduler) shouldRunAnySchedule(schedules []string, now time.Time) bool {
	for _, scheduleStr := range schedules {
		schedule, err := ParseCron(scheduleStr)
		if err != nil {
			logging.LogSystemd("Failed to parse schedule '%s': %v", scheduleStr, err)
			continue
		}
		if schedule.ShouldRun(now) {
			return true
		}
	}
	return false
}

func (s *Scheduler) deployEnvironment(env environment.Environment) {
	envName := env.Name
	logging.LogEnvironmentOperation(envName, "DEPLOY", "Starting deployment")

	s.state.SetEnvironmentStatus(envName, StatusDeploying)
	_ = s.SaveState()

	if err := s.client.Deploy(env.Path); err != nil {
		// Log high-level failure to systemd
		logging.LogEnvironmentOperation(envName, "DEPLOY", "Failed: %s", getHighLevelError(err))

		// Log detailed error only to environment file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogEnvironmentOnly(envName, "DEPLOY: Failed: %s", cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getEnvironmentLogFile(envName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetEnvironmentError(envName, true, err.Error())
	} else {
		logging.LogEnvironmentOperation(envName, "DEPLOY", "Successfully completed")
		s.state.SetEnvironmentStatus(envName, StatusDeployed)
	}

	_ = s.SaveState()
}

func (s *Scheduler) destroyEnvironment(env environment.Environment) {
	envName := env.Name
	logging.LogEnvironmentOperation(envName, "DESTROY", "Starting destruction")

	s.state.SetEnvironmentStatus(envName, StatusDestroying)
	_ = s.SaveState()

	if err := s.client.DestroyEnvironment(env.Path); err != nil {
		// Log high-level failure to systemd
		logging.LogEnvironmentOperation(envName, "DESTROY", "Failed: %s", getHighLevelError(err))

		// Log detailed error only to environment file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogEnvironmentOnly(envName, "DESTROY: Failed: %s", cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getEnvironmentLogFile(envName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetEnvironmentError(envName, false, err.Error())
	} else {
		logging.LogEnvironmentOperation(envName, "DESTROY", "Successfully completed")
		s.state.SetEnvironmentStatus(envName, StatusDestroyed)
	}

	_ = s.SaveState()
}

// hasConfigChanged checks if any configuration files have been modified
func (s *Scheduler) hasConfigChanged() bool {
	environmentsDir := filepath.Join(s.configDir, "environments")

	var hasChanged bool
	envConfigChanges := make(map[string]time.Time)

	// Walk through all environment directories
	err := filepath.Walk(environmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on error
		}

		// Check config.json and .tf files
		if filepath.Base(path) == "config.json" || filepath.Ext(path) == ".tf" {
			if info.ModTime().After(s.lastConfigCheck) {
				logging.LogSystemd("Config file changed: %s (modified: %s)", path, info.ModTime().Format("2006-01-02 15:04:05"))
				hasChanged = true

				// Extract environment name from path
				envName := filepath.Base(filepath.Dir(path))
				if existingTime, exists := envConfigChanges[envName]; !exists || info.ModTime().After(existingTime) {
					envConfigChanges[envName] = info.ModTime()
				}
			}
		}

		return nil
	})

	if err != nil {
		logging.LogSystemd("Error walking config directory: %v", err)
	}

	// Update per-environment config modification times and check for immediate deployment
	now := time.Now()
	for envName, modTime := range envConfigChanges {
		s.state.SetEnvironmentConfigModified(envName, modTime)
		logging.LogSystemd("Environment %s configuration updated, resetting failed state if applicable", envName)

		// Check if this environment should be deployed immediately
		s.checkEnvironmentForImmediateDeployment(envName, now)
	}

	return hasChanged
}

// getEnvironmentLogFile returns the log file path for an environment
func (s *Scheduler) getEnvironmentLogFile(envName string) string {
	logDir := getLogDir()
	return filepath.Join(logDir, fmt.Sprintf("%s.log", envName))
}

// getLogDir determines the log directory using auto-discovery (same logic as logging package)
func getLogDir() string {
	// First check environment variable (explicit override)
	if logDir := os.Getenv("PROVISIONER_LOG_DIR"); logDir != "" {
		return logDir
	}

	// Auto-detect system installation by checking if /var/log/provisioner exists
	systemLogDir := "/var/log/provisioner"
	if _, err := os.Stat(systemLogDir); err == nil {
		return systemLogDir
	}

	// Fall back to development default
	return "logs"
}

// checkEnvironmentForImmediateDeployment checks if an environment should be deployed immediately after config change
func (s *Scheduler) checkEnvironmentForImmediateDeployment(envName string, now time.Time) {
	// Find the environment by name
	var targetEnv *environment.Environment
	for i, env := range s.environments {
		if env.Name == envName {
			targetEnv = &s.environments[i]
			break
		}
	}

	if targetEnv == nil {
		logging.LogSystemd("Environment %s not found for immediate deployment check", envName)
		return
	}

	// Check if environment is enabled
	if !targetEnv.Config.Enabled {
		logging.LogEnvironment(envName, "Environment disabled, skipping immediate deployment")
		return
	}

	envState := s.state.GetEnvironmentState(envName)

	// Skip if environment is currently being deployed or destroyed
	if envState.Status == StatusDeploying || envState.Status == StatusDestroying {
		logging.LogEnvironment(envName, "Environment is busy (%s), skipping immediate deployment", envState.Status)
		return
	}

	// Check deploy schedules
	deploySchedules, err := targetEnv.Config.GetDeploySchedules()
	if err != nil {
		logging.LogEnvironment(envName, "Invalid deploy schedule for immediate deployment: %v", err)
		return
	}

	if s.ShouldRunDeploySchedule(deploySchedules, now, envState) {
		logging.LogEnvironment(envName, "Triggering immediate deployment after config change")
		go s.deployEnvironment(*targetEnv)
	}
}

// getHighLevelError extracts the main error message without detailed output
func getHighLevelError(err error) string {
	errorMsg := err.Error()

	// Split on "Detailed output:" to get just the main error
	if idx := strings.Index(errorMsg, "\n\nDetailed output:"); idx != -1 {
		return errorMsg[:idx]
	}

	return errorMsg
}

// stripANSIColors removes ANSI color escape sequences from text
func stripANSIColors(text string) string {
	// Regex to match ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiRegex.ReplaceAllString(text, "")
}

// getConfigDir determines the configuration directory using auto-discovery
func getConfigDir() string {
	// First check environment variable (explicit override)
	if configDir := os.Getenv("PROVISIONER_CONFIG_DIR"); configDir != "" {
		return configDir
	}

	// Auto-detect system installation
	if _, err := os.Stat("/etc/provisioner"); err == nil {
		return "/etc/provisioner"
	}

	// Fall back to development default
	return "."
}

// getStateDir determines the state directory using auto-discovery
func getStateDir() string {
	// First check environment variable (explicit override)
	if stateDir := os.Getenv("PROVISIONER_STATE_DIR"); stateDir != "" {
		return stateDir
	}

	// Auto-detect system installation
	if _, err := os.Stat("/var/lib/provisioner"); err == nil {
		return "/var/lib/provisioner"
	}

	// Fall back to development default
	return "state"
}

// ManualDeploy deploys a specific environment immediately, bypassing schedule checks
func (s *Scheduler) ManualDeploy(envName string) error {
	// Find the environment by name
	var targetEnv *environment.Environment
	for i, env := range s.environments {
		if env.Name == envName {
			targetEnv = &s.environments[i]
			break
		}
	}

	if targetEnv == nil {
		return fmt.Errorf("environment '%s' not found in configuration", envName)
	}

	// Check if environment is enabled
	if !targetEnv.Config.Enabled {
		return fmt.Errorf("environment '%s' is disabled in configuration", envName)
	}

	envState := s.state.GetEnvironmentState(envName)

	// Check if environment is currently busy
	if envState.Status == StatusDeploying || envState.Status == StatusDestroying {
		return fmt.Errorf("environment '%s' is currently %s, cannot deploy", envName, envState.Status)
	}

	logging.LogSystemd("Manual deployment requested for environment: %s", envName)

	// Execute deployment directly (not in goroutine for immediate feedback)
	s.manualDeployEnvironment(*targetEnv)

	// Save state after manual operation
	if err := s.SaveState(); err != nil {
		logging.LogSystemd("Error saving state after manual deploy: %v", err)
		return fmt.Errorf("deployment completed but failed to save state: %w", err)
	}

	return nil
}

// ManualDestroy destroys a specific environment immediately, bypassing schedule checks
func (s *Scheduler) ManualDestroy(envName string) error {
	// Find the environment by name
	var targetEnv *environment.Environment
	for i, env := range s.environments {
		if env.Name == envName {
			targetEnv = &s.environments[i]
			break
		}
	}

	if targetEnv == nil {
		return fmt.Errorf("environment '%s' not found in configuration", envName)
	}

	// Check if environment is enabled
	if !targetEnv.Config.Enabled {
		return fmt.Errorf("environment '%s' is disabled in configuration", envName)
	}

	envState := s.state.GetEnvironmentState(envName)

	// Check if environment is currently busy
	if envState.Status == StatusDeploying || envState.Status == StatusDestroying {
		return fmt.Errorf("environment '%s' is currently %s, cannot destroy", envName, envState.Status)
	}

	logging.LogSystemd("Manual destruction requested for environment: %s", envName)

	// Execute destruction directly (not in goroutine for immediate feedback)
	s.manualDestroyEnvironment(*targetEnv)

	// Save state after manual operation
	if err := s.SaveState(); err != nil {
		logging.LogSystemd("Error saving state after manual destroy: %v", err)
		return fmt.Errorf("destruction completed but failed to save state: %w", err)
	}

	return nil
}

// manualDeployEnvironment is similar to deployEnvironment but for manual operations
func (s *Scheduler) manualDeployEnvironment(env environment.Environment) {
	envName := env.Name
	logging.LogEnvironmentOperation(envName, "MANUAL DEPLOY", "Starting manual deployment")

	s.state.SetEnvironmentStatus(envName, StatusDeploying)
	_ = s.SaveState()

	// Initialize OpenTofu client if not provided
	if s.client == nil {
		client, err := opentofu.New()
		if err != nil {
			logging.LogEnvironmentOperation(envName, "MANUAL DEPLOY", "Failed to initialize OpenTofu client: %s", err.Error())
			s.state.SetEnvironmentError(envName, true, fmt.Sprintf("Failed to initialize OpenTofu client: %s", err.Error()))
			return
		}
		s.client = client
	}

	if err := s.client.Deploy(env.Path); err != nil {
		// Log high-level failure to systemd
		logging.LogEnvironmentOperation(envName, "MANUAL DEPLOY", "Failed: %s", getHighLevelError(err))

		// Log detailed error only to environment file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogEnvironmentOnly(envName, "MANUAL DEPLOY: Failed: %s", cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getEnvironmentLogFile(envName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetEnvironmentError(envName, true, err.Error())
	} else {
		logging.LogEnvironmentOperation(envName, "MANUAL DEPLOY", "Successfully completed")
		s.state.SetEnvironmentStatus(envName, StatusDeployed)
	}
}

// manualDestroyEnvironment is similar to destroyEnvironment but for manual operations
func (s *Scheduler) manualDestroyEnvironment(env environment.Environment) {
	envName := env.Name
	logging.LogEnvironmentOperation(envName, "MANUAL DESTROY", "Starting manual destruction")

	s.state.SetEnvironmentStatus(envName, StatusDestroying)
	_ = s.SaveState()

	// Initialize OpenTofu client if not provided
	if s.client == nil {
		client, err := opentofu.New()
		if err != nil {
			logging.LogEnvironmentOperation(envName, "MANUAL DESTROY", "Failed to initialize OpenTofu client: %s", err.Error())
			s.state.SetEnvironmentError(envName, false, fmt.Sprintf("Failed to initialize OpenTofu client: %s", err.Error()))
			return
		}
		s.client = client
	}

	if err := s.client.DestroyEnvironment(env.Path); err != nil {
		// Log high-level failure to systemd
		logging.LogEnvironmentOperation(envName, "MANUAL DESTROY", "Failed: %s", getHighLevelError(err))

		// Log detailed error only to environment file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogEnvironmentOnly(envName, "MANUAL DESTROY: Failed: %s", cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getEnvironmentLogFile(envName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetEnvironmentError(envName, false, err.Error())
	} else {
		logging.LogEnvironmentOperation(envName, "MANUAL DESTROY", "Successfully completed")
		s.state.SetEnvironmentStatus(envName, StatusDestroyed)
	}
}

// ShowStatus displays the status of environments
func (s *Scheduler) ShowStatus(envName string) error {
	if err := s.LoadEnvironments(); err != nil {
		return fmt.Errorf("failed to load environments: %w", err)
	}

	if err := s.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if envName != "" {
		// Show specific environment status
		env := s.findEnvironment(envName)
		if env == nil {
			return fmt.Errorf("environment '%s' not found", envName)
		}
		s.printEnvironmentStatus(*env)
	} else {
		// Show all environments status
		fmt.Printf("%-15s %-12s %-20s %-20s %-10s\n", "ENVIRONMENT", "STATUS", "LAST DEPLOYED", "LAST DESTROYED", "ERRORS")
		fmt.Printf("%-15s %-12s %-20s %-20s %-10s\n", "-----------", "------", "-------------", "--------------", "------")

		for _, env := range s.environments {
			state := s.state.GetEnvironmentState(env.Name)
			s.printEnvironmentStatusLine(env, state)
		}
	}

	return nil
}

// ListEnvironments displays all configured environments
func (s *Scheduler) ListEnvironments() error {
	if err := s.LoadEnvironments(); err != nil {
		return fmt.Errorf("failed to load environments: %w", err)
	}

	fmt.Printf("%-15s %-8s %-30s %-30s\n", "ENVIRONMENT", "ENABLED", "DEPLOY SCHEDULE", "DESTROY SCHEDULE")
	fmt.Printf("%-15s %-8s %-30s %-30s\n", "-----------", "-------", "---------------", "----------------")

	for _, env := range s.environments {
		deploySchedules, _ := env.Config.GetDeploySchedules()
		destroySchedules, _ := env.Config.GetDestroySchedules()

		deploySchedule := formatSchedules(deploySchedules)
		destroySchedule := formatSchedules(destroySchedules)

		fmt.Printf("%-15s %-8t %-30s %-30s\n",
			env.Name,
			env.Config.Enabled,
			deploySchedule,
			destroySchedule)
	}

	return nil
}

// ShowLogs displays recent logs for an environment
func (s *Scheduler) ShowLogs(envName string) error {
	if err := s.LoadEnvironments(); err != nil {
		return fmt.Errorf("failed to load environments: %w", err)
	}

	env := s.findEnvironment(envName)
	if env == nil {
		return fmt.Errorf("environment '%s' not found", envName)
	}

	logFile := s.getEnvironmentLogFile(envName)

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Printf("No log file found for environment '%s'\n", envName)
		fmt.Printf("Expected location: %s\n", logFile)
		return nil
	}

	// Read and display the log file
	data, err := os.ReadFile(logFile)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	fmt.Printf("=== Recent logs for environment '%s' ===\n", envName)
	fmt.Printf("Log file: %s\n\n", logFile)
	fmt.Print(string(data))

	return nil
}

// Helper methods for CLI commands

func (s *Scheduler) findEnvironment(name string) *environment.Environment {
	for _, env := range s.environments {
		if env.Name == name {
			return &env
		}
	}
	return nil
}

func (s *Scheduler) printEnvironmentStatus(env environment.Environment) {
	state := s.state.GetEnvironmentState(env.Name)

	deploySchedules, _ := env.Config.GetDeploySchedules()
	destroySchedules, _ := env.Config.GetDestroySchedules()

	fmt.Printf("Environment: %s\n", env.Name)
	fmt.Printf("Status: %s\n", state.Status)
	fmt.Printf("Enabled: %t\n", env.Config.Enabled)
	fmt.Printf("Deploy Schedule: %s\n", formatSchedules(deploySchedules))
	fmt.Printf("Destroy Schedule: %s\n", formatSchedules(destroySchedules))

	if state.LastDeployed != nil {
		fmt.Printf("Last Deployed: %s\n", state.LastDeployed.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Deployed: Never\n")
	}

	if state.LastDestroyed != nil {
		fmt.Printf("Last Destroyed: %s\n", state.LastDestroyed.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Printf("Last Destroyed: Never\n")
	}

	if state.LastConfigModified != nil {
		fmt.Printf("Config Modified: %s\n", state.LastConfigModified.Format("2006-01-02 15:04:05"))
	}

	if state.LastDeployError != "" {
		fmt.Printf("Last Deploy Error: %s\n", state.LastDeployError)
	}

	if state.LastDestroyError != "" {
		fmt.Printf("Last Destroy Error: %s\n", state.LastDestroyError)
	}

	logFile := s.getEnvironmentLogFile(env.Name)
	fmt.Printf("Log File: %s\n", logFile)
}

func (s *Scheduler) printEnvironmentStatusLine(env environment.Environment, state *EnvironmentState) {
	lastDeployed := "Never"
	if state.LastDeployed != nil {
		lastDeployed = state.LastDeployed.Format("2006-01-02 15:04")
	}

	lastDestroyed := "Never"
	if state.LastDestroyed != nil {
		lastDestroyed = state.LastDestroyed.Format("2006-01-02 15:04")
	}

	errors := "None"
	if state.LastDeployError != "" || state.LastDestroyError != "" {
		errors = "Yes"
	}

	fmt.Printf("%-15s %-12s %-20s %-20s %-10s\n",
		env.Name,
		string(state.Status),
		lastDeployed,
		lastDestroyed,
		errors)
}

func formatSchedules(schedules []string) string {
	if len(schedules) == 0 {
		return "Permanent"
	}
	if len(schedules) == 1 {
		return schedules[0]
	}
	return fmt.Sprintf("[%s]", strings.Join(schedules, ", "))
}
