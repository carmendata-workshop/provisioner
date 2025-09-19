package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
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
}

func New() *Scheduler {
	stateDir := os.Getenv("PROVISIONER_STATE_DIR")
	if stateDir == "" {
		stateDir = "state"
	}

	configDir := os.Getenv("PROVISIONER_CONFIG_DIR")
	if configDir == "" {
		configDir = "."
	}

	return &Scheduler{
		statePath: filepath.Join(stateDir, "scheduler.json"),
		stopChan:  make(chan bool),
		configDir: configDir,
	}
}

func NewWithClient(client opentofu.TofuClient) *Scheduler {
	stateDir := os.Getenv("PROVISIONER_STATE_DIR")
	if stateDir == "" {
		stateDir = "state"
	}

	configDir := os.Getenv("PROVISIONER_CONFIG_DIR")
	if configDir == "" {
		configDir = "."
	}

	return &Scheduler{
		client:    client,
		statePath: filepath.Join(stateDir, "scheduler.json"),
		stopChan:  make(chan bool),
		configDir: configDir,
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

	logging.LogSystemd("Loaded %d environments (%d enabled, %d disabled)", len(s.environments), enabledCount, len(s.environments)-enabledCount)

	for _, env := range s.environments {
		status := "disabled"
		if env.Config.Enabled {
			status = "enabled"
		}
		logging.LogSystemd("Environment: %s (%s) - deploy: %s, destroy: %s",
			env.Name,
			status,
			env.Config.DeploySchedule,
			env.Config.DestroySchedule)
	}

	return nil
}

func (s *Scheduler) LoadState() error {
	state, err := LoadState(s.statePath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	s.state = state
	logging.LogSystemd("State loaded with %d environment records", len(s.state.Environments))
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
	logging.LogSystemd("Checking schedules at %s", now.Format("2006-01-02 15:04:05"))

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
		logging.LogEnvironmentOperation(envName, "DEPLOY", "Failed: %v", err)

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
		logging.LogEnvironmentOperation(envName, "DESTROY", "Failed: %v", err)

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
	logDir := os.Getenv("PROVISIONER_LOG_DIR")
	if logDir == "" {
		logDir = "/var/log/provisioner"
	}
	return filepath.Join(logDir, fmt.Sprintf("%s.log", envName))
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
