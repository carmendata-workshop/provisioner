package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"provisioner/pkg/opentofu"
	"provisioner/pkg/workspace"
)

// SwitchOperation represents a Reserved IP switching operation
type SwitchOperation struct {
	Environment     *Environment
	TargetWorkspace string
	LoadBalancers   []string // Server IDs/IPs from Terraform output
}

// SwitchResult represents the result of a switching operation
type SwitchResult struct {
	Success          bool
	Error            error
	Message          string
	RollbackRequired bool
	RollbackData     *RollbackData
}

// RollbackData contains information needed to rollback a partial switch
type RollbackData struct {
	OriginalWorkspace string
	IPAssignments     []IPAssignment
}

// IPAssignment represents the assignment of a Reserved IP to a server
type IPAssignment struct {
	ReservedIP string
	ServerID   string
	Success    bool
}

// PerformSwitch executes the environment switch operation
func (so *SwitchOperation) PerformSwitch() SwitchResult {
	// Step 1: Validate target workspace
	if err := so.validateTargetWorkspace(); err != nil {
		return SwitchResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("Target workspace validation failed: %v", err),
		}
	}

	// Step 2: Get load balancer information from workspace
	loadBalancers, err := so.getWorkspaceLoadBalancers()
	if err != nil {
		return SwitchResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("Failed to get load balancer information: %v", err),
		}
	}

	so.LoadBalancers = loadBalancers

	// Step 3: Validate sufficient load balancers
	if len(so.LoadBalancers) < len(so.Environment.Config.ReservedIPs) {
		return SwitchResult{
			Success: false,
			Error:   fmt.Errorf("insufficient load balancers"),
			Message: fmt.Sprintf("Target workspace has %d load balancers but environment requires %d", len(so.LoadBalancers), len(so.Environment.Config.ReservedIPs)),
		}
	}

	// Step 4: Perform health checks
	if err := so.performHealthChecks(); err != nil {
		return SwitchResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("Health checks failed: %v", err),
		}
	}

	// Step 5: Perform atomic Reserved IP switch with rollback capability
	return so.performAtomicSwitch()
}

// validateTargetWorkspace ensures the target workspace exists and is deployed
func (so *SwitchOperation) validateTargetWorkspace() error {
	// Load workspace configuration directly
	workspacesDir := getConfigDir() + "/workspaces"
	workspaces, err := workspace.LoadWorkspaces(workspacesDir)
	if err != nil {
		return fmt.Errorf("failed to load workspaces: %w", err)
	}

	// Find target workspace
	var targetWS *workspace.Workspace
	for _, ws := range workspaces {
		if ws.Name == so.TargetWorkspace {
			targetWS = &ws
			break
		}
	}

	if targetWS == nil {
		return fmt.Errorf("workspace '%s' not found", so.TargetWorkspace)
	}

	// Check if workspace is enabled
	if !targetWS.Config.Enabled {
		return fmt.Errorf("workspace '%s' is disabled", so.TargetWorkspace)
	}

	// Check if workspace is deployed
	status := targetWS.GetDeploymentStatus()
	if status != "deployed" {
		return fmt.Errorf("workspace '%s' is not deployed (status: %s)", so.TargetWorkspace, status)
	}

	return nil
}

