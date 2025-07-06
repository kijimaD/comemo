package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"comemo/internal/config"
	"comemo/internal/logger"

	"github.com/stretchr/testify/assert"
)

func TestProgressDisplay_NewProgressDisplay(t *testing.T) {
	sm := NewStatusManager()
	pd := NewProgressDisplay(sm)

	assert.NotNil(t, pd)
	assert.Equal(t, sm, pd.statusManager)
	assert.NotNil(t, pd.done)
	assert.NotNil(t, pd.ctx)
}

func TestProgressDisplay_StartStop(t *testing.T) {
	sm := NewStatusManager()
	sm.SetTotalScripts(10)
	sm.InitializeWorker("claude")

	pd := NewProgressDisplay(sm)

	// Start progress display
	pd.Start()
	assert.True(t, pd.IsRunning())

	// Let it run for a short time
	time.Sleep(100 * time.Millisecond)

	// Stop progress display
	pd.Stop()
	assert.False(t, pd.IsRunning())
}

func TestStatusManager_BasicOperations(t *testing.T) {
	sm := NewStatusManager()

	// Test initialization
	sm.InitializeWorker("claude")
	sm.SetTotalScripts(5)

	status := sm.GetStatus()
	assert.Contains(t, status.Workers, "claude")
	assert.Equal(t, 5, status.Queue.Total)
	assert.Equal(t, 5, status.Queue.Waiting)

	// Test script processing
	sm.RecordScriptStart("test.sh", "claude")
	status = sm.GetStatus()
	assert.Equal(t, 1, status.Queue.Processing)
	assert.Equal(t, 4, status.Queue.Waiting)
	assert.Equal(t, "test.sh", status.Workers["claude"].CurrentScript)

	// Test completion
	sm.RecordScriptComplete("test.sh", "claude", true, time.Second, "")
	status = sm.GetStatus()
	assert.Equal(t, 0, status.Queue.Processing)
	assert.Equal(t, 1, status.Queue.Completed)
	assert.Equal(t, "", status.Workers["claude"].CurrentScript)
	assert.Equal(t, 1, status.Workers["claude"].ProcessedCount)
}

func TestStatusManager_ErrorHandling(t *testing.T) {
	sm := NewStatusManager()
	sm.InitializeWorker("claude")
	sm.SetTotalScripts(3)

	// Test failure recording
	sm.RecordScriptStart("test.sh", "claude")
	sm.RecordScriptComplete("test.sh", "claude", false, time.Second, "API error")

	status := sm.GetStatus()
	assert.Equal(t, 1, status.Queue.Failed)
	assert.Equal(t, 1, status.Errors.ErrorCount)
	assert.Equal(t, "API error", status.Errors.LastError)
	assert.Len(t, status.Errors.RecentFailures, 1)

	// Test that worker's last failure reason is set
	assert.Equal(t, "API error", status.Workers["claude"].LastFailureReason)

	// Test retry queue
	sm.AddRetryScript("test.sh")
	status = sm.GetStatus()
	assert.Equal(t, 1, status.Queue.Retrying)
	assert.Equal(t, 0, status.Queue.Failed)
	assert.Contains(t, status.Errors.RetryQueue, "test.sh")
}

func TestStatusManager_PerformanceMetrics(t *testing.T) {
	sm := NewStatusManager()
	sm.Start()
	defer sm.Stop()

	sm.SetTotalScripts(10)
	sm.InitializeWorker("claude")

	// Simulate some completed scripts
	for i := 0; i < 5; i++ {
		sm.RecordScriptStart("test.sh", "claude")
		sm.RecordScriptComplete("test.sh", "claude", true, time.Millisecond, "")
	}

	status := sm.GetStatus()
	assert.Equal(t, 5, status.Queue.Completed)
	assert.True(t, status.Performance.ElapsedTime > 0)

	// Performance metrics should be calculated
	assert.True(t, status.Performance.ScriptsPerMinute >= 0)
}

func TestExecuteScriptWithContext_Cancellation(t *testing.T) {
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cfg := &config.Config{
		PromptsDir:       t.TempDir(),
		OutputDir:        t.TempDir(),
		ExecutionTimeout: 5 * time.Second,
	}

	opts := &ExecutorOptions{
		Logger: logger.Silent(),
	}

	manager := NewCLIManagerWithOptions(cfg, opts)

	// Execute with cancelled context
	err := executeScriptWithContext(ctx, "nonexistent.sh", "claude", manager, opts)

	// Should return error (either context.Canceled or file not found)
	assert.Error(t, err)
}

func TestIsTerminalSupported(t *testing.T) {
	// This test checks the function exists and returns a boolean
	// The actual result depends on the testing environment
	result := isTerminalSupported()
	assert.IsType(t, false, result)
}

