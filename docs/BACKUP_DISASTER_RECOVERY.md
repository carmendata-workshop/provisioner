# Backup & Disaster Recovery Plan - OpenTofu Workspace Scheduler

**Status:** 🔴 Critical - Required for Production
**Last Updated:** September 28, 2025

## Overview

This document defines comprehensive backup strategies and disaster recovery procedures for production deployment. Currently, the system has **no backup mechanisms** and relies on fragile local file storage, making it vulnerable to data loss.

## Current Data Protection State

### ❌ Critical Gaps
- **No automated backups** - State files could be lost permanently
- **Local file storage** - Single point of failure for all state data
- **No disaster recovery** - No documented recovery procedures
- **No backup validation** - No testing of backup integrity
- **No cross-region replication** - Vulnerable to regional disasters
- **No backup monitoring** - No alerts for backup failures

### 📊 Data Assets to Protect
```yaml
Critical Data:
  - Workspace state (scheduler.json)
  - Job execution state (jobs.json)
  - OpenTofu state files (terraform.tfstate)
  - Configuration files (config.json per workspace)
  - Template definitions and metadata
  - Audit logs and security events

Data Volumes:
  - State files: < 10MB (typical)
  - OpenTofu states: 1-100MB per workspace
  - Logs: 1-10GB per month
  - Templates: 10-100MB per template
```

## Backup Strategy

### 1. Recovery Objectives

#### RTO/RPO Targets
```yaml
Critical Systems:
  RTO (Recovery Time Objective): < 15 minutes
  RPO (Recovery Point Objective): < 5 minutes
  Backup Frequency: Continuous + Hourly snapshots
  Retention: 90 days

Important Systems:
  RTO: < 1 hour
  RPO: < 30 minutes
  Backup Frequency: Every 4 hours
  Retention: 30 days

Supporting Systems:
  RTO: < 4 hours
  RPO: < 4 hours
  Backup Frequency: Daily
  Retention: 7 days
```

#### Data Classification
```yaml
Critical (Tier 1):
  - Workspace deployment states
  - Active OpenTofu state files
  - Security audit logs
  - Configuration data

Important (Tier 2):
  - Job execution history
  - Historical state data
  - Performance metrics
  - Application logs

Supporting (Tier 3):
  - Template cache
  - Temporary files
  - Debug logs
```

### 2. Backup Architecture

#### Multi-Tier Backup Strategy
```yaml
Tier 1 - Real-time Replication:
  Method: Database synchronous replication
  Target: Secondary region
  RPO: 0 seconds
  RTO: < 5 minutes

Tier 2 - Continuous Backup:
  Method: Write-ahead log shipping
  Target: Cloud storage (S3/GCS)
  RPO: < 5 minutes
  RTO: < 15 minutes

Tier 3 - Snapshot Backup:
  Method: Scheduled snapshots
  Target: Multiple cloud regions
  RPO: 1 hour
  RTO: < 1 hour

Tier 4 - Archive Backup:
  Method: Daily archive to cold storage
  Target: Glacier/Archive storage
  RPO: 24 hours
  RTO: 12-48 hours
```

#### Backup Infrastructure
```yaml
Primary Database:
  Type: PostgreSQL with streaming replication
  Backup Method: pg_basebackup + WAL archiving
  Frequency: Continuous WAL + Daily base backup
  Retention: 30 days

State Files:
  Type: File-system snapshots
  Backup Method: rsync to object storage
  Frequency: Every 15 minutes
  Retention: 7 days

Configuration:
  Type: Git repository with automation
  Backup Method: Git push to multiple remotes
  Frequency: On every change
  Retention: Unlimited (version control)

Logs:
  Type: Log aggregation to central storage
  Backup Method: Fluentd/Filebeat to Elasticsearch
  Frequency: Real-time streaming
  Retention: 90 days
```

### 3. Backup Implementation