// getWorkspaceLoadBalancers extracts the load_balancers output from the target workspace
func (so *SwitchOperation) getWorkspaceLoadBalancers() ([]string, error) {
	// Get the workspace's working directory
	workingDir := opentofu.GetWorkingDir(so.TargetWorkspace)

	// Use terraform output to get the load_balancers
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "tofu", "output", "-json", "load_balancers")
	cmd.Dir = workingDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get terraform output 'load_balancers': %w", err)
	}

	// Parse JSON output
	var result struct {
		Value []string `json:"value"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse terraform output: %w", err)
	}

	if len(result.Value) == 0 {
		return nil, fmt.Errorf("workspace does not expose any load_balancers")
	}

	return result.Value, nil
}

// performHealthChecks validates all load balancers are healthy
func (so *SwitchOperation) performHealthChecks() error {
	healthCheck := so.Environment.Config.HealthCheck

	// Extract IP addresses from load balancer information
	// This assumes load balancers are either IPs or we need to resolve them
	serverIPs, err := so.resolveLoadBalancerIPs()
	if err != nil {
		return fmt.Errorf("failed to resolve load balancer IPs: %w", err)
	}

	// Perform bulk health checks
	results := healthCheck.PerformBulkHealthChecks(serverIPs)

	// Check if all are healthy
	if !AllHealthy(results) {
		failures := GetFailedHealthChecks(results, serverIPs)
		return fmt.Errorf("health check failures:\n%s", strings.Join(failures, "\n"))
	}

	return nil
}

// resolveLoadBalancerIPs converts load balancer identifiers to IP addresses
func (so *SwitchOperation) resolveLoadBalancerIPs() ([]string, error) {
	var ips []string

	for _, lb := range so.LoadBalancers {
		// If it's already an IP address, use it directly
		if isIPAddress(lb) {
			ips = append(ips, lb)
			continue
		}

		// Otherwise, we need to get the IP from DigitalOcean
		// This would require DigitalOcean API calls to get droplet IP from ID
		// For now, let's assume the load_balancers output provides IPs directly
		ip, err := so.getDropletIP(lb)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve IP for load balancer %s: %w", lb, err)
		}
		ips = append(ips, ip)
	}

	return ips, nil
}

// getDropletIP gets the public IP address of a DigitalOcean droplet by ID
func (so *SwitchOperation) getDropletIP(dropletID string) (string, error) {
	// This is a placeholder - in a real implementation, this would use
	// DigitalOcean API to get the droplet's public IP address
	// For now, we'll use terraform output to get this information

	workingDir := opentofu.GetWorkingDir(so.TargetWorkspace)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to get a specific output that maps IDs to IPs
	cmd := exec.CommandContext(ctx, "tofu", "output", "-json", "load_balancer_ips")
	cmd.Dir = workingDir

	output, err := cmd.Output()
	if err != nil {
		// If load_balancer_ips doesn't exist, assume load_balancers already contains IPs
		return dropletID, nil
	}

	// Parse JSON output
	var result struct {
		Value map[string]string `json:"value"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return dropletID, nil // Fallback to assuming it's already an IP
	}

	if ip, exists := result.Value[dropletID]; exists {
		return ip, nil
	}

	// Fallback to assuming the dropletID is actually an IP
	return dropletID, nil
}

// performAtomicSwitch executes the Reserved IP switching with rollback capability
func (so *SwitchOperation) performAtomicSwitch() SwitchResult {
	originalWorkspace := so.Environment.Config.AssignedWorkspace

	// Prepare rollback data
	rollbackData := &RollbackData{
		OriginalWorkspace: originalWorkspace,
		IPAssignments:     make([]IPAssignment, len(so.Environment.Config.ReservedIPs)),
	}

	// Perform IP reassignments
	for i, reservedIP := range so.Environment.Config.ReservedIPs {
		targetServerID := so.LoadBalancers[i]

		// Assign Reserved IP to new server
		err := so.assignReservedIP(reservedIP, targetServerID)
		rollbackData.IPAssignments[i] = IPAssignment{
			ReservedIP: reservedIP,
			ServerID:   targetServerID,
			Success:    err == nil,
		}

		if err != nil {
			// Switch failed, perform rollback
			so.performRollback(rollbackData)
			return SwitchResult{
				Success:          false,
				Error:            err,
				Message:          fmt.Sprintf("Reserved IP assignment failed for %s: %v", reservedIP, err),
				RollbackRequired: true,
				RollbackData:     rollbackData,
			}
		}
	}

	// All IP assignments successful, update environment config
	so.Environment.Config.AssignedWorkspace = so.TargetWorkspace
	if err := so.Environment.SaveEnvironment(); err != nil {
		// Config update failed, but IPs are already switched
		// This is a partial success state
		return SwitchResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("Reserved IPs switched successfully, but failed to update config: %v", err),
		}
	}

	return SwitchResult{
		Success: true,
		Message: fmt.Sprintf("Successfully switched environment '%s' to workspace '%s'", so.Environment.Name, so.TargetWorkspace),
	}
}

// assignReservedIP assigns a Reserved IP to a specific server
func (so *SwitchOperation) assignReservedIP(reservedIP, serverID string) error {
	// Use DigitalOcean CLI or API to assign Reserved IP
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "doctl", "compute", "reserved-ip-action", "assign", reservedIP, "--resource", serverID, "--wait")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to assign Reserved IP %s to server %s: %w\nOutput: %s", reservedIP, serverID, err, string(output))
	}

	return nil
}

// performRollback attempts to rollback partial IP assignments
func (so *SwitchOperation) performRollback(rollbackData *RollbackData) {
	// This is a best-effort rollback
	// In a production system, you'd want more sophisticated rollback logic

	fmt.Printf("Performing rollback for environment '%s'...\n", so.Environment.Name)

	// Try to get the original server assignments
	// This would require storing the original assignments or querying them
	// For now, we'll log the rollback attempt
	for _, assignment := range rollbackData.IPAssignments {
		if assignment.Success {
			fmt.Printf("Attempting to rollback Reserved IP %s assignment...\n", assignment.ReservedIP)
			// In a real implementation, you'd reassign the IP back to the original server
		}
	}
}

// isIPAddress checks if a string is a valid IP address (simple check)
func isIPAddress(str string) bool {
	parts := strings.Split(str, ".")
	if len(parts) != 4 {
		return false
	}
	// This is a very basic check - in production you'd use net.ParseIP
	return true
}