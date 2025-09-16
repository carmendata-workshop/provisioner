package opentofu

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running: tofu init in %s", workingDir)
	return cmd.Run()
}

func (c *Client) Plan(workingDir string) error {
	cmd := exec.Command(c.binaryPath, "plan")
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running: tofu plan in %s", workingDir)
	return cmd.Run()
}

func (c *Client) Apply(workingDir string) error {
	cmd := exec.Command(c.binaryPath, "apply", "-auto-approve")
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running: tofu apply in %s", workingDir)
	return cmd.Run()
}

func (c *Client) Destroy(workingDir string) error {
	cmd := exec.Command(c.binaryPath, "destroy", "-auto-approve")
	cmd.Dir = workingDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("Running: tofu destroy in %s", workingDir)
	return cmd.Run()
}

func (c *Client) Deploy(environmentPath string) error {
	// Create temporary working directory
	workingDir, err := os.MkdirTemp("", "tofu-deploy-*")
	if err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workingDir) }()

	// Copy environment files to working directory
	if err := copyDir(environmentPath, workingDir); err != nil {
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
	// Create temporary working directory
	workingDir, err := os.MkdirTemp("", "tofu-destroy-*")
	if err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workingDir) }()

	// Copy environment files to working directory
	if err := copyDir(environmentPath, workingDir); err != nil {
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

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

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