#### Database Backup Service
```go
// Database backup service
type DatabaseBackupService struct {
    db            *sql.DB
    config        BackupConfig
    storage       StorageService
    scheduler     *cron.Cron
    metrics       *BackupMetrics
}

type BackupConfig struct {
    Schedule         string        `json:"schedule"`          // Cron expression
    RetentionDays    int          `json:"retention_days"`
    StorageBucket    string        `json:"storage_bucket"`
    CompressionLevel int          `json:"compression_level"`
    EncryptionKey    string        `json:"encryption_key"`
    VerifyBackups    bool         `json:"verify_backups"`
}

type BackupMetadata struct {
    ID                string    `json:"id"`
    Timestamp         time.Time `json:"timestamp"`
    Size              int64     `json:"size"`
    CompressedSize    int64     `json:"compressed_size"`
    Checksum          string    `json:"checksum"`
    EncryptionKey     string    `json:"encryption_key"`
    DatabaseVersion   string    `json:"database_version"`
    BackupType        string    `json:"backup_type"`      // "full", "incremental"
    Status            string    `json:"status"`           // "in_progress", "completed", "failed"
    RestorationTested bool      `json:"restoration_tested"`
}

func NewDatabaseBackupService(db *sql.DB, config BackupConfig) *DatabaseBackupService {
    service := &DatabaseBackupService{
        db:        db,
        config:    config,
        storage:   NewCloudStorageService(config.StorageBucket),
        scheduler: cron.New(),
        metrics:   NewBackupMetrics(),
    }

    // Schedule regular backups
    service.scheduler.AddFunc(config.Schedule, service.performBackup)
    service.scheduler.Start()

    return service
}

func (b *DatabaseBackupService) performBackup() error {
    backupID := generateBackupID()

    b.metrics.backupsStarted.Inc()
    start := time.Now()

    // Create backup metadata
    metadata := BackupMetadata{
        ID:            backupID,
        Timestamp:     start,
        BackupType:    "full",
        Status:        "in_progress",
        DatabaseVersion: b.getDatabaseVersion(),
    }

    // Perform database dump
    dumpPath, err := b.createDatabaseDump(backupID)
    if err != nil {
        b.metrics.backupsFailed.Inc()
        return fmt.Errorf("failed to create database dump: %w", err)
    }
    defer os.Remove(dumpPath)

    // Compress backup
    compressedPath, err := b.compressBackup(dumpPath)
    if err != nil {
        b.metrics.backupsFailed.Inc()
        return fmt.Errorf("failed to compress backup: %w", err)
    }
    defer os.Remove(compressedPath)

    // Calculate checksum
    checksum, err := b.calculateChecksum(compressedPath)
    if err != nil {
        return fmt.Errorf("failed to calculate checksum: %w", err)
    }

    // Encrypt backup
    encryptedPath, err := b.encryptBackup(compressedPath)
    if err != nil {
        b.metrics.backupsFailed.Inc()
        return fmt.Errorf("failed to encrypt backup: %w", err)
    }
    defer os.Remove(encryptedPath)

    // Upload to cloud storage
    if err := b.storage.Upload(encryptedPath, fmt.Sprintf("backups/database/%s.enc", backupID)); err != nil {
        b.metrics.backupsFailed.Inc()
        return fmt.Errorf("failed to upload backup: %w", err)
    }

    // Update metadata
    fileInfo, _ := os.Stat(dumpPath)
    encryptedInfo, _ := os.Stat(encryptedPath)

    metadata.Size = fileInfo.Size()
    metadata.CompressedSize = encryptedInfo.Size()
    metadata.Checksum = checksum
    metadata.Status = "completed"

    // Store metadata
    if err := b.storeBackupMetadata(metadata); err != nil {
        return fmt.Errorf("failed to store backup metadata: %w", err)
    }

    // Verify backup if configured
    if b.config.VerifyBackups {
        if err := b.verifyBackup(metadata); err != nil {
            logging.LogSystemd("Backup verification failed: %v", err)
        }
    }

    duration := time.Since(start)
    b.metrics.backupDuration.Observe(duration.Seconds())
    b.metrics.backupsCompleted.Inc()
    b.metrics.lastBackupSize.Set(float64(metadata.CompressedSize))

    logging.LogSystemd("Database backup completed: %s (size: %d bytes, duration: %v)",
        backupID, metadata.CompressedSize, duration)

    // Clean up old backups
    go b.cleanupOldBackups()

    return nil
}

func (b *DatabaseBackupService) createDatabaseDump(backupID string) (string, error) {
    outputPath := fmt.Sprintf("/tmp/backup_%s.sql", backupID)

    cmd := exec.Command("pg_dump",
        "-h", b.config.DatabaseHost,
        "-p", strconv.Itoa(b.config.DatabasePort),
        "-U", b.config.DatabaseUser,
        "-d", b.config.DatabaseName,
        "-f", outputPath,
        "--verbose",
        "--no-password",
    )

    // Set environment variables
    cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", b.config.DatabasePassword))

    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("pg_dump failed: %w, output: %s", err, output)
    }

    return outputPath, nil
}
```

