package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"provisioner/pkg/logging"
	"provisioner/pkg/opentofu"
	"provisioner/pkg/workspace"
)

type Scheduler struct {
	workspaces      []workspace.Workspace
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

func (s *Scheduler) LoadWorkspaces() error {
	workspacesDir := filepath.Join(s.configDir, "workspaces")

	workspaces, err := workspace.LoadWorkspaces(workspacesDir)
	if err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	s.workspaces = workspaces
	s.lastConfigCheck = time.Now()

	enabledCount := 0
	for _, workspace := range s.workspaces {
		if workspace.Config.Enabled {
			enabledCount++
		}
	}

	if !s.quietMode {
		logging.LogSystemd("Loaded %d workspaces (%d enabled, %d disabled)", len(s.workspaces), enabledCount, len(s.workspaces)-enabledCount)

		for _, workspace := range s.workspaces {
			status := "disabled"
			if workspace.Config.Enabled {
				status = "enabled"
			}

			deploySchedules, _ := workspace.Config.GetDeploySchedules()
			destroySchedules, _ := workspace.Config.GetDestroySchedules()

			logging.LogSystemd("Workspace: %s (%s) - deploy: %s, destroy: %s",
				workspace.Name,
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
		logging.LogSystemd("State loaded with %d workspace records", len(s.state.Workspaces))
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
			logging.LogSystemd("Configuration changes detected, reloading workspaces...")
			if err := s.LoadWorkspaces(); err != nil {
				logging.LogSystemd("Error reloading workspaces: %v", err)
			}
		} else {
			s.lastConfigCheck = now
		}
	}

	for _, workspace := range s.workspaces {
		// Only check schedules for enabled workspaces
		if workspace.Config.Enabled {
			s.checkWorkspaceSchedules(workspace, now)
		}
	}

	// Save state after checking all schedules
	if err := s.SaveState(); err != nil {
		logging.LogSystemd("Error saving state: %v", err)
	}
}

func (s *Scheduler) checkWorkspaceSchedules(workspace workspace.Workspace, now time.Time) {
	workspaceState := s.state.GetWorkspaceState(workspace.Name)

	// Skip if workspace is currently being deployed or destroyed
	if workspaceState.Status == StatusDeploying || workspaceState.Status == StatusDestroying {
		logging.LogWorkspace(workspace.Name, "Workspace is busy (%s), skipping", workspaceState.Status)
		return
	}

	// Check deploy schedules
	deploySchedules, err := workspace.Config.GetDeploySchedules()
	if err != nil {
		logging.LogWorkspace(workspace.Name, "Invalid deploy schedule: %v", err)
	} else if s.ShouldRunDeploySchedule(deploySchedules, now, workspaceState) {
		logging.LogWorkspace(workspace.Name, "Triggering deployment")
		go s.deployWorkspace(workspace)
	}

	// Check destroy schedules
	destroySchedules, err := workspace.Config.GetDestroySchedules()
	if err != nil {
		logging.LogWorkspace(workspace.Name, "Invalid destroy schedule: %v", err)
	} else if len(destroySchedules) == 0 {
		// Permanent deployment - no destroy schedules (destroy_schedule: false)
		// Log only in verbose mode to avoid spam
	} else if s.ShouldRunDestroySchedule(destroySchedules, now, workspaceState) {
		logging.LogWorkspace(workspace.Name, "Triggering destruction")
		go s.destroyWorkspace(workspace)
	}
}

// ShouldRunDeploySchedule checks if workspace should be deployed based on schedule and current state
func (s *Scheduler) ShouldRunDeploySchedule(schedules []string, now time.Time, workspaceState *WorkspaceState) bool {
	// Don't deploy if already deployed
	if workspaceState.Status == StatusDeployed {
		return false
	}

	// Don't retry deployment if in failed state (wait for config change)
	if workspaceState.Status == StatusDeployFailed {
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
			if workspaceState.LastDeployed == nil || workspaceState.LastDeployed.Before(*lastScheduledTime) {
				// Note: We don't log here since this will be logged in checkWorkspaceSchedules
				return true
			}
		}
	}
	return false
}

// ShouldRunDestroySchedule checks if workspace should be destroyed based on schedule and current state
func (s *Scheduler) ShouldRunDestroySchedule(schedules []string, now time.Time, workspaceState *WorkspaceState) bool {
	// Don't destroy if already destroyed
	if workspaceState.Status == StatusDestroyed {
		return false
	}

	// Don't retry destruction if in failed state (wait for config change)
	if workspaceState.Status == StatusDestroyFailed {
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
			if workspaceState.LastDestroyed == nil || workspaceState.LastDestroyed.Before(*lastScheduledTime) {
				// Note: We don't log here since this will be logged in checkWorkspaceSchedules
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

func (s *Scheduler) deployWorkspace(workspace workspace.Workspace) {
	workspaceName := workspace.Name
	logging.LogWorkspaceOperation(workspaceName, "DEPLOY", "Starting deployment")

	s.state.SetWorkspaceStatus(workspaceName, StatusDeploying)
	_ = s.SaveState()

	if err := s.client.Deploy(&workspace); err != nil {
		// Log high-level failure to systemd
		logging.LogWorkspaceOperation(workspaceName, "DEPLOY", "Failed: %s", getHighLevelError(err))

		// Log detailed error only to workspace file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogWorkspaceOnly(workspaceName, "DEPLOY: Failed: %s", cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getWorkspaceLogFile(workspaceName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetWorkspaceError(workspaceName, true, err.Error())
	} else {
		logging.LogWorkspaceOperation(workspaceName, "DEPLOY", "Successfully completed")
		s.state.SetWorkspaceStatus(workspaceName, StatusDeployed)
	}

	_ = s.SaveState()
}

func (s *Scheduler) destroyWorkspace(workspace workspace.Workspace) {
	workspaceName := workspace.Name
	logging.LogWorkspaceOperation(workspaceName, "DESTROY", "Starting destruction")

	s.state.SetWorkspaceStatus(workspaceName, StatusDestroying)
	_ = s.SaveState()

	if err := s.client.DestroyWorkspace(&workspace); err != nil {
		// Log high-level failure to systemd
		logging.LogWorkspaceOperation(workspaceName, "DESTROY", "Failed: %s", getHighLevelError(err))

		// Log detailed error only to workspace file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogWorkspaceOnly(workspaceName, "DESTROY: Failed: %s", cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getWorkspaceLogFile(workspaceName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetWorkspaceError(workspaceName, false, err.Error())
	} else {
		logging.LogWorkspaceOperation(workspaceName, "DESTROY", "Successfully completed")
		s.state.SetWorkspaceStatus(workspaceName, StatusDestroyed)
	}

	_ = s.SaveState()
}

// hasConfigChanged checks if any configuration files have been modified
func (s *Scheduler) hasConfigChanged() bool {
	workspacesDir := filepath.Join(s.configDir, "workspaces")

	var hasChanged bool
	workspaceConfigChanges := make(map[string]time.Time)

	// Walk through all workspace directories
	err := filepath.Walk(workspacesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on error
		}

		// Check config.json and .tf files
		if filepath.Base(path) == "config.json" || filepath.Ext(path) == ".tf" {
			if info.ModTime().After(s.lastConfigCheck) {
				logging.LogSystemd("Config file changed: %s (modified: %s)", path, info.ModTime().Format("2006-01-02 15:04:05"))
				hasChanged = true

				// Extract workspace name from path
				workspaceName := filepath.Base(filepath.Dir(path))
				if existingTime, exists := workspaceConfigChanges[workspaceName]; !exists || info.ModTime().After(existingTime) {
					workspaceConfigChanges[workspaceName] = info.ModTime()
				}
			}
		}

		return nil
	})

	if err != nil {
		logging.LogSystemd("Error walking config directory: %v", err)
	}

	// Update per-workspace config modification times and check for immediate deployment
	now := time.Now()
	for workspaceName, modTime := range workspaceConfigChanges {
		s.state.SetWorkspaceConfigModified(workspaceName, modTime)
		logging.LogSystemd("Workspace %s configuration updated, resetting failed state if applicable", workspaceName)

		// Check if this workspace should be deployed immediately
		s.checkWorkspaceForImmediateDeployment(workspaceName, now)
	}

	return hasChanged
}

// getWorkspaceLogFile returns the log file path for an workspace
func (s *Scheduler) getWorkspaceLogFile(workspaceName string) string {
	logDir := getLogDir()
	return filepath.Join(logDir, fmt.Sprintf("%s.log", workspaceName))
}

// getLogDir determines the log directory using auto-discovery (same logic as logging package)
func getLogDir() string {
	// First check workspace variable (explicit override)
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

// checkWorkspaceForImmediateDeployment checks if an workspace should be deployed immediately after config change
func (s *Scheduler) checkWorkspaceForImmediateDeployment(workspaceName string, now time.Time) {
	// Find the workspace by name
	var targetWorkspace *workspace.Workspace
	for i, workspace := range s.workspaces {
		if workspace.Name == workspaceName {
			targetWorkspace = &s.workspaces[i]
			break
		}
	}

	if targetWorkspace == nil {
		logging.LogSystemd("Workspace %s not found for immediate deployment check", workspaceName)
		return
	}

	// Check if workspace is enabled
	if !targetWorkspace.Config.Enabled {
		logging.LogWorkspace(workspaceName, "Workspace disabled, skipping immediate deployment")
		return
	}

	workspaceState := s.state.GetWorkspaceState(workspaceName)

	// Skip if workspace is currently being deployed or destroyed
	if workspaceState.Status == StatusDeploying || workspaceState.Status == StatusDestroying {
		logging.LogWorkspace(workspaceName, "Workspace is busy (%s), skipping immediate deployment", workspaceState.Status)
		return
	}

	// Check deploy schedules
	deploySchedules, err := targetWorkspace.Config.GetDeploySchedules()
	if err != nil {
		logging.LogWorkspace(workspaceName, "Invalid deploy schedule for immediate deployment: %v", err)
		return
	}

	if s.ShouldRunDeploySchedule(deploySchedules, now, workspaceState) {
		logging.LogWorkspace(workspaceName, "Triggering immediate deployment after config change")
		go s.deployWorkspace(*targetWorkspace)
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
	// First check workspace variable (explicit override)
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
	// First check workspace variable (explicit override)
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

// ManualDeploy deploys a specific workspace immediately, bypassing schedule checks
func (s *Scheduler) ManualDeploy(workspaceName string) error {
	// Find the workspace by name
	var targetWorkspace *workspace.Workspace
	for i, workspace := range s.workspaces {
		if workspace.Name == workspaceName {
			targetWorkspace = &s.workspaces[i]
			break
		}
	}

	if targetWorkspace == nil {
		return fmt.Errorf("workspace '%s' not found in configuration", workspaceName)
	}

	// Check if workspace is enabled
	if !targetWorkspace.Config.Enabled {
		return fmt.Errorf("workspace '%s' is disabled in configuration", workspaceName)
	}

	workspaceState := s.state.GetWorkspaceState(workspaceName)

	// Check if workspace is currently busy
	if workspaceState.Status == StatusDeploying || workspaceState.Status == StatusDestroying {
		return fmt.Errorf("workspace '%s' is currently %s, cannot deploy", workspaceName, workspaceState.Status)
	}

	logging.LogSystemd("Manual deployment requested for workspace: %s", workspaceName)

	// Execute deployment directly (not in goroutine for immediate feedback)
	s.manualDeployWorkspace(*targetWorkspace)

	// Save state after manual operation
	if err := s.SaveState(); err != nil {
		logging.LogSystemd("Error saving state after manual deploy: %v", err)
		return fmt.Errorf("deployment completed but failed to save state: %w", err)
	}

	return nil
}

// ManualDestroy destroys a specific workspace immediately, bypassing schedule checks
func (s *Scheduler) ManualDestroy(workspaceName string) error {
	// Find the workspace by name
	var targetWorkspace *workspace.Workspace
	for i, workspace := range s.workspaces {
		if workspace.Name == workspaceName {
			targetWorkspace = &s.workspaces[i]
			break
		}
	}

	if targetWorkspace == nil {
		return fmt.Errorf("workspace '%s' not found in configuration", workspaceName)
	}

	// Check if workspace is enabled
	if !targetWorkspace.Config.Enabled {
		return fmt.Errorf("workspace '%s' is disabled in configuration", workspaceName)
	}

	workspaceState := s.state.GetWorkspaceState(workspaceName)

	// Check if workspace is currently busy
	if workspaceState.Status == StatusDeploying || workspaceState.Status == StatusDestroying {
		return fmt.Errorf("workspace '%s' is currently %s, cannot destroy", workspaceName, workspaceState.Status)
	}

	logging.LogSystemd("Manual destruction requested for workspace: %s", workspaceName)

	// Execute destruction directly (not in goroutine for immediate feedback)
	s.manualDestroyWorkspace(*targetWorkspace)

	// Save state after manual operation
	if err := s.SaveState(); err != nil {
		logging.LogSystemd("Error saving state after manual destroy: %v", err)
		return fmt.Errorf("destruction completed but failed to save state: %w", err)
	}

	return nil
}

// GetWorkspace returns a workspace by name
func (s *Scheduler) GetWorkspace(workspaceName string) *workspace.Workspace {
	for i, workspace := range s.workspaces {
		if workspace.Name == workspaceName {
			return &s.workspaces[i]
		}
	}
	return nil
}

// ManualDeployInMode deploys a specific workspace in a specific mode immediately
func (s *Scheduler) ManualDeployInMode(workspaceName, mode string) error {
	// Find the workspace by name
	targetWorkspace := s.GetWorkspace(workspaceName)
	if targetWorkspace == nil {
		return fmt.Errorf("workspace '%s' not found in configuration", workspaceName)
	}

	// Check if workspace is enabled
	if !targetWorkspace.Config.Enabled {
		return fmt.Errorf("workspace '%s' is disabled in configuration", workspaceName)
	}

	// Validate mode if workspace uses mode scheduling
	if len(targetWorkspace.Config.ModeSchedules) > 0 {
		modeSchedules, err := targetWorkspace.Config.GetModeSchedules()
		if err != nil {
			return fmt.Errorf("invalid mode schedules for workspace '%s': %w", workspaceName, err)
		}

		// Check if requested mode is available
		if _, exists := modeSchedules[mode]; !exists {
			availableModes := make([]string, 0, len(modeSchedules))
			for availableMode := range modeSchedules {
				availableModes = append(availableModes, availableMode)
			}
			return fmt.Errorf("mode '%s' not available for workspace '%s'. Available modes: %v", mode, workspaceName, availableModes)
		}
	} else if targetWorkspace.Config.DeploySchedule != nil {
		// Traditional workspace - reject mode parameter
		return fmt.Errorf("workspace '%s' uses traditional scheduling. Use 'deploy' command without mode parameter", workspaceName)
	}

	workspaceState := s.state.GetWorkspaceState(workspaceName)

	// Check if workspace is currently busy
	if workspaceState.Status == StatusDeploying || workspaceState.Status == StatusDestroying {
		return fmt.Errorf("workspace '%s' is currently %s, cannot deploy", workspaceName, workspaceState.Status)
	}

	// Get current deployment mode
	currentMode := workspaceState.DeploymentMode
	if currentMode == mode && workspaceState.Status == StatusDeployed {
		fmt.Printf("Workspace '%s' is already deployed in '%s' mode.\n", workspaceName, mode)
		return nil
	}

	// Confirm mode change if already deployed in different mode
	if currentMode != "" && currentMode != mode && workspaceState.Status == StatusDeployed {
		fmt.Printf("Workspace '%s' is currently deployed in '%s' mode.\n", workspaceName, currentMode)
		fmt.Printf("Change to '%s' mode? (y/N): ", mode)
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			fmt.Println("Cancelled")
			return nil
		}
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	logging.LogSystemd("Manual deployment requested for workspace: %s in mode: %s", workspaceName, mode)

	// Set the deployment mode in state
	workspaceState.DeploymentMode = mode
	s.state.SetWorkspaceState(workspaceName, workspaceState)

	// Execute deployment directly (not in goroutine for immediate feedback)
	s.manualDeployWorkspaceInMode(*targetWorkspace, mode)

	// Save state after manual operation
	if err := s.SaveState(); err != nil {
		logging.LogSystemd("Error saving state after manual deploy: %v", err)
		return fmt.Errorf("deployment completed but failed to save state: %w", err)
	}

	return nil
}

// manualDeployWorkspace is similar to deployWorkspace but for manual operations
func (s *Scheduler) manualDeployWorkspace(workspace workspace.Workspace) {
	workspaceName := workspace.Name
	logging.LogWorkspaceOperation(workspaceName, "MANUAL DEPLOY", "Starting manual deployment")

	s.state.SetWorkspaceStatus(workspaceName, StatusDeploying)
	_ = s.SaveState()

	// Initialize OpenTofu client if not provided
	if s.client == nil {
		client, err := opentofu.New()
		if err != nil {
			logging.LogWorkspaceOperation(workspaceName, "MANUAL DEPLOY", "Failed to initialize OpenTofu client: %s", err.Error())
			s.state.SetWorkspaceError(workspaceName, true, fmt.Sprintf("Failed to initialize OpenTofu client: %s", err.Error()))
			return
		}
		s.client = client
	}

	if err := s.client.Deploy(&workspace); err != nil {
		// Log high-level failure to systemd
		logging.LogWorkspaceOperation(workspaceName, "MANUAL DEPLOY", "Failed: %s", getHighLevelError(err))

		// Log detailed error only to workspace file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogWorkspaceOnly(workspaceName, "MANUAL DEPLOY: Failed: %s", cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getWorkspaceLogFile(workspaceName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetWorkspaceError(workspaceName, true, err.Error())
	} else {
		logging.LogWorkspaceOperation(workspaceName, "MANUAL DEPLOY", "Successfully completed")
		s.state.SetWorkspaceStatus(workspaceName, StatusDeployed)
	}
}

// manualDeployWorkspaceInMode is similar to manualDeployWorkspace but deploys in a specific mode
func (s *Scheduler) manualDeployWorkspaceInMode(workspace workspace.Workspace, mode string) {
	workspaceName := workspace.Name
	logging.LogWorkspaceOperation(workspaceName, "MANUAL DEPLOY MODE", "Starting manual deployment in mode: %s", mode)

	s.state.SetWorkspaceStatus(workspaceName, StatusDeploying)
	_ = s.SaveState()

	// Initialize OpenTofu client if not provided
	if s.client == nil {
		client, err := opentofu.New()
		if err != nil {
			logging.LogWorkspaceOperation(workspaceName, "MANUAL DEPLOY MODE", "Failed to initialize OpenTofu client: %s", err.Error())
			s.state.SetWorkspaceError(workspaceName, true, fmt.Sprintf("Failed to initialize OpenTofu client: %s", err.Error()))
			return
		}
		s.client = client
	}

	if err := s.client.DeployInMode(&workspace, mode); err != nil {
		// Log high-level failure to systemd
		logging.LogWorkspaceOperation(workspaceName, "MANUAL DEPLOY MODE", "Failed in mode %s: %s", mode, getHighLevelError(err))

		// Log detailed error only to workspace file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogWorkspaceOnly(workspaceName, "MANUAL DEPLOY MODE (%s): Failed: %s", mode, cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getWorkspaceLogFile(workspaceName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetWorkspaceError(workspaceName, true, err.Error())
	} else {
		logging.LogWorkspaceOperation(workspaceName, "MANUAL DEPLOY MODE", "Successfully completed in mode: %s", mode)
		s.state.SetWorkspaceStatus(workspaceName, StatusDeployed)

		// Update deployment mode in state
		workspaceState := s.state.GetWorkspaceState(workspaceName)
		workspaceState.DeploymentMode = mode
		s.state.SetWorkspaceState(workspaceName, workspaceState)
	}
}

// manualDestroyWorkspace is similar to destroyWorkspace but for manual operations
func (s *Scheduler) manualDestroyWorkspace(workspace workspace.Workspace) {
	workspaceName := workspace.Name
	logging.LogWorkspaceOperation(workspaceName, "MANUAL DESTROY", "Starting manual destruction")

	s.state.SetWorkspaceStatus(workspaceName, StatusDestroying)
	_ = s.SaveState()

	// Initialize OpenTofu client if not provided
	if s.client == nil {
		client, err := opentofu.New()
		if err != nil {
			logging.LogWorkspaceOperation(workspaceName, "MANUAL DESTROY", "Failed to initialize OpenTofu client: %s", err.Error())
			s.state.SetWorkspaceError(workspaceName, false, fmt.Sprintf("Failed to initialize OpenTofu client: %s", err.Error()))
			return
		}
		s.client = client
	}

	if err := s.client.DestroyWorkspace(&workspace); err != nil {
		// Log high-level failure to systemd
		logging.LogWorkspaceOperation(workspaceName, "MANUAL DESTROY", "Failed: %s", getHighLevelError(err))

		// Log detailed error only to workspace file (strip ANSI colors)
		cleanError := stripANSIColors(err.Error())
		logging.LogWorkspaceOnly(workspaceName, "MANUAL DESTROY: Failed: %s", cleanError)

		// Add log file location reference to systemd logs for easier debugging
		logFile := s.getWorkspaceLogFile(workspaceName)
		logging.LogSystemd("For detailed error information see: %s", logFile)

		s.state.SetWorkspaceError(workspaceName, false, err.Error())
	} else {
		logging.LogWorkspaceOperation(workspaceName, "MANUAL DESTROY", "Successfully completed")
		s.state.SetWorkspaceStatus(workspaceName, StatusDestroyed)
	}
}

// ShowStatus displays the status of workspaces
func (s *Scheduler) ShowStatus(workspaceName string) error {
	if err := s.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	if err := s.LoadState(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	if workspaceName != "" {
		// Show specific workspace status
		workspace := s.findWorkspace(workspaceName)
		if workspace == nil {
			return fmt.Errorf("workspace '%s' not found", workspaceName)
		}
		s.printWorkspaceStatus(*workspace)
	} else {
		// Show all workspaces status
		fmt.Printf("%-15s %-12s %-20s %-20s %-10s\n", "WORKSPACE", "STATUS", "LAST DEPLOYED", "LAST DESTROYED", "ERRORS")
		fmt.Printf("%-15s %-12s %-20s %-20s %-10s\n", "-----------", "------", "-------------", "--------------", "------")

		for _, workspace := range s.workspaces {
			state := s.state.GetWorkspaceState(workspace.Name)
			s.printWorkspaceStatusLine(workspace, state)
		}
	}

	return nil
}

// ListWorkspaces displays all configured workspaces
func (s *Scheduler) ListWorkspaces() error {
	if err := s.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	fmt.Printf("%-15s %-8s %-30s %-30s\n", "WORKSPACE", "ENABLED", "DEPLOY SCHEDULE", "DESTROY SCHEDULE")
	fmt.Printf("%-15s %-8s %-30s %-30s\n", "-----------", "-------", "---------------", "----------------")

	for _, workspace := range s.workspaces {
		deploySchedules, _ := workspace.Config.GetDeploySchedules()
		destroySchedules, _ := workspace.Config.GetDestroySchedules()

		deploySchedule := formatSchedules(deploySchedules)
		destroySchedule := formatSchedules(destroySchedules)

		fmt.Printf("%-15s %-8t %-30s %-30s\n",
			workspace.Name,
			workspace.Config.Enabled,
			deploySchedule,
			destroySchedule)
	}

	return nil
}

// ShowLogs displays recent logs for an workspace
func (s *Scheduler) ShowLogs(workspaceName string) error {
	if err := s.LoadWorkspaces(); err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	workspace := s.findWorkspace(workspaceName)
	if workspace == nil {
		return fmt.Errorf("workspace '%s' not found", workspaceName)
	}

	logFile := s.getWorkspaceLogFile(workspaceName)

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Printf("No log file found for workspace '%s'\n", workspaceName)
		fmt.Printf("Expected location: %s\n", logFile)
		return nil
	}

	// Read and display the log file
	data, err := os.ReadFile(logFile)
	if err != nil {
		return fmt.Errorf("failed to read log file: %w", err)
	}

	fmt.Printf("=== Recent logs for workspace '%s' ===\n", workspaceName)
	fmt.Printf("Log file: %s\n\n", logFile)
	fmt.Print(string(data))

	return nil
}

// Helper methods for CLI commands

func (s *Scheduler) findWorkspace(name string) *workspace.Workspace {
	for _, workspace := range s.workspaces {
		if workspace.Name == name {
			return &workspace
		}
	}
	return nil
}

func (s *Scheduler) printWorkspaceStatus(workspace workspace.Workspace) {
	state := s.state.GetWorkspaceState(workspace.Name)

	deploySchedules, _ := workspace.Config.GetDeploySchedules()
	destroySchedules, _ := workspace.Config.GetDestroySchedules()

	// Use actual OpenTofu state as source of truth for deployment status
	actualStatus := workspace.GetDeploymentStatus()

	fmt.Printf("Workspace: %s\n", workspace.Name)
	fmt.Printf("Status: %s\n", actualStatus)
	fmt.Printf("Enabled: %t\n", workspace.Config.Enabled)
	fmt.Printf("Deploy Schedule: %s\n", formatSchedules(deploySchedules))
	fmt.Printf("Destroy Schedule: %s\n", formatSchedules(destroySchedules))

	// Use filesystem timestamps as more accurate source, fall back to managed state
	if stateChangeTime := workspace.GetLastStateChangeTime(); stateChangeTime != nil {
		if actualStatus == "deployed" {
			fmt.Printf("Last Deployed: %s\n", stateChangeTime.Format("2006-01-02 15:04:05"))
			if state.LastDestroyed != nil {
				fmt.Printf("Last Destroyed: %s\n", state.LastDestroyed.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("Last Destroyed: Never\n")
			}
		} else {
			if state.LastDeployed != nil {
				fmt.Printf("Last Deployed: %s\n", state.LastDeployed.Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("Last Deployed: Never\n")
			}
			fmt.Printf("Last Destroyed: %s\n", stateChangeTime.Format("2006-01-02 15:04:05"))
		}
	} else {
		// Fall back to managed state timestamps
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

	logFile := s.getWorkspaceLogFile(workspace.Name)
	fmt.Printf("Log File: %s\n", logFile)
}

func (s *Scheduler) printWorkspaceStatusLine(workspace workspace.Workspace, state *WorkspaceState) {
	// Use actual OpenTofu state as source of truth for deployment status
	actualStatus := workspace.GetDeploymentStatus()

	// Use filesystem timestamps as more accurate source, fall back to managed state
	lastDeployed := "Never"
	lastDestroyed := "Never"

	if stateChangeTime := workspace.GetLastStateChangeTime(); stateChangeTime != nil {
		if actualStatus == "deployed" {
			lastDeployed = stateChangeTime.Format("2006-01-02 15:04")
			// Use managed state for last destroyed if available
			if state.LastDestroyed != nil {
				lastDestroyed = state.LastDestroyed.Format("2006-01-02 15:04")
			}
		} else {
			lastDestroyed = stateChangeTime.Format("2006-01-02 15:04")
			// Use managed state for last deployed if available
			if state.LastDeployed != nil {
				lastDeployed = state.LastDeployed.Format("2006-01-02 15:04")
			}
		}
	} else {
		// Fall back to managed state timestamps
		if state.LastDeployed != nil {
			lastDeployed = state.LastDeployed.Format("2006-01-02 15:04")
		}
		if state.LastDestroyed != nil {
			lastDestroyed = state.LastDestroyed.Format("2006-01-02 15:04")
		}
	}

	errors := "None"
	if state.LastDeployError != "" || state.LastDestroyError != "" {
		errors = "Yes"
	}

	fmt.Printf("%-15s %-12s %-20s %-20s %-10s\n",
		workspace.Name,
		actualStatus,
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
