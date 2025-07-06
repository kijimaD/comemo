package executor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Worker processes scripts for a specific CLI
func Worker(cliName string, scriptQueue <-chan string, manager *CLIManager) {
	WorkerWithOptions(cliName, scriptQueue, manager, manager.Options)
}

// WorkerWithOptions processes scripts for a specific CLI with configurable output
func WorkerWithOptions(cliName string, scriptQueue <-chan string, manager *CLIManager, opts *ExecutorOptions) {
	pendingScripts := make(map[string]bool)
	lastUnavailableLogTime := time.Time{}
	unavailableLogInterval := 30 * time.Second
	wasUnavailable := false

	for {
		select {
		case fileName, ok := <-scriptQueue:
			if !ok {
				if len(pendingScripts) > 0 && manager.IsAvailable(cliName) {
					processPendingScriptsWithOptions(pendingScripts, cliName, manager, opts)
				}
				return
			}

			if !manager.IsAvailable(cliName) {
				if !pendingScripts[fileName] {
					pendingScripts[fileName] = true

					now := time.Now()
					if now.Sub(lastUnavailableLogTime) > unavailableLogInterval {
						opts.Logger.Debug("CLI %s is not available, queuing %d scripts for retry", cliName, len(pendingScripts))
						lastUnavailableLogTime = now
					}
				}
				wasUnavailable = true
				continue
			}

			if wasUnavailable && len(pendingScripts) > 0 {
				opts.Logger.Debug("CLI %s is now available, processing %d pending scripts", cliName, len(pendingScripts))
				wasUnavailable = false
			}

			cli, exists := manager.GetCLICommand(cliName)
			if !exists {
				opts.Logger.Warn("CLI %s not found", cliName)
				continue
			}

			success := processScriptWithRetryWithOptions(fileName, cli, cliName, manager, opts)
			if !success {
				if !pendingScripts[fileName] {
					pendingScripts[fileName] = true
					opts.Logger.Debug("Script %s failed, added to pending queue", fileName)
				}
			} else {
				delete(pendingScripts, fileName)
			}

		case <-time.After(30 * time.Second):
			if len(pendingScripts) > 0 && manager.IsAvailable(cliName) {
				opts.Logger.Debug("CLI %s is now available, processing %d pending scripts", cliName, len(pendingScripts))
				processPendingScriptsWithOptions(pendingScripts, cliName, manager, opts)
			} else if len(pendingScripts) > 0 {
				opts.Logger.Debug("CLI %s still not available, %d scripts pending", cliName, len(pendingScripts))
			}
		}
	}
}

// processPendingScripts processes all pending scripts
func processPendingScripts(pendingScripts map[string]bool, cliName string, manager *CLIManager) {
	processPendingScriptsWithOptions(pendingScripts, cliName, manager, manager.Options)
}

// processPendingScriptsWithOptions processes all pending scripts with configurable output
func processPendingScriptsWithOptions(pendingScripts map[string]bool, cliName string, manager *CLIManager, opts *ExecutorOptions) {
	cli, exists := manager.GetCLICommand(cliName)
	if !exists {
		return
	}

	for fileName := range pendingScripts {
		success := processScriptWithRetryWithOptions(fileName, cli, cliName, manager, opts)
		if success {
			delete(pendingScripts, fileName)
			opts.Logger.Debug("Successfully processed pending script: %s", fileName)
		} else {
			opts.Logger.Debug("Pending script %s failed again, keeping in queue", fileName)
		}
	}
}

// processScriptWithRetry wraps processScript and returns success status
func processScriptWithRetry(fileName string, cli CLICommand, cliName string, manager *CLIManager) bool {
	return processScriptWithRetryWithOptions(fileName, cli, cliName, manager, manager.Options)
}

// processScriptWithRetryWithOptions wraps processScript and returns success status with configurable output
func processScriptWithRetryWithOptions(fileName string, cli CLICommand, cliName string, manager *CLIManager, opts *ExecutorOptions) bool {
	originalAvailable := manager.IsAvailable(cliName)

	scriptPath := filepath.Join(manager.Config.PromptsDir, fileName)
	baseName := strings.TrimSuffix(fileName, ".sh")
	outputPath := filepath.Join(manager.Config.OutputDir, baseName+".md")

	scriptExistsBefore := true
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptExistsBefore = false
	}

	processScriptWithOptions(fileName, cli, cliName, manager, opts)

	newAvailable := manager.IsAvailable(cliName)
	if originalAvailable && !newAvailable {
		return false
	}

	scriptExistsAfter := true
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptExistsAfter = false
	}

	outputExists := false
	if _, err := os.Stat(outputPath); err == nil {
		outputExists = true
	}

	if scriptExistsBefore && !scriptExistsAfter && outputExists {
		return true
	}

	if scriptExistsBefore && scriptExistsAfter && outputExists {
		opts.Logger.Debug("Quality check failed for %s, output file removed, script kept for retry", fileName)
		return false
	}

	return false
}