#### State File Backup Service
```go
// State file backup service
type StateFileBackupService struct {
    config      StateBackupConfig
    storage     StorageService
    scheduler   *cron.Cron
    watcher     *fsnotify.Watcher
    metrics     *BackupMetrics
}

type StateBackupConfig struct {
    StateDir        string   `json:"state_dir"`
    BackupSchedule  string   `json:"backup_schedule"`
    WatchFiles      bool     `json:"watch_files"`
    StorageBucket   string   `json:"storage_bucket"`
    RetentionDays   int      `json:"retention_days"`
    IncludePatterns []string `json:"include_patterns"`
    ExcludePatterns []string `json:"exclude_patterns"`
}

func NewStateFileBackupService(config StateBackupConfig) (*StateFileBackupService, error) {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, err
    }

    service := &StateFileBackupService{
        config:    config,
        storage:   NewCloudStorageService(config.StorageBucket),
        scheduler: cron.New(),
        watcher:   watcher,
        metrics:   NewBackupMetrics(),
    }

    // Schedule regular backups
    service.scheduler.AddFunc(config.BackupSchedule, service.performStateBackup)
    service.scheduler.Start()

    // Set up file watching if enabled
    if config.WatchFiles {
        service.setupFileWatcher()
    }

    return service, nil
}

func (s *StateFileBackupService) performStateBackup() error {
    backupID := generateBackupID()
    start := time.Now()

    s.metrics.stateBackupsStarted.Inc()

    // Create temporary directory for backup
    tempDir, err := os.MkdirTemp("", fmt.Sprintf("state_backup_%s", backupID))
    if err != nil {
        return err
    }
    defer os.RemoveAll(tempDir)

    // Copy state files to temporary directory
    copiedFiles, err := s.copyStateFiles(tempDir)
    if err != nil {
        s.metrics.stateBackupsFailed.Inc()
        return fmt.Errorf("failed to copy state files: %w", err)
    }

    // Create archive
    archivePath := filepath.Join(tempDir, fmt.Sprintf("%s.tar.gz", backupID))
    if err := s.createArchive(tempDir, archivePath, copiedFiles); err != nil {
        s.metrics.stateBackupsFailed.Inc()
        return fmt.Errorf("failed to create archive: %w", err)
    }

    // Upload to cloud storage
    cloudPath := fmt.Sprintf("backups/state/%s.tar.gz", backupID)
    if err := s.storage.Upload(archivePath, cloudPath); err != nil {
        s.metrics.stateBackupsFailed.Inc()
        return fmt.Errorf("failed to upload state backup: %w", err)
    }

    // Record backup metadata
    metadata := StateBackupMetadata{
        ID:        backupID,
        Timestamp: start,
        Files:     copiedFiles,
        CloudPath: cloudPath,
        Size:      getFileSize(archivePath),
    }

    if err := s.storeStateBackupMetadata(metadata); err != nil {
        return fmt.Errorf("failed to store backup metadata: %w", err)
    }

    duration := time.Since(start)
    s.metrics.stateBackupDuration.Observe(duration.Seconds())
    s.metrics.stateBackupsCompleted.Inc()

    logging.LogSystemd("State backup completed: %s (%d files, duration: %v)",
        backupID, len(copiedFiles), duration)

    return nil
}

func (s *StateFileBackupService) setupFileWatcher() {
    go func() {
        for {
            select {
            case event, ok := <-s.watcher.Events:
                if !ok {
                    return
                }

                if event.Op&fsnotify.Write == fsnotify.Write ||
                   event.Op&fsnotify.Create == fsnotify.Create {
                    // File changed, trigger backup after delay
                    go s.scheduleIncrementalBackup(event.Name)
                }

            case err, ok := <-s.watcher.Errors:
                if !ok {
                    return
                }
                logging.LogSystemd("File watcher error: %v", err)
            }
        }
    }()

    // Add state directory to watcher
    s.watcher.Add(s.config.StateDir)
}
```

