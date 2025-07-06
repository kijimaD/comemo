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
	pendingScripts := make(map[string]bool)
	lastUnavailableLogTime := time.Time{}
	unavailableLogInterval := 30 * time.Second
	wasUnavailable := false

	for {
		select {
		case fileName, ok := <-scriptQueue:
			if !ok {
				if len(pendingScripts) > 0 && manager.IsAvailable(cliName) {
					processPendingScripts(pendingScripts, cliName, manager)
				}
				return
			}

			if !manager.IsAvailable(cliName) {
				if !pendingScripts[fileName] {
					pendingScripts[fileName] = true
					
					now := time.Now()
					if now.Sub(lastUnavailableLogTime) > unavailableLogInterval {
						fmt.Printf("CLI %s is not available, queuing %d scripts for retry\n", cliName, len(pendingScripts))
						lastUnavailableLogTime = now
					}
				}
				wasUnavailable = true
				continue
			}

			if wasUnavailable && len(pendingScripts) > 0 {
				fmt.Printf("CLI %s is now available, processing %d pending scripts\n", cliName, len(pendingScripts))
				wasUnavailable = false
			}

			cli, exists := manager.GetCLICommand(cliName)
			if !exists {
				fmt.Printf("CLI %s not found\n", cliName)
				continue
			}

			success := processScriptWithRetry(fileName, cli, cliName, manager)
			if !success {
				if !pendingScripts[fileName] {
					pendingScripts[fileName] = true
					fmt.Printf("Script %s failed, added to pending queue\n", fileName)
				}
			} else {
				delete(pendingScripts, fileName)
			}

		case <-time.After(30 * time.Second):
			if len(pendingScripts) > 0 && manager.IsAvailable(cliName) {
				fmt.Printf("CLI %s is now available, processing %d pending scripts\n", cliName, len(pendingScripts))
				processPendingScripts(pendingScripts, cliName, manager)
			} else if len(pendingScripts) > 0 {
				fmt.Printf("CLI %s still not available, %d scripts pending\n", cliName, len(pendingScripts))
			}
		}
	}
}

// processPendingScripts processes all pending scripts
func processPendingScripts(pendingScripts map[string]bool, cliName string, manager *CLIManager) {
	cli, exists := manager.GetCLICommand(cliName)
	if !exists {
		return
	}

	for fileName := range pendingScripts {
		success := processScriptWithRetry(fileName, cli, cliName, manager)
		if success {
			delete(pendingScripts, fileName)
			fmt.Printf("Successfully processed pending script: %s\n", fileName)
		} else {
			fmt.Printf("Pending script %s failed again, keeping in queue\n", fileName)
		}
	}
}

// processScriptWithRetry wraps processScript and returns success status
func processScriptWithRetry(fileName string, cli CLICommand, cliName string, manager *CLIManager) bool {
	originalAvailable := manager.IsAvailable(cliName)
	
	scriptPath := filepath.Join(manager.Config.PromptsDir, fileName)
	baseName := strings.TrimSuffix(fileName, ".sh")
	outputPath := filepath.Join(manager.Config.OutputDir, baseName+".md")
	
	scriptExistsBefore := true
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptExistsBefore = false
	}

	processScript(fileName, cli, cliName, manager)

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
		fmt.Printf("Quality check failed for %s, output file removed, script kept for retry\n", fileName)
		return false
	}

	return false
}

// processScript executes a single script
func processScript(scriptName string, cli CLICommand, cliName string, manager *CLIManager) {
	scriptPath := filepath.Join(manager.Config.PromptsDir, scriptName)
	baseName := strings.TrimSuffix(scriptName, ".sh")
	outputPath := filepath.Join(manager.Config.OutputDir, baseName+".md")

	fmt.Printf("--- Processing: %s with %s ---\n", scriptPath, cliName)

	content, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading script %s: %v\n", scriptPath, err)
		return
	}

	modifiedContent := strings.ReplaceAll(string(content), "{{AI_CLI_COMMAND}}", cli.Command)

	ctx, cancel := context.WithTimeout(context.Background(), manager.Config.ExecutionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", modifiedContent)
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "Script %s timed out after %v\n", scriptPath, manager.Config.ExecutionTimeout)
			return
		}
		fmt.Fprintf(os.Stderr, "Script %s failed: %v\n", scriptPath, err)
		fmt.Fprintf(os.Stderr, "Output: %s\n", string(output))

		if IsQuotaError(string(output)) {
			fmt.Printf("Quota limit detected for %s. Marking as unavailable for %v.\n", cliName, manager.Config.QuotaRetryDelay)
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
			fmt.Fprintf(os.Stderr, "Error writing output file %s: %v\n", outputPath, err)
			return
		}
		fmt.Printf("Generated: %s\n", outputPath)

		if err := os.Remove(scriptPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting script %s: %v\n", scriptPath, err)
		} else {
			fmt.Printf("Deleted script: %s\n", scriptPath)
		}
	} else {
		fmt.Fprintf(os.Stderr, "--- ⚠️ Script executed but output is incomplete or invalid: %s ---\n", scriptPath)
		fmt.Fprintf(os.Stderr, "Output length: %d characters\n", len(aiOutputContent))
		fmt.Fprintf(os.Stderr, "Found valid content: %v\n", foundValidContent)

		if _, err := os.Stat(outputPath); err == nil {
			if removeErr := os.Remove(outputPath); removeErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to remove incomplete output file %s: %v\n", outputPath, removeErr)
			} else {
				fmt.Printf("Removed incomplete output file: %s\n", outputPath)
			}
		}

		if len(outputStr) > 500 {
			fmt.Fprintf(os.Stderr, "Output preview:\n%s...\n", outputStr[:500])
		} else {
			fmt.Fprintf(os.Stderr, "Full output:\n%s\n", outputStr)
		}

		fmt.Printf("Script %s kept for retry\n", scriptPath)
		manager.UpdateRetryInfo(scriptName, "quality_check_failed")
	}
}