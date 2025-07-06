package executor

import (
	"io"
	"time"

	"comemo/internal/logger"
)

// CLICommand represents supported AI CLI commands
type CLICommand struct {
	Name    string
	Command string
}

// ScriptRetryInfo tracks retry information for scripts
type ScriptRetryInfo struct {
	FileName    string
	RetryCount  int
	LastAttempt time.Time
	FailReason  string
}

// CLIState manages the state of a CLI execution
type CLIState struct {
	Name           string
	Command        CLICommand
	Available      bool
	LastQuotaError time.Time
	RecoveryDelay  time.Duration // Individual recovery delay for this CLI
	PendingScripts []string
}

// ExecutorOptions provides configuration for executor functions
type ExecutorOptions struct {
	Logger              *logger.Logger
	TaskLogWriter       io.Writer // タスク実行ログの出力先
	EventStatusManager  *EventStatusManager // イベントステータス管理
	TaskEventLogger     *TaskEventLogger // タスクイベントロガー
}

// ErrorType represents different types of execution errors
type ErrorType int

const (
	// ErrorTypeQuota indicates a quota limit error
	ErrorTypeQuota ErrorType = iota
	// ErrorTypeTimeout indicates a timeout error
	ErrorTypeTimeout
	// ErrorTypeCritical indicates a critical error that should stop execution
	ErrorTypeCritical
	// ErrorTypeQuality indicates a quality test error that can be retried with short delay
	ErrorTypeQuality
	// ErrorTypeRetryable indicates an error that can be retried
	ErrorTypeRetryable
)

// String returns the string representation of ErrorType
func (e ErrorType) String() string {
	switch e {
	case ErrorTypeQuota:
		return "quota"
	case ErrorTypeTimeout:
		return "timeout"
	case ErrorTypeCritical:
		return "critical"
	case ErrorTypeQuality:
		return "quality"
	case ErrorTypeRetryable:
		return "retryable"
	default:
		return "unknown"
	}
}

// ExecutionError represents an error that occurred during script execution
type ExecutionError struct {
	Type     ErrorType
	Message  string
	Output   string
	Script   string
	CLIName  string
	Original error
}

// Error implements the error interface
func (e *ExecutionError) Error() string {
	return e.Message
}

// SupportedCLIs contains all supported AI CLI tools
var SupportedCLIs = map[string]CLICommand{
	"claude": {"claude", "claude --model sonnet"},
	"gemini": {"gemini", "gemini -m gemini-2.0-flash -p"},
}