### 4. Disaster Recovery Procedures

#### Recovery Scenarios
```yaml
Scenario 1: Single File Corruption
  Impact: Low
  RTO: < 5 minutes
  Procedure:
    1. Identify corrupted file from monitoring alerts
    2. Stop affected service temporarily
    3. Restore file from latest backup
    4. Verify file integrity
    5. Restart service

Scenario 2: Database Failure
  Impact: Medium
  RTO: < 15 minutes
  Procedure:
    1. Promote read replica to primary
    2. Update application connection strings
    3. Verify data consistency
    4. Resume operations

Scenario 3: Complete System Failure
  Impact: High
  RTO: < 1 hour
  Procedure:
    1. Activate disaster recovery site
    2. Restore database from backup
    3. Restore state files from backup
    4. Deploy application infrastructure
    5. Verify system functionality
    6. Update DNS to point to DR site

Scenario 4: Regional Disaster
  Impact: Critical
  RTO: < 4 hours
  Procedure:
    1. Activate cross-region DR procedures
    2. Restore from geographically distributed backups
    3. Rebuild infrastructure in alternate region
    4. Update global load balancer configuration
    5. Notify stakeholders of region switch
```

#### Recovery Automation
```go
// Disaster recovery service
type DisasterRecoveryService struct {
    backupService    *DatabaseBackupService
    stateService     *StateFileBackupService
    infraService     *InfrastructureService
    notificationSvc  *NotificationService
    config           DRConfig
}

type DRConfig struct {
    PrimaryRegion    string   `json:"primary_region"`
    BackupRegions    []string `json:"backup_regions"`
    AutoFailover     bool     `json:"auto_failover"`
    FailoverThreshold int     `json:"failover_threshold"`  // consecutive failures
    NotificationChannels []string `json:"notification_channels"`
}

func (dr *DisasterRecoveryService) InitiateDisasterRecovery(scenario string) error {
    logging.LogSystemd("Initiating disaster recovery for scenario: %s", scenario)

    // Send notification
    dr.notificationSvc.SendAlert(fmt.Sprintf("Disaster recovery initiated: %s", scenario))

    switch scenario {
    case "database_failure":
        return dr.recoverFromDatabaseFailure()
    case "system_failure":
        return dr.recoverFromSystemFailure()
    case "regional_disaster":
        return dr.recoverFromRegionalDisaster()
    default:
        return fmt.Errorf("unknown disaster recovery scenario: %s", scenario)
    }
}

func (dr *DisasterRecoveryService) recoverFromDatabaseFailure() error {
    // 1. Promote read replica
    if err := dr.promoteReadReplica(); err != nil {
        return fmt.Errorf("failed to promote read replica: %w", err)
    }

    // 2. Update application configuration
    if err := dr.updateDatabaseConfig(); err != nil {
        return fmt.Errorf("failed to update database config: %w", err)
    }

    // 3. Restart application services
    if err := dr.restartServices(); err != nil {
        return fmt.Errorf("failed to restart services: %w", err)
    }

    // 4. Verify recovery
    if err := dr.verifyRecovery(); err != nil {
        return fmt.Errorf("recovery verification failed: %w", err)
    }

    logging.LogSystemd("Database failure recovery completed successfully")
    dr.notificationSvc.SendAlert("Database failure recovery completed")

    return nil
}

func (dr *DisasterRecoveryService) recoverFromSystemFailure() error {
    // 1. Restore database from latest backup
    latestBackup, err := dr.backupService.GetLatestBackup()
    if err != nil {
        return fmt.Errorf("failed to find latest backup: %w", err)
    }

    if err := dr.backupService.RestoreDatabase(latestBackup.ID); err != nil {
        return fmt.Errorf("failed to restore database: %w", err)
    }

    // 2. Restore state files
    latestStateBackup, err := dr.stateService.GetLatestBackup()
    if err != nil {
        return fmt.Errorf("failed to find latest state backup: %w", err)
    }

    if err := dr.stateService.RestoreStateFiles(latestStateBackup.ID); err != nil {
        return fmt.Errorf("failed to restore state files: %w", err)
    }

    // 3. Deploy infrastructure
    if err := dr.infraService.DeployInfrastructure(); err != nil {
        return fmt.Errorf("failed to deploy infrastructure: %w", err)
    }

    // 4. Start application services
    if err := dr.startApplicationServices(); err != nil {
        return fmt.Errorf("failed to start application services: %w", err)
    }

    // 5. Verify recovery
    if err := dr.verifyFullRecovery(); err != nil {
        return fmt.Errorf("full recovery verification failed: %w", err)
    }

    logging.LogSystemd("System failure recovery completed successfully")
    dr.notificationSvc.SendAlert("System failure recovery completed")

    return nil
}
```

