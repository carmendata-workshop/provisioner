package environment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WorkspaceMetadata tracks template information for environment workspaces
type WorkspaceMetadata struct {
	EnvironmentName string    `json:"environment_name"`
	TemplateName    string    `json:"template_name,omitempty"`
	TemplateHash    string    `json:"template_hash,omitempty"`
	LastUpdated     time.Time `json:"last_updated"`
	CreatedAt       time.Time `json:"created_at"`
}

// GetWorkspaceMetadataPath returns the path to workspace metadata file
func GetWorkspaceMetadataPath(stateDir, envName string) string {
	return filepath.Join(stateDir, "workspaces", envName, ".provisioner-metadata.json")
}

// LoadWorkspaceMetadata loads metadata for an environment workspace
func LoadWorkspaceMetadata(stateDir, envName string) (*WorkspaceMetadata, error) {
	metadataPath := GetWorkspaceMetadataPath(stateDir, envName)

	// Return default metadata if file doesn't exist
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return &WorkspaceMetadata{
			EnvironmentName: envName,
			CreatedAt:       time.Now(),
			LastUpdated:     time.Now(),
		}, nil
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workspace metadata: %w", err)
	}

	var metadata WorkspaceMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace metadata: %w", err)
	}

	return &metadata, nil
}

// SaveWorkspaceMetadata saves metadata for an environment workspace
func SaveWorkspaceMetadata(stateDir, envName string, metadata *WorkspaceMetadata) error {
	metadataPath := GetWorkspaceMetadataPath(stateDir, envName)

	// Ensure workspace directory exists
	workspaceDir := filepath.Dir(metadataPath)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Update timestamp
	metadata.LastUpdated = time.Now()

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal workspace metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write workspace metadata: %w", err)
	}

	return nil
}

// IsTemplateOutdated checks if the workspace template is outdated compared to current template
func IsTemplateOutdated(stateDir, envName, currentTemplateHash string) (bool, error) {
	metadata, err := LoadWorkspaceMetadata(stateDir, envName)
	if err != nil {
		return false, err
	}

	// If no template hash recorded, consider outdated
	if metadata.TemplateHash == "" {
		return true, nil
	}

	// Compare hashes
	return metadata.TemplateHash != currentTemplateHash, nil
}

// UpdateWorkspaceTemplate updates workspace metadata with new template information
func UpdateWorkspaceTemplate(stateDir, envName, templateName, templateHash string) error {
	metadata, err := LoadWorkspaceMetadata(stateDir, envName)
	if err != nil {
		return err
	}

	metadata.TemplateName = templateName
	metadata.TemplateHash = templateHash

	return SaveWorkspaceMetadata(stateDir, envName, metadata)
}