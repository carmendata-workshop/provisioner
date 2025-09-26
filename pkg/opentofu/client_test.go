package opentofu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanWorkingDirectory(t *testing.T) {
	// Create temporary working directory
	tempDir, err := os.MkdirTemp("", "test-working-dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create files that should be preserved
	preservedFiles := []string{
		"terraform.tfstate",
		"terraform.tfstate.backup",
		".terraform.lock.hcl",
		"workspace.tfvars",
		"custom.tfvars.json",
		"terraform.tfvars",
		".provisioner-metadata.json",
	}

	for _, file := range preservedFiles {
		filePath := filepath.Join(tempDir, file)
		if err := os.WriteFile(filePath, []byte("preserved content"), 0644); err != nil {
			t.Fatalf("Failed to create preserved file %s: %v", file, err)
		}
	}

	// Create .terraform directory (should be preserved)
	terraformDir := filepath.Join(tempDir, ".terraform")
	if err := os.MkdirAll(terraformDir, 0755); err != nil {
		t.Fatalf("Failed to create .terraform dir: %v", err)
	}
	providerFile := filepath.Join(terraformDir, "providers", "local.json")
	if err := os.MkdirAll(filepath.Dir(providerFile), 0755); err != nil {
		t.Fatalf("Failed to create provider dir: %v", err)
	}
	if err := os.WriteFile(providerFile, []byte("provider cache"), 0644); err != nil {
		t.Fatalf("Failed to create provider file: %v", err)
	}

	// Create files that should be removed (stale template files)
	staleFiles := []string{
		"main.tf",
		"variables.tf",
		"outputs.tf",
		"old-module.tf",
		"README.md",
	}

	for _, file := range staleFiles {
		filePath := filepath.Join(tempDir, file)
		if err := os.WriteFile(filePath, []byte("stale content"), 0644); err != nil {
			t.Fatalf("Failed to create stale file %s: %v", file, err)
		}
	}

	// Run cleanup
	err = cleanWorkingDirectory(tempDir)
	if err != nil {
		t.Fatalf("cleanWorkingDirectory failed: %v", err)
	}

	// Verify preserved files still exist
	for _, file := range preservedFiles {
		filePath := filepath.Join(tempDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Preserved file %s was incorrectly removed", file)
		}
	}

	// Verify .terraform directory still exists
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		t.Error(".terraform directory was incorrectly removed")
	}
	if _, err := os.Stat(providerFile); os.IsNotExist(err) {
		t.Error("Provider cache file was incorrectly removed")
	}

	// Verify stale files were removed
	for _, file := range staleFiles {
		filePath := filepath.Join(tempDir, file)
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("Stale file %s was not removed", file)
		}
	}
}

func TestCopyDirectoryFilesWithCleanup(t *testing.T) {
	// Create source directory (template)
	srcDir, err := os.MkdirTemp("", "test-template")
	if err != nil {
		t.Fatalf("Failed to create source dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// Create destination directory (working directory)
	dstDir, err := os.MkdirTemp("", "test-working")
	if err != nil {
		t.Fatalf("Failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	// Create new template files
	templateFiles := map[string]string{
		"main.tf":      "# New main.tf content",
		"variables.tf": "# New variables.tf content",
		"outputs.tf":   "# New outputs.tf content",
	}

	for file, content := range templateFiles {
		filePath := filepath.Join(srcDir, file)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create template file %s: %v", file, err)
		}
	}

	// Pre-populate working directory with old files
	oldFiles := map[string]string{
		"main.tf":                    "# Old main.tf content",
		"old-variables.tf":           "# This file was deleted from template",
		"removed-module.tf":          "# This module was removed",
		"terraform.tfstate":          "# Terraform state - should be preserved",
		"workspace.tfvars":           "# Workspace vars - should be preserved",
		".provisioner-metadata.json": "# Metadata - should be preserved",
	}

	for file, content := range oldFiles {
		filePath := filepath.Join(dstDir, file)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create old file %s: %v", file, err)
		}
	}

	// Run copy operation
	err = copyDirectoryFiles(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDirectoryFiles failed: %v", err)
	}

	// Verify new template files were copied
	for file, expectedContent := range templateFiles {
		filePath := filepath.Join(dstDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("New template file %s not found: %v", file, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("Template file %s has wrong content. Expected: %s, Got: %s",
				file, expectedContent, string(content))
		}
	}

	// Verify preserved files still exist with original content
	preservedFiles := []string{
		"terraform.tfstate",
		"workspace.tfvars",
		".provisioner-metadata.json",
	}

	for _, file := range preservedFiles {
		filePath := filepath.Join(dstDir, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Preserved file %s not found: %v", file, err)
			continue
		}
		expectedContent := oldFiles[file]
		if string(content) != expectedContent {
			t.Errorf("Preserved file %s was modified. Expected: %s, Got: %s",
				file, expectedContent, string(content))
		}
	}

	// Verify stale template files were removed
	staleFiles := []string{
		"old-variables.tf",
		"removed-module.tf",
	}

	for _, file := range staleFiles {
		filePath := filepath.Join(dstDir, file)
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("Stale file %s was not removed", file)
		}
	}
}

func TestShouldPreserveFile(t *testing.T) {
	testCases := []struct {
		file     string
		preserve bool
	}{
		// Should preserve
		{"terraform.tfstate", true},
		{"terraform.tfstate.backup", true},
		{".terraform.lock.hcl", true},
		{"workspace.tfvars", true},
		{"dev.tfvars", true},
		{"staging.tfvars.json", true},
		{"terraform.tfvars", true},
		{"terraform.tfvars.json", true},
		{".provisioner-metadata.json", true},
		{".terraform/providers/local.json", true},

		// Should not preserve (stale template files)
		{"main.tf", false},
		{"variables.tf", false},
		{"outputs.tf", false},
		{"modules.tf", false},
		{"README.md", false},
		{"config.json", false},
		{"old-file.tf", false},
	}

	for _, tc := range testCases {
		result := shouldPreserveFile(tc.file)
		if result != tc.preserve {
			t.Errorf("shouldPreserveFile(%s) = %v, expected %v", tc.file, result, tc.preserve)
		}
	}
}

func TestCleanWorkingDirectoryNonExistent(t *testing.T) {
	// Test cleaning a non-existent directory (should not error)
	nonExistentDir := "/tmp/does-not-exist-12345"
	err := cleanWorkingDirectory(nonExistentDir)
	if err != nil {
		t.Errorf("cleanWorkingDirectory on non-existent directory should not error, got: %v", err)
	}
}
