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

// isTaskCompleted checks if a task is already completed
func isTaskCompleted(scriptName string, manager *CLIManager) bool {
	// Check if output file already exists (task completed successfully)
	outputPath := filepath.Join(manager.Config.OutputDir, strings.TrimSuffix(scriptName, ".sh")+".md")
	if _, err := os.Stat(outputPath); err == nil {
		return true
	}

	// Check TaskStateManager if available
	if manager.Options.TaskStateManager != nil {
		state := manager.Options.TaskStateManager.GetTaskState(scriptName)
		if state != nil && state.State == TaskStateCompleted {
			return true
		}
	}

	return false
}

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

			// Check if task is already completed before executing
			if isTaskCompleted(task.Script, manager) {
				logger.Debug("[%s] タスク %s は既に完了済み - スキップ", name, task.Script)
				continue
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

	// Double-check if task is already completed before starting execution
	if isTaskCompleted(task.Script, manager) {
		logger.Debug("[%s] タスク %s は既に完了済み - 実行スキップ", workerName, task.Script)
		return WorkerResult{
			Script:   task.Script,
			CLI:      task.CLI,
			Success:  true,
			Duration: time.Since(startTime),
		}
	}

	logger.Debug("[%s] タスク実行開始: %s (CLI: %s)", workerName, task.Script, task.CLI)

	// タスク開始をログに記録
	if manager.Options.TaskLogWriter != nil {
		LogTaskStart(manager.Options.TaskLogWriter, task.Script, task.CLI)
	}

	// イベントステータス管理: 実行開始
	if manager.Options.EventStatusManager != nil {
		manager.Options.EventStatusManager.StartExecution(task.Script, task.CLI)
	}

	// 一元的なタスク状態管理: 実行開始
	if manager.Options.TaskStateManager != nil {
		manager.Options.TaskStateManager.TransitionToStarted(task.Script, task.CLI)
	} else {
		// Fallback to direct event logging if TaskStateManager is not available
		if manager.Options.TaskEventLogger != nil {
			if manager.Options.EventStatusManager != nil {
				retryCount, retryReason := manager.Options.EventStatusManager.GetRetryInfo(task.Script)
				if retryCount > 0 {
					manager.Options.TaskEventLogger.LogStartedWithRetry(task.Script, task.CLI, retryCount, retryReason)
				} else {
					manager.Options.TaskEventLogger.LogStarted(task.Script, task.CLI)
				}
			} else {
				manager.Options.TaskEventLogger.LogStarted(task.Script, task.CLI)
			}
		}
	}

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

	// Inherit environment variables from parent process
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Debug log for output
	logger.Debug("[%s] コマンド実行結果 - エラー: %v, 出力長: %d", workerName, err, len(outputStr))
	if len(outputStr) > 0 {
		// Log first 500 characters of output for debugging
		debugOutput := outputStr
		if len(debugOutput) > 500 {
			debugOutput = debugOutput[:500] + "..."
		}
		logger.Debug("[%s] 出力内容: %s", workerName, debugOutput)
	}

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

		duration := time.Since(startTime)

		// タスク失敗をログに記録（実行結果含む）
		if manager.Options.TaskLogWriter != nil {
			retryCount := manager.GetRetryCount(task.Script)
			LogTaskFailureWithDetails(manager.Options.TaskLogWriter, task.Script, task.CLI, err.Error(), retryCount, outputStr, duration)
		}

		// イベントステータス管理: エラー種別に応じた処理
		if manager.Options.EventStatusManager != nil {
			switch execError.Type {
			case ErrorTypeQuota:
				manager.Options.EventStatusManager.SetRetryWaiting(task.Script, RetryDelayQuota, "quota limit exceeded")
			case ErrorTypeTimeout:
				manager.Options.EventStatusManager.SetRetryWaiting(task.Script, RetryDelayTimeout, "timeout")
			default:
				manager.Options.EventStatusManager.SetRetryWaiting(task.Script, RetryDelayOther, err.Error())
			}
		}

		// 一元的なタスク状態管理: エラー種別に応じた処理
		if manager.Options.TaskStateManager != nil {
			retryCount := manager.GetRetryCount(task.Script)
			switch execError.Type {
			case ErrorTypeQuota:
				manager.Options.TaskStateManager.TransitionToQuotaExceeded(task.Script, task.CLI)
			case ErrorTypeTimeout:
				manager.Options.TaskStateManager.TransitionToTimeout(task.Script, task.CLI, outputStr, duration)
			default:
				manager.Options.TaskStateManager.TransitionToFailed(task.Script, task.CLI, err.Error(), outputStr, retryCount, duration)
			}
		} else {
			// Fallback to direct event logging
			if manager.Options.TaskEventLogger != nil {
				retryCount := manager.GetRetryCount(task.Script)
				switch execError.Type {
				case ErrorTypeQuota:
					manager.Options.TaskEventLogger.LogQuotaExceeded(task.Script, task.CLI)
				case ErrorTypeTimeout:
					manager.Options.TaskEventLogger.LogTimeoutWithOutput(task.Script, task.CLI, duration, outputStr)
				default:
					manager.Options.TaskEventLogger.LogFailedWithOutput(task.Script, task.CLI, err.Error(), retryCount, outputStr)
				}
			}
		}

		return WorkerResult{
			Script:       task.Script,
			CLI:          task.CLI,
			Success:      false,
			IsQuotaError: execError.Type == ErrorTypeQuota,
			IsRetryable:  execError.Type == ErrorTypeRetryable,
			IsCritical:   execError.Type == ErrorTypeCritical,
			Error:        err,
			Output:       outputStr,
			Duration:     duration,
		}
	}

	// Check if AI CLI already saved the file
	fileContent, err := os.ReadFile(outputPath)
	if err != nil {
		return WorkerResult{
			Script:      task.Script,
			CLI:         task.CLI,
			Success:     false,
			IsRetryable: true,
			Error:       fmt.Errorf("error reading AI-generated file: %w", err),
			Output:      outputStr,
			Duration:    time.Since(startTime),
		}
	}

	fileContentStr := string(fileContent)
	foundValidContent := strings.Contains(fileContentStr, "## コアとなるコードの解説") ||
		strings.Contains(fileContentStr, "## 技術的詳細") ||
		strings.Contains(fileContentStr, "# [インデックス")

	if len(fileContentStr) > 500 && foundValidContent {
		// Quality check passed - delete the script file on success
		if err := os.Remove(scriptPath); err != nil {
			logger.Warn("[%s] Failed to delete script: %v", workerName, err)
		}

		logger.Debug("[%s] タスク完了: %s (所要時間: %v)", workerName, task.Script, time.Since(startTime))

		duration := time.Since(startTime)

		// タスク成功をログに記録（実行結果含む）
		if manager.Options.TaskLogWriter != nil {
			LogTaskSuccessWithDetails(manager.Options.TaskLogWriter, task.Script, task.CLI, outputPath, outputStr, duration)
		}

		// イベントステータス管理: 成功完了
		if manager.Options.EventStatusManager != nil {
			manager.Options.EventStatusManager.CompleteSuccess(task.Script, duration)
		}

		// 一元的なタスク状態管理: 成功完了
		if manager.Options.TaskStateManager != nil {
			manager.Options.TaskStateManager.TransitionToCompleted(task.Script, task.CLI, duration, outputPath, outputStr)
		} else {
			// Fallback to direct event logging
			if manager.Options.TaskEventLogger != nil {
				manager.Options.TaskEventLogger.LogCompletedWithOutput(task.Script, task.CLI, duration, outputPath, outputStr)
			}
		}

		return WorkerResult{
			Script:   task.Script,
			CLI:      task.CLI,
			Success:  true,
			Output:   outputStr,
			Duration: duration,
		}
	}

	// Quality check failed - analyze reasons based on written file content
	var failureReasons []string

	if len(fileContentStr) <= 500 {
		failureReasons = append(failureReasons, fmt.Sprintf("insufficient content length (%d chars, required: >500)", len(fileContentStr)))
	}

	if !foundValidContent {
		missingElements := []string{}
		if !strings.Contains(fileContentStr, "## コアとなるコードの解説") {
			missingElements = append(missingElements, "コアとなるコードの解説")
		}
		if !strings.Contains(fileContentStr, "## 技術的詳細") {
			missingElements = append(missingElements, "技術的詳細")
		}
		if !strings.Contains(fileContentStr, "# [インデックス") {
			missingElements = append(missingElements, "インデックス")
		}
		failureReasons = append(failureReasons, fmt.Sprintf("missing required sections: %s", strings.Join(missingElements, ", ")))
	}

	qualityFailureDetail := strings.Join(failureReasons, "; ")

	// Output was invalid
	logger.Warn("[%s] 品質チェック失敗: %s - %s",
		workerName, task.Script, qualityFailureDetail)

	// Remove the failed output file
	if err := os.Remove(outputPath); err != nil {
		logger.Warn("[%s] Failed to remove invalid output file: %v", workerName, err)
	}

	duration := time.Since(startTime)

	// タスク失敗をログに記録（品質チェック失敗、詳細理由含む）
	if manager.Options.TaskLogWriter != nil {
		retryCount := manager.GetRetryCount(task.Script)
		detailedError := fmt.Sprintf("quality check failed: %s", qualityFailureDetail)
		LogTaskFailureWithDetails(manager.Options.TaskLogWriter, task.Script, task.CLI, detailedError, retryCount, outputStr, duration)
	}

	// イベントステータス管理: リトライ待ち設定（品質チェック失敗）
	if manager.Options.EventStatusManager != nil {
		detailedReason := fmt.Sprintf("quality check failed: %s", qualityFailureDetail)
		manager.Options.EventStatusManager.SetRetryWaiting(task.Script, RetryDelayQuality, detailedReason)
	}

	// 一元的なタスク状態管理: 品質チェック失敗
	if manager.Options.TaskStateManager != nil {
		retryCount := manager.GetRetryCount(task.Script)
		manager.Options.TaskStateManager.TransitionToQualityFailed(task.Script, task.CLI, retryCount, qualityFailureDetail, outputStr)
	} else {
		// Fallback to direct event logging
		if manager.Options.TaskEventLogger != nil {
			retryCount := manager.GetRetryCount(task.Script)
			manager.Options.TaskEventLogger.LogQualityFailedWithDetails(task.Script, task.CLI, retryCount, qualityFailureDetail, outputStr)
		}
	}

	return WorkerResult{
		Script:      task.Script,
		CLI:         task.CLI,
		Success:     false,
		IsRetryable: true,
		Error:       fmt.Errorf("quality check failed: %s", qualityFailureDetail),
		Output:      outputStr,
		Duration:    duration,
	}
}
