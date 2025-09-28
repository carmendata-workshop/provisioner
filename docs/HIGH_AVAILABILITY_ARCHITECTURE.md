# High Availability Architecture - OpenTofu Workspace Scheduler

**Status:** 🔴 Critical - Required for Production
**Last Updated:** September 28, 2025

## Overview

This document defines the high availability (HA) architecture required for production deployment. Currently, the system is designed as a **single-instance MVP** with no clustering, failover, or resilience mechanisms, making it unsuitable for production use.

## Current Architecture Limitations

### ❌ Single Points of Failure
- **Single scheduler instance** - No clustering or redundancy
- **Local file storage** - State files on local filesystem
- **In-memory state** - State lost on process restart
- **No leader election** - Cannot run multiple instances safely
- **No health monitoring** - No automatic failure detection
- **No retry logic** - Permanent failures on transient errors

### ❌ Resilience Gaps
- No circuit breakers for external dependencies
- No graceful degradation during failures
- No automatic recovery mechanisms
- No load balancing or traffic distribution
- No data replication or backup

## High Availability Requirements

### RTO/RPO Targets
```yaml
Recovery Time Objective (RTO): < 5 minutes
Recovery Point Objective (RPO): < 1 minute
Availability Target: 99.9% (8.77 hours downtime/year)
Mean Time to Recovery (MTTR): < 10 minutes
```

## HA Architecture Design

### 1. Clustering & Leader Election

#### Architecture Overview
```yaml
Deployment Model: Active-Passive Clustering
Minimum Instances: 3 (for quorum)
Leader Election: etcd/Consul-based
State Sharing: Distributed database
Failover Time: < 30 seconds
```

#### Leader Election Implementation
```go
// Leader election service
type LeaderElection struct {
    client     *clientv3.Client  // etcd client
    session    *concurrency.Session
    election   *concurrency.Election
    isLeader   bool
    callbacks  LeaderCallbacks
    ctx        context.Context
    cancel     context.CancelFunc
}

type LeaderCallbacks struct {
    OnElected    func()
    OnDefeated   func()
    OnError      func(error)
}

func NewLeaderElection(endpoints []string, identity string) (*LeaderElection, error) {
    client, err := clientv3.New(clientv3.Config{
        Endpoints:   endpoints,
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        return nil, err
    }

    session, err := concurrency.NewSession(client, concurrency.WithTTL(30))
    if err != nil {
        return nil, err
    }

    election := concurrency.NewElection(session, "/provisioner/leader")

    ctx, cancel := context.WithCancel(context.Background())

    return &LeaderElection{
        client:   client,
        session:  session,
        election: election,
        ctx:      ctx,
        cancel:   cancel,
    }, nil
}

func (le *LeaderElection) Campaign() error {
    // Campaign to become leader
    go func() {
        if err := le.election.Campaign(le.ctx, le.identity); err != nil {
            if le.callbacks.OnError != nil {
                le.callbacks.OnError(err)
            }
            return
        }

        le.isLeader = true
        if le.callbacks.OnElected != nil {
            le.callbacks.OnElected()
        }

        // Watch for leadership changes
        ch := le.election.Observe(le.ctx)
        for resp := range ch {
            if string(resp.Kvs[0].Value) != le.identity {
                le.isLeader = false
                if le.callbacks.OnDefeated != nil {
                    le.callbacks.OnDefeated()
                }
                break
            }
        }
    }()

    return nil
}

func (le *LeaderElection) IsLeader() bool {
    return le.isLeader
}
```

