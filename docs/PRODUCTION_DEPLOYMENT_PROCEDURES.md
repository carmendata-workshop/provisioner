# Production Deployment Procedures - OpenTofu Workspace Scheduler

**Status:** 🔴 Critical - Required for Production
**Last Updated:** September 28, 2025

## Overview

This document defines comprehensive production deployment procedures, environment management, and operational guidelines. Currently, the system lacks formal deployment processes and production-grade infrastructure management.

## Current Deployment State

### ✅ Existing Capabilities
- **Basic build system** - Makefile with cross-compilation support
- **GitHub Actions CI/CD** - Automated testing, linting, and building
- **Version management** - Semantic versioning with conventional commits
- **Binary artifacts** - Cross-platform binary generation

### ❌ Missing Capabilities
- **Production infrastructure** - No IaC or automated provisioning
- **Environment management** - No dev/staging/prod environment separation
- **Deployment automation** - No automated deployment pipelines
- **Configuration management** - No environment-specific configuration
- **Release management** - No formal release processes
- **Rollback procedures** - No automated rollback capabilities

## Environment Strategy

### 1. Environment Tiers

#### Environment Definitions
```yaml
Development:
  purpose: Active development and testing
  stability: Unstable
  deployment: Automatic on main branch
  data: Mock/synthetic data
  monitoring: Basic logging
  availability: Best effort

Staging:
  purpose: Pre-production validation
  stability: Stable
  deployment: Manual promotion from dev
  data: Production-like test data
  monitoring: Full monitoring stack
  availability: Business hours

Production:
  purpose: Live customer workloads
  stability: Highly stable
  deployment: Manual promotion with approvals
  data: Real production data
  monitoring: Comprehensive monitoring + alerting
  availability: 99.9% SLA

Disaster Recovery:
  purpose: Production backup environment
  stability: Highly stable
  deployment: Synchronized with production
  data: Production replica
  monitoring: Full monitoring
  availability: Hot standby
```

#### Infrastructure Requirements
```yaml
Development:
  compute: 1x small instance (2 CPU, 4GB RAM)
  database: SQLite or small PostgreSQL
  storage: Local filesystem
  networking: Internal only
  backup: None required

Staging:
  compute: 2x medium instances (4 CPU, 8GB RAM)
  database: PostgreSQL cluster
  storage: Cloud object storage
  networking: Load balancer + internal
  backup: Daily backups

Production:
  compute: 3x large instances (8 CPU, 16GB RAM)
  database: PostgreSQL HA cluster
  storage: Replicated cloud storage
  networking: Global load balancer
  backup: Continuous + point-in-time recovery

Disaster Recovery:
  compute: 3x large instances (standby)
  database: Cross-region replica
  storage: Cross-region replication
  networking: Failover load balancer
  backup: Real-time synchronization
```

### 2. Infrastructure as Code

