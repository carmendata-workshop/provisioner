# Security Hardening Guide - OpenTofu Workspace Scheduler

**Status:** 🔴 Critical - Required for Production
**Last Updated:** September 28, 2025

## Overview

This document outlines comprehensive security hardening requirements for production deployment of the OpenTofu Workspace Scheduler. Currently, the system has **no security controls** and is unsuitable for production use.

## Current Security Posture

### ❌ Missing Security Controls
- No authentication mechanisms
- No authorization/RBAC system
- No encrypted communications (TLS/HTTPS)
- No secrets management
- No security audit logging
- No input validation/sanitization
- No rate limiting or DDoS protection
- No network security controls

## Security Architecture Requirements

### 1. Authentication & Authorization

#### API Authentication
```yaml
Tier 1 (Minimum):
  - API Key authentication for CLI tools
  - Bearer token validation
  - Token expiration and rotation

Tier 2 (Recommended):
  - OAuth2/OIDC integration
  - JWT token validation
  - Multi-factor authentication (MFA)

Tier 3 (Enterprise):
  - SAML/LDAP integration
  - Certificate-based authentication
  - Hardware security modules (HSM)
```

#### Role-Based Access Control (RBAC)
```yaml
Roles:
  admin:
    permissions:
      - workspace:* (all operations)
      - template:* (all operations)
      - job:* (all operations)
      - system:* (configuration, health)

  operator:
    permissions:
      - workspace:read
      - workspace:deploy
      - workspace:destroy
      - job:read
      - job:run

  viewer:
    permissions:
      - workspace:read
      - template:read
      - job:read

Permission Model:
  - Resource-level permissions (workspace:deploy:my-app)
  - Action-based permissions (create, read, update, delete)
  - Time-based permissions (emergency access)
```

#### Implementation Plan
```go
// Authentication middleware
type AuthMiddleware struct {
    tokenValidator TokenValidator
    rbacService    RBACService
}

// Authorization service
type AuthorizationService interface {
    Authorize(user User, resource string, action string) error
    GetUserPermissions(user User) []Permission
}

// Secure configuration
type SecurityConfig struct {
    AuthProvider     string `json:"auth_provider"`     // "api_key", "oidc", "ldap"
    TokenTTL         time.Duration `json:"token_ttl"`
    RequireMFA       bool   `json:"require_mfa"`
    AllowedOrigins   []string `json:"allowed_origins"`
}
```

### 2. Network Security

#### TLS/HTTPS Requirements
```yaml
TLS Configuration:
  minimum_version: TLS 1.3
  cipher_suites:
    - TLS_AES_256_GCM_SHA384
    - TLS_CHACHA20_POLY1305_SHA256
    - TLS_AES_128_GCM_SHA256

Certificate Management:
  - Automatic certificate rotation
  - Certificate transparency logging
  - OCSP stapling support
  - HSTS headers enabled
```

#### Network Controls
```yaml
Firewall Rules:
  inbound:
    - port: 8080 (HTTPS API)
      source: trusted_networks
    - port: 9090 (metrics)
      source: monitoring_network

  outbound:
    - OpenTofu provider APIs
    - Template repositories
    - Authentication providers

Network Segmentation:
  - Separate network for provisioner services
  - DMZ for public-facing components
  - Private network for state storage
```

### 3. Secrets Management

#### Secrets Architecture
```yaml
Secrets Storage:
  primary: HashiCorp Vault
  fallback: Kubernetes Secrets
  local_dev: File-based (encrypted)

Secret Types:
  - API tokens and keys
  - Database credentials
  - TLS certificates
  - OpenTofu provider credentials
  - Template repository access tokens

Rotation Policy:
  - API keys: 90 days
  - Database passwords: 30 days
  - Certificates: 365 days
  - Emergency rotation: immediate
```

#### Implementation
```go
// Secrets interface
type SecretsManager interface {
    GetSecret(path string) (string, error)
    SetSecret(path string, value string) error
    RotateSecret(path string) error
    ListSecrets(path string) ([]string, error)
}

// Vault implementation
type VaultSecretsManager struct {
    client *vault.Client
    config VaultConfig
}

// Configuration integration
type WorkspaceConfig struct {
    Enabled         bool              `json:"enabled"`
    Template        string            `json:"template"`
    Schedule        string            `json:"schedule"`
    Secrets         map[string]string `json:"secrets"`        // Reference to secret paths
    EncryptedVars   map[string]string `json:"encrypted_vars"` // Encrypted variables
}
```

### 4. Input Validation & Sanitization