// processScript executes a single script
func processScript(scriptName string, cli CLICommand, cliName string, manager *CLIManager) {
	processScriptWithOptions(scriptName, cli, cliName, manager, manager.Options)
}

// processScriptWithOptions executes a single script with configurable output
func processScriptWithOptions(scriptName string, cli CLICommand, cliName string, manager *CLIManager, opts *ExecutorOptions) {
	scriptPath := filepath.Join(manager.Config.PromptsDir, scriptName)
	baseName := strings.TrimSuffix(scriptName, ".sh")
	outputPath := filepath.Join(manager.Config.OutputDir, baseName+".md")

	opts.Logger.Debug("--- Processing: %s with %s ---", scriptPath, cliName)

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		opts.Logger.Error("Error reading script %s: %v", scriptPath, err)
		return
	}

	modifiedContent := strings.ReplaceAll(string(content), "{{AI_CLI_COMMAND}}", cli.Command)

	ctx, cancel := context.WithTimeout(context.Background(), manager.Config.ExecutionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", modifiedContent)
	// Set working directory to project root for consistent path resolution
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}
	output, err := cmd.CombinedOutput()

	if err != nil {
		execError := CreateExecutionError(err, string(output), scriptPath, cliName)

		// Handle different error types
		switch execError.Type {
		case ErrorTypeTimeout:
			opts.Logger.Error("Script %s timed out after %v", scriptPath, manager.Config.ExecutionTimeout)
			manager.UpdateRetryInfo(scriptName, "timeout")
			return

		case ErrorTypeQuota:
			opts.Logger.Debug("Quota limit detected for %s. Marking as unavailable for %v.", cliName, manager.Config.QuotaRetryDelay)
			manager.MarkUnavailable(cliName)
			manager.UpdateRetryInfo(scriptName, "quota_error")
			return

		case ErrorTypeCritical:
			opts.Logger.Error("=== CRITICAL ERROR DETECTED ===")
			opts.Logger.Error("Script: %s", scriptPath)
			opts.Logger.Error("CLI: %s", cliName)
			opts.Logger.Error("Error: %v", err)
			opts.Logger.Error("Full Output:")
			opts.Logger.Error("=====================================")
			opts.Logger.Error("%s", string(output))
			opts.Logger.Error("=====================================")
			opts.Logger.Error("Execution stopped due to critical error.")

			// Critical error should stop the entire execution
			// We'll panic to stop all processing
			panic(execError)

		case ErrorTypeRetryable:
			opts.Logger.Error("Script %s failed with retryable error: %v", scriptPath, err)
			opts.Logger.Debug("Output: %s", string(output))
			manager.UpdateRetryInfo(scriptName, "retryable_error")
			return

		default:
			opts.Logger.Error("Script %s failed with unknown error type: %v", scriptPath, err)
			opts.Logger.Debug("Output: %s", string(output))
			manager.UpdateRetryInfo(scriptName, "unknown_error")
			return
		}
	}

	outputStr := string(output)
	aiOutputStart := strings.LastIndex(outputStr, "🚀 Generating explanation for commit")
	if aiOutputStart == -1 {
		aiOutputStart = 0
	} else {
		nextNewline := strings.Index(outputStr[aiOutputStart:], "\n")
		if nextNewline != -1 {
			aiOutputStart += nextNewline + 1
		}
	}
	aiOutputContent := strings.TrimSpace(outputStr[aiOutputStart:])

	foundValidContent := strings.Contains(aiOutputContent, "## コアとなるコードの解説") ||
		strings.Contains(aiOutputContent, "## 技術的詳細") ||
		strings.Contains(aiOutputContent, "# [インデックス")

	if len(aiOutputContent) > 1000 && foundValidContent {
		if err := os.WriteFile(outputPath, []byte(aiOutputContent), 0644); err != nil {
			opts.Logger.Error("Error writing output file %s: %v", outputPath, err)
			return
		}
		opts.Logger.Debug("Generated: %s", outputPath)

		if err := os.Remove(scriptPath); err != nil {
			opts.Logger.Error("Error deleting script %s: %v", scriptPath, err)
		} else {
			opts.Logger.Debug("Deleted script: %s", scriptPath)
		}
	} else {
		opts.Logger.Warn("Script executed but output is incomplete or invalid: %s", scriptPath)
		opts.Logger.Debug("Output length: %d characters", len(aiOutputContent))
		opts.Logger.Debug("Found valid content: %v", foundValidContent)

		if _, err := os.Stat(outputPath); err == nil {
			if removeErr := os.Remove(outputPath); removeErr != nil {
				opts.Logger.Error("Failed to remove incomplete output file %s: %v", outputPath, removeErr)
			} else {
				opts.Logger.Debug("Removed incomplete output file: %s", outputPath)
			}
		}

		if len(outputStr) > 500 {
			opts.Logger.Debug("Output preview:\n%s...", outputStr[:500])
		} else {
			opts.Logger.Debug("Full output:\n%s", outputStr)
		}

		opts.Logger.Debug("Script %s kept for retry", scriptPath)
		manager.UpdateRetryInfo(scriptName, "quality_check_failed")
	}
}
