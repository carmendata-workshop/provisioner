# Monitoring & Observability Standards - OpenTofu Workspace Scheduler

**Status:** 🔴 Critical - Required for Production
**Last Updated:** September 28, 2025

## Overview

This document defines comprehensive monitoring and observability requirements for production deployment. Currently, the system lacks production-grade monitoring capabilities and provides limited operational visibility.

## Current Observability State

### ✅ Existing Capabilities
- **Basic logging** - systemd journal + per-workspace file logging
- **State tracking** - JSON-based state persistence
- **Configuration hot-reload** - Automatic detection of config changes

### ❌ Missing Capabilities
- No metrics collection or exposition
- No health check endpoints
- No alerting or notification system
- No distributed tracing
- No performance monitoring
- No business metrics tracking
- No operational dashboards

## Monitoring Architecture Requirements

### 1. Metrics & Instrumentation

#### Prometheus Metrics Endpoint
```yaml
Endpoint: /metrics (port 9090)
Format: Prometheus exposition format
Security: Basic auth or mutual TLS
Update Interval: 15 seconds
```

#### Core Metrics Categories
```yaml
System Metrics:
  - go_memstats_* (Go runtime metrics)
  - process_* (process-level metrics)
  - http_requests_* (HTTP request metrics)

Business Metrics:
  - provisioner_workspaces_total (gauge)
  - provisioner_deployments_total (counter)
  - provisioner_deployment_duration_seconds (histogram)
  - provisioner_deployment_failures_total (counter)
  - provisioner_jobs_running_total (gauge)
  - provisioner_jobs_completed_total (counter)
  - provisioner_templates_total (gauge)

Scheduler Metrics:
  - provisioner_scheduler_loop_duration_seconds (histogram)
  - provisioner_config_reload_total (counter)
  - provisioner_cron_evaluations_total (counter)
  - provisioner_schedule_matches_total (counter)

OpenTofu Metrics:
  - provisioner_tofu_operations_total (counter)
  - provisioner_tofu_operation_duration_seconds (histogram)
  - provisioner_tofu_binary_downloads_total (counter)
```

#### Implementation
```go
// Metrics service
type MetricsService struct {
    registry prometheus.Registerer

    // System metrics
    httpRequestsTotal       *prometheus.CounterVec
    httpRequestDuration     *prometheus.HistogramVec

    // Business metrics
    workspacesTotal         prometheus.Gauge
    deploymentsTotal        *prometheus.CounterVec
    deploymentDuration      *prometheus.HistogramVec
    deploymentFailures      *prometheus.CounterVec
    jobsRunning            prometheus.Gauge
    jobsCompleted          *prometheus.CounterVec

    // Scheduler metrics
    schedulerLoopDuration   prometheus.Histogram
    configReloads          prometheus.Counter
    cronEvaluations        prometheus.Counter
    scheduleMatches        *prometheus.CounterVec
}

func NewMetricsService() *MetricsService {
    m := &MetricsService{
        registry: prometheus.NewRegistry(),
    }

    // Initialize metrics
    m.httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )

    m.deploymentsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "provisioner_deployments_total",
            Help: "Total number of workspace deployments",
        },
        []string{"workspace", "operation", "result"},
    )

    m.deploymentDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "provisioner_deployment_duration_seconds",
            Help: "Duration of workspace deployments",
            Buckets: []float64{30, 60, 120, 300, 600, 1200, 1800},
        },
        []string{"workspace", "operation"},
    )

    // Register all metrics
    m.registry.MustRegister(m.httpRequestsTotal)
    m.registry.MustRegister(m.deploymentsTotal)
    m.registry.MustRegister(m.deploymentDuration)

    return m
}

// Usage in scheduler
func (s *Scheduler) executeDeployment(workspace string) error {
    start := time.Now()
    defer func() {
        duration := time.Since(start).Seconds()
        s.metrics.deploymentDuration.WithLabelValues(workspace, "deploy").Observe(duration)
    }()

    err := s.client.Deploy(workspace)
    if err != nil {
        s.metrics.deploymentsTotal.WithLabelValues(workspace, "deploy", "failure").Inc()
        return err
    }

    s.metrics.deploymentsTotal.WithLabelValues(workspace, "deploy", "success").Inc()
    return nil
}
```

### 2. Health Checks & Readiness Probes