#### Terraform Infrastructure
```hcl
# environments/production/main.tf
terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket         = "provisioner-terraform-state"
    key            = "environments/production/terraform.tfstate"
    region         = "us-west-2"
    encrypt        = true
    dynamodb_table = "terraform-state-lock"
  }
}

# VPC and networking
module "vpc" {
  source = "../../modules/vpc"

  name               = "provisioner-prod"
  cidr               = "10.0.0.0/16"
  availability_zones = ["us-west-2a", "us-west-2b", "us-west-2c"]

  public_subnets  = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  private_subnets = ["10.0.11.0/24", "10.0.12.0/24", "10.0.13.0/24"]

  enable_nat_gateway = true
  enable_vpn_gateway = false

  tags = {
    Environment = "production"
    Project     = "provisioner"
  }
}

# Application load balancer
module "alb" {
  source = "../../modules/alb"

  name               = "provisioner-prod-alb"
  vpc_id             = module.vpc.vpc_id
  subnet_ids         = module.vpc.public_subnet_ids
  certificate_arn    = aws_acm_certificate.provisioner.arn

  health_check_path  = "/health"
  health_check_port  = 8080

  tags = {
    Environment = "production"
    Project     = "provisioner"
  }
}

# ECS cluster for container orchestration
module "ecs_cluster" {
  source = "../../modules/ecs"

  name                = "provisioner-prod"
  vpc_id              = module.vpc.vpc_id
  subnet_ids          = module.vpc.private_subnet_ids
  target_group_arn    = module.alb.target_group_arn

  # Application configuration
  app_image           = "provisioner:${var.app_version}"
  app_port            = 8080
  app_cpu             = 1024
  app_memory          = 2048
  app_desired_count   = 3

  # Environment variables
  environment_variables = {
    PROVISIONER_CONFIG_DIR = "/etc/provisioner"
    PROVISIONER_STATE_DIR  = "/var/lib/provisioner"
    PROVISIONER_LOG_DIR    = "/var/log/provisioner"
    DATABASE_HOST          = module.rds.endpoint
    DATABASE_NAME          = module.rds.database_name
    ETCD_ENDPOINTS         = join(",", module.etcd.endpoints)
  }

  # Secrets from AWS Secrets Manager
  secrets = {
    DATABASE_PASSWORD = aws_secretsmanager_secret.db_password.arn
    ENCRYPTION_KEY    = aws_secretsmanager_secret.encryption_key.arn
  }

  tags = {
    Environment = "production"
    Project     = "provisioner"
  }
}

# RDS PostgreSQL cluster
module "rds" {
  source = "../../modules/rds"

  identifier                = "provisioner-prod"
  engine                    = "aurora-postgresql"
  engine_version            = "14.9"
  instance_class            = "db.r6g.large"
  allocated_storage         = 100
  max_allocated_storage     = 1000

  database_name             = "provisioner"
  master_username           = "provisioner"
  master_password           = random_password.db_password.result

  vpc_security_group_ids    = [aws_security_group.rds.id]
  db_subnet_group_name      = aws_db_subnet_group.main.name

  backup_retention_period   = 30
  backup_window            = "03:00-04:00"
  maintenance_window       = "Sun:04:00-Sun:05:00"

  deletion_protection      = true
  skip_final_snapshot      = false
  final_snapshot_identifier = "provisioner-prod-final-snapshot"

  performance_insights_enabled = true
  monitoring_interval         = 60

  tags = {
    Environment = "production"
    Project     = "provisioner"
  }
}

# etcd cluster for leader election
module "etcd" {
  source = "../../modules/etcd"

  name               = "provisioner-prod-etcd"
  vpc_id             = module.vpc.vpc_id
  subnet_ids         = module.vpc.private_subnet_ids
  instance_count     = 3
  instance_type      = "t3.medium"

  tags = {
    Environment = "production"
    Project     = "provisioner"
  }
}

# S3 bucket for state backups
resource "aws_s3_bucket" "backups" {
  bucket = "provisioner-prod-backups"

  tags = {
    Environment = "production"
    Project     = "provisioner"
  }
}

resource "aws_s3_bucket_versioning" "backups" {
  bucket = aws_s3_bucket.backups.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_encryption" "backups" {
  bucket = aws_s3_bucket.backups.id

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        kms_master_key_id = aws_kms_key.backup_encryption.arn
        sse_algorithm     = "aws:kms"
      }
    }
  }
}
```

