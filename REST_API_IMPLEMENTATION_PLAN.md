# REST API Implementation Plan for OpenTofu Environment Scheduler

## Overview

This plan outlines adding REST API functionality to the existing `provisioner` binary, creating a unified daemon that provides both scheduled environment management and HTTP API access to all features.

## Architecture Decision

- **Single Binary**: Extend existing `provisioner` binary to include optional API server
- **Combined Mode**: API server and scheduler daemon run in the same process
- **Shared State**: Both HTTP API and scheduler use the same state files and business logic
- **No Versioning in URLs**: API versioning handled via request headers when needed

## Current Binary Structure

```bash
./bin/provisioner          # Scheduler daemon only (current)
./bin/environmentctl        # Environment CLI
./bin/templatectl          # Template CLI
```

## New Binary Structure

```bash
./bin/provisioner          # Scheduler daemon + Optional API server (enhanced)
./bin/environmentctl        # Environment CLI (unchanged)
./bin/templatectl          # Template CLI (unchanged)
```

## API Server Modes

### Mode 1: Scheduler Only (Current Behavior)
```bash
./bin/provisioner
# Runs CRON scheduler daemon only
```

### Mode 2: API Only
```bash
./bin/provisioner --api-only --port 8080
# Runs HTTP API server only (no scheduling)
```

### Mode 3: Combined (Recommended)
```bash
./bin/provisioner --api --port 8080
# Runs both scheduler daemon and API server
```

## REST API Endpoints

### Health and System
```http
GET /health                    # Health check and basic status
GET /version                   # Version information
GET /scheduler/status          # Scheduler daemon status and statistics
POST /scheduler/reload         # Reload configuration (trigger hot-reload)
```

### Environment Operations
```http
GET /environments              # List all environments
POST /environments             # Create new environment
GET /environments/{name}       # Get environment details and status
PUT /environments/{name}       # Update environment configuration
DELETE /environments/{name}    # Delete environment
POST /environments/{name}/validate # Validate environment configuration

POST /environments/{name}/deploy   # Manual deploy environment
POST /environments/{name}/destroy  # Manual destroy environment
GET /environments/{name}/logs      # Get environment logs
```

### Template Operations
```http
GET /templates                 # List all templates
POST /templates                # Add new template
GET /templates/{name}          # Get template details
PUT /templates/{name}          # Update template
DELETE /templates/{name}       # Remove template
POST /templates/{name}/validate # Validate template
POST /templates/{name}/refresh  # Refresh template from source
```

### State and Monitoring
```http
GET /state                     # Get current scheduler state
GET /metrics                   # Basic metrics and statistics
```

## Implementation Plan

### Phase 1: HTTP Server Foundation

#### 1.1 Create API Package Structure
```
pkg/api/
├── server.go          # HTTP server setup and lifecycle
├── handlers/
│   ├── health.go      # Health and system endpoints
│   ├── environments.go # Environment management endpoints
│   ├── templates.go   # Template management endpoints
│   └── scheduler.go   # Scheduler control endpoints
├── middleware/
│   ├── logging.go     # Request logging
│   ├── cors.go        # CORS headers
│   └── recovery.go    # Panic recovery
├── types.go           # API request/response types
└── routes.go          # Route definitions and setup
```

#### 1.2 Enhance Main Binary
```go
// cmd/provisioner/main.go
func main() {
    var apiEnabled = flag.Bool("api", false, "Enable API server alongside scheduler")
    var apiOnly = flag.Bool("api-only", false, "Run API server only (no scheduler)")
    var apiPort = flag.Int("port", 8080, "API server port")

    if *apiOnly {
        runAPIServer(*apiPort)
    } else if *apiEnabled {
        go runScheduler()
        runAPIServer(*apiPort)
    } else {
        runScheduler() // Current behavior
    }
}
```

### Phase 2: Core API Implementation

#### 2.1 Server Setup
```go
// pkg/api/server.go
type Server struct {
    scheduler *scheduler.Scheduler
    templates *template.Manager
    router    *http.ServeMux
    port      int
}

func NewServer(scheduler *scheduler.Scheduler, port int) *Server {
    server := &Server{
        scheduler: scheduler,
        templates: template.NewManager(getTemplatesDir()),
        router:    http.NewServeMux(),
        port:      port,
    }

    server.setupRoutes()
    server.setupMiddleware()
    return server
}
```