#### Health Check Endpoints
```yaml
Endpoints:
  /health:
    purpose: Basic health check
    checks: [scheduler_running, config_loaded]
    timeout: 5s

  /ready:
    purpose: Readiness probe
    checks: [scheduler_running, config_loaded, dependencies_available]
    timeout: 10s

  /health/deep:
    purpose: Comprehensive health check
    checks: [all_basic_checks, opentofu_available, state_accessible, templates_available]
    timeout: 30s

Response Format:
  status: "healthy" | "degraded" | "unhealthy"
  checks: [
    {
      name: "scheduler_running",
      status: "healthy",
      message: "Scheduler is running",
      last_check: "2025-09-28T10:30:00Z"
    }
  ]
  timestamp: "2025-09-28T10:30:00Z"
  version: "v1.2.3"
```

#### Implementation
```go
// Health check service
type HealthService struct {
    scheduler   *Scheduler
    client      opentofu.TofuClient
    templateMgr *template.Manager
}

type HealthCheck struct {
    Name        string    `json:"name"`
    Status      string    `json:"status"`
    Message     string    `json:"message"`
    LastCheck   time.Time `json:"last_check"`
    Duration    time.Duration `json:"duration,omitempty"`
}

type HealthResponse struct {
    Status    string        `json:"status"`
    Checks    []HealthCheck `json:"checks"`
    Timestamp time.Time     `json:"timestamp"`
    Version   string        `json:"version"`
}

func (h *HealthService) CheckHealth(deep bool) HealthResponse {
    checks := []HealthCheck{}

    // Basic checks
    checks = append(checks, h.checkSchedulerRunning())
    checks = append(checks, h.checkConfigLoaded())

    if deep {
        checks = append(checks, h.checkOpenTofuAvailable())
        checks = append(checks, h.checkStateAccessible())
        checks = append(checks, h.checkTemplatesAvailable())
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

func (h *HealthService) checkSchedulerRunning() HealthCheck {
    start := time.Now()

    if h.scheduler.IsRunning() {
        return HealthCheck{
            Name:      "scheduler_running",
            Status:    "healthy",
            Message:   "Scheduler is running",
            LastCheck: time.Now(),
            Duration:  time.Since(start),
        }
    }

    return HealthCheck{
        Name:      "scheduler_running",
        Status:    "unhealthy",
        Message:   "Scheduler is not running",
        LastCheck: time.Now(),
        Duration:  time.Since(start),
    }
}
```

### 3. Structured Logging

#### Log Format & Standards
```json
{
  "timestamp": "2025-09-28T10:30:00.123Z",
  "level": "info",
  "logger": "scheduler",
  "message": "Workspace deployment started",
  "workspace": "my-app",
  "operation": "deploy",
  "correlation_id": "req-123",
  "duration_ms": 1250,
  "error": null,
  "tags": {
    "component": "scheduler",
    "version": "v1.2.3",
    "environment": "production"
  }
}
```

#### Log Levels & Categories
```yaml
Log Levels:
  error: System errors, deployment failures
  warn: Configuration issues, deprecation warnings
  info: Normal operations, state changes
  debug: Detailed operation information
  trace: Verbose debugging information

Log Categories:
  scheduler: Scheduler operations and CRON evaluations
  workspace: Workspace lifecycle operations
  job: Job execution and management
  template: Template operations and downloads
  opentofu: OpenTofu client operations
  auth: Authentication and authorization
  audit: Security and compliance events
```

#### Implementation
```go
// Structured logger
type Logger struct {
    *logrus.Logger
    component string
    version   string
}

func NewLogger(component string) *Logger {
    logger := logrus.New()
    logger.SetFormatter(&logrus.JSONFormatter{
        TimestampFormat: time.RFC3339Nano,
        FieldMap: logrus.FieldMap{
            logrus.FieldKeyTime:  "timestamp",
            logrus.FieldKeyLevel: "level",
            logrus.FieldKeyMsg:   "message",
        },
    })

    return &Logger{
        Logger:    logger,
        component: component,
        version:   version.GetVersion(),
    }
}

func (l *Logger) WithOperation(operation string) *logrus.Entry {
    return l.WithFields(logrus.Fields{
        "operation": operation,
        "component": l.component,
        "version":   l.version,
    })
}

func (l *Logger) WithWorkspace(workspace string) *logrus.Entry {
    return l.WithFields(logrus.Fields{
        "workspace": workspace,
        "component": l.component,
        "version":   l.version,
    })
}

// Usage
logger := NewLogger("scheduler")
logger.WithWorkspace("my-app").WithOperation("deploy").Info("Starting deployment")
```

