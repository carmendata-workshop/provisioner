package scheduler

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"environment-scheduler/pkg/environment"
	"environment-scheduler/pkg/opentofu"
)

type Scheduler struct {
	environments []environment.Environment
	state        *State
	client       opentofu.TofuClient
	statePath    string
	stopChan     chan bool
}

func New() *Scheduler {
	stateDir := os.Getenv("PROVISIONER_STATE_DIR")
	if stateDir == "" {
		stateDir = "state"
	}

	return &Scheduler{
		statePath: filepath.Join(stateDir, "scheduler.json"),
		stopChan:  make(chan bool),
	}
}

func NewWithClient(client opentofu.TofuClient) *Scheduler {
	stateDir := os.Getenv("PROVISIONER_STATE_DIR")
	if stateDir == "" {
		stateDir = "state"
	}

	return &Scheduler{
		client:    client,
		statePath: filepath.Join(stateDir, "scheduler.json"),
		stopChan:  make(chan bool),
	}
}

func (s *Scheduler) LoadEnvironments() error {
	configDir := os.Getenv("PROVISIONER_CONFIG_DIR")
	if configDir == "" {
		configDir = "."
	}
	environmentsDir := filepath.Join(configDir, "environments")

	environments, err := environment.LoadEnvironments(environmentsDir)
	if err != nil {
		return fmt.Errorf("failed to load environments: %w", err)
	}

	s.environments = environments
	log.Printf("Loaded %d environments", len(s.environments))

	for _, env := range s.environments {
		log.Printf("Environment: %s (deploy: %s, destroy: %s)",
			env.Config.Name,
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

	for _, env := range s.environments {
		s.checkEnvironmentSchedules(env, now)
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

	// Check deploy schedule
	deploySchedule, err := ParseCron(env.Config.DeploySchedule)
	if err != nil {
		log.Printf("Invalid deploy schedule for %s: %v", env.Config.Name, err)
	} else if deploySchedule.ShouldRun(now) && envState.Status != StatusDeployed {
		log.Printf("Deploying environment %s", env.Config.Name)
		go s.deployEnvironment(env)
	}

	// Check destroy schedule
	destroySchedule, err := ParseCron(env.Config.DestroySchedule)
	if err != nil {
		log.Printf("Invalid destroy schedule for %s: %v", env.Config.Name, err)
	} else if destroySchedule.ShouldRun(now) && envState.Status != StatusDestroyed {
		log.Printf("Destroying environment %s", env.Config.Name)
		go s.destroyEnvironment(env)
	}
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
