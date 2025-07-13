package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"comemo/internal/logger"
)

// Worker represents a script execution worker with dedicated methods
type Worker struct {
	name       string      // ワーカー名
	cliManager *CLIManager // CLI管理
	logger     *logger.Logger
}

// NewWorker creates a new worker instance
func NewWorker(name string, cliManager *CLIManager) *Worker {
	return &Worker{
		name:       name,
		cliManager: cliManager,
		logger:     cliManager.Options.Logger,
	}
}

// Run starts the worker main loop
func (w *Worker) Run(ctx context.Context, tasks <-chan Task, results chan<- WorkerResult) {
	w.logger.Debug("[%s] ワーカー開始", w.name)

	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("[%s] コンテキストキャンセル - ワーカー終了", w.name)
			return
		case task, ok := <-tasks:
			if !ok {
				w.logger.Debug("[%s] チャネルクローズ - ワーカー終了", w.name)
				return
			}

			// Check if task is already completed before executing
			if w.IsTaskCompleted(task.Script) {
				w.logger.Debug("[%s] タスク %s は既に完了済み - スキップ", w.name, task.Script)
				continue
			}

			// Execute task with context
			result := w.ExecuteTask(ctx, task)

			// Try to send result, but respect context cancellation
			select {
			case <-ctx.Done():
				w.logger.Debug("[%s] コンテキストキャンセル - 結果送信中止", w.name)
				return
			case results <- result:
				// Successfully sent result
			}
		}
	}
}

// ExecuteTask executes a single task and returns the result
func (w *Worker) ExecuteTask(ctx context.Context, task Task) WorkerResult {
	startTime := time.Now()

	// Double-check if task is already completed before starting execution
	if w.IsTaskCompleted(task.Script) {
		w.logger.Debug("[%s] タスク %s は既に完了済み - 実行スキップ", w.name, task.Script)
		return WorkerResult{
			Script:   task.Script,
			CLI:      task.CLI,
			Success:  true,
			Duration: time.Since(startTime),
		}
	}

	w.logger.Debug("[%s] タスク実行開始: %s (CLI: %s)", w.name, task.Script, task.CLI)

	// Log task start
	w.logTaskStart(task)

	// Record event status: execution start
	w.recordExecutionStart(task)

	// Get CLI command
	cli, exists := w.cliManager.GetCLICommand(task.CLI)
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

	// Read and prepare script
	scriptPath := filepath.Join(w.cliManager.Config.PromptsDir, task.Script)
	outputPath := filepath.Join(w.cliManager.Config.OutputDir, strings.TrimSuffix(task.Script, ".sh")+".md")

	content, err := w.readScript(scriptPath)
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

	// Execute the script
	result := w.executeScript(ctx, task, cli.Command, content, outputPath, startTime)

	return result
}

// IsTaskCompleted checks if a task is already completed (public method)
func (w *Worker) IsTaskCompleted(scriptName string) bool {
	// Check if output file already exists (task completed successfully)
	outputPath := filepath.Join(w.cliManager.Config.OutputDir, strings.TrimSuffix(scriptName, ".sh")+".md")
	if _, err := os.Stat(outputPath); err == nil {
		return true
	}

	// Check TaskStateManager if available
	if w.cliManager.Options.TaskStateManager != nil {
		state := w.cliManager.Options.TaskStateManager.GetTaskState(scriptName)
		if state != nil && state.State == TaskStateCompleted {
			return true
		}
	}

	return false
}

// logTaskStart logs the start of task execution
func (w *Worker) logTaskStart(task Task) {
	if w.cliManager.Options.TaskLogWriter != nil {
		LogTaskStart(w.cliManager.Options.TaskLogWriter, task.Script, task.CLI)
	}
}