#### Validation Framework
```go
// Input validation service
type ValidationService struct {
    rules map[string]ValidationRule
}

type ValidationRule struct {
    Required    bool
    Type        string // "string", "int", "cron", "duration"
    MinLength   int
    MaxLength   int
    Pattern     *regexp.Regexp
    Whitelist   []string
    Blacklist   []string
}

// Validation rules
var ValidationRules = map[string]ValidationRule{
    "workspace_name": {
        Required:  true,
        Type:      "string",
        MinLength: 3,
        MaxLength: 50,
        Pattern:   regexp.MustCompile(`^[a-z0-9-]+$`),
    },
    "cron_expression": {
        Required: true,
        Type:     "cron",
        Pattern:  cronPattern,
    },
    "template_path": {
        Required:  true,
        Type:      "string",
        Pattern:   regexp.MustCompile(`^[a-zA-Z0-9/_.-]+$`),
        Blacklist: []string{"../", "~", "$"},
    },
}
```

#### Sanitization
```go
// Input sanitization
func SanitizeInput(input string, ruleName string) (string, error) {
    rule := ValidationRules[ruleName]

    // Remove dangerous characters
    sanitized := strings.ReplaceAll(input, "../", "")
    sanitized = strings.ReplaceAll(sanitized, "~", "")

    // Validate against rule
    if err := ValidateInput(sanitized, rule); err != nil {
        return "", err
    }

    return sanitized, nil
}
```

### 5. Security Audit Logging

#### Audit Events
```yaml
Security Events:
  authentication:
    - login_success
    - login_failure
    - token_refresh
    - logout

  authorization:
    - permission_granted
    - permission_denied
    - role_assignment
    - role_removal

  resource_access:
    - workspace_deploy
    - workspace_destroy
    - template_create
    - template_delete
    - job_execution
    - configuration_change

  security:
    - failed_authentication_attempts
    - privilege_escalation_attempts
    - suspicious_activity
    - security_policy_violations
```

#### Audit Log Format
```json
{
  "timestamp": "2025-09-28T10:30:00Z",
  "event_type": "workspace_deploy",
  "severity": "info",
  "user": {
    "id": "user123",
    "email": "user@company.com",
    "roles": ["operator"]
  },
  "resource": {
    "type": "workspace",
    "name": "my-app",
    "id": "ws-123"
  },
  "action": "deploy",
  "result": "success",
  "source_ip": "10.0.1.100",
  "user_agent": "workspacectl/1.0.0",
  "session_id": "sess-456",
  "correlation_id": "req-789"
}
```

#### Implementation
```go
// Audit logger
type AuditLogger struct {
    writer io.Writer
    format string // "json", "syslog"
}

type AuditEvent struct {
    Timestamp     time.Time  `json:"timestamp"`
    EventType     string     `json:"event_type"`
    Severity      string     `json:"severity"`
    User          User       `json:"user"`
    Resource      Resource   `json:"resource"`
    Action        string     `json:"action"`
    Result        string     `json:"result"`
    SourceIP      string     `json:"source_ip"`
    UserAgent     string     `json:"user_agent"`
    SessionID     string     `json:"session_id"`
    CorrelationID string     `json:"correlation_id"`
    Details       map[string]interface{} `json:"details,omitempty"`
}

func (a *AuditLogger) LogEvent(event AuditEvent) error {
    event.Timestamp = time.Now().UTC()
    return json.NewEncoder(a.writer).Encode(event)
}
```

### 6. Rate Limiting & DDoS Protection

#### Rate Limiting Strategy
```yaml
Rate Limits:
  global:
    requests_per_minute: 1000
    burst: 100

  per_user:
    requests_per_minute: 100
    burst: 20

  per_endpoint:
    deploy_workspace: 10/minute
    destroy_workspace: 10/minute
    list_workspaces: 60/minute
    status_check: 120/minute

Implementation:
  algorithm: token_bucket
  storage: redis_cluster
  graceful_degradation: true
```

#### Implementation
```go
// Rate limiter
type RateLimiter struct {
    store  RateLimitStore
    config RateLimitConfig
}

type RateLimitConfig struct {
    GlobalLimit    int           `json:"global_limit"`
    UserLimit      int           `json:"user_limit"`
    EndpointLimits map[string]int `json:"endpoint_limits"`
    WindowSize     time.Duration `json:"window_size"`
}

func (r *RateLimiter) AllowRequest(userID string, endpoint string) (bool, error) {
    // Check global limit
    if !r.checkGlobalLimit() {
        return false, ErrGlobalRateLimit
    }

    // Check user limit
    if !r.checkUserLimit(userID) {
        return false, ErrUserRateLimit
    }

    // Check endpoint limit
    if !r.checkEndpointLimit(userID, endpoint) {
        return false, ErrEndpointRateLimit
    }

    return true, nil
}
```

## Security Implementation Phases

