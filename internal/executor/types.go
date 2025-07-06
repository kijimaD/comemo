package executor

import (
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
	PendingScripts []string
}

// ExecutorOptions provides configuration for executor functions
type ExecutorOptions struct {
	Logger *logger.Logger
}

// SupportedCLIs contains all supported AI CLI tools
var SupportedCLIs = map[string]CLICommand{
	"claude": {"claude", "claude --model sonnet"},
	"gemini": {"gemini", "gemini -m gemini-2.0-flash -p"},
}