// recordExecutionStart records execution start in event status management
func (w *Worker) recordExecutionStart(task Task) {
	// Event status management: execution start
	if w.cliManager.Options.EventStatusManager != nil {
		w.cliManager.Options.EventStatusManager.StartExecution(task.Script, task.CLI)
	}

	// Unified task state management: execution start
	if w.cliManager.Options.TaskStateManager != nil {
		w.cliManager.Options.TaskStateManager.TransitionToStarted(task.Script, task.CLI)
	} else {
		// Fallback to direct event logging if TaskStateManager is not available
		if w.cliManager.Options.TaskEventLogger != nil {
			if w.cliManager.Options.EventStatusManager != nil {
				retryCount, retryReason := w.cliManager.Options.EventStatusManager.GetRetryInfo(task.Script)
				if retryCount > 0 {
					w.cliManager.Options.TaskEventLogger.LogStartedWithRetry(task.Script, task.CLI, retryCount, retryReason)
				} else {
					w.cliManager.Options.TaskEventLogger.LogStarted(task.Script, task.CLI)
				}
			} else {
				w.cliManager.Options.TaskEventLogger.LogStarted(task.Script, task.CLI)
			}
		}
	}
}

// readScript reads and validates the script file
func (w *Worker) readScript(scriptPath string) ([]byte, error) {
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, err
	}
	return content, nil
}

