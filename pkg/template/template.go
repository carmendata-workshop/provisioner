package template

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Template struct {
	Name         string    `json:"name"`
	SourceURL    string    `json:"source_url"`
	SourcePath   string    `json:"source_path,omitempty"`
	SourceRef    string    `json:"source_ref"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Description  string    `json:"description,omitempty"`
	Version      string    `json:"version,omitempty"`
	ContentHash  string    `json:"content_hash,omitempty"`
}

type Registry struct {
	Templates map[string]Template `json:"templates"`
}

type Manager struct {
	templatesDir string
	registryPath string
}

func NewManager(templatesDir string) *Manager {
	registryPath := filepath.Join(templatesDir, "registry.json")
	return &Manager{
		templatesDir: templatesDir,
		registryPath: registryPath,
	}
}

func (m *Manager) LoadRegistry() (*Registry, error) {
	registry := &Registry{
		Templates: make(map[string]Template),
	}

	// Create templates directory if it doesn't exist
	if err := os.MkdirAll(m.templatesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create templates directory: %w", err)
	}

	// Return empty registry if file doesn't exist
	if _, err := os.Stat(m.registryPath); os.IsNotExist(err) {
		return registry, nil
	}

	data, err := os.ReadFile(m.registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry file: %w", err)
	}

	if err := json.Unmarshal(data, registry); err != nil {
		return nil, fmt.Errorf("failed to unmarshal registry: %w", err)
	}

	return registry, nil
}

func (m *Manager) SaveRegistry(registry *Registry) error {
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(m.registryPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	return nil
}

func (m *Manager) AddTemplate(name, sourceURL, sourcePath, sourceRef, description string) error {
	registry, err := m.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Check if template already exists
	if _, exists := registry.Templates[name]; exists {
		return fmt.Errorf("template '%s' already exists", name)
	}

	// Default ref to 'main' if not specified
	if sourceRef == "" {
		sourceRef = "main"
	}

	template := Template{
		Name:        name,
		SourceURL:   sourceURL,
		SourcePath:  sourcePath,
		SourceRef:   sourceRef,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Description: description,
	}

	// Download the template and calculate content hash
	if err := m.downloadTemplate(template); err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}

	// Calculate content hash for change detection
	contentHash, err := m.calculateTemplateHash(template.Name)
	if err != nil {
		return fmt.Errorf("failed to calculate template hash: %w", err)
	}
	template.ContentHash = contentHash

	// Add to registry
	registry.Templates[name] = template

	// Save registry
	if err := m.SaveRegistry(registry); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

func (m *Manager) RemoveTemplate(name string, force bool) error {
	registry, err := m.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Check if template exists
	if _, exists := registry.Templates[name]; !exists {
		return fmt.Errorf("template '%s' does not exist", name)
	}

	// Remove template directory
	templatePath := filepath.Join(m.templatesDir, name)
	if err := os.RemoveAll(templatePath); err != nil {
		return fmt.Errorf("failed to remove template directory: %w", err)
	}

	// Remove from registry
	delete(registry.Templates, name)

	// Save registry
	if err := m.SaveRegistry(registry); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

func (m *Manager) UpdateTemplate(name string) error {
	registry, err := m.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	template, exists := registry.Templates[name]
	if !exists {
		return fmt.Errorf("template '%s' does not exist", name)
	}

	// Remove existing template directory
	templatePath := filepath.Join(m.templatesDir, name)
	if err := os.RemoveAll(templatePath); err != nil {
		return fmt.Errorf("failed to remove existing template: %w", err)
	}

	// Download updated template
	if err := m.downloadTemplate(template); err != nil {
		return fmt.Errorf("failed to download updated template: %w", err)
	}

	// Calculate new content hash
	newContentHash, err := m.calculateTemplateHash(template.Name)
	if err != nil {
		return fmt.Errorf("failed to calculate template hash: %w", err)
	}

	// Check if content actually changed
	if newContentHash == template.ContentHash {
		// No changes, just update timestamp
		template.UpdatedAt = time.Now()
	} else {
		// Content changed - this will trigger environment redeployment
		template.ContentHash = newContentHash
		template.UpdatedAt = time.Now()
	}
	registry.Templates[name] = template

	// Save registry
	if err := m.SaveRegistry(registry); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	return nil
}

func (m *Manager) ListTemplates() ([]Template, error) {
	registry, err := m.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	templates := make([]Template, 0, len(registry.Templates))
	for _, template := range registry.Templates {
		templates = append(templates, template)
	}

	return templates, nil
}

func (m *Manager) GetTemplate(name string) (*Template, error) {
	registry, err := m.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	template, exists := registry.Templates[name]
	if !exists {
		return nil, fmt.Errorf("template '%s' does not exist", name)
	}

	return &template, nil
}

func (m *Manager) GetTemplatePath(name string) string {
	return filepath.Join(m.templatesDir, name)
}

func (m *Manager) ValidateTemplate(name string) error {
	templatePath := m.GetTemplatePath(name)

	// Check if template directory exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("template directory does not exist: %s", templatePath)
	}

	// Check for main.tf file
	mainTFPath := filepath.Join(templatePath, "main.tf")
	if _, err := os.Stat(mainTFPath); os.IsNotExist(err) {
		return fmt.Errorf("template missing main.tf file: %s", mainTFPath)
	}

	return nil
}

func (m *Manager) downloadTemplate(template Template) error {
	// TODO: Implement actual GitHub download logic
	// For now, create a placeholder directory with a sample main.tf
	templatePath := filepath.Join(m.templatesDir, template.Name)

	if err := os.MkdirAll(templatePath, 0755); err != nil {
		return fmt.Errorf("failed to create template directory: %w", err)
	}

	// Create a placeholder main.tf
	mainTFContent := fmt.Sprintf(`# Template: %s
# Source: %s
# Ref: %s
# Path: %s

terraform {
  required_providers {
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
  }
}

resource "local_file" "template_marker" {
  content  = "Template: %s\nDeployed at: $${timestamp()}\n"
  filename = "/tmp/$${var.environment_name}_%s_deployed.txt"
}

variable "environment_name" {
  description = "Name of the environment"
  type        = string
  default     = "template"
}

output "deployment_file" {
  value = local_file.template_marker.filename
}
`, template.Name, template.SourceURL, template.SourceRef, template.SourcePath, template.Name, template.Name)

	mainTFPath := filepath.Join(templatePath, "main.tf")
	if err := os.WriteFile(mainTFPath, []byte(mainTFContent), 0644); err != nil {
		return fmt.Errorf("failed to write main.tf: %w", err)
	}

	return nil
}

// calculateTemplateHash calculates a hash of all template files for change detection
func (m *Manager) calculateTemplateHash(templateName string) (string, error) {
	templatePath := m.GetTemplatePath(templateName)

	// Collect all files and their hashes
	var fileHashes []string

	err := filepath.Walk(templatePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate file hash
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}

		// Get relative path for consistent hashing
		relPath, err := filepath.Rel(templatePath, path)
		if err != nil {
			return err
		}

		// Combine filename and content hash
		fileHash := fmt.Sprintf("%s:%x", relPath, hash.Sum(nil))
		fileHashes = append(fileHashes, fileHash)

		return nil
	})

	if err != nil {
		return "", err
	}

	// Sort for consistent ordering
	sort.Strings(fileHashes)

	// Hash the combined file hashes
	combinedHash := sha256.New()
	for _, fileHash := range fileHashes {
		combinedHash.Write([]byte(fileHash))
	}

	return hex.EncodeToString(combinedHash.Sum(nil)), nil
}

// GetTemplateContentHash returns the content hash for a template
func (m *Manager) GetTemplateContentHash(templateName string) (string, error) {
	registry, err := m.LoadRegistry()
	if err != nil {
		return "", fmt.Errorf("failed to load registry: %w", err)
	}

	template, exists := registry.Templates[templateName]
	if !exists {
		return "", fmt.Errorf("template '%s' does not exist", templateName)
	}

	return template.ContentHash, nil
}

// HasTemplateChanged checks if a template's content has changed since last recorded
func (m *Manager) HasTemplateChanged(templateName string) (bool, error) {
	registry, err := m.LoadRegistry()
	if err != nil {
		return false, fmt.Errorf("failed to load registry: %w", err)
	}

	template, exists := registry.Templates[templateName]
	if !exists {
		return false, fmt.Errorf("template '%s' does not exist", templateName)
	}

	// Calculate current hash
	currentHash, err := m.calculateTemplateHash(templateName)
	if err != nil {
		return false, fmt.Errorf("failed to calculate current hash: %w", err)
	}

	// Compare with stored hash
	return currentHash != template.ContentHash, nil
}