#### Kubernetes Deployment
```yaml
# k8s/production/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: provisioner-prod
  labels:
    environment: production
    project: provisioner

---
# k8s/production/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: provisioner-config
  namespace: provisioner-prod
data:
  config.yaml: |
    server:
      port: 8080
      metrics_port: 9090

    database:
      host: postgresql-ha.provisioner-prod.svc.cluster.local
      port: 5432
      database: provisioner
      ssl_mode: require
      max_connections: 20

    etcd:
      endpoints:
        - etcd-0.etcd.provisioner-prod.svc.cluster.local:2379
        - etcd-1.etcd.provisioner-prod.svc.cluster.local:2379
        - etcd-2.etcd.provisioner-prod.svc.cluster.local:2379

    logging:
      level: info
      format: json

    backup:
      enabled: true
      schedule: "0 */6 * * *"
      retention_days: 30
      storage_bucket: "provisioner-prod-backups"

---
# k8s/production/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: provisioner
  namespace: provisioner-prod
  labels:
    app: provisioner
    version: v1.0.0
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  selector:
    matchLabels:
      app: provisioner
  template:
    metadata:
      labels:
        app: provisioner
        version: v1.0.0
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: provisioner
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
        fsGroup: 1001
      containers:
      - name: provisioner
        image: provisioner:1.0.0
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        - name: metrics
          containerPort: 9090
          protocol: TCP
        env:
        - name: PROVISIONER_CONFIG_FILE
          value: /etc/provisioner/config.yaml
        - name: INSTANCE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: DATABASE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: provisioner-secrets
              key: database-password
        - name: ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: provisioner-secrets
              key: encryption-key
        volumeMounts:
        - name: config
          mountPath: /etc/provisioner
          readOnly: true
        - name: state
          mountPath: /var/lib/provisioner
        - name: logs
          mountPath: /var/log/provisioner
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
      volumes:
      - name: config
        configMap:
          name: provisioner-config
      - name: state
        persistentVolumeClaim:
          claimName: provisioner-state
      - name: logs
        emptyDir: {}
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - provisioner
              topologyKey: kubernetes.io/hostname
```

### 3. CI/CD Pipeline

#### GitHub Actions Workflow
```yaml
# .github/workflows/deploy-production.yml
name: Deploy to Production

on:
  push:
    tags:
      - 'v*'

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  security-scan:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4

    - name: Run security scan
      uses: securecodewarrior/github-action-add-sarif@v1
      with:
        sarif-file: security-scan-results.sarif

    - name: Run dependency check
      run: |
        go install github.com/sonatypecommunity/nancy@latest
        go list -json -deps ./... | nancy sleuth

  build-and-test:
    runs-on: ubuntu-latest
    needs: security-scan
    steps:
    - uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v6
      with:
        go-version: '1.25.1'

    - name: Run tests with coverage
      run: |
        go test -v -race -coverprofile=coverage.out ./...
        go tool cover -html=coverage.out -o coverage.html

    - name: Upload coverage reports
      uses: codecov/codecov-action@v4
      with:
        file: ./coverage.out

    - name: Build application
      run: make build-all

    - name: Run integration tests
      run: make test-integration

  build-container:
    runs-on: ubuntu-latest
    needs: build-and-test
    outputs:
      image-digest: ${{ steps.build.outputs.digest }}
    steps:
    - uses: actions/checkout@v4

    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3

    - name: Log in to Container Registry
      uses: docker/login-action@v3
      with:
        registry: ${{ env.REGISTRY }}
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}

    - name: Extract metadata
      id: meta
      uses: docker/metadata-action@v5
      with:
        images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
        tags: |
          type=ref,event=tag
          type=semver,pattern={{version}}
          type=semver,pattern={{major}}.{{minor}}

    - name: Build and push container image
      id: build
      uses: docker/build-push-action@v5
      with:
        context: .
        platforms: linux/amd64,linux/arm64
        push: true
        tags: ${{ steps.meta.outputs.tags }}
        labels: ${{ steps.meta.outputs.labels }}
        cache-from: type=gha
        cache-to: type=gha,mode=max

  deploy-staging:
    runs-on: ubuntu-latest
    needs: build-container
    environment: staging
    steps:
    - uses: actions/checkout@v4

    - name: Configure AWS credentials
      uses: aws-actions/configure-aws-credentials@v4
      with:
        aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
        aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
        aws-region: us-west-2

    - name: Deploy to staging
      run: |
        # Update ECS service with new image
        aws ecs update-service \
          --cluster provisioner-staging \
          --service provisioner \
          --force-new-deployment \
          --task-definition provisioner-staging:$(aws ecs describe-task-definition \
            --task-definition provisioner-staging \
            --query 'taskDefinition.revision')

    - name: Wait for deployment
      run: |
        aws ecs wait services-stable \
          --cluster provisioner-staging \
          --services provisioner

    - name: Run smoke tests
      run: |
        # Wait for health checks to pass
        sleep 60
        # Run basic smoke tests
        curl -f https://staging.provisioner.example.com/health
        curl -f https://staging.provisioner.example.com/ready

  deploy-production:
    runs-on: ubuntu-latest
    needs: [deploy-staging]
    environment: production
    steps:
    - uses: actions/checkout@v4

    - name: Configure AWS credentials
      uses: aws-actions/configure-aws-credentials@v4
      with:
        aws-access-key-id: ${{ secrets.PROD_AWS_ACCESS_KEY_ID }}
        aws-secret-access-key: ${{ secrets.PROD_AWS_SECRET_ACCESS_KEY }}
        aws-region: us-west-2

    - name: Deploy to production
      run: |
        # Blue-green deployment
        ./scripts/deploy-production.sh ${{ needs.build-container.outputs.image-digest }}

    - name: Verify deployment
      run: |
        ./scripts/verify-production-deployment.sh

    - name: Notify deployment
      uses: 8398a7/action-slack@v3
      with:
        status: ${{ job.status }}
        text: 'Production deployment completed for ${{ github.ref }}'
      env:
        SLACK_WEBHOOK_URL: ${{ secrets.SLACK_WEBHOOK_URL }}
```