// executeScript executes the script and handles the result
func (w *Worker) executeScript(ctx context.Context, task Task, cliCommand string, content []byte, outputPath string, startTime time.Time) WorkerResult {
	// Replace placeholder
	modifiedContent := strings.ReplaceAll(string(content), "{{AI_CLI_COMMAND}}", cliCommand)

	// Create execution context with timeout, respecting parent context
	timeoutCtx, cancel := context.WithTimeout(ctx, w.cliManager.Config.ExecutionTimeout)
	defer cancel()

	// Execute the script
	cmd := exec.CommandContext(timeoutCtx, "bash", "-c", modifiedContent)
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Debug log for output
	w.logger.Debug("[%s] コマンド実行結果 - エラー: %v, 出力長: %d", w.name, err, len(outputStr))
	if len(outputStr) > 0 {
		// Log first 500 characters of output for debugging
		debugOutput := outputStr
		if len(debugOutput) > 500 {
			debugOutput = debugOutput[:500] + "..."
		}
		w.logger.Debug("[%s] 出力内容: %s", w.name, debugOutput)
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

	// Handle execution result
	if err != nil {
		return w.handleExecutionError(task, err, outputStr, startTime)
	}

	return w.handleExecutionSuccess(task, outputStr, outputPath, startTime)
}

// handleExecutionError handles script execution errors
func (w *Worker) handleExecutionError(task Task, err error, outputStr string, startTime time.Time) WorkerResult {
	// Check error type
	execError := CreateExecutionError(err, outputStr, filepath.Join(w.cliManager.Config.PromptsDir, task.Script), task.CLI)
	duration := time.Since(startTime)

	// Log task failure with details
	w.logTaskFailure(task, err, outputStr, duration)

	// Record error in event status management
	w.recordExecutionError(task, execError, err)

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

// cleanGeneratedContent removes any content before RequiredTitlePattern
func (w *Worker) cleanGeneratedContent(content string) string {
	// Find the position of the required title pattern
	index := strings.Index(content, RequiredTitlePattern)
	
	// If pattern not found, return original content
	if index == -1 {
		w.logger.Debug("[%s] 警告: '%s' パターンが見つかりません", w.name, RequiredTitlePattern)
		return content
	}
	
	// Return content starting from the pattern
	cleaned := content[index:]
	w.logger.Debug("[%s] コンテンツ整形: %d文字削除", w.name, index)
	return cleaned
}

// handleExecutionSuccess handles successful script execution
func (w *Worker) handleExecutionSuccess(task Task, outputStr string, outputPath string, startTime time.Time) WorkerResult {
	// Always write output to file (AI generates content to stdout, Go writes to file)
	if len(outputStr) > 0 {
		if err := os.WriteFile(outputPath, []byte(outputStr), 0644); err != nil {
			return WorkerResult{
				Script:      task.Script,
				CLI:         task.CLI,
				Success:     false,
				IsRetryable: true,
				Error:       fmt.Errorf("ファイル書き込みエラー: %w", err),
				Output:      outputStr,
				Duration:    time.Since(startTime),
			}
		}
	}

	// Perform quality validation on generated content
	qualityResult, qualityErr := ValidateGeneratedContent(outputPath)

	if qualityErr != nil {
		return WorkerResult{
			Script:      task.Script,
			CLI:         task.CLI,
			Success:     false,
			IsRetryable: true,
			Error:       fmt.Errorf("品質検証でエラーが発生: %w", qualityErr),
			Output:      outputStr,
			Duration:    time.Since(startTime),
		}
	}

	if qualityResult.Passed {
		return w.handleQualitySuccess(task, outputStr, outputPath, startTime)
	}

	return w.handleQualityFailure(task, outputStr, outputPath, qualityResult.FailureReason, startTime)
}

// handleQualitySuccess handles successful quality validation
func (w *Worker) handleQualitySuccess(task Task, outputStr string, outputPath string, startTime time.Time) WorkerResult {
	// Quality check passed - clean and re-save the content
	cleanedOutput := w.cleanGeneratedContent(outputStr)
	
	// Re-write the cleaned content to file
	if len(cleanedOutput) > 0 {
		if err := os.WriteFile(outputPath, []byte(cleanedOutput), 0644); err != nil {
			w.logger.Warn("[%s] Failed to re-write cleaned content: %v", w.name, err)
			// Continue even if re-write fails, as original content is already saved
		}
	}
	
	// Delete the script file on success
	scriptPath := filepath.Join(w.cliManager.Config.PromptsDir, task.Script)
	if err := os.Remove(scriptPath); err != nil {
		w.logger.Warn("[%s] Failed to delete script: %v", w.name, err)
	}

	w.logger.Debug("[%s] タスク完了: %s (所要時間: %v)", w.name, task.Script, time.Since(startTime))

	duration := time.Since(startTime)

	// Log task success with details
	w.logTaskSuccess(task, outputPath, cleanedOutput, duration)

	// Record success in event status management
	w.recordExecutionSuccess(task, duration, outputPath, cleanedOutput)

	return WorkerResult{
		Script:   task.Script,
		CLI:      task.CLI,
		Success:  true,
		Output:   cleanedOutput,
		Duration: duration,
	}
}

// handleQualityFailure handles quality validation failure
func (w *Worker) handleQualityFailure(task Task, outputStr string, outputPath string, failureReason string, startTime time.Time) WorkerResult {
	// Quality check failed - use detailed failure reason from quality validation
	w.logger.Warn("[%s] 品質チェック失敗: %s - %s", w.name, task.Script, failureReason)

	// Remove the failed output file
	if err := os.Remove(outputPath); err != nil {
		w.logger.Warn("[%s] Failed to remove invalid output file: %v", w.name, err)
	}

	duration := time.Since(startTime)

	// Log task failure with quality check details
	w.logQualityFailure(task, failureReason, outputStr, duration)

	// Record quality failure in event status management
	w.recordQualityFailure(task, failureReason, outputStr)

	return WorkerResult{
		Script:      task.Script,
		CLI:         task.CLI,
		Success:     false,
		IsRetryable: true,
		Error:       fmt.Errorf("quality check failed: %s", failureReason),
		Output:      outputStr,
		Duration:    duration,
	}
}

// logTaskFailure logs task failure with details
func (w *Worker) logTaskFailure(task Task, err error, outputStr string, duration time.Duration) {
	if w.cliManager.Options.TaskLogWriter != nil {
		retryCount := w.cliManager.GetRetryCount(task.Script)
		LogTaskFailureWithDetails(w.cliManager.Options.TaskLogWriter, task.Script, task.CLI, err.Error(), retryCount, outputStr, duration)
	}
}

// logTaskSuccess logs task success with details
func (w *Worker) logTaskSuccess(task Task, outputPath string, outputStr string, duration time.Duration) {
	if w.cliManager.Options.TaskLogWriter != nil {
		LogTaskSuccessWithDetails(w.cliManager.Options.TaskLogWriter, task.Script, task.CLI, outputPath, outputStr, duration)
	}
}

// logQualityFailure logs quality check failure
func (w *Worker) logQualityFailure(task Task, failureReason string, outputStr string, duration time.Duration) {
	if w.cliManager.Options.TaskLogWriter != nil {
		retryCount := w.cliManager.GetRetryCount(task.Script)
		detailedError := fmt.Sprintf("quality check failed: %s", failureReason)
		LogTaskFailureWithDetails(w.cliManager.Options.TaskLogWriter, task.Script, task.CLI, detailedError, retryCount, outputStr, duration)
	}
}

// recordExecutionError records execution error in event status management
func (w *Worker) recordExecutionError(task Task, execError *ExecutionError, err error) {
	// Event status management: error type specific handling
	if w.cliManager.Options.EventStatusManager != nil {
		switch execError.Type {
		case ErrorTypeQuota:
			w.cliManager.Options.EventStatusManager.SetRetryWaiting(task.Script, RetryDelayQuota, "quota limit exceeded")
		case ErrorTypeTimeout:
			w.cliManager.Options.EventStatusManager.SetRetryWaiting(task.Script, RetryDelayTimeout, "timeout")
		default:
			w.cliManager.Options.EventStatusManager.SetRetryWaiting(task.Script, RetryDelayOther, err.Error())
		}
	}

	// Unified task state management: error type specific handling
	if w.cliManager.Options.TaskStateManager != nil {
		retryCount := w.cliManager.GetRetryCount(task.Script)
		switch execError.Type {
		case ErrorTypeQuota:
			w.cliManager.Options.TaskStateManager.TransitionToQuotaExceeded(task.Script, task.CLI)
		case ErrorTypeTimeout:
			w.cliManager.Options.TaskStateManager.TransitionToTimeout(task.Script, task.CLI, execError.Output, time.Duration(0))
		default:
			w.cliManager.Options.TaskStateManager.TransitionToFailed(task.Script, task.CLI, err.Error(), execError.Output, retryCount, time.Duration(0))
		}
	} else {
		// Fallback to direct event logging
		if w.cliManager.Options.TaskEventLogger != nil {
			retryCount := w.cliManager.GetRetryCount(task.Script)
			switch execError.Type {
			case ErrorTypeQuota:
				w.cliManager.Options.TaskEventLogger.LogQuotaExceeded(task.Script, task.CLI)
			case ErrorTypeTimeout:
				w.cliManager.Options.TaskEventLogger.LogTimeoutWithOutput(task.Script, task.CLI, time.Duration(0), execError.Output)
			default:
				w.cliManager.Options.TaskEventLogger.LogFailedWithOutput(task.Script, task.CLI, err.Error(), retryCount, execError.Output)
			}
		}
	}
}

// recordExecutionSuccess records successful execution in event status management
func (w *Worker) recordExecutionSuccess(task Task, duration time.Duration, outputPath string, outputStr string) {
	// Event status management: successful completion
	if w.cliManager.Options.EventStatusManager != nil {
		w.cliManager.Options.EventStatusManager.CompleteSuccess(task.Script, duration)
	}

	// Unified task state management: successful completion
	if w.cliManager.Options.TaskStateManager != nil {
		w.cliManager.Options.TaskStateManager.TransitionToCompleted(task.Script, task.CLI, duration, outputPath, outputStr)
	} else {
		// Fallback to direct event logging
		if w.cliManager.Options.TaskEventLogger != nil {
			w.cliManager.Options.TaskEventLogger.LogCompletedWithOutput(task.Script, task.CLI, duration, outputPath, outputStr)
		}
	}
}

// recordQualityFailure records quality failure in event status management
func (w *Worker) recordQualityFailure(task Task, failureReason string, outputStr string) {
	// Event status management: retry waiting (quality check failure)
	if w.cliManager.Options.EventStatusManager != nil {
		detailedReason := fmt.Sprintf("quality check failed: %s", failureReason)
		w.cliManager.Options.EventStatusManager.SetRetryWaiting(task.Script, RetryDelayQuality, detailedReason)
	}

	// Unified task state management: quality check failure
	if w.cliManager.Options.TaskStateManager != nil {
		retryCount := w.cliManager.GetRetryCount(task.Script)
		w.cliManager.Options.TaskStateManager.TransitionToQualityFailed(task.Script, task.CLI, retryCount, failureReason, outputStr)
	} else {
		// Fallback to direct event logging
		if w.cliManager.Options.TaskEventLogger != nil {
			retryCount := w.cliManager.GetRetryCount(task.Script)
			w.cliManager.Options.TaskEventLogger.LogQualityFailedWithDetails(task.Script, task.CLI, retryCount, failureReason, outputStr)
		}
	}
}

// GetName returns the worker name
func (w *Worker) GetName() string {
	return w.name
}

// GetCLIManager returns the CLI manager
func (w *Worker) GetCLIManager() *CLIManager {
	return w.cliManager
}

// TODO: Simpleがつく意味は?
// TODO: この関数シグネチャの意味あるか? WithContext系
// SimpleWorker creates a new worker and runs it with the given channels
func (w *Worker) SimpleWorker(tasks <-chan Task, results chan<- WorkerResult) {
	ctx := context.Background()
	w.SimpleWorkerWithContext(ctx, tasks, results)
}

// SimpleWorkerWithContext creates a new worker and runs it with context support
func (w *Worker) SimpleWorkerWithContext(ctx context.Context, tasks <-chan Task, results chan<- WorkerResult) {
	w.Run(ctx, tasks, results)
}

// ExecuteSimpleTask executes a single task and returns the result
func (w *Worker) ExecuteSimpleTask(task Task) WorkerResult {
	ctx := context.Background()
	return w.ExecuteSimpleTaskWithContext(ctx, task)
}

// ExecuteSimpleTaskWithContext executes a single task with context and returns the result
func (w *Worker) ExecuteSimpleTaskWithContext(ctx context.Context, task Task) WorkerResult {
	return w.ExecuteTask(ctx, task)
}

// Static utility functions that can be used independently

// CreateWorkerAndRun creates a new worker and runs it immediately
func CreateWorkerAndRun(name string, manager *CLIManager, tasks <-chan Task, results chan<- WorkerResult) {
	ctx := context.Background()
	CreateWorkerAndRunWithContext(ctx, name, manager, tasks, results)
}

// CreateWorkerAndRunWithContext creates a new worker and runs it with context support
func CreateWorkerAndRunWithContext(ctx context.Context, name string, manager *CLIManager, tasks <-chan Task, results chan<- WorkerResult) {
	worker := NewWorker(name, manager)
	worker.Run(ctx, tasks, results)
}

// ExecuteTaskWithManager executes a single task using a temporary worker
func ExecuteTaskWithManager(workerName string, task Task, manager *CLIManager) WorkerResult {
	ctx := context.Background()
	return ExecuteTaskWithManagerAndContext(ctx, workerName, task, manager)
}

// ExecuteTaskWithManagerAndContext executes a single task using a temporary worker with context
func ExecuteTaskWithManagerAndContext(ctx context.Context, workerName string, task Task, manager *CLIManager) WorkerResult {
	worker := NewWorker(workerName, manager)
	return worker.ExecuteTask(ctx, task)
}
