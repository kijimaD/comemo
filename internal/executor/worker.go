package executor

import (
	"context"
	"fmt"
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
						fmt.Fprintf(opts.Output, "CLI %s is not available, queuing %d scripts for retry\n", cliName, len(pendingScripts))
						lastUnavailableLogTime = now
					}
				}
				wasUnavailable = true
				continue
			}

			if wasUnavailable && len(pendingScripts) > 0 {
				fmt.Fprintf(opts.Output, "CLI %s is now available, processing %d pending scripts\n", cliName, len(pendingScripts))
				wasUnavailable = false
			}

			cli, exists := manager.GetCLICommand(cliName)
			if !exists {
				fmt.Fprintf(opts.Output, "CLI %s not found\n", cliName)
				continue
			}

			success := processScriptWithRetryWithOptions(fileName, cli, cliName, manager, opts)
			if !success {
				if !pendingScripts[fileName] {
					pendingScripts[fileName] = true
					fmt.Fprintf(opts.Output, "Script %s failed, added to pending queue\n", fileName)
				}
			} else {
				delete(pendingScripts, fileName)
			}

		case <-time.After(30 * time.Second):
			if len(pendingScripts) > 0 && manager.IsAvailable(cliName) {
				fmt.Fprintf(opts.Output, "CLI %s is now available, processing %d pending scripts\n", cliName, len(pendingScripts))
				processPendingScriptsWithOptions(pendingScripts, cliName, manager, opts)
			} else if len(pendingScripts) > 0 {
				fmt.Fprintf(opts.Output, "CLI %s still not available, %d scripts pending\n", cliName, len(pendingScripts))
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
			fmt.Fprintf(opts.Output, "Successfully processed pending script: %s\n", fileName)
		} else {
			fmt.Fprintf(opts.Output, "Pending script %s failed again, keeping in queue\n", fileName)
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
		fmt.Fprintf(opts.Output, "Quality check failed for %s, output file removed, script kept for retry\n", fileName)
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

	fmt.Fprintf(opts.Output, "--- Processing: %s with %s ---\n", scriptPath, cliName)

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(opts.Error, "Error reading script %s: %v\n", scriptPath, err)
		return
	}

	modifiedContent := strings.ReplaceAll(string(content), "{{AI_CLI_COMMAND}}", cli.Command)

	ctx, cancel := context.WithTimeout(context.Background(), manager.Config.ExecutionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", modifiedContent)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(opts.Error, "Script %s timed out after %v\n", scriptPath, manager.Config.ExecutionTimeout)
			return
		}
		fmt.Fprintf(opts.Error, "Script %s failed: %v\n", scriptPath, err)
		fmt.Fprintf(opts.Error, "Output: %s\n", string(output))

		if IsQuotaError(string(output)) {
			fmt.Fprintf(opts.Output, "Quota limit detected for %s. Marking as unavailable for %v.\n", cliName, manager.Config.QuotaRetryDelay)
			manager.MarkUnavailable(cliName)
			manager.UpdateRetryInfo(scriptName, "quota_error")
		}
		return
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
			fmt.Fprintf(opts.Error, "Error writing output file %s: %v\n", outputPath, err)
			return
		}
		fmt.Fprintf(opts.Output, "Generated: %s\n", outputPath)

		if err := os.Remove(scriptPath); err != nil {
			fmt.Fprintf(opts.Error, "Error deleting script %s: %v\n", scriptPath, err)
		} else {
			fmt.Fprintf(opts.Output, "Deleted script: %s\n", scriptPath)
		}
	} else {
		fmt.Fprintf(opts.Error, "--- ⚠️ Script executed but output is incomplete or invalid: %s ---\n", scriptPath)
		fmt.Fprintf(opts.Error, "Output length: %d characters\n", len(aiOutputContent))
		fmt.Fprintf(opts.Error, "Found valid content: %v\n", foundValidContent)

		if _, err := os.Stat(outputPath); err == nil {
			if removeErr := os.Remove(outputPath); removeErr != nil {
				fmt.Fprintf(opts.Error, "Failed to remove incomplete output file %s: %v\n", outputPath, removeErr)
			} else {
				fmt.Fprintf(opts.Output, "Removed incomplete output file: %s\n", outputPath)
			}
		}

		if len(outputStr) > 500 {
			fmt.Fprintf(opts.Error, "Output preview:\n%s...\n", outputStr[:500])
		} else {
			fmt.Fprintf(opts.Error, "Full output:\n%s\n", outputStr)
		}

		fmt.Fprintf(opts.Output, "Script %s kept for retry\n", scriptPath)
		manager.UpdateRetryInfo(scriptName, "quality_check_failed")
	}
}