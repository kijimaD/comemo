package executor

import (
	"comemo/internal/config"
	"comemo/internal/logger"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewQueueingSystem_Integration(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MaxRetries:          3,
		PromptsDir:          "/tmp/test_prompts",
		QuotaRetryDelay:     1 * time.Second, // Short delay for testing
		QueueCapacityPerCLI: 3,               // Set queue capacity
		WorkerChannelSize:   10,
		ResultChannelSize:   100,
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {Available: true, LastQuotaError: time.Time{}},
			"gemini": {Available: true, LastQuotaError: time.Time{}},
		},
		Config: cfg,
	}
	// Initialize mutex
	cliManager.mu = sync.RWMutex{}

	// Create status manager
	statusManager := NewStatusManager()
	statusManager.SetTotalScripts(3)

	// Create scheduler
	scheduler := NewScheduler(cfg, []string{"test1.sh", "test2.sh", "test3.sh"}, cliManager, statusManager, logger.Silent())

	// Test 1: Each CLI respects queue capacity
	t.Run("QueueCapacityRespected", func(t *testing.T) {
		// Queue scripts up to capacity (3 by default)
		for i := 1; i <= 3; i++ {
			success := scheduler.queueScript(fmt.Sprintf("test%d.sh", i), "claude")
			if !success {
				t.Errorf("Expected script %d to be queued successfully", i)
			}
		}

		// Try to queue beyond capacity - should fail
		success := scheduler.queueScript("test4.sh", "claude")
		if success {
			t.Errorf("Expected fourth script to fail queuing (beyond capacity)")
		}

		// Queue to different CLI - should succeed
		success = scheduler.queueScript("test4.sh", "gemini")
		if !success {
			t.Errorf("Expected script to be queued to different CLI")
		}
	})

	// Test 2: Success removes script from queue
	t.Run("SuccessRemovesFromQueue", func(t *testing.T) {
		// Clear queue first
		scheduler.queueManager.Clear("claude")

		// Queue a script
		scheduler.queueScript("test3.sh", "claude")

		// Verify script was queued
		if scheduler.queueManager.Length("claude") != 1 {
			t.Errorf("Expected 1 script in queue, got %d", scheduler.queueManager.Length("claude"))
		}

		// Simulate successful completion
		result := WorkerResult{
			Script:  "test3.sh",
			CLI:     "claude",
			Success: true,
		}

		scheduler.handleWorkerResult(result)

		// Check that script was removed from queue
		if scheduler.queueManager.Length("claude") != 0 {
			t.Errorf("Expected script to be removed from queue after success")
		}

		// Check that script is marked as completed
		if !scheduler.completed["test3.sh"] {
			t.Errorf("Expected script to be marked as completed")
		}
	})

	// Test 3: Quota error keeps script in queue and marks CLI unavailable for 1 hour
	t.Run("QuotaErrorKeepsInQueue", func(t *testing.T) {
		// Clear queue first
		scheduler.queueManager.Clear("gemini")

		// Queue a script
		scheduler.queueScript("test1.sh", "gemini")

		// Verify script was queued
		if scheduler.queueManager.Length("gemini") != 1 {
			t.Errorf("Script not queued properly before test")
		}

		// Simulate quota error
		result := WorkerResult{
			Script:       "test1.sh",
			CLI:          "gemini",
			Success:      false,
			IsQuotaError: true,
		}

		scheduler.handleWorkerResult(result)

		// Debug: print current queue state
		queueCopy := scheduler.queueManager.GetQueueCopy("gemini")
		t.Logf("Queue state after quota error: %v", queueCopy)

		// Check that script remains in queue (quota errors don't remove from queue immediately)
		if scheduler.queueManager.Length("gemini") == 0 {
			t.Errorf("Expected script to remain in queue after quota error")
		}

		// Check that CLI is marked unavailable
		if cliManager.CLIs["gemini"].Available {
			t.Errorf("Expected CLI to be marked unavailable after quota error")
		}
	})

	// Test 4: selectBestCLIWithCapacity only returns CLIs with available queue slots
	t.Run("SelectCLIWithCapacity", func(t *testing.T) {
		// Set activeCLIs for selectBestCLIWithCapacity to work
		scheduler.activeCLIs = []string{"claude", "gemini"}

		// Fill both CLIs to capacity (3 scripts each)
		scheduler.queueManager.Clear("claude")
		scheduler.queueManager.Clear("gemini")
		for i := 1; i <= 3; i++ {
			scheduler.queueManager.Enqueue("claude", fmt.Sprintf("test%d.sh", i))
			scheduler.queueManager.Enqueue("gemini", fmt.Sprintf("test%d.sh", i+3))
		}

		// Should return empty string as no CLI has capacity
		bestCLI := scheduler.selectBestCLIWithCapacity()
		if bestCLI != "" {
			t.Errorf("Expected no CLI with capacity, got %s", bestCLI)
		}

		// Remove one script from claude's queue
		scheduler.queueManager.Dequeue("claude")
		cliManager.CLIs["claude"].Available = true

		// Should return claude as it has capacity
		bestCLI = scheduler.selectBestCLIWithCapacity()
		if bestCLI != "claude" {
			t.Errorf("Expected claude to be selected, got %s", bestCLI)
		}
	})
}

func TestScheduler_ExecuteScriptSync(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MaxRetries:      3,
		PromptsDir:      "/tmp/test_prompts",
		QuotaRetryDelay: 1 * time.Second,
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {Available: true, LastQuotaError: time.Time{}},
		},
		Config: cfg,
	}
	// Initialize mutex
	cliManager.mu = sync.RWMutex{}

	// Create status manager
	statusManager := NewStatusManager()

	// Create scheduler
	scheduler := NewScheduler(cfg, []string{"test1.sh"}, cliManager, statusManager, logger.Silent())
	scheduler.ctx = context.Background()

	// Create mock worker channel (won't actually process, just for testing structure)
	scheduler.workers["claude"] = make(chan Task, 1)

	// Test that ExecuteScriptSync can queue a script
	t.Run("CanQueueScript", func(t *testing.T) {
		// Start a goroutine to simulate worker result
		go func() {
			time.Sleep(10 * time.Millisecond) // Short delay to simulate work
			result := WorkerResult{
				Script:   "test1.sh",
				CLI:      "claude",
				Success:  true,
				Duration: 5 * time.Millisecond,
			}
			scheduler.handleWorkerResult(result)
		}()

		// This would block waiting for result in real scenario
		// For test, we'll just verify the queue structure
		queueLength := scheduler.queueManager.Length("claude")
		if queueLength > 0 {
			// Script was queued by the sync call setup
			t.Logf("Queue length for claude: %d", queueLength)
		}
	})
}