### 5. Backup Monitoring & Alerting

#### Backup Health Metrics
```yaml
Backup Success Rate:
  metric: backup_success_rate
  threshold: > 99%
  alert_severity: critical

Backup Duration:
  metric: backup_duration_seconds
  threshold: < 1800 (30 minutes)
  alert_severity: warning

Time Since Last Backup:
  metric: seconds_since_last_backup
  threshold: < 7200 (2 hours)
  alert_severity: critical

Backup Size Growth:
  metric: backup_size_growth_rate
  threshold: < 50% week-over-week
  alert_severity: warning

Restore Test Success:
  metric: restore_test_success_rate
  threshold: > 95%
  alert_severity: critical
```

#### Monitoring Implementation
```go
// Backup monitoring service
type BackupMonitoringService struct {
    metrics     *BackupMetrics
    alerting    *AlertingService
    scheduler   *cron.Cron
}

type BackupMetrics struct {
    backupsTotal        prometheus.Counter
    backupsSuccessful   prometheus.Counter
    backupDuration      prometheus.Histogram
    backupSize          prometheus.Gauge
    timeSinceLastBackup prometheus.Gauge
    restoreTestResults  prometheus.Counter
}

func NewBackupMonitoringService() *BackupMonitoringService {
    metrics := &BackupMetrics{
        backupsTotal: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "backup_operations_total",
            Help: "Total number of backup operations",
        }),
        backupsSuccessful: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "backup_operations_successful_total",
            Help: "Total number of successful backup operations",
        }),
        backupDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
            Name: "backup_duration_seconds",
            Help: "Duration of backup operations",
            Buckets: []float64{60, 300, 600, 1800, 3600},
        }),
        backupSize: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "backup_size_bytes",
            Help: "Size of latest backup in bytes",
        }),
        timeSinceLastBackup: prometheus.NewGauge(prometheus.GaugeOpts{
            Name: "seconds_since_last_backup",
            Help: "Seconds since last successful backup",
        }),
        restoreTestResults: prometheus.NewCounter(prometheus.CounterOpts{
            Name: "restore_test_results_total",
            Help: "Results of restore tests",
        }),
    }

    service := &BackupMonitoringService{
        metrics:   metrics,
        alerting:  NewAlertingService(),
        scheduler: cron.New(),
    }

    // Schedule backup health checks
    service.scheduler.AddFunc("*/5 * * * *", service.checkBackupHealth)
    service.scheduler.Start()

    return service
}

func (bm *BackupMonitoringService) checkBackupHealth() {
    // Check time since last backup
    lastBackupTime, err := bm.getLastBackupTime()
    if err != nil {
        logging.LogSystemd("Failed to get last backup time: %v", err)
        return
    }

    timeSinceLastBackup := time.Since(lastBackupTime).Seconds()
    bm.metrics.timeSinceLastBackup.Set(timeSinceLastBackup)

    // Alert if backup is overdue
    if timeSinceLastBackup > 7200 { // 2 hours
        bm.alerting.SendAlert("critical", "Backup overdue",
            fmt.Sprintf("Last backup was %.1f hours ago", timeSinceLastBackup/3600))
    }

    // Check backup success rate
    successRate := bm.getBackupSuccessRate()
    if successRate < 0.99 {
        bm.alerting.SendAlert("critical", "Low backup success rate",
            fmt.Sprintf("Backup success rate is %.2f%%", successRate*100))
    }
}
```

