package executor

import (
	"os"
	"strings"
	"sync"
	"time"

	"comemo/internal/config"
	"comemo/internal/logger"
)

// CLIManager manages multiple CLI states
type CLIManager struct {
	CLIs       map[string]*CLIState
	Config     *config.Config
	RetryQueue chan string
	RetryInfo  map[string]*ScriptRetryInfo
	Options    *ExecutorOptions
	mu         sync.RWMutex
	retryMu    sync.RWMutex
}

// NewCLIManager creates a new CLI manager
func NewCLIManager(cfg *config.Config) *CLIManager {
	return NewCLIManagerWithOptions(cfg, &ExecutorOptions{
		Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
	})
}

// NewCLIManagerWithOptions creates a new CLI manager with configurable output
func NewCLIManagerWithOptions(cfg *config.Config, opts *ExecutorOptions) *CLIManager {
	if opts == nil {
		opts = &ExecutorOptions{
			Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
		}
	}

	manager := &CLIManager{
		CLIs:       make(map[string]*CLIState),
		Config:     cfg,
		RetryQueue: make(chan string, 10000),
		RetryInfo:  make(map[string]*ScriptRetryInfo),
		Options:    opts,
	}

	// Initialize CLI states
	for name, cmd := range SupportedCLIs {
		manager.CLIs[name] = &CLIState{
			Name:           name,
			Command:        cmd,
			Available:      true,
			LastQuotaError: time.Time{},
			RecoveryDelay:  0, // Default to config delay
			PendingScripts: []string{},
		}
	}

	return manager
}

// IsAvailable checks if a CLI is available (not in quota error state)
func (m *CLIManager) IsAvailable(cliName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cli, exists := m.CLIs[cliName]
	if !exists {
		return false
	}

	// If not available, check if recovery delay has passed
	recoveryDelay := cli.RecoveryDelay
	if recoveryDelay == 0 {
		// Use default if no individual delay is set
		recoveryDelay = m.Config.QuotaRetryDelay
	}

	if !cli.Available && time.Since(cli.LastQuotaError) > recoveryDelay {
		m.mu.RUnlock()
		m.mu.Lock()
		cli.Available = true
		cli.LastQuotaError = time.Time{}
		cli.RecoveryDelay = 0 // Reset recovery delay
		m.mu.Unlock()
		m.mu.RLock()
		return true
	}

	return cli.Available
}

// MarkUnavailable marks a CLI as unavailable due to quota error
func (m *CLIManager) MarkUnavailable(cliName string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cli, exists := m.CLIs[cliName]; exists {
		cli.Available = false
		cli.LastQuotaError = time.Now()
	}
}

// MarkUnavailableForDuration marks a CLI as unavailable for a specific duration
func (m *CLIManager) MarkUnavailableForDuration(cliName string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cli, exists := m.CLIs[cliName]; exists {
		cli.Available = false
		cli.LastQuotaError = time.Now()
		cli.RecoveryDelay = duration // Set individual recovery delay
	}
}

// GetCLICommand returns the CLI command for a given name
func (m *CLIManager) GetCLICommand(cliName string) (CLICommand, bool) {
	cmd, exists := SupportedCLIs[cliName]
	return cmd, exists
}

// IsQuotaError checks if an error message indicates a quota limit
func IsQuotaError(output string) bool {
	lowerOutput := strings.ToLower(output)
	for _, errorPattern := range config.QuotaErrors {
		if strings.Contains(lowerOutput, strings.ToLower(errorPattern)) {
			return true
		}
	}
	return false
}

// ClassifyError analyzes an error and returns the appropriate error type
func ClassifyError(err error, output string) ErrorType {
	if err == nil {
		return ErrorTypeRetryable
	}

	errorMessage := strings.ToLower(err.Error())
	outputLower := strings.ToLower(output)

	// Check for quota errors first
	if IsQuotaError(output) {
		return ErrorTypeQuota
	}

	// Check for quality test errors
	qualityErrorPatterns := []string{
		"test failed",
		"assertion failed",
		"quality check failed",
		"quality test failed",
		"validation failed",
		"lint error",
		"format error",
		"style check failed",
		"syntax error",
		"compilation failed",
		"build failed",
	}

	for _, pattern := range qualityErrorPatterns {
		if strings.Contains(outputLower, pattern) || strings.Contains(errorMessage, pattern) {
			return ErrorTypeQuality
		}
	}

	// Check for timeout
	if strings.Contains(errorMessage, "deadline exceeded") ||
		strings.Contains(errorMessage, "context deadline exceeded") {
		return ErrorTypeTimeout
	}

	// Check for critical exit statuses that indicate system-level problems
	criticalExitStatuses := []string{
		"exit status 127", // Command not found
		"exit status 126", // Command cannot execute (permission)
		"exit status 125", // Docker container error
		"exit status 124", // Timeout by timeout command
		"exit status 2",   // Misuse of shell builtins
	}

	for _, status := range criticalExitStatuses {
		if strings.Contains(errorMessage, status) {
			return ErrorTypeCritical
		}
	}

	// Check for critical errors that should stop execution
	criticalPatterns := []string{
		"command not found",
		"permission denied",
		"no such file or directory",
		"authentication failed",
		"invalid api key",
		"api key not found",
		"access denied",
		"unauthorized",
		"forbidden",
		"bad request",
		"invalid request",
		"executable file not found",
		"exec format error",
		"operation not permitted",
		"network is unreachable",
		"connection refused",
	}

	for _, pattern := range criticalPatterns {
		if strings.Contains(outputLower, pattern) || strings.Contains(errorMessage, pattern) {
			return ErrorTypeCritical
		}
	}

	// Default to retryable for other errors
	return ErrorTypeRetryable
}

// CreateExecutionError creates a new ExecutionError with proper classification
func CreateExecutionError(err error, output, script, cliName string) *ExecutionError {
	errorType := ClassifyError(err, output)

	var message string
	if err != nil {
		message = err.Error()
	} else {
		message = "実行が完了しましたが、期待される出力が得られませんでした"
	}

	return &ExecutionError{
		Type:     errorType,
		Message:  message,
		Output:   output,
		Script:   script,
		CLIName:  cliName,
		Original: err,
	}
}

// UpdateRetryInfo updates retry information for a script
func (m *CLIManager) UpdateRetryInfo(fileName string, reason string) {
	m.retryMu.Lock()
	defer m.retryMu.Unlock()

	if info, exists := m.RetryInfo[fileName]; exists {
		info.RetryCount++
		info.LastAttempt = time.Now()
		info.FailReason = reason
	} else {
		m.RetryInfo[fileName] = &ScriptRetryInfo{
			FileName:    fileName,
			RetryCount:  1,
			LastAttempt: time.Now(),
			FailReason:  reason,
		}
	}
}

// GetRetryCount returns the retry count for a script
func (m *CLIManager) GetRetryCount(fileName string) int {
	m.retryMu.RLock()
	defer m.retryMu.RUnlock()

	if info, exists := m.RetryInfo[fileName]; exists {
		return info.RetryCount
	}
	return 0
}

// ShouldRetry determines if a script should be retried
func (m *CLIManager) ShouldRetry(fileName string) bool {
	return m.GetRetryCount(fileName) < m.Config.MaxRetries
}
