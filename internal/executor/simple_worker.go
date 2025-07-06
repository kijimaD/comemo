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

// SimpleWorker executes tasks without any decision logic
func SimpleWorker(name string, tasks <-chan Task, results chan<- WorkerResult, manager *CLIManager) {
	ctx := context.Background()
	SimpleWorkerWithContext(ctx, name, tasks, results, manager)
}

// SimpleWorkerWithContext executes tasks with context cancellation support
func SimpleWorkerWithContext(ctx context.Context, name string, tasks <-chan Task, results chan<- WorkerResult, manager *CLIManager) {
	logger := manager.Options.Logger
	logger.Debug("[%s] ワーカー開始", name)

	for {
		select {
		case <-ctx.Done():
			logger.Debug("[%s] コンテキストキャンセル - ワーカー終了", name)
			return
		case task, ok := <-tasks:
			if !ok {
				logger.Debug("[%s] チャネルクローズ - ワーカー終了", name)
				return
			}

			// Execute task with context
			result := executeSimpleTaskWithContext(ctx, name, task, manager)

			// Try to send result, but respect context cancellation
			select {
			case <-ctx.Done():
				logger.Debug("[%s] コンテキストキャンセル - 結果送信中止", name)
				return
			case results <- result:
				// Successfully sent result
			}
		}
	}
}

// executeSimpleTask executes a single task and returns the result
func executeSimpleTask(workerName string, task Task, manager *CLIManager) WorkerResult {
	ctx := context.Background()
	return executeSimpleTaskWithContext(ctx, workerName, task, manager)
}

// executeSimpleTaskWithContext executes a single task with context and returns the result
func executeSimpleTaskWithContext(ctx context.Context, workerName string, task Task, manager *CLIManager) WorkerResult {
	startTime := time.Now()
	logger := manager.Options.Logger

	logger.Debug("[%s] タスク実行開始: %s (CLI: %s)", workerName, task.Script, task.CLI)

	// Get CLI command
	cli, exists := manager.GetCLICommand(task.CLI)
	if !exists {
		return WorkerResult{
			Script:     task.Script,
			CLI:        task.CLI,
			Success:    false,
			IsCritical: true,
			Error:      fmt.Errorf("CLI command %s not found", task.CLI),
			Duration:   time.Since(startTime),
		}
	}

	// Execute the script
	scriptPath := filepath.Join(manager.Config.PromptsDir, task.Script)
	outputPath := filepath.Join(manager.Config.OutputDir, strings.TrimSuffix(task.Script, ".sh")+".md")

	// Read script content
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return WorkerResult{
			Script:      task.Script,
			CLI:         task.CLI,
			Success:     false,
			IsCritical:  false,
			IsRetryable: true,
			Error:       fmt.Errorf("error reading script: %w", err),
			Duration:    time.Since(startTime),
		}
	}

	// Replace placeholder
	modifiedContent := strings.ReplaceAll(string(content), "{{AI_CLI_COMMAND}}", cli.Command)

	// Create execution context with timeout, respecting parent context
	timeoutCtx, cancel := context.WithTimeout(ctx, manager.Config.ExecutionTimeout)
	defer cancel()

	// Execute the script
	cmd := exec.CommandContext(timeoutCtx, "bash", "-c", modifiedContent)
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Check if context was cancelled during execution
	if ctx.Err() != nil {
		return WorkerResult{
			Script:   task.Script,
			CLI:      task.CLI,
			Success:  false,
			Error:    ctx.Err(),
			Duration: time.Since(startTime),
		}
	}

	// Analyze the result
	if err != nil {
		// Check error type
		execError := CreateExecutionError(err, outputStr, scriptPath, task.CLI)

		return WorkerResult{
			Script:       task.Script,
			CLI:          task.CLI,
			Success:      false,
			IsQuotaError: execError.Type == ErrorTypeQuota,
			IsRetryable:  execError.Type == ErrorTypeRetryable,
			IsCritical:   execError.Type == ErrorTypeCritical,
			Error:        err,
			Output:       outputStr,
			Duration:     time.Since(startTime),
		}
	}

	// Check output validity
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
		// Write output file
		if err := os.WriteFile(outputPath, []byte(aiOutputContent), 0644); err != nil {
			return WorkerResult{
				Script:      task.Script,
				CLI:         task.CLI,
				Success:     false,
				IsRetryable: true,
				Error:       fmt.Errorf("error writing output: %w", err),
				Output:      outputStr,
				Duration:    time.Since(startTime),
			}
		}

		// Delete the script file on success
		if err := os.Remove(scriptPath); err != nil {
			logger.Warn("[%s] Failed to delete script: %v", workerName, err)
		}

		logger.Debug("[%s] タスク完了: %s (所要時間: %v)", workerName, task.Script, time.Since(startTime))

		return WorkerResult{
			Script:   task.Script,
			CLI:      task.CLI,
			Success:  true,
			Output:   outputStr,
			Duration: time.Since(startTime),
		}
	}

	// Output was invalid
	logger.Warn("[%s] 出力が不完全: %s (長さ: %d, 有効コンテンツ: %v)",
		workerName, task.Script, len(aiOutputContent), foundValidContent)

	// Remove any partially written output file
	if _, err := os.Stat(outputPath); err == nil {
		os.Remove(outputPath)
	}

	return WorkerResult{
		Script:      task.Script,
		CLI:         task.CLI,
		Success:     false,
		IsRetryable: true,
		Error:       fmt.Errorf("output validation failed"),
		Output:      outputStr,
		Duration:    time.Since(startTime),
	}
}