#### Scheduler Integration
```go
// HA Scheduler with leader election
type HAScheduler struct {
    *Scheduler
    election   *LeaderElection
    isActive   bool
    mutex      sync.RWMutex
}

func NewHAScheduler(config Config) (*HAScheduler, error) {
    scheduler := New()

    election, err := NewLeaderElection(config.EtcdEndpoints, config.InstanceID)
    if err != nil {
        return nil, err
    }

    ha := &HAScheduler{
        Scheduler: scheduler,
        election:  election,
    }

    // Set up leader election callbacks
    election.callbacks = LeaderCallbacks{
        OnElected: func() {
            ha.mutex.Lock()
            ha.isActive = true
            ha.mutex.Unlock()

            logging.LogSystemd("Became leader, starting scheduler")
            ha.Start()
        },
        OnDefeated: func() {
            ha.mutex.Lock()
            ha.isActive = false
            ha.mutex.Unlock()

            logging.LogSystemd("Lost leadership, stopping scheduler")
            ha.Stop()
        },
        OnError: func(err error) {
            logging.LogSystemd("Leader election error: %v", err)
        },
    }

    return ha, nil
}

func (ha *HAScheduler) Start() error {
    // Start leader election
    if err := ha.election.Campaign(); err != nil {
        return err
    }

    // Only start scheduler if we're the leader
    ha.mutex.RLock()
    if ha.isActive {
        go ha.Scheduler.Start()
    }
    ha.mutex.RUnlock()

    return nil
}
```

### 2. State Management & Data Layer

#### Distributed State Architecture
```yaml
Primary Storage: PostgreSQL (with HA)
Caching Layer: Redis Cluster
State Replication: Synchronous (within region)
Backup Strategy: Point-in-time recovery
Consistency Model: Strong consistency
```

#### Database Schema
```sql
-- Workspace state table
CREATE TABLE workspace_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    status VARCHAR(50) NOT NULL,
    deployment_mode VARCHAR(50),
    last_deployed TIMESTAMP WITH TIME ZONE,
    last_destroyed TIMESTAMP WITH TIME ZONE,
    last_deploy_error TEXT,
    last_destroy_error TEXT,
    last_config_modified TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    version INTEGER DEFAULT 1
);

-- Job state table
CREATE TABLE job_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID REFERENCES workspace_states(id),
    name VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    last_run TIMESTAMP WITH TIME ZONE,
    last_success TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    run_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(workspace_id, name)
);

-- Leader election table
CREATE TABLE leader_election (
    id VARCHAR(255) PRIMARY KEY,
    leader_id VARCHAR(255) NOT NULL,
    lease_expiry TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Configuration change tracking
CREATE TABLE config_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_name VARCHAR(255),
    change_type VARCHAR(50) NOT NULL,
    old_config JSONB,
    new_config JSONB,
    changed_by VARCHAR(255),
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

#### Database Service Implementation
```go
// Database service with connection pooling
type DatabaseService struct {
    db     *sql.DB
    config DatabaseConfig
}

type DatabaseConfig struct {
    Host            string `json:"host"`
    Port            int    `json:"port"`
    Database        string `json:"database"`
    Username        string `json:"username"`
    Password        string `json:"password"`
    SSLMode         string `json:"ssl_mode"`
    MaxConnections  int    `json:"max_connections"`
    MaxIdleConns    int    `json:"max_idle_conns"`
    ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

func NewDatabaseService(config DatabaseConfig) (*DatabaseService, error) {
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        config.Host, config.Port, config.Username, config.Password,
        config.Database, config.SSLMode)

    db, err := sql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }

    // Configure connection pool
    db.SetMaxOpenConns(config.MaxConnections)
    db.SetMaxIdleConns(config.MaxIdleConns)
    db.SetConnMaxLifetime(config.ConnMaxLifetime)

    // Test connection
    if err := db.Ping(); err != nil {
        return nil, err
    }

    return &DatabaseService{
        db:     db,
        config: config,
    }, nil
}

func (d *DatabaseService) GetWorkspaceState(name string) (*WorkspaceState, error) {
    query := `
        SELECT name, status, deployment_mode, last_deployed, last_destroyed,
               last_deploy_error, last_destroy_error, last_config_modified,
               version
        FROM workspace_states
        WHERE name = $1
    `

    row := d.db.QueryRow(query, name)

    var state WorkspaceState
    err := row.Scan(
        &state.Name, &state.Status, &state.DeploymentMode,
        &state.LastDeployed, &state.LastDestroyed,
        &state.LastDeployError, &state.LastDestroyError,
        &state.LastConfigModified, &state.Version,
    )

    if err == sql.ErrNoRows {
        return nil, ErrWorkspaceNotFound
    }
    if err != nil {
        return nil, err
    }

    return &state, nil
}

