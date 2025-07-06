package executor

import (
	"os"

	"comemo/internal/config"
	"comemo/internal/logger"
)

// ExecutePrompts executes generated prompt scripts
func ExecutePrompts(cfg *config.Config, cliCommand string) error {
	return ExecutePromptsWithOptions(cfg, cliCommand, &ExecutorOptions{
		Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
	})
}

// ExecutePromptsWithOptions executes generated prompt scripts with configurable output
func ExecutePromptsWithOptions(cfg *config.Config, cliCommand string, opts *ExecutorOptions) error {
	// Use the new scheduler-based implementation
	return ExecutePromptsWithScheduler(cfg, cliCommand, opts)
}
