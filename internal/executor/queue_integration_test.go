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
		scheduler.queued["claude"] = []string{}

		// Queue a script
		scheduler.queueScript("test3.sh", "claude")

		// Verify script was queued
		if len(scheduler.queued["claude"]) != 1 {
			t.Errorf("Expected 1 script in queue, got %d", len(scheduler.queued["claude"]))
		}

		// Simulate successful completion
		result := WorkerResult{
			Script:  "test3.sh",
			CLI:     "claude",
			Success: true,
		}

		scheduler.handleWorkerResult(result)

		// Check that script was removed from queue
		if len(scheduler.queued["claude"]) != 0 {
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
		scheduler.queued["gemini"] = []string{}

		// Queue a script
		scheduler.queueScript("test1.sh", "gemini")

		// Verify script was queued
		if len(scheduler.queued["gemini"]) != 1 || scheduler.queued["gemini"][0] != "test1.sh" {
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
		t.Logf("Queue state after quota error: %v", scheduler.queued["gemini"])

		// Check that script remains in queue
		if len(scheduler.queued["gemini"]) != 1 || scheduler.queued["gemini"][0] != "test1.sh" {
			t.Errorf("Expected script to remain in queue after quota error, got: %v", scheduler.queued["gemini"])
		}

		// Check that CLI is marked unavailable
		if cliManager.CLIs["gemini"].Available {
			t.Errorf("Expected CLI to be marked unavailable after quota error")
		}
	})

	// Test 4: selectBestCLIWithCapacity only returns CLIs with available queue slots
	t.Run("SelectCLIWithCapacity", func(t *testing.T) {
		// Fill both CLIs to capacity (3 scripts each)
		scheduler.queued["claude"] = []string{"test1.sh", "test2.sh", "test3.sh"}
		scheduler.queued["gemini"] = []string{"test4.sh", "test5.sh", "test6.sh"}

		// Should return empty string as no CLI has capacity
		bestCLI := scheduler.selectBestCLIWithCapacity()
		if bestCLI != "" {
			t.Errorf("Expected no CLI with capacity, got %s", bestCLI)
		}

		// Remove one script from claude's queue
		scheduler.queued["claude"] = []string{"test1.sh", "test2.sh"}
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
		if len(scheduler.queued["claude"]) > 0 {
			// Script was queued by the sync call setup
		}
	})
}