### 6. Backup Testing & Validation

#### Automated Restore Testing
```yaml
Test Schedule: Weekly
Test Scope: Random backup selection
Test Environment: Isolated test cluster
Success Criteria:
  - Restore completes without errors
  - Data integrity verification passes
  - Application functionality verified
  - Performance within acceptable bounds

Test Types:
  Full Restore: Complete system recovery
  Partial Restore: Individual workspace recovery
  Point-in-Time: Recovery to specific timestamp
  Cross-Region: Restore in different region
```

#### Test Implementation
```go
// Backup testing service
type BackupTestingService struct {
    testScheduler    *cron.Cron
    testEnvironment  *TestEnvironment
    backupService    *DatabaseBackupService
    stateService     *StateFileBackupService
    metrics          *TestMetrics
}

func (bt *BackupTestingService) performRestoreTest() error {
    testID := generateTestID()
    start := time.Now()

    logging.LogSystemd("Starting backup restore test: %s", testID)

    // Select random backup to test
    backup, err := bt.selectRandomBackup()
    if err != nil {
        return fmt.Errorf("failed to select backup for testing: %w", err)
    }

    // Create isolated test environment
    testEnv, err := bt.testEnvironment.Create(testID)
    if err != nil {
        return fmt.Errorf("failed to create test environment: %w", err)
    }
    defer testEnv.Cleanup()

    // Restore backup to test environment
    if err := bt.restoreBackupToTestEnv(backup, testEnv); err != nil {
        bt.metrics.restoreTestsFailed.Inc()
        return fmt.Errorf("failed to restore backup to test environment: %w", err)
    }

    // Verify data integrity
    if err := bt.verifyDataIntegrity(testEnv); err != nil {
        bt.metrics.restoreTestsFailed.Inc()
        return fmt.Errorf("data integrity verification failed: %w", err)
    }

    // Test application functionality
    if err := bt.testApplicationFunctionality(testEnv); err != nil {
        bt.metrics.restoreTestsFailed.Inc()
        return fmt.Errorf("application functionality test failed: %w", err)
    }

    duration := time.Since(start)
    bt.metrics.restoreTestDuration.Observe(duration.Seconds())
    bt.metrics.restoreTestsSuccessful.Inc()

    logging.LogSystemd("Backup restore test completed successfully: %s (duration: %v)",
        testID, duration)

    return nil
}
```

## Implementation Timeline

### Phase 1: Basic Backup (Weeks 1-2)
```yaml
Deliverables:
  - Database backup service
  - State file backup service
  - Cloud storage integration
  - Basic backup scheduling

Tasks:
  1. Implement database backup service
  2. Create state file backup service
  3. Set up cloud storage (S3/GCS)
  4. Configure backup scheduling
  5. Test basic backup functionality
```

### Phase 2: Advanced Backup (Weeks 3-4)
```yaml
Deliverables:
  - Backup encryption and compression
  - Backup metadata management
  - File change monitoring
  - Backup verification

Tasks:
  1. Add backup encryption
  2. Implement backup compression
  3. Create backup metadata system
  4. Set up file change monitoring
  5. Add backup verification
```

### Phase 3: Disaster Recovery (Weeks 5-6)
```yaml
Deliverables:
  - Automated restore procedures
  - Disaster recovery automation
  - Cross-region replication
  - Recovery testing framework

Tasks:
  1. Implement automated restore
  2. Create DR automation
  3. Set up cross-region replication
  4. Build recovery testing framework
  5. Document DR procedures
```

### Phase 4: Monitoring & Testing (Weeks 7-8)
```yaml
Deliverables:
  - Backup monitoring and alerting
  - Automated restore testing
  - Performance optimization
  - Documentation and runbooks

Tasks:
  1. Implement backup monitoring
  2. Set up automated testing
  3. Optimize backup performance
  4. Create operational runbooks
  5. Train operations team
```

---

**Next Steps:**
1. Review backup and DR strategy
2. Set up cloud storage infrastructure
3. Begin Phase 1 implementation
4. Plan DR testing procedures
5. Document recovery runbooks

*This document should be reviewed and updated quarterly to ensure DR procedures remain current and effective.*