### 4. Deployment Scripts

#### Blue-Green Deployment Script
```bash
#!/bin/bash
# scripts/deploy-production.sh

set -euo pipefail

IMAGE_DIGEST="$1"
CLUSTER_NAME="provisioner-prod"
SERVICE_NAME="provisioner"
TASK_FAMILY="provisioner-prod"

echo "Starting blue-green deployment for image: $IMAGE_DIGEST"

# Get current task definition
CURRENT_TASK_DEF=$(aws ecs describe-services \
  --cluster "$CLUSTER_NAME" \
  --services "$SERVICE_NAME" \
  --query 'services[0].taskDefinition' \
  --output text)

echo "Current task definition: $CURRENT_TASK_DEF"

# Create new task definition with new image
NEW_TASK_DEF=$(aws ecs describe-task-definition \
  --task-definition "$CURRENT_TASK_DEF" \
  --query 'taskDefinition' \
  --output json | \
  jq --arg IMAGE "$IMAGE_DIGEST" '.containerDefinitions[0].image = $IMAGE' | \
  jq 'del(.taskDefinitionArn, .revision, .status, .requiresAttributes, .placementConstraints, .compatibilities, .registeredAt, .registeredBy)')

# Register new task definition
NEW_TASK_DEF_ARN=$(echo "$NEW_TASK_DEF" | \
  aws ecs register-task-definition \
  --cli-input-json file:///dev/stdin \
  --query 'taskDefinition.taskDefinitionArn' \
  --output text)

echo "New task definition registered: $NEW_TASK_DEF_ARN"

# Update service to use new task definition
aws ecs update-service \
  --cluster "$CLUSTER_NAME" \
  --service "$SERVICE_NAME" \
  --task-definition "$NEW_TASK_DEF_ARN"

echo "Service update initiated"

# Wait for deployment to complete
echo "Waiting for deployment to stabilize..."
aws ecs wait services-stable \
  --cluster "$CLUSTER_NAME" \
  --services "$SERVICE_NAME"

echo "Deployment completed successfully"

# Verify health checks
echo "Verifying health checks..."
LOAD_BALANCER_DNS=$(aws elbv2 describe-load-balancers \
  --names "provisioner-prod-alb" \
  --query 'LoadBalancers[0].DNSName' \
  --output text)

# Wait for health checks to pass
sleep 30

for i in {1..10}; do
  if curl -f "https://$LOAD_BALANCER_DNS/health"; then
    echo "Health check passed"
    break
  fi

  if [ $i -eq 10 ]; then
    echo "Health check failed after 10 attempts"
    exit 1
  fi

  sleep 10
done

echo "Blue-green deployment completed successfully"
```

