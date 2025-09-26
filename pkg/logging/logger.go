package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Logger handles both systemd and per-workspace file logging
type Logger struct {
	systemdLogger    *log.Logger
	workspaceLoggers map[string]*log.Logger
	logDir           string
	mu               sync.RWMutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// GetLogger returns the singleton logger instance
func GetLogger() *Logger {
	once.Do(func() {
		logDir := getLogDir()

		defaultLogger = &Logger{
			// Systemd logger without timestamps (journalctl adds them)
			systemdLogger:    log.New(os.Stdout, "", 0),
			workspaceLoggers: make(map[string]*log.Logger),
			logDir:           logDir,
		}

		// Ensure log directory exists
		if err := os.MkdirAll(logDir, 0755); err != nil && logDir != "/tmp" {
			defaultLogger.systemdLogger.Printf("Warning: failed to create log directory %s: %v", logDir, err)
		}
	})
	return defaultLogger
}

// getWorkspaceLogger gets or creates a logger for a specific workspace
func (l *Logger) getWorkspaceLogger(workspaceName string) *log.Logger {
	l.mu.RLock()
	if logger, exists := l.workspaceLoggers[workspaceName]; exists {
		l.mu.RUnlock()
		return logger
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double-check after acquiring write lock
	if logger, exists := l.workspaceLoggers[workspaceName]; exists {
		return logger
	}

	// Create log file for workspace
	logFile := filepath.Join(l.logDir, fmt.Sprintf("%s.log", workspaceName))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Attempt to create the log directory if it doesn't exist
		if os.IsNotExist(err) {
			if mkdirErr := os.MkdirAll(l.logDir, 0755); mkdirErr == nil {
				// Retry file creation after creating directory
				file, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
				if err == nil {
					// Success after creating directory
					logger := log.New(file, "", log.LstdFlags)
					l.workspaceLoggers[workspaceName] = logger
					return logger
				}
			}
		}

		// Failed to create file even after attempting directory creation
		// In test workspaces, fall back silently to systemd logger
		if l.logDir != "/tmp" { // Only log warnings for production paths
			l.systemdLogger.Printf("Warning: failed to create log file %s: %v", logFile, err)
		}
		// Return systemd logger as fallback
		return l.systemdLogger
	}

	// Create logger with timestamp for file output
	logger := log.New(file, "", log.LstdFlags)
	l.workspaceLoggers[workspaceName] = logger
	return logger
}

// LogSystemd logs to systemd/journalctl (no timestamp)
func (l *Logger) LogSystemd(format string, v ...interface{}) {
	l.systemdLogger.Printf(format, v...)
}

// LogWorkspace logs to both systemd and workspace-specific file
func (l *Logger) LogWorkspace(workspaceName, format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)

	// Log to systemd (no timestamp)
	l.systemdLogger.Printf("[%s] %s", workspaceName, message)

	// Log to workspace file (with timestamp)
	workspaceLogger := l.getWorkspaceLogger(workspaceName)
	workspaceLogger.Printf("%s", message)
}

// LogWorkspaceOperation logs deployment/destruction operations
func (l *Logger) LogWorkspaceOperation(workspaceName, operation, format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)

	// Log to systemd (no timestamp)
	l.systemdLogger.Printf("[%s] %s: %s", workspaceName, operation, message)

	// Log to workspace file (with timestamp)
	workspaceLogger := l.getWorkspaceLogger(workspaceName)
	workspaceLogger.Printf("%s: %s", operation, message)
}

// LogWorkspaceOnly logs only to workspace file (not systemd)
func (l *Logger) LogWorkspaceOnly(workspaceName, format string, v ...interface{}) {
	message := fmt.Sprintf(format, v...)

	// Log only to workspace file (with timestamp)
	workspaceLogger := l.getWorkspaceLogger(workspaceName)
	workspaceLogger.Printf("%s", message)
}

// Convenience functions for global usage
func LogSystemd(format string, v ...interface{}) {
	GetLogger().LogSystemd(format, v...)
}

func LogWorkspace(workspaceName, format string, v ...interface{}) {
	GetLogger().LogWorkspace(workspaceName, format, v...)
}

func LogWorkspaceOperation(workspaceName, operation, format string, v ...interface{}) {
	GetLogger().LogWorkspaceOperation(workspaceName, operation, format, v...)
}

func LogWorkspaceOnly(workspaceName, format string, v ...interface{}) {
	GetLogger().LogWorkspaceOnly(workspaceName, format, v...)
}

// Close closes all open log files
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, logger := range l.workspaceLoggers {
		if writer, ok := logger.Writer().(io.Closer); ok {
			_ = writer.Close()
		}
	}
	l.workspaceLoggers = make(map[string]*log.Logger)
}

// ResetSingleton resets the logger singleton (for testing only)
func ResetSingleton() {
	if defaultLogger != nil {
		defaultLogger.Close()
	}
	defaultLogger = nil
	once = sync.Once{}
}

// getLogDir determines the log directory using auto-discovery
func getLogDir() string {
	// First check workspace variable (explicit override)
	if logDir := os.Getenv("PROVISIONER_LOG_DIR"); logDir != "" {
		return logDir
	}

	// Auto-detect system installation by checking if /var/log/provisioner exists or can be created
	systemLogDir := "/var/log/provisioner"
	if _, err := os.Stat(systemLogDir); err == nil {
		return systemLogDir
	}

	// Try to create system log directory (in case this is first run after installation)
	if err := os.MkdirAll(systemLogDir, 0755); err == nil {
		return systemLogDir
	}

	// Fall back to development default
	return "logs"
}
