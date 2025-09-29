package environment

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
)

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	Success bool
	Error   error
	Message string
}

// PerformHealthCheck executes a health check against a server
func (h *HealthCheck) PerformHealthCheck(serverIP string) HealthCheckResult {
	timeout, err := h.GetTimeoutDuration()
	if err != nil {
		return HealthCheckResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("Invalid timeout configuration: %v", err),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch h.Type {
	case "http":
		return h.performHTTPCheck(ctx, serverIP)
	case "tcp":
		return h.performTCPCheck(ctx, serverIP)
	case "command":
		return h.performCommandCheck(ctx, serverIP)
	default:
		return HealthCheckResult{
			Success: false,
			Error:   fmt.Errorf("unknown health check type: %s", h.Type),
			Message: fmt.Sprintf("Unknown health check type: %s", h.Type),
		}
	}
}

// performHTTPCheck performs an HTTP health check
func (h *HealthCheck) performHTTPCheck(ctx context.Context, serverIP string) HealthCheckResult {
	// Construct URL
	scheme := "http"
	if h.Port == 443 {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d%s", scheme, serverIP, h.Port, h.Path)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return HealthCheckResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("Failed to create HTTP request: %v", err),
		}
	}

	// Perform request
	client := &http.Client{
		Timeout: 0, // Timeout is handled by context
	}

	resp, err := client.Do(req)
	if err != nil {
		return HealthCheckResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("HTTP request failed: %v", err),
		}
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return HealthCheckResult{
			Success: false,
			Error:   fmt.Errorf("HTTP status %d", resp.StatusCode),
			Message: fmt.Sprintf("HTTP health check failed with status %d", resp.StatusCode),
		}
	}

	return HealthCheckResult{
		Success: true,
		Message: fmt.Sprintf("HTTP health check successful (status %d)", resp.StatusCode),
	}
}

// performTCPCheck performs a TCP connectivity health check
func (h *HealthCheck) performTCPCheck(ctx context.Context, serverIP string) HealthCheckResult {
	// Create dialer with timeout
	dialer := &net.Dialer{}

	// Attempt connection
	address := fmt.Sprintf("%s:%d", serverIP, h.Port)
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return HealthCheckResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("TCP connection failed: %v", err),
		}
	}
	defer conn.Close()

	return HealthCheckResult{
		Success: true,
		Message: fmt.Sprintf("TCP connection successful to %s", address),
	}
}

// performCommandCheck performs a custom command health check
func (h *HealthCheck) performCommandCheck(ctx context.Context, serverIP string) HealthCheckResult {
	// Replace {server} placeholder with actual server IP
	command := strings.ReplaceAll(h.Command, "{server}", serverIP)

	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return HealthCheckResult{
			Success: false,
			Error:   fmt.Errorf("empty command"),
			Message: "Health check command is empty",
		}
	}

	// Create command with context for timeout
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return HealthCheckResult{
			Success: false,
			Error:   err,
			Message: fmt.Sprintf("Command failed: %v\nOutput: %s", err, string(output)),
		}
	}

	return HealthCheckResult{
		Success: true,
		Message: fmt.Sprintf("Command executed successfully\nOutput: %s", string(output)),
	}
}

// PerformBulkHealthChecks performs health checks on multiple servers and returns results
func (h *HealthCheck) PerformBulkHealthChecks(serverIPs []string) []HealthCheckResult {
	results := make([]HealthCheckResult, len(serverIPs))

	// Perform checks in parallel for better performance
	resultChan := make(chan struct {
		index  int
		result HealthCheckResult
	}, len(serverIPs))

	for i, serverIP := range serverIPs {
		go func(index int, ip string) {
			result := h.PerformHealthCheck(ip)
			resultChan <- struct {
				index  int
				result HealthCheckResult
			}{index, result}
		}(i, serverIP)
	}

	// Collect results
	for i := 0; i < len(serverIPs); i++ {
		result := <-resultChan
		results[result.index] = result.result
	}

	return results
}

// AllHealthy checks if all health check results are successful
func AllHealthy(results []HealthCheckResult) bool {
	for _, result := range results {
		if !result.Success {
			return false
		}
	}
	return true
}

// GetFailedHealthChecks returns a list of error messages for failed health checks
func GetFailedHealthChecks(results []HealthCheckResult, serverIPs []string) []string {
	var failures []string
	for i, result := range results {
		if !result.Success {
			serverInfo := "unknown"
			if i < len(serverIPs) {
				serverInfo = serverIPs[i]
			}
			failures = append(failures, fmt.Sprintf("Server %s: %s", serverInfo, result.Message))
		}
	}
	return failures
}