#### Rollback Script
```bash
#!/bin/bash
# scripts/rollback-production.sh

set -euo pipefail

CLUSTER_NAME="provisioner-prod"
SERVICE_NAME="provisioner"
ROLLBACK_REVISION="${1:-previous}"

echo "Starting rollback to revision: $ROLLBACK_REVISION"

if [ "$ROLLBACK_REVISION" = "previous" ]; then
  # Get previous task definition
  TASK_DEFINITIONS=$(aws ecs list-task-definitions \
    --family-prefix "provisioner-prod" \
    --status ACTIVE \
    --sort DESC \
    --query 'taskDefinitionArns[1]' \
    --output text)

  ROLLBACK_TASK_DEF="$TASK_DEFINITIONS"
else
  ROLLBACK_TASK_DEF="provisioner-prod:$ROLLBACK_REVISION"
fi

echo "Rolling back to task definition: $ROLLBACK_TASK_DEF"

# Update service to use rollback task definition
aws ecs update-service \
  --cluster "$CLUSTER_NAME" \
  --service "$SERVICE_NAME" \
  --task-definition "$ROLLBACK_TASK_DEF"

# Wait for rollback to complete
echo "Waiting for rollback to complete..."
aws ecs wait services-stable \
  --cluster "$CLUSTER_NAME" \
  --services "$SERVICE_NAME"

# Verify health checks
echo "Verifying health checks after rollback..."
LOAD_BALANCER_DNS=$(aws elbv2 describe-load-balancers \
  --names "provisioner-prod-alb" \
  --query 'LoadBalancers[0].DNSName' \
  --output text)

sleep 30

for i in {1..5}; do
  if curl -f "https://$LOAD_BALANCER_DNS/health"; then
    echo "Rollback health check passed"
    break
  fi

  if [ $i -eq 5 ]; then
    echo "Rollback health check failed"
    exit 1
  fi

  sleep 10
done

echo "Rollback completed successfully"
```

### 5. Configuration Management

#### Environment-Specific Configuration
```yaml
# config/environments/production.yaml
server:
  port: 8080
  metrics_port: 9090
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s

database:
  host: ${DATABASE_HOST}
  port: 5432
  database: provisioner
  username: provisioner
  password: ${DATABASE_PASSWORD}
  ssl_mode: require
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 300s

etcd:
  endpoints: ${ETCD_ENDPOINTS}
  dial_timeout: 5s
  request_timeout: 10s

security:
  auth_provider: oidc
  oidc_issuer: ${OIDC_ISSUER}
  oidc_client_id: ${OIDC_CLIENT_ID}
  oidc_client_secret: ${OIDC_CLIENT_SECRET}
  require_mfa: true
  session_timeout: 24h

backup:
  enabled: true
  schedule: "0 */6 * * *"
  retention_days: 90
  storage_bucket: provisioner-prod-backups
  encryption_key: ${BACKUP_ENCRYPTION_KEY}
  verify_backups: true

monitoring:
  prometheus_enabled: true
  jaeger_endpoint: ${JAEGER_ENDPOINT}
  log_level: info
  log_format: json

rate_limiting:
  enabled: true
  global_limit: 1000
  user_limit: 100
  endpoint_limits:
    deploy_workspace: 10
    destroy_workspace: 10
    list_workspaces: 60
```

#### Secret Management
```yaml
# Using AWS Secrets Manager
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: aws-secrets-manager
  namespace: provisioner-prod
spec:
  provider:
    aws:
      service: SecretsManager
      region: us-west-2
      auth:
        serviceAccount:
          name: provisioner

---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: provisioner-secrets
  namespace: provisioner-prod
spec:
  refreshInterval: 300s
  secretStoreRef:
    name: aws-secrets-manager
    kind: SecretStore
  target:
    name: provisioner-secrets
    creationPolicy: Owner
  data:
  - secretKey: database-password
    remoteRef:
      key: provisioner/prod/database
      property: password
  - secretKey: encryption-key
    remoteRef:
      key: provisioner/prod/encryption
      property: key
  - secretKey: oidc-client-secret
    remoteRef:
      key: provisioner/prod/oidc
      property: client_secret
```

### 6. Monitoring & Alerting Setup

#### Prometheus Configuration
```yaml
# monitoring/prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "provisioner_rules.yml"

scrape_configs:
  - job_name: 'provisioner'
    kubernetes_sd_configs:
    - role: pod
      namespaces:
        names:
        - provisioner-prod
    relabel_configs:
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
      action: keep
      regex: true
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
      action: replace
      target_label: __metrics_path__
      regex: (.+)
    - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
      action: replace
      regex: ([^:]+)(?::\d+)?;(\d+)
      replacement: $1:$2
      target_label: __address__

alerting:
  alertmanagers:
    - kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
          - monitoring
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: alertmanager
```

