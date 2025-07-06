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

	// If not available, check if quota retry delay has passed
	if !cli.Available && time.Since(cli.LastQuotaError) > m.Config.QuotaRetryDelay {
		m.mu.RUnlock()
		m.mu.Lock()
		cli.Available = true
		cli.LastQuotaError = time.Time{}
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
