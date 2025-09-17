package scheduler

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"provisioner/pkg/environment"
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

	log.Printf("Loaded %d environments (%d enabled, %d disabled)", len(s.environments), enabledCount, len(s.environments)-enabledCount)

	for _, env := range s.environments {
		status := "disabled"
		if env.Config.Enabled {
			status = "enabled"
		}
		log.Printf("Environment: %s (%s) - deploy: %s, destroy: %s",
			env.Config.Name,
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
	log.Printf("State loaded with %d environment records", len(s.state.Environments))
	return nil
}

func (s *Scheduler) SaveState() error {
	if s.state == nil {
		return fmt.Errorf("no state to save")
	}

	return s.state.SaveState(s.statePath)
}

func (s *Scheduler) Start() {
	log.Println("Starting scheduler loop...")

	// Initialize OpenTofu client if not provided
	if s.client == nil {
		client, err := opentofu.New()
		if err != nil {
			log.Printf("Failed to initialize OpenTofu client: %v", err)
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
			log.Println("Scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopChan)
}

func (s *Scheduler) checkSchedules() {
	now := time.Now()
	log.Printf("Checking schedules at %s", now.Format("2006-01-02 15:04:05"))

	// Check for configuration changes every 30 seconds
	if now.Sub(s.lastConfigCheck) > 30*time.Second {
		if s.hasConfigChanged() {
			log.Printf("Configuration changes detected, reloading environments...")
			if err := s.LoadEnvironments(); err != nil {
				log.Printf("Error reloading environments: %v", err)
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
		log.Printf("Error saving state: %v", err)
	}
}

func (s *Scheduler) checkEnvironmentSchedules(env environment.Environment, now time.Time) {
	envState := s.state.GetEnvironmentState(env.Config.Name)

	// Skip if environment is currently being deployed or destroyed
	if envState.Status == StatusDeploying || envState.Status == StatusDestroying {
		log.Printf("Environment %s is busy (%s), skipping", env.Config.Name, envState.Status)
		return
	}

	// Check deploy schedules
	deploySchedules, err := env.Config.GetDeploySchedules()
	if err != nil {
		log.Printf("Invalid deploy schedule for %s: %v", env.Config.Name, err)
	} else if s.shouldRunAnySchedule(deploySchedules, now) && envState.Status != StatusDeployed {
		log.Printf("Deploying environment %s", env.Config.Name)
		go s.deployEnvironment(env)
	}

	// Check destroy schedules
	destroySchedules, err := env.Config.GetDestroySchedules()
	if err != nil {
		log.Printf("Invalid destroy schedule for %s: %v", env.Config.Name, err)
	} else if s.shouldRunAnySchedule(destroySchedules, now) && envState.Status != StatusDestroyed {
		log.Printf("Destroying environment %s", env.Config.Name)
		go s.destroyEnvironment(env)
	}
}

// shouldRunAnySchedule checks if any of the provided schedules should run at the given time
func (s *Scheduler) shouldRunAnySchedule(schedules []string, now time.Time) bool {
	for _, scheduleStr := range schedules {
		schedule, err := ParseCron(scheduleStr)
		if err != nil {
			log.Printf("Failed to parse schedule '%s': %v", scheduleStr, err)
			continue
		}
		if schedule.ShouldRun(now) {
			return true
		}
	}
	return false
}

func (s *Scheduler) deployEnvironment(env environment.Environment) {
	envName := env.Config.Name
	log.Printf("Starting deployment of %s", envName)

	s.state.SetEnvironmentStatus(envName, StatusDeploying)
	_ = s.SaveState()

	if err := s.client.Deploy(env.Path); err != nil {
		log.Printf("Failed to deploy %s: %v", envName, err)
		s.state.SetEnvironmentError(envName, true, err.Error())
	} else {
		log.Printf("Successfully deployed %s", envName)
		s.state.SetEnvironmentStatus(envName, StatusDeployed)
	}

	_ = s.SaveState()
}

func (s *Scheduler) destroyEnvironment(env environment.Environment) {
	envName := env.Config.Name
	log.Printf("Starting destruction of %s", envName)

	s.state.SetEnvironmentStatus(envName, StatusDestroying)
	_ = s.SaveState()

	if err := s.client.DestroyEnvironment(env.Path); err != nil {
		log.Printf("Failed to destroy %s: %v", envName, err)
		s.state.SetEnvironmentError(envName, false, err.Error())
	} else {
		log.Printf("Successfully destroyed %s", envName)
		s.state.SetEnvironmentStatus(envName, StatusDestroyed)
	}

	_ = s.SaveState()
}

// hasConfigChanged checks if any configuration files have been modified
func (s *Scheduler) hasConfigChanged() bool {
	environmentsDir := filepath.Join(s.configDir, "environments")

	var hasChanged bool

	// Walk through all environment directories
	err := filepath.Walk(environmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on error
		}

		// Check config.json and .tf files
		if filepath.Base(path) == "config.json" || filepath.Ext(path) == ".tf" {
			if info.ModTime().After(s.lastConfigCheck) {
				log.Printf("Config file changed: %s (modified: %s)", path, info.ModTime().Format("2006-01-02 15:04:05"))
				hasChanged = true
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Error walking config directory: %v", err)
	}

	return hasChanged
}
