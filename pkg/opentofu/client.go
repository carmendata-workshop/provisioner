package opentofu

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"provisioner/pkg/template"
	"provisioner/pkg/workspace"

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

	// Include detailed output in error for workspace logs
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

	// Include detailed output in error for workspace logs
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

	// Include detailed output in error for workspace logs
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

func (c *Client) PlanWithMode(workingDir, mode string) error {
	cmd := exec.Command(c.binaryPath, "plan", "-var", fmt.Sprintf("deployment_mode=%s", mode))
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Include detailed output in error for workspace logs
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

func (c *Client) ApplyWithMode(workingDir, mode string) error {
	cmd := exec.Command(c.binaryPath, "apply", "-auto-approve", "-var", fmt.Sprintf("deployment_mode=%s", mode))
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Include detailed output in error for workspace logs
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

	// Include detailed output in error for workspace logs
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

func (c *Client) Deploy(ws *workspace.Workspace) error {
	// Create persistent working directory based on workspace name
	stateDir := getStateDir()
	workingDir := filepath.Join(stateDir, "deployments", ws.Name)

	// Ensure working directory exists
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Copy workspace template files to working directory (preserving state files)
	if err := copyWorkspaceTemplateFiles(ws, workingDir); err != nil {
		return fmt.Errorf("failed to copy workspace files: %w", err)
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

func (c *Client) DeployInMode(ws *workspace.Workspace, mode string) error {
	// Create persistent working directory based on workspace name
	stateDir := getStateDir()
	workingDir := filepath.Join(stateDir, "deployments", ws.Name)

	// Ensure working directory exists
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Copy workspace template files to working directory (preserving state files)
	if err := copyWorkspaceTemplateFiles(ws, workingDir); err != nil {
		return fmt.Errorf("failed to copy workspace files: %w", err)
	}

	// Run OpenTofu sequence: init → plan → apply with mode variable
	if err := c.Init(workingDir); err != nil {
		return fmt.Errorf("init failed: %w", err)
	}

	if err := c.PlanWithMode(workingDir, mode); err != nil {
		return fmt.Errorf("plan failed: %w", err)
	}

	if err := c.ApplyWithMode(workingDir, mode); err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	return nil
}

func (c *Client) DestroyWorkspace(ws *workspace.Workspace) error {
	// Use persistent working directory based on workspace name
	stateDir := getStateDir()
	workingDir := filepath.Join(stateDir, "deployments", ws.Name)

	// Ensure working directory exists
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Copy workspace template files to working directory (preserving state files)
	if err := copyWorkspaceTemplateFiles(ws, workingDir); err != nil {
		return fmt.Errorf("failed to copy workspace files: %w", err)
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

// copyWorkspaceTemplateFiles copies template files to working directory while preserving OpenTofu state
func copyWorkspaceTemplateFiles(ws *workspace.Workspace, workingDir string) error {
	// Determine source directory for templates
	srcDir := ""
	templateName := ""
	templateHash := ""

	if ws.IsUsingTemplate() {
		// Using a template reference - copy from template directory
		srcDir = ws.GetTemplateDir()
		if srcDir == "" {
			return fmt.Errorf("template directory not found for template '%s'", ws.Config.Template)
		}
		templateName = ws.Config.Template

		// Get template hash for change tracking
		if hash, err := getTemplateHash(ws.Config.Template); err == nil {
			templateHash = hash
		}
	} else {
		// Using local files - copy from workspace directory
		srcDir = ws.Path
	}

	// Copy template files while preserving state
	if err := copyDirectoryFiles(srcDir, workingDir); err != nil {
		return err
	}

	// Update deployment metadata with template information
	if templateName != "" {
		stateDir := getStateDir()
		if err := workspace.UpdateDeploymentTemplate(stateDir, ws.Name, templateName, templateHash); err != nil {
			// Log warning but don't fail deployment
			fmt.Printf("Warning: failed to update deployment template metadata: %v\n", err)
		}
	}

	return nil
}

// copyDirectoryFiles copies files from src to dst while preserving OpenTofu state and workspace files
func copyDirectoryFiles(src, dst string) error {
	// Clean working directory first (preserve important files)
	if err := cleanWorkingDirectory(dst); err != nil {
		return fmt.Errorf("failed to clean working directory: %w", err)
	}

	// Copy fresh template files
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		// Skip files that should not be copied from template
		if shouldSkipTemplateFile(relPath) {
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

// shouldSkipTemplateFile determines if a file should be skipped when copying from template
func shouldSkipTemplateFile(relPath string) bool {
	// Don't copy state files from template (shouldn't exist but be safe)
	return shouldSkipFile(relPath)
}

// shouldPreserveFile determines if a file should be preserved during working directory cleanup
func shouldPreserveFile(relPath string) bool {
	// Preserve OpenTofu state and cache files
	if shouldSkipFile(relPath) {
		return true
	}

	// Preserve workspace-specific variable files
	if strings.HasSuffix(relPath, ".tfvars") || strings.HasSuffix(relPath, ".tfvars.json") {
		return true
	}

	// Preserve auto-generated variable files
	if relPath == "terraform.tfvars" || relPath == "terraform.tfvars.json" {
		return true
	}

	// Preserve provisioner metadata
	if relPath == ".provisioner-metadata.json" {
		return true
	}

	// Preserve lock files
	if relPath == ".terraform.lock.hcl" {
		return true
	}

	return false
}

// cleanWorkingDirectory removes stale files while preserving important workspace-specific files
func cleanWorkingDirectory(workingDir string) error {
	// Check if directory exists
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist yet, nothing to clean
	}

	// Read directory contents
	entries, err := os.ReadDir(workingDir)
	if err != nil {
		return fmt.Errorf("failed to read working directory: %w", err)
	}

	// Remove files that should not be preserved
	for _, entry := range entries {
		relPath := entry.Name()

		// Preserve important files
		if shouldPreserveFile(relPath) {
			continue
		}

		// Remove stale template files
		fullPath := filepath.Join(workingDir, relPath)
		if err := os.RemoveAll(fullPath); err != nil {
			return fmt.Errorf("failed to remove stale file %s: %w", relPath, err)
		}
	}

	return nil
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

	// Try to create system state directory (in case this is first run after installation)
	if err := os.MkdirAll("/var/lib/provisioner", 0755); err == nil {
		return "/var/lib/provisioner"
	}

	// Fall back to development default
	return "state"
}

// GetWorkingDir returns the working directory for a workspace
func GetWorkingDir(wsName string) string {
	stateDir := getStateDir()
	return filepath.Join(stateDir, "deployments", wsName)
}

// WorkingDirExists checks if a working directory exists for a workspace
func WorkingDirExists(wsName string) bool {
	workingDir := GetWorkingDir(wsName)
	_, err := os.Stat(workingDir)
	return err == nil
}

// CleanWorkingDir removes the working directory for a workspace
func CleanWorkingDir(wsName string) error {
	workingDir := GetWorkingDir(wsName)
	return os.RemoveAll(workingDir)
}

// getTemplateHash gets the content hash for a template
func getTemplateHash(templateName string) (string, error) {
	templatesDir := getTemplatesDir()
	manager := template.NewManager(templatesDir)
	return manager.GetTemplateContentHash(templateName)
}

// getTemplatesDir returns the templates directory path
func getTemplatesDir() string {
	stateDir := getStateDir()
	return filepath.Join(stateDir, "templates")
}
