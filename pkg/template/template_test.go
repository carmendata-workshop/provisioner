package template

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTemplateManager(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "provisioner-template-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewManager(tempDir)

	// Test adding a template
	err = manager.AddTemplate("test-template", "https://github.com/test/repo", "path/to/template", "main", "Test template")
	if err != nil {
		t.Fatalf("Failed to add template: %v", err)
	}

	// Test getting the template
	template, err := manager.GetTemplate("test-template")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if template.Name != "test-template" {
		t.Errorf("Expected name 'test-template', got '%s'", template.Name)
	}
	if template.SourceURL != "https://github.com/test/repo" {
		t.Errorf("Expected source URL 'https://github.com/test/repo', got '%s'", template.SourceURL)
	}
	if template.SourcePath != "path/to/template" {
		t.Errorf("Expected source path 'path/to/template', got '%s'", template.SourcePath)
	}
	if template.SourceRef != "main" {
		t.Errorf("Expected source ref 'main', got '%s'", template.SourceRef)
	}

	// Test listing templates
	templates, err := manager.ListTemplates()
	if err != nil {
		t.Fatalf("Failed to list templates: %v", err)
	}

	if len(templates) != 1 {
		t.Errorf("Expected 1 template, got %d", len(templates))
	}

	// Test template validation
	err = manager.ValidateTemplate("test-template")
	if err != nil {
		t.Errorf("Template validation failed: %v", err)
	}

	// Test removing template
	err = manager.RemoveTemplate("test-template", true)
	if err != nil {
		t.Fatalf("Failed to remove template: %v", err)
	}

	// Verify template is removed
	templates, err = manager.ListTemplates()
	if err != nil {
		t.Fatalf("Failed to list templates after removal: %v", err)
	}

	if len(templates) != 0 {
		t.Errorf("Expected 0 templates after removal, got %d", len(templates))
	}
}

func TestTemplateRegistry(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "provisioner-registry-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewManager(tempDir)

	// Test empty registry
	registry, err := manager.LoadRegistry()
	if err != nil {
		t.Fatalf("Failed to load empty registry: %v", err)
	}

	if len(registry.Templates) != 0 {
		t.Errorf("Expected empty registry, got %d templates", len(registry.Templates))
	}

	// Add a template to registry
	template := Template{
		Name:        "test",
		SourceURL:   "https://github.com/test/repo",
		SourcePath:  "templates/web",
		SourceRef:   "v1.0.0",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Description: "Test template",
		Version:     "1.0.0",
	}

	registry.Templates["test"] = template

	// Save registry
	err = manager.SaveRegistry(registry)
	if err != nil {
		t.Fatalf("Failed to save registry: %v", err)
	}

	// Load registry again
	loadedRegistry, err := manager.LoadRegistry()
	if err != nil {
		t.Fatalf("Failed to reload registry: %v", err)
	}

	if len(loadedRegistry.Templates) != 1 {
		t.Errorf("Expected 1 template in loaded registry, got %d", len(loadedRegistry.Templates))
	}

	loadedTemplate := loadedRegistry.Templates["test"]
	if loadedTemplate.Name != template.Name {
		t.Errorf("Template name mismatch: expected '%s', got '%s'", template.Name, loadedTemplate.Name)
	}
	if loadedTemplate.SourceURL != template.SourceURL {
		t.Errorf("Template source URL mismatch: expected '%s', got '%s'", template.SourceURL, loadedTemplate.SourceURL)
	}
}

func TestTemplateDefaults(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "provisioner-defaults-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewManager(tempDir)

	// Test adding template with empty ref (should default to "main")
	err = manager.AddTemplate("default-ref", "https://github.com/test/repo", "", "", "")
	if err != nil {
		t.Fatalf("Failed to add template with defaults: %v", err)
	}

	template, err := manager.GetTemplate("default-ref")
	if err != nil {
		t.Fatalf("Failed to get template: %v", err)
	}

	if template.SourceRef != "main" {
		t.Errorf("Expected default ref 'main', got '%s'", template.SourceRef)
	}
}

func TestTemplatePaths(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "provisioner-paths-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := NewManager(tempDir)

	templateName := "path-test"
	expectedPath := filepath.Join(tempDir, templateName)

	actualPath := manager.GetTemplatePath(templateName)
	if actualPath != expectedPath {
		t.Errorf("Expected template path '%s', got '%s'", expectedPath, actualPath)
	}
}