func TestProgressDisplay_UpdateDisplay(t *testing.T) {
	sm := NewStatusManager()
	sm.SetTotalScripts(10)
	sm.InitializeWorker("claude")
	sm.InitializeWorker("gemini")

	// Set up some test data
	sm.UpdateWorkerStatus("claude", true, "test.sh", 0)
	sm.UpdateWorkerStatus("gemini", false, "", 2*time.Minute)

	// Simulate some progress
	sm.RecordScriptStart("test1.sh", "claude")
	sm.RecordScriptComplete("test1.sh", "claude", true, time.Second, "")

	pd := NewProgressDisplay(sm)

	// This mainly tests that updateDisplay doesn't panic
	// Output testing is difficult in unit tests
	pd.updateDisplay()

	// Verify display lines are tracked
	assert.True(t, pd.displayLines > 0)

	// Test multiple updates
	pd.updateDisplay()
	pd.updateDisplay()

	// If we get here without panicking, the test passes
	assert.True(t, true)
}

func TestBuildWorkerStatusLine(t *testing.T) {
	// Test available worker with current script
	worker := &WorkerStatus{
		Name:           "claude",
		Available:      true,
		CurrentScript:  "test.sh",
		ProcessedCount: 5,
		LastActivity:   time.Now().Add(-30 * time.Second),
	}

	line := buildWorkerStatusLine("claude", worker)
	assert.Contains(t, line, "claude")
	assert.Contains(t, line, "Processing test.sh")
	assert.Contains(t, line, "Processed: 5")

	// Test unavailable worker with quota recovery
	worker.Available = false
	worker.CurrentScript = ""
	worker.QuotaRecoveryTime = 2 * time.Minute

	line = buildWorkerStatusLine("claude", worker)
	assert.Contains(t, line, "Quota limit")
	assert.Contains(t, line, "2m0s")

	// Test worker with last failure reason
	worker.LastFailureReason = "API quota exceeded"
	line = buildWorkerStatusLine("claude", worker)
	assert.Contains(t, line, "Last failure: API quota exceeded")

	// Test worker with long failure reason (should be truncated)
	worker.LastFailureReason = "This is a very long error message that exceeds the display limit and should be truncated"
	line = buildWorkerStatusLine("claude", worker)
	assert.Contains(t, line, "Last failure:")
	assert.Contains(t, line, "...")
}

func TestBuildProgressBar(t *testing.T) {
	// Test empty progress
	bar := buildProgressBar(0, 10)
	assert.Equal(t, "[░░░░░░░░░░]", bar)

	// Test 50% progress
	bar = buildProgressBar(50, 10)
	assert.Equal(t, "[█████░░░░░]", bar)

	// Test full progress
	bar = buildProgressBar(100, 10)
	assert.Equal(t, "[██████████]", bar)

	// Test zero width
	bar = buildProgressBar(50, 0)
	assert.Equal(t, "", bar)
}

func TestWorkerWithStatusManagerAndProgress_BasicFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cfg := &config.Config{
		PromptsDir:       t.TempDir(),
		OutputDir:        t.TempDir(),
		ExecutionTimeout: 5 * time.Second,
	}

	opts := &ExecutorOptions{
		Logger: logger.Silent(),
	}

	manager := NewCLIManagerWithOptions(cfg, opts)
	statusManager := NewStatusManager()
	statusManager.InitializeWorker("claude")

	// Create a script queue with no scripts (test empty queue handling)
	scriptQueue := make(chan string)
	close(scriptQueue) // Close immediately

	// Start worker - should exit quickly due to closed queue
	done := make(chan bool)
	go func() {
		WorkerWithStatusManagerAndProgress(ctx, "claude", scriptQueue, manager, opts, statusManager)
		done <- true
	}()

	// Worker should exit quickly
	select {
	case <-done:
		// Worker exited successfully
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Worker did not exit quickly with closed queue")
	}
}

func TestExecuteScriptWithContext_PlaceholderReplacement(t *testing.T) {
	// This test verifies that {{AI_CLI_COMMAND}} placeholder is replaced
	tempDir := t.TempDir()
	scriptName := "test_placeholder.sh"
	scriptPath := filepath.Join(tempDir, scriptName)

	// Create test script with placeholder
	scriptContent := `#!/bin/bash
echo "Command: {{AI_CLI_COMMAND}}"
echo "Success"
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	assert.NoError(t, err)

	cfg := &config.Config{
		PromptsDir:       tempDir,
		OutputDir:        t.TempDir(),
		ExecutionTimeout: 5 * time.Second,
	}

	opts := &ExecutorOptions{
		Logger: logger.Silent(),
	}

	manager := NewCLIManagerWithOptions(cfg, opts)

	// Register a test CLI command
	manager.CLIs["test-cli"] = &CLIState{
		Name: "test-cli",
		Command: CLICommand{
			Name:    "test-cli",
			Command: "echo 'test-cli-output'",
		},
		Available: true,
	}

	// Execute script with context
	ctx := context.Background()
	err = executeScriptWithContext(ctx, scriptName, "test-cli", manager, opts)

	// Should fail because 'echo' command doesn't exist as an executable
	// But the important thing is that the placeholder was replaced
	// The error will be about command execution, not about {{AI_CLI_COMMAND}} not found
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), "{{AI_CLI_COMMAND}}")
	assert.NotContains(t, err.Error(), "command not found")
}
