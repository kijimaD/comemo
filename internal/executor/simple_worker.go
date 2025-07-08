package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
	// Create dedicated worker instance and use it
	worker := NewWorker(name, manager)
	worker.Run(ctx, tasks, results)
}

// executeSimpleTask executes a single task and returns the result
func executeSimpleTask(workerName string, task Task, manager *CLIManager) WorkerResult {
	ctx := context.Background()
	return executeSimpleTaskWithContext(ctx, workerName, task, manager)
}

// executeSimpleTaskWithContext executes a single task with context and returns the result
func executeSimpleTaskWithContext(ctx context.Context, workerName string, task Task, manager *CLIManager) WorkerResult {
	// Create dedicated worker instance and use it
	worker := NewWorker(workerName, manager)
	return worker.ExecuteTask(ctx, task)
}