### 4. Distributed Tracing

#### Tracing Implementation
```yaml
Provider: OpenTelemetry
Exporter: Jaeger
Sampling: 10% (production), 100% (development)
Trace ID: Propagated through all operations
```

#### Trace Spans
```go
// Tracing service
type TracingService struct {
    tracer trace.Tracer
}

func NewTracingService() *TracingService {
    provider := trace.NewTracerProvider(
        trace.WithSampler(trace.TraceIDRatioBased(0.1)), // 10% sampling
        trace.WithBatcher(jaeger.New("http://jaeger:14268/api/traces")),
    )

    return &TracingService{
        tracer: provider.Tracer("provisioner"),
    }
}

func (t *TracingService) StartSpan(ctx context.Context, operationName string) (context.Context, trace.Span) {
    return t.tracer.Start(ctx, operationName)
}

// Usage in scheduler
func (s *Scheduler) ExecuteWorkspaceOperation(ctx context.Context, workspace string, operation string) error {
    ctx, span := s.tracing.StartSpan(ctx, fmt.Sprintf("workspace.%s", operation))
    defer span.End()

    span.SetAttributes(
        attribute.String("workspace.name", workspace),
        attribute.String("operation", operation),
    )

    // Execute operation
    err := s.executeOperation(ctx, workspace, operation)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }

    return err
}
```

### 5. Alerting & Notifications

#### Alert Rules
```yaml
Critical Alerts:
  - name: SchedulerDown
    condition: up{job="provisioner"} == 0
    for: 5m
    severity: critical

  - name: HighDeploymentFailureRate
    condition: rate(provisioner_deployment_failures_total[5m]) > 0.1
    for: 2m
    severity: critical

  - name: LongRunningDeployment
    condition: provisioner_deployment_duration_seconds > 1800
    for: 0m
    severity: warning

Warning Alerts:
  - name: ConfigReloadFailure
    condition: increase(provisioner_config_reload_failures_total[10m]) > 0
    for: 0m
    severity: warning

  - name: HighMemoryUsage
    condition: go_memstats_sys_bytes / 1024 / 1024 > 500
    for: 10m
    severity: warning
```

#### Notification Channels
```yaml
Channels:
  critical:
    - pagerduty: provisioner-critical
    - slack: #alerts-critical
    - email: ops-team@company.com

  warning:
    - slack: #alerts-warning
    - email: dev-team@company.com

Escalation:
  critical: immediate
  warning: business_hours_only
```

### 6. Dashboards & Visualization

#### Grafana Dashboards
```yaml
Operations Dashboard:
  panels:
    - Scheduler Status
    - Active Workspaces
    - Deployment Success Rate
    - Average Deployment Duration
    - Error Rate by Workspace
    - Job Execution Status

Performance Dashboard:
  panels:
    - Memory Usage
    - CPU Usage
    - Goroutine Count
    - HTTP Request Latency
    - Database Connection Pool

Business Dashboard:
  panels:
    - Workspace Deployment Trends
    - Cost Savings (estimated)
    - Template Usage
    - Schedule Efficiency
    - User Activity
```

#### Dashboard Implementation
```json
{
  "dashboard": {
    "title": "Provisioner Operations",
    "panels": [
      {
        "title": "Scheduler Status",
        "type": "stat",
        "targets": [
          {
            "expr": "up{job=\"provisioner\"}",
            "legendFormat": "Scheduler"
          }
        ]
      },
      {
        "title": "Active Workspaces",
        "type": "stat",
        "targets": [
          {
            "expr": "provisioner_workspaces_total",
            "legendFormat": "Total Workspaces"
          },
          {
            "expr": "sum(provisioner_workspaces_total{status=\"deployed\"})",
            "legendFormat": "Deployed"
          }
        ]
      },
      {
        "title": "Deployment Success Rate",
        "type": "stat",
        "targets": [
          {
            "expr": "rate(provisioner_deployments_total{result=\"success\"}[5m]) / rate(provisioner_deployments_total[5m]) * 100",
            "legendFormat": "Success Rate %"
          }
        ]
      }
    ]
  }
}
```

## Implementation Phases

### Phase 1: Basic Monitoring (Weeks 1-2)
```yaml
Deliverables:
  - Prometheus metrics endpoint
  - Basic health checks
  - Structured logging
  - Grafana dashboards

Tasks:
  1. Implement MetricsService
  2. Add /health and /ready endpoints
  3. Convert logging to structured format
  4. Create basic Grafana dashboards
  5. Set up Prometheus scraping
```