func (d *DatabaseService) UpdateWorkspaceState(state *WorkspaceState) error {
    query := `
        UPDATE workspace_states
        SET status = $2, deployment_mode = $3, last_deployed = $4,
            last_destroyed = $5, last_deploy_error = $6, last_destroy_error = $7,
            last_config_modified = $8, updated_at = NOW(), version = version + 1
        WHERE name = $1 AND version = $9
    `

    result, err := d.db.Exec(query,
        state.Name, state.Status, state.DeploymentMode,
        state.LastDeployed, state.LastDestroyed,
        state.LastDeployError, state.LastDestroyError,
        state.LastConfigModified, state.Version,
    )

    if err != nil {
        return err
    }

    rowsAffected, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rowsAffected == 0 {
        return ErrOptimisticLockFailure
    }

    return nil
}
```

### 3. Circuit Breaker & Retry Logic

#### Circuit Breaker Implementation
```go
// Circuit breaker for OpenTofu operations
type CircuitBreaker struct {
    maxRequests  uint32
    interval     time.Duration
    timeout      time.Duration
    maxFailures  uint32
    state        State
    counts       Counts
    mutex        sync.RWMutex
}

type State int

const (
    StateClosed State = iota
    StateHalfOpen
    StateOpen
)

type Counts struct {
    Requests             uint32
    TotalSuccesses       uint32
    TotalFailures        uint32
    ConsecutiveSuccesses uint32
    ConsecutiveFailures  uint32
}

func NewCircuitBreaker(maxFailures uint32, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        maxRequests: 3,
        interval:    60 * time.Second,
        timeout:     timeout,
        maxFailures: maxFailures,
        state:       StateClosed,
    }
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    if !cb.canExecute() {
        return ErrCircuitBreakerOpen
    }

    start := time.Now()
    err := fn()
    duration := time.Since(start)

    cb.recordResult(err == nil, duration)
    return err
}

func (cb *CircuitBreaker) canExecute() bool {
    cb.mutex.Lock()
    defer cb.mutex.Unlock()

    now := time.Now()

    switch cb.state {
    case StateClosed:
        return true
    case StateOpen:
        if cb.timeout > 0 && now.Sub(cb.lastFailure) > cb.timeout {
            cb.setState(StateHalfOpen)
            return true
        }
        return false
    case StateHalfOpen:
        return cb.counts.Requests < cb.maxRequests
    }

    return false
}
```

#### Retry Logic Implementation
```go
// Retry service with exponential backoff
type RetryService struct {
    maxRetries    int
    baseDelay     time.Duration
    maxDelay      time.Duration
    backoffFactor float64
}

func NewRetryService() *RetryService {
    return &RetryService{
        maxRetries:    3,
        baseDelay:     1 * time.Second,
        maxDelay:      30 * time.Second,
        backoffFactor: 2.0,
    }
}

