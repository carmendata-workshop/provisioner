package opentofu

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/opentofu/tofudl"
)

type Client struct {
	binaryPath string
}

func New() (*Client, error) {
	// First try to find tofu in PATH
	if binaryPath, err := exec.LookPath("tofu"); err == nil {
		return &Client{binaryPath: binaryPath}, nil
	}

	// Fall back to downloading with TofuDL
	downloader, err := tofudl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create downloader: %w", err)
	}

	// Download the binary as bytes
	binaryData, err := downloader.Download(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to download OpenTofu: %w", err)
	}

	// Create a temporary file for the binary
	tmpFile, err := os.CreateTemp("", "tofu-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Write binary data to file
	if _, err := tmpFile.Write(binaryData); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write binary: %w", err)
	}

	_ = tmpFile.Close()

	// Make it executable
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		_ = os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to make binary executable: %w", err)
	}

	return &Client{binaryPath: tmpFile.Name()}, nil
}

func (c *Client) Init(workingDir string) error {
	cmd := exec.Command(c.binaryPath, "init")
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Include detailed output in error for environment logs
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w\n\nDetailed output:\n%s", err, stderr.String())
		}
		if stdout.Len() > 0 {
			return fmt.Errorf("%w\n\nDetailed output:\n%s", err, stdout.String())
		}
	}

	return err
}

func (c *Client) Plan(workingDir string) error {
	cmd := exec.Command(c.binaryPath, "plan")
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Include detailed output in error for environment logs
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w\n\nDetailed output:\n%s", err, stderr.String())
		}
		if stdout.Len() > 0 {
			return fmt.Errorf("%w\n\nDetailed output:\n%s", err, stdout.String())
		}
	}

	return err
}

func (c *Client) Apply(workingDir string) error {
	cmd := exec.Command(c.binaryPath, "apply", "-auto-approve")
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Include detailed output in error for environment logs
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w\n\nDetailed output:\n%s", err, stderr.String())
		}
		if stdout.Len() > 0 {
			return fmt.Errorf("%w\n\nDetailed output:\n%s", err, stdout.String())
		}
	}

	return err
}

func (c *Client) Destroy(workingDir string) error {
	cmd := exec.Command(c.binaryPath, "destroy", "-auto-approve")
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Include detailed output in error for environment logs
	if err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w\n\nDetailed output:\n%s", err, stderr.String())
		}
		if stdout.Len() > 0 {
			return fmt.Errorf("%w\n\nDetailed output:\n%s", err, stdout.String())
		}
	}

	return err
}

func (c *Client) Deploy(environmentPath string) error {
	// Create persistent working directory based on environment name
	envName := filepath.Base(environmentPath)
	stateDir := getStateDir()
	workingDir := filepath.Join(stateDir, envName)

	// Ensure working directory exists
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Copy environment template files to working directory (preserving state files)
	if err := copyEnvironmentFiles(environmentPath, workingDir); err != nil {
		return fmt.Errorf("failed to copy environment files: %w", err)
	}

	// Run OpenTofu sequence: init → plan → apply
	if err := c.Init(workingDir); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	if err := c.Plan(workingDir); err != nil {
		return fmt.Errorf("plan failed: %w", err)
	}

	if err := c.Apply(workingDir); err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	return nil
}

func (c *Client) DestroyEnvironment(environmentPath string) error {
	// Use persistent working directory based on environment name
	envName := filepath.Base(environmentPath)
	stateDir := getStateDir()
	workingDir := filepath.Join(stateDir, envName)

	// Ensure working directory exists
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Copy environment template files to working directory (preserving state files)
	if err := copyEnvironmentFiles(environmentPath, workingDir); err != nil {
		return fmt.Errorf("failed to copy environment files: %w", err)
	}

	// Run OpenTofu sequence: init → destroy
	if err := c.Init(workingDir); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	if err := c.Destroy(workingDir); err != nil {
		return fmt.Errorf("destroy failed: %w", err)
	}

	return nil
}

// copyEnvironmentFiles copies template files while preserving OpenTofu state files
func copyEnvironmentFiles(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		// Skip OpenTofu state and cache files to preserve state
		if shouldSkipFile(relPath) {
			return nil
		}

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// shouldSkipFile determines if a file should be skipped during copy to preserve OpenTofu state
func shouldSkipFile(relPath string) bool {
	// Skip OpenTofu state files
	if relPath == "terraform.tfstate" || relPath == "terraform.tfstate.backup" {
		return true
	}
	// Skip .terraform directory (provider cache, etc.)
	if relPath == ".terraform" || strings.HasPrefix(relPath, ".terraform/") {
		return true
	}
	// Skip plan files
	if strings.HasSuffix(relPath, ".tfplan") {
		return true
	}
	return false
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

	// Try to create system state directory (in case this is first run after installation)
	if err := os.MkdirAll("/var/lib/provisioner", 0755); err == nil {
		return "/var/lib/provisioner"
	}

	// Fall back to development default
	return "state"
}