### Phase 2: Advanced Observability (Weeks 3-4)
```yaml
Deliverables:
  - Distributed tracing
  - Alert rules and notifications
  - Performance monitoring
  - Business metrics

Tasks:
  1. Implement OpenTelemetry tracing
  2. Configure Jaeger for trace collection
  3. Create Prometheus alert rules
  4. Set up notification channels
  5. Add business-specific metrics
```

### Phase 3: Operational Excellence (Weeks 5-6)
```yaml
Deliverables:
  - Comprehensive dashboards
  - SLA monitoring
  - Capacity planning metrics
  - Automated reporting

Tasks:
  1. Build comprehensive Grafana dashboards
  2. Implement SLA tracking
  3. Add capacity planning metrics
  4. Create automated reports
  5. Document runbooks
```

## Monitoring Configuration

### Prometheus Configuration
```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "provisioner_rules.yml"

scrape_configs:
  - job_name: 'provisioner'
    static_configs:
      - targets: ['provisioner:9090']
    scrape_interval: 15s
    metrics_path: /metrics
    basic_auth:
      username: prometheus
      password: ${PROMETHEUS_PASSWORD}

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

### Alert Manager Configuration
```yaml
# alertmanager.yml
global:
  pagerduty_url: 'https://events.pagerduty.com/v2/enqueue'

route:
  group_by: ['alertname']
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 12h
  receiver: 'default'
  routes:
    - match:
        severity: critical
      receiver: 'critical'
    - match:
        severity: warning
      receiver: 'warning'

receivers:
  - name: 'critical'
    pagerduty_configs:
      - service_key: ${PAGERDUTY_SERVICE_KEY}
    slack_configs:
      - api_url: ${SLACK_WEBHOOK_URL}
        channel: '#alerts-critical'

  - name: 'warning'
    slack_configs:
      - api_url: ${SLACK_WEBHOOK_URL}
        channel: '#alerts-warning'
```

## Monitoring Best Practices

### Metrics Design
```yaml
Naming Conventions:
  - Use snake_case for metric names
  - Include unit in metric name (e.g., _seconds, _bytes)
  - Use consistent label names
  - Avoid high cardinality labels

Label Guidelines:
  - Use labels for dimensions you want to aggregate by
  - Keep label cardinality under 10,000
  - Use const labels for metadata
  - Avoid labels that change frequently

Histogram Buckets:
  - Use appropriate buckets for your use case
  - Include +Inf bucket
  - Consider log-scale buckets for wide ranges
```

### Dashboard Design
```yaml
Dashboard Principles:
  - One purpose per dashboard
  - Most important metrics at the top
  - Use consistent time ranges
  - Include documentation panels

Panel Guidelines:
  - Clear, descriptive titles
  - Appropriate visualization types
  - Consistent color schemes
  - Meaningful thresholds and alerts
```

### Alert Design
```yaml
Alert Principles:
  - Alert on symptoms, not causes
  - Make alerts actionable
  - Avoid alert fatigue
  - Test alert rules regularly

Alert Guidelines:
  - Use appropriate severity levels
  - Include runbook links
  - Provide context in alert messages
  - Set reasonable thresholds
```

## SLA & SLI Definitions

### Service Level Indicators (SLIs)
```yaml
Availability:
  - Scheduler uptime: 99.9%
  - API availability: 99.95%
  - Health check success rate: 99.9%

Performance:
  - Deployment latency (P95): < 300 seconds
  - API response time (P95): < 500ms
  - Health check response time: < 5 seconds

Quality:
  - Deployment success rate: > 99%
  - Configuration reload success rate: > 99.9%
  - Data consistency: 100%
```

### Service Level Objectives (SLOs)
```yaml
Monthly SLOs:
  - 99.9% uptime (43 minutes downtime)
  - 99% deployment success rate
  - 95% of deployments complete within 5 minutes
  - 99.5% API availability

Error Budgets:
  - 0.1% error budget for availability
  - 1% error budget for deployment failures
  - 5% error budget for slow deployments
```

---

**Next Steps:**
1. Review monitoring architecture
2. Set up Prometheus and Grafana infrastructure
3. Begin Phase 1 implementation
4. Define alert thresholds
5. Create operational runbooks

*This document should be updated as monitoring requirements evolve and new observability tools are adopted.*