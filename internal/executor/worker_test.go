package executor

import (
	"comemo/internal/config"
	"comemo/internal/logger"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWorker_NewWorker(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		PromptsDir: "/tmp/test_prompts",
		OutputDir:  "/tmp/test_output",
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {Available: true},
		},
		Config: cfg,
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}
	cliManager.mu = sync.RWMutex{}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Verify worker properties
	if worker.GetName() != "test-worker" {
		t.Errorf("Expected worker name to be 'test-worker', got '%s'", worker.GetName())
	}

	if worker.GetCLIManager() != cliManager {
		t.Errorf("Expected worker to have the correct CLI manager")
	}
}

func TestWorker_IsTaskCompleted(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test configuration
	cfg := &config.Config{
		PromptsDir: tempDir,
		OutputDir:  outputDir,
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {Available: true},
		},
		Config: cfg,
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}
	cliManager.mu = sync.RWMutex{}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Test case 1: Task not completed (no output file)
	if worker.IsTaskCompleted("test1.sh") {
		t.Errorf("Expected task to not be completed when output file doesn't exist")
	}

	// Test case 2: Task completed (output file exists)
	outputPath := filepath.Join(outputDir, "test1.md")
	if err := os.WriteFile(outputPath, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	if !worker.IsTaskCompleted("test1.sh") {
		t.Errorf("Expected task to be completed when output file exists")
	}
}