func (r *RetryService) ExecuteWithRetry(ctx context.Context, operation func() error) error {
    var lastErr error

    for attempt := 0; attempt <= r.maxRetries; attempt++ {
        if attempt > 0 {
            delay := r.calculateDelay(attempt)

            select {
            case <-time.After(delay):
                // Continue with retry
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        err := operation()
        if err == nil {
            return nil
        }

        lastErr = err

        // Check if error is retryable
        if !r.isRetryableError(err) {
            return err
        }

        logging.LogSystemd("Operation failed (attempt %d/%d): %v",
            attempt+1, r.maxRetries+1, err)
    }

    return fmt.Errorf("operation failed after %d attempts: %w",
        r.maxRetries+1, lastErr)
}

func (r *RetryService) calculateDelay(attempt int) time.Duration {
    delay := float64(r.baseDelay) * math.Pow(r.backoffFactor, float64(attempt-1))

    if delay > float64(r.maxDelay) {
        delay = float64(r.maxDelay)
    }

    // Add jitter (±25%)
    jitter := delay * 0.25 * (2*rand.Float64() - 1)
    return time.Duration(delay + jitter)
}

func (r *RetryService) isRetryableError(err error) bool {
    // Define which errors are retryable
    retryableErrors := []string{
        "connection refused",
        "timeout",
        "temporary failure",
        "rate limit",
        "service unavailable",
    }

    errStr := strings.ToLower(err.Error())
    for _, retryable := range retryableErrors {
        if strings.Contains(errStr, retryable) {
            return true
        }
    }

    return false
}
```

### 4. Load Balancing & Traffic Management

#### Load Balancer Configuration
```yaml
# HAProxy configuration
global:
    daemon
    maxconn 4096

defaults:
    mode http
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms
    option httplog

frontend provisioner_frontend:
    bind *:80
    bind *:443 ssl crt /etc/ssl/certs/provisioner.pem
    redirect scheme https if !{ ssl_fc }
    default_backend provisioner_backend

backend provisioner_backend:
    balance roundrobin
    option httpchk GET /health
    http-check expect status 200

    server provisioner-1 provisioner-1:8080 check
    server provisioner-2 provisioner-2:8080 check
    server provisioner-3 provisioner-3:8080 check
```

#### Kubernetes Deployment
```yaml
# provisioner-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: provisioner
  namespace: provisioner
spec:
  replicas: 3
  selector:
    matchLabels:
      app: provisioner
  template:
    metadata:
      labels:
        app: provisioner
    spec:
      containers:
      - name: provisioner
        image: provisioner:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: INSTANCE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: ETCD_ENDPOINTS
          value: "etcd-0.etcd:2379,etcd-1.etcd:2379,etcd-2.etcd:2379"
        - name: DATABASE_HOST
          value: "postgresql-ha"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"

---
apiVersion: v1
kind: Service
metadata:
  name: provisioner
  namespace: provisioner
spec:
  selector:
    app: provisioner
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
  type: LoadBalancer
```

### 5. Graceful Shutdown & Health Checks

#### Graceful Shutdown Implementation
```go
// Graceful shutdown handler
type GracefulShutdown struct {
    scheduler    *HAScheduler
    httpServer   *http.Server
    shutdownChan chan os.Signal
    timeout      time.Duration
}

func NewGracefulShutdown(scheduler *HAScheduler, httpServer *http.Server) *GracefulShutdown {
    gs := &GracefulShutdown{
        scheduler:    scheduler,
        httpServer:   httpServer,
        shutdownChan: make(chan os.Signal, 1),
        timeout:      30 * time.Second,
    }

    signal.Notify(gs.shutdownChan, syscall.SIGINT, syscall.SIGTERM)
    return gs
}

func (gs *GracefulShutdown) WaitForShutdown() {
    <-gs.shutdownChan
    logging.LogSystemd("Received shutdown signal, starting graceful shutdown")

    ctx, cancel := context.WithTimeout(context.Background(), gs.timeout)
    defer cancel()

    // Stop accepting new requests
    gs.httpServer.SetKeepAlivesEnabled(false)

    // Stop scheduler gracefully
    if gs.scheduler.IsActive() {
        logging.LogSystemd("Stopping scheduler...")
        gs.scheduler.Stop()
    }

    // Shutdown HTTP server
    if err := gs.httpServer.Shutdown(ctx); err != nil {
        logging.LogSystemd("Error during HTTP server shutdown: %v", err)
    }

    // Release leadership
    if gs.scheduler.election != nil {
        gs.scheduler.election.Resign()
    }

    logging.LogSystemd("Graceful shutdown completed")
}
```

#### Advanced Health Checks
```go
// Advanced health check service
type AdvancedHealthService struct {
    scheduler   *HAScheduler
    database    *DatabaseService
    election    *LeaderElection
    checks      map[string]HealthCheck
    lastChecks  map[string]time.Time
    mutex       sync.RWMutex
}

func (h *AdvancedHealthService) PerformHealthCheck(checkType string) HealthResponse {
    h.mutex.Lock()
    defer h.mutex.Unlock()

    var checks []HealthCheck

    switch checkType {
    case "liveness":
        checks = h.performLivenessChecks()
    case "readiness":
        checks = h.performReadinessChecks()
    case "deep":
        checks = h.performDeepHealthChecks()
    default:
        checks = h.performBasicHealthChecks()
    }

    // Determine overall status
    status := "healthy"
    for _, check := range checks {
        if check.Status == "unhealthy" {
            status = "unhealthy"
            break
        } else if check.Status == "degraded" && status == "healthy" {
            status = "degraded"
        }
    }

    return HealthResponse{
        Status:    status,
        Checks:    checks,
        Timestamp: time.Now(),
        Version:   version.GetVersion(),
    }
}

func (h *AdvancedHealthService) performReadinessChecks() []HealthCheck {
    checks := []HealthCheck{}

    // Check if we're the leader
    checks = append(checks, HealthCheck{
        Name:   "leader_election",
        Status: h.getLeadershipStatus(),
        Message: h.getLeadershipMessage(),
    })

    // Check database connectivity
    checks = append(checks, h.checkDatabaseConnectivity())

    // Check configuration loaded
    checks = append(checks, h.checkConfigurationLoaded())

    return checks
}

func (h *AdvancedHealthService) checkDatabaseConnectivity() HealthCheck {
    start := time.Now()

    err := h.database.Ping()
    duration := time.Since(start)

    if err != nil {
        return HealthCheck{
            Name:     "database_connectivity",
            Status:   "unhealthy",
            Message:  fmt.Sprintf("Database unreachable: %v", err),
            Duration: duration,
        }
    }

    if duration > 5*time.Second {
        return HealthCheck{
            Name:     "database_connectivity",
            Status:   "degraded",
            Message:  "Database responding slowly",
            Duration: duration,
        }
    }

    return HealthCheck{
        Name:     "database_connectivity",
        Status:   "healthy",
        Message:  "Database connectivity OK",
        Duration: duration,
    }
}
```

## Deployment Strategies

### 1. Blue-Green Deployment
```yaml
Strategy: Blue-Green
Deployment Time: 5-10 minutes
Rollback Time: < 2 minutes
Risk Level: Low
Automation: Full

Process:
  1. Deploy new version to Green environment
  2. Run health checks and smoke tests
  3. Switch traffic from Blue to Green
  4. Monitor for issues
  5. Keep Blue as rollback option
```

### 2. Canary Deployment
```yaml
Strategy: Canary
Traffic Split: 5% -> 25% -> 50% -> 100%
Deployment Time: 30-60 minutes
Rollback Time: < 1 minute
Risk Level: Very Low
Automation: Full

Process:
  1. Deploy to small subset (5% traffic)
  2. Monitor metrics and error rates
  3. Gradually increase traffic
  4. Full rollout if all metrics good
  5. Automatic rollback on failures
```

## Disaster Recovery

### Recovery Procedures
```yaml
Scenario 1: Single Instance Failure
  RTO: < 5 minutes
  Steps:
    1. Leader election selects new primary
    2. New leader starts scheduler
    3. Resume operations from last known state

Scenario 2: Data Center Failure
  RTO: < 15 minutes
  Steps:
    1. DNS failover to backup region
    2. Restore database from backup
    3. Start scheduler cluster in backup region

Scenario 3: Complete System Failure
  RTO: < 60 minutes
  Steps:
    1. Restore infrastructure from IaC
    2. Restore database from point-in-time backup
    3. Deploy application from artifacts
    4. Verify system functionality
```

## Implementation Phases

### Phase 1: Leader Election (Weeks 1-2)
```yaml
Deliverables:
  - etcd integration
  - Leader election service
  - Active-passive clustering
  - Basic failover testing

Tasks:
  1. Set up etcd cluster
  2. Implement leader election
  3. Modify scheduler for HA
  4. Test failover scenarios
```

### Phase 2: Distributed State (Weeks 3-4)
```yaml
Deliverables:
  - PostgreSQL integration
  - State migration from JSON
  - Connection pooling
  - Database health checks

Tasks:
  1. Set up PostgreSQL cluster
  2. Create database schema
  3. Implement database service
  4. Migrate existing state
```

### Phase 3: Resilience Features (Weeks 5-6)
```yaml
Deliverables:
  - Circuit breaker implementation
  - Retry logic with exponential backoff
  - Graceful shutdown
  - Advanced health checks

Tasks:
  1. Implement circuit breaker
  2. Add retry mechanisms
  3. Create graceful shutdown
  4. Enhance health checks
```

---

**Next Steps:**
1. Review HA architecture design
2. Set up etcd and PostgreSQL infrastructure
3. Begin Phase 1 implementation
4. Plan failover testing procedures
5. Document operational runbooks

*This document should be updated as the HA architecture evolves and new requirements emerge.*