#### Alert Rules
```yaml
# monitoring/prometheus/provisioner_rules.yml
groups:
- name: provisioner.rules
  rules:
  - alert: ProvisionerDown
    expr: up{job="provisioner"} == 0
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "Provisioner instance is down"
      description: "Provisioner instance {{ $labels.instance }} has been down for more than 5 minutes."

  - alert: HighDeploymentFailureRate
    expr: rate(provisioner_deployments_total{result="failure"}[5m]) / rate(provisioner_deployments_total[5m]) > 0.1
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "High deployment failure rate"
      description: "Deployment failure rate is {{ $value | humanizePercentage }} over the last 5 minutes."

  - alert: DatabaseConnectionFailures
    expr: increase(provisioner_database_connection_failures_total[5m]) > 5
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Database connection failures"
      description: "{{ $value }} database connection failures in the last 5 minutes."
```

### 7. Operational Procedures

#### Deployment Checklist
```yaml
Pre-Deployment:
  - [ ] Security scan passed
  - [ ] All tests passing
  - [ ] Performance tests completed
  - [ ] Staging deployment verified
  - [ ] Database migrations tested
  - [ ] Rollback plan prepared
  - [ ] Change management approval
  - [ ] Monitoring alerts configured

During Deployment:
  - [ ] Deployment initiated
  - [ ] Health checks monitored
  - [ ] Performance metrics watched
  - [ ] Error rates monitored
  - [ ] User impact assessed
  - [ ] Rollback triggered if needed

Post-Deployment:
  - [ ] Deployment verified successful
  - [ ] Smoke tests passed
  - [ ] Performance within baseline
  - [ ] No error rate increase
  - [ ] Documentation updated
  - [ ] Stakeholders notified
  - [ ] Post-deployment review scheduled
```

#### Emergency Procedures
```yaml
Incident Response:
  Critical (P0):
    - Response time: 15 minutes
    - Actions: Page on-call engineer, war room
    - Escalation: 30 minutes to management

  High (P1):
    - Response time: 1 hour
    - Actions: Notify on-call engineer
    - Escalation: 2 hours to management

  Medium (P2):
    - Response time: 4 hours
    - Actions: Create incident ticket
    - Escalation: Next business day

Rollback Criteria:
  - Error rate > 5% for 5 minutes
  - Response time > 2x baseline for 10 minutes
  - Deployment health checks failing
  - Customer complaints about service issues
  - Security vulnerabilities discovered
```

## Implementation Timeline

### Phase 1: Infrastructure Setup (Weeks 1-2)
```yaml
Deliverables:
  - Terraform infrastructure code
  - Environment configuration
  - Basic CI/CD pipeline
  - Container registry setup

Tasks:
  1. Create Terraform modules
  2. Set up AWS/cloud infrastructure
  3. Configure CI/CD pipeline
  4. Set up container registry
  5. Test basic deployment
```

### Phase 2: Advanced Deployment (Weeks 3-4)
```yaml
Deliverables:
  - Blue-green deployment
  - Configuration management
  - Secret management
  - Monitoring setup

Tasks:
  1. Implement blue-green deployment
  2. Set up configuration management
  3. Configure secret management
  4. Deploy monitoring stack
  5. Create deployment scripts
```

### Phase 3: Operations & Documentation (Weeks 5-6)
```yaml
Deliverables:
  - Operational procedures
  - Emergency runbooks
  - Training documentation
  - Automation tools

Tasks:
  1. Create operational procedures
  2. Write emergency runbooks
  3. Document deployment processes
  4. Build automation tools
  5. Train operations team
```

---

**Next Steps:**
1. Review deployment strategy
2. Set up cloud infrastructure
3. Begin Phase 1 implementation
4. Create deployment automation
5. Document operational procedures

*This document should be updated as deployment practices evolve and new requirements emerge.*