func TestWorker_ReadScript(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create test script
	scriptPath := filepath.Join(tempDir, "test.sh")
	scriptContent := "echo 'Hello World'"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create test configuration
	cfg := &config.Config{
		PromptsDir: tempDir,
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		Config: cfg,
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Test reading script
	content, err := worker.readScript(scriptPath)
	if err != nil {
		t.Errorf("Expected readScript to succeed, got error: %v", err)
	}

	if string(content) != scriptContent {
		t.Errorf("Expected script content '%s', got '%s'", scriptContent, string(content))
	}

	// Test reading non-existent script
	_, err = worker.readScript(filepath.Join(tempDir, "nonexistent.sh"))
	if err == nil {
		t.Errorf("Expected readScript to fail for non-existent file")
	}
}

func TestWorker_ExecuteTask_CompletedTask(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test configuration
	cfg := &config.Config{
		PromptsDir:       tempDir,
		OutputDir:        outputDir,
		ExecutionTimeout: 30 * time.Second,
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {Available: true},
		},
		Config: cfg,
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}
	cliManager.mu = sync.RWMutex{}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Create completed task (output file already exists)
	outputPath := filepath.Join(outputDir, "test.md")
	if err := os.WriteFile(outputPath, []byte("already completed"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create task
	task := Task{
		Script:  "test.sh",
		CLI:     "claude",
		AddedAt: time.Now(),
	}

	// Execute task
	ctx := context.Background()
	result := worker.ExecuteTask(ctx, task)

	// Verify result
	if !result.Success {
		t.Errorf("Expected task to succeed for already completed task")
	}

	if result.Script != "test.sh" {
		t.Errorf("Expected script name 'test.sh', got '%s'", result.Script)
	}

	if result.CLI != "claude" {
		t.Errorf("Expected CLI 'claude', got '%s'", result.CLI)
	}
}

func TestWorker_ExecuteTask_InvalidCLI(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create test configuration
	cfg := &config.Config{
		PromptsDir:       tempDir,
		ExecutionTimeout: 30 * time.Second,
	}

	// Create test CLI manager (without the CLI we'll try to use)
	cliManager := &CLIManager{
		CLIs:   map[string]*CLIState{},
		Config: cfg,
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}
	cliManager.mu = sync.RWMutex{}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Create task with invalid CLI
	task := Task{
		Script:  "test.sh",
		CLI:     "nonexistent",
		AddedAt: time.Now(),
	}

	// Execute task
	ctx := context.Background()
	result := worker.ExecuteTask(ctx, task)

	// Verify result
	if result.Success {
		t.Errorf("Expected task to fail for invalid CLI")
	}

	if !result.IsCritical {
		t.Errorf("Expected task to be marked as critical for invalid CLI")
	}

	if result.Error == nil {
		t.Errorf("Expected error for invalid CLI")
	}
}

func TestWorker_ExecuteTask_ScriptNotFound(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()

	// Create test configuration
	cfg := &config.Config{
		PromptsDir:       tempDir,
		ExecutionTimeout: 30 * time.Second,
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {Available: true},
		},
		Config: cfg,
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}
	cliManager.mu = sync.RWMutex{}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Create task with non-existent script
	task := Task{
		Script:  "nonexistent.sh",
		CLI:     "claude",
		AddedAt: time.Now(),
	}

	// Execute task
	ctx := context.Background()
	result := worker.ExecuteTask(ctx, task)

	// Verify result
	if result.Success {
		t.Errorf("Expected task to fail for non-existent script")
	}

	if result.IsCritical {
		t.Errorf("Expected task to not be critical for script read error")
	}

	if !result.IsRetryable {
		t.Errorf("Expected task to be retryable for script read error")
	}

	if result.Error == nil {
		t.Errorf("Expected error for non-existent script")
	}
}

func TestWorker_Run_ContextCancellation(t *testing.T) {
	// Create test CLI manager
	cliManager := &CLIManager{
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Create channels
	tasks := make(chan Task)
	results := make(chan WorkerResult, 1)

	// Start worker in goroutine
	done := make(chan struct{})
	go func() {
		worker.Run(ctx, tasks, results)
		close(done)
	}()

	// Cancel context immediately
	cancel()

	// Wait for worker to finish (with timeout)
	select {
	case <-done:
		// Worker finished as expected
	case <-time.After(1 * time.Second):
		t.Errorf("Worker did not finish within timeout after context cancellation")
	}
}

func TestWorker_Run_ChannelClose(t *testing.T) {
	// Create test CLI manager
	cliManager := &CLIManager{
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Create context
	ctx := context.Background()

	// Create channels
	tasks := make(chan Task)
	results := make(chan WorkerResult, 1)

	// Start worker in goroutine
	done := make(chan struct{})
	go func() {
		worker.Run(ctx, tasks, results)
		close(done)
	}()

	// Close tasks channel
	close(tasks)

	// Wait for worker to finish (with timeout)
	select {
	case <-done:
		// Worker finished as expected
	case <-time.After(1 * time.Second):
		t.Errorf("Worker did not finish within timeout after channel close")
	}
}

func TestWorker_SimpleWorkerMethods(t *testing.T) {
	// Create test CLI manager
	cliManager := &CLIManager{
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Test SimpleWorker method
	tasks := make(chan Task, 1)
	results := make(chan WorkerResult, 1)

	// Close channels immediately for testing
	close(tasks)

	// This should not hang or panic
	worker.SimpleWorker(tasks, results)
}

func TestWorker_ExecuteSimpleTaskMethods(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test configuration
	cfg := &config.Config{
		PromptsDir:       tempDir,
		OutputDir:        outputDir,
		ExecutionTimeout: 30 * time.Second,
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {Available: true},
		},
		Config: cfg,
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}
	cliManager.mu = sync.RWMutex{}

	// Create worker
	worker := NewWorker("test-worker", cliManager)

	// Create completed task (output file already exists)
	outputPath := filepath.Join(outputDir, "test.md")
	if err := os.WriteFile(outputPath, []byte("already completed"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create task
	task := Task{
		Script:  "test.sh",
		CLI:     "claude",
		AddedAt: time.Now(),
	}

	// Test ExecuteSimpleTask method
	result := worker.ExecuteSimpleTask(task)
	if !result.Success {
		t.Errorf("Expected task to succeed for already completed task")
	}

	// Test ExecuteSimpleTaskWithContext method
	ctx := context.Background()
	result = worker.ExecuteSimpleTaskWithContext(ctx, task)
	if !result.Success {
		t.Errorf("Expected task to succeed for already completed task")
	}
}

func TestCreateWorkerAndRun(t *testing.T) {
	// Create test CLI manager
	cliManager := &CLIManager{
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}

	// Create channels
	tasks := make(chan Task)
	results := make(chan WorkerResult, 1)

	// Start worker in goroutine
	done := make(chan struct{})
	go func() {
		CreateWorkerAndRun("test-worker", cliManager, tasks, results)
		close(done)
	}()

	// Close tasks channel immediately
	close(tasks)

	// Wait for worker to finish (with timeout)
	select {
	case <-done:
		// Worker finished as expected
	case <-time.After(1 * time.Second):
		t.Errorf("Worker did not finish within timeout after channel close")
	}
}

func TestExecuteTaskWithManager(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test configuration
	cfg := &config.Config{
		PromptsDir:       tempDir,
		OutputDir:        outputDir,
		ExecutionTimeout: 30 * time.Second,
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {Available: true},
		},
		Config: cfg,
		Options: &ExecutorOptions{
			Logger: logger.Silent(),
		},
	}
	cliManager.mu = sync.RWMutex{}

	// Create completed task (output file already exists)
	outputPath := filepath.Join(outputDir, "test.md")
	if err := os.WriteFile(outputPath, []byte("already completed"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create task
	task := Task{
		Script:  "test.sh",
		CLI:     "claude",
		AddedAt: time.Now(),
	}

	// Test ExecuteTaskWithManager function
	result := ExecuteTaskWithManager("test-worker", task, cliManager)
	if !result.Success {
		t.Errorf("Expected task to succeed for already completed task")
	}

	// Test ExecuteTaskWithManagerAndContext function
	ctx := context.Background()
	result = ExecuteTaskWithManagerAndContext(ctx, "test-worker", task, cliManager)
	if !result.Success {
		t.Errorf("Expected task to succeed for already completed task")
	}
}