### Phase 1: Foundation (Weeks 1-2)
```yaml
Deliverables:
  - API key authentication
  - Basic RBAC implementation
  - TLS/HTTPS support
  - Input validation framework
  - Security audit logging

Implementation Steps:
  1. Create authentication middleware
  2. Implement API key storage and validation
  3. Add TLS configuration to HTTP server
  4. Create RBAC service with basic roles
  5. Implement audit logging service
  6. Add input validation to all endpoints
```

### Phase 2: Secrets & Authorization (Weeks 3-4)
```yaml
Deliverables:
  - HashiCorp Vault integration
  - Advanced RBAC with resource-level permissions
  - Secrets rotation automation
  - Configuration encryption

Implementation Steps:
  1. Integrate HashiCorp Vault client
  2. Migrate hardcoded secrets to Vault
  3. Implement resource-level authorization
  4. Add automatic secret rotation
  5. Encrypt sensitive configuration files
```

### Phase 3: Advanced Security (Weeks 5-6)
```yaml
Deliverables:
  - OAuth2/OIDC integration
  - Rate limiting implementation
  - Security scanning integration
  - Compliance reporting

Implementation Steps:
  1. Implement OIDC authentication flow
  2. Add rate limiting middleware
  3. Integrate security scanning tools
  4. Create compliance dashboard
  5. Implement security metrics
```

## Security Testing

### Security Test Plan
```yaml
Authentication Testing:
  - Valid/invalid API key tests
  - Token expiration tests
  - Brute force protection tests
  - Session hijacking tests

Authorization Testing:
  - Role-based access tests
  - Privilege escalation tests
  - Resource isolation tests
  - Permission inheritance tests

Input Validation Testing:
  - SQL injection tests
  - Command injection tests
  - Path traversal tests
  - XSS prevention tests

Network Security Testing:
  - TLS configuration tests
  - Certificate validation tests
  - Network isolation tests
  - Firewall rule tests
```

### Automated Security Scanning
```yaml
Static Analysis:
  - gosec (Go security scanner)
  - semgrep (multi-language analysis)
  - CodeQL (GitHub security analysis)

Dependency Scanning:
  - govulncheck (Go vulnerability scanner)
  - Snyk (dependency vulnerability scanner)
  - OWASP Dependency Check

Runtime Security:
  - Container image scanning
  - Runtime behavior analysis
  - Network traffic monitoring
```

## Security Monitoring

### Security Metrics
```yaml
Authentication Metrics:
  - failed_login_attempts_total
  - successful_logins_total
  - token_validation_failures_total
  - session_duration_seconds

Authorization Metrics:
  - permission_denied_total
  - role_assignments_total
  - privilege_escalation_attempts_total

Security Events:
  - suspicious_activity_total
  - security_policy_violations_total
  - rate_limit_exceeded_total
  - input_validation_failures_total
```

### Security Alerts
```yaml
Critical Alerts:
  - Multiple failed authentication attempts
  - Privilege escalation attempts
  - Unusual admin activity
  - Security policy violations

Warning Alerts:
  - High rate of permission denials
  - Suspicious user patterns
  - Configuration changes
  - Certificate expiration warnings
```

## Compliance Requirements

### SOC 2 Type II Controls
```yaml
Access Controls:
  - User authentication required
  - Role-based access controls
  - Least privilege principle
  - Regular access reviews

Audit Logging:
  - All user actions logged
  - System changes tracked
  - Log integrity protection
  - Centralized log management

Data Protection:
  - Encryption at rest
  - Encryption in transit
  - Key management procedures
  - Data retention policies
```

### GDPR Compliance
```yaml
Data Protection:
  - Personal data encryption
  - Data minimization
  - Purpose limitation
  - Storage limitation

User Rights:
  - Right to access
  - Right to rectification
  - Right to erasure
  - Right to portability

Privacy by Design:
  - Default privacy settings
  - Privacy impact assessments
  - Data protection officer
  - Regular compliance audits
```

## Security Runbooks

### Incident Response
```yaml
Security Incident Types:
  - Unauthorized access
  - Data breach
  - Service disruption
  - Compliance violation

Response Procedures:
  1. Immediate containment
  2. Impact assessment
  3. Evidence collection
  4. Stakeholder notification
  5. Recovery actions
  6. Post-incident review
```

### Emergency Procedures
```yaml
Emergency Access:
  - Break-glass procedures
  - Emergency admin accounts
  - Incident commander designation
  - Communication protocols

Security Updates:
  - Critical patch procedures
  - Emergency deployment process
  - Rollback procedures
  - Validation testing
```

---

**Next Steps:**
1. Review and approve security architecture
2. Prioritize implementation phases
3. Allocate development resources
4. Begin Phase 1 implementation
5. Establish security testing procedures

*This document should be reviewed and updated quarterly as security requirements evolve.*