#### 2.2 Environment Handlers
```go
// pkg/api/handlers/environments.go
type EnvironmentHandler struct {
    scheduler *scheduler.Scheduler
}

func (h *EnvironmentHandler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
    // Use scheduler.LoadEnvironments() and return JSON
}

func (h *EnvironmentHandler) DeployEnvironment(w http.ResponseWriter, r *http.Request) {
    envName := extractEnvName(r.URL.Path)
    err := h.scheduler.ManualDeploy(envName)
    // Return appropriate HTTP status and JSON response
}
```

#### 2.3 Template Handlers
```go
// pkg/api/handlers/templates.go
type TemplateHandler struct {
    manager *template.Manager
}

func (h *TemplateHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
    templates, err := h.manager.ListTemplates()
    // Return JSON response
}
```

### Phase 3: Request/Response Types

#### 3.1 API Types
```go
// pkg/api/types.go
type EnvironmentResponse struct {
    Name        string    `json:"name"`
    Enabled     bool      `json:"enabled"`
    Status      string    `json:"status"`
    Template    string    `json:"template,omitempty"`
    Description string    `json:"description,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type CreateEnvironmentRequest struct {
    Name           string `json:"name"`
    Template       string `json:"template,omitempty"`
    Description    string `json:"description,omitempty"`
    DeploySchedule string `json:"deploy_schedule,omitempty"`
    DestroySchedule string `json:"destroy_schedule,omitempty"`
    Enabled        bool   `json:"enabled"`
}

type TemplateResponse struct {
    Name        string    `json:"name"`
    SourceURL   string    `json:"source_url"`
    SourceRef   string    `json:"source_ref"`
    Description string    `json:"description,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

#### 3.2 Error Responses
```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code,omitempty"`
    Details string `json:"details,omitempty"`
}
```

### Phase 4: Middleware and Cross-Cutting Concerns

#### 4.1 Request Logging
```go
// pkg/api/middleware/logging.go
func RequestLogging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        duration := time.Since(start)

        log.Printf("API %s %s %v", r.Method, r.URL.Path, duration)
    })
}
```

#### 4.2 CORS Support
```go
// pkg/api/middleware/cors.go
func CORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### Phase 5: Integration with Existing Code

#### 5.1 Reuse Existing Business Logic
- Environment operations use existing `pkg/environment` functions
- Template operations use existing `pkg/template` methods
- Manual deploy/destroy use existing `scheduler.ManualDeploy()` and `scheduler.ManualDestroy()`
- Status and logs use existing `scheduler.ShowStatus()` and `scheduler.ShowLogs()`

#### 5.2 Shared State Management
- API server reads from same state files as scheduler
- No state synchronization issues since both run in same process
- OpenTofu operations use same workspace directories

#### 5.3 Configuration Loading
- API server uses same environment discovery as CLI tools
- Template management uses same registry.json file
- All tools continue to work unchanged

## Example API Usage

### Create Environment
```bash
curl -X POST http://localhost:8080/environments \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dev-api",
    "template": "web-app",
    "description": "Development environment via API",
    "deploy_schedule": "0 9 * * 1-5",
    "destroy_schedule": "0 18 * * 1-5",
    "enabled": true
  }'
```

### Deploy Environment
```bash
curl -X POST http://localhost:8080/environments/dev-api/deploy
```

### Get Environment Status
```bash
curl http://localhost:8080/environments/dev-api
```

### List All Environments
```bash
curl http://localhost:8080/environments
```

## Benefits

1. **Code Reuse**: API wraps existing CLI business logic with zero duplication
2. **Consistency**: API and CLI use identical code paths and state
3. **Simple Deployment**: Single binary with optional API mode
4. **Backwards Compatibility**: All existing CLI tools continue to work unchanged
5. **Flexible Usage**: Can run API-only, scheduler-only, or combined
6. **Real-time Accuracy**: API reflects actual scheduler state immediately

## Implementation Timeline

- **Week 1**: Phase 1 - HTTP server foundation and basic structure
- **Week 2**: Phase 2 - Core environment and template endpoints
- **Week 3**: Phase 3 - Request/response types and error handling
- **Week 4**: Phase 4 - Middleware, logging, and polish
- **Week 5**: Integration testing and documentation

## Dependencies

- **Standard Library Only**: Uses `net/http`, `encoding/json`, `log` (no external dependencies)
- **Existing Packages**: Builds on `pkg/scheduler`, `pkg/template`, `pkg/environment`
- **Minimal Changes**: Main changes in `cmd/provisioner/main.go` and new `pkg/api/` package

This plan maintains the existing architecture while providing comprehensive REST API access to all provisioner functionality, enabling programmatic access and future web UI development.