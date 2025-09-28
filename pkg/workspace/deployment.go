package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DeploymentMetadata tracks template information for workspace deployments
type DeploymentMetadata struct {
	WorkspaceName string    `json:"workspace_name"`
	TemplateName  string    `json:"template_name,omitempty"`
	TemplateHash  string    `json:"template_hash,omitempty"`
	LastUpdated   time.Time `json:"last_updated"`
	CreatedAt     time.Time `json:"created_at"`
}

// GetDeploymentMetadataPath returns the path to deployment metadata file
func GetDeploymentMetadataPath(stateDir, wsName string) string {
	return filepath.Join(stateDir, "deployments", wsName, ".provisioner-metadata.json")
}

// LoadDeploymentMetadata loads metadata for a workspace deployment
func LoadDeploymentMetadata(stateDir, wsName string) (*DeploymentMetadata, error) {
	metadataPath := GetDeploymentMetadataPath(stateDir, wsName)

	// Return default metadata if file doesn't exist
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return &DeploymentMetadata{
			WorkspaceName: wsName,
			CreatedAt:     time.Now(),
			LastUpdated:   time.Now(),
		}, nil
	}

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read deployment metadata: %w", err)
	}

	var metadata DeploymentMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment metadata: %w", err)
	}

	return &metadata, nil
}

// SaveDeploymentMetadata saves metadata for a workspace deployment
func SaveDeploymentMetadata(stateDir, wsName string, metadata *DeploymentMetadata) error {
	metadataPath := GetDeploymentMetadataPath(stateDir, wsName)

	// Ensure deployment directory exists
	deploymentDir := filepath.Dir(metadataPath)
	if err := os.MkdirAll(deploymentDir, 0755); err != nil {
		return fmt.Errorf("failed to create deployment directory: %w", err)
	}

	// Update timestamp
	metadata.LastUpdated = time.Now()

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal deployment metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write deployment metadata: %w", err)
	}

	return nil
}

// IsTemplateOutdated checks if the deployment template is outdated compared to current template
func IsTemplateOutdated(stateDir, wsName, currentTemplateHash string) (bool, error) {
	metadata, err := LoadDeploymentMetadata(stateDir, wsName)
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

// UpdateDeploymentTemplate updates deployment metadata with new template information
func UpdateDeploymentTemplate(stateDir, wsName, templateName, templateHash string) error {
	metadata, err := LoadDeploymentMetadata(stateDir, wsName)
	if err != nil {
		return err
	}

	metadata.TemplateName = templateName
	metadata.TemplateHash = templateHash

	return SaveDeploymentMetadata(stateDir, wsName, metadata)
}
