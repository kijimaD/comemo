package executor

import (
	"comemo/internal/config"
	"comemo/internal/logger"
	"sync"
	"testing"
	"time"
)

func TestQuotaErrorDoesNotIncreaseFailed(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MaxRetries:          3,
		PromptsDir:          "/tmp/test_prompts",
		QuotaRetryDelay:     5 * time.Minute,
		QueueCapacityPerCLI: 3,
		WorkerChannelSize:   10,
		ResultChannelSize:   100,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 10 * time.Second,
			OtherError:   5 * time.Minute,
		},
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {
				Name:           "claude",
				Available:      true,
				LastQuotaError: time.Time{},
				RecoveryDelay:  0,
			},
		},
		Config: cfg,
	}
	// Initialize mutex
	cliManager.mu = sync.RWMutex{}

	// Create status manager
	statusManager := NewStatusManager()
	statusManager.SetTotalScripts(1)
	statusManager.InitializeWorker("claude")

	// Create scheduler
	scheduler := NewScheduler(cfg, []string{"test1.sh"}, cliManager, statusManager, logger.Silent())

	t.Run("QuotaErrorDoesNotIncreaseFailed", func(t *testing.T) {
		// Queue a script
		scheduler.queueScript("test1.sh", "claude")

		// Record script start (to increment processing count)
		statusManager.RecordScriptStart("test1.sh", "claude")

		// Get initial status
		initialStatus := statusManager.GetStatus()
		initialFailed := initialStatus.Queue.Failed
		initialProcessing := initialStatus.Queue.Processing

		t.Logf("Initial state - Failed: %d, Processing: %d", initialFailed, initialProcessing)

		// Simulate quota error
		result := WorkerResult{
			Script:       "test1.sh",
			CLI:          "claude",
			Success:      false,
			IsQuotaError: true,
			Duration:     100 * time.Millisecond,
		}

		scheduler.handleWorkerResult(result)

		// Get status after quota error
		finalStatus := statusManager.GetStatus()
		finalFailed := finalStatus.Queue.Failed
		finalProcessing := finalStatus.Queue.Processing
		finalWaiting := finalStatus.Queue.Waiting

		t.Logf("Final state - Failed: %d, Processing: %d, Waiting: %d", finalFailed, finalProcessing, finalWaiting)

		// Check that failed count did not increase
		if finalFailed != initialFailed {
			t.Errorf("Expected failed count to remain %d, but got %d", initialFailed, finalFailed)
		}

		// Check that processing count decreased
		if finalProcessing != initialProcessing-1 {
			t.Errorf("Expected processing count to decrease by 1, but got %d (was %d)", finalProcessing, initialProcessing)
		}

		// Check that waiting count increased (script back in queue)
		if finalWaiting != initialStatus.Queue.Waiting+1 {
			t.Errorf("Expected waiting count to increase by 1, but got %d (was %d)", finalWaiting, initialStatus.Queue.Waiting)
		}

		// Check that script remains in queue
		if len(scheduler.queued["claude"]) != 1 || scheduler.queued["claude"][0] != "test1.sh" {
			t.Errorf("Expected script to remain in queue after quota error")
		}

		// Check that CLI is marked unavailable
		if cliManager.CLIs["claude"].Available {
			t.Errorf("Expected CLI to be marked unavailable after quota error")
		}
	})
}

func TestRegularErrorIncreasesFailed(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MaxRetries:          3,
		PromptsDir:          "/tmp/test_prompts",
		QuotaRetryDelay:     5 * time.Minute,
		QueueCapacityPerCLI: 3,
		WorkerChannelSize:   10,
		ResultChannelSize:   100,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 10 * time.Second,
			OtherError:   5 * time.Minute,
		},
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {
				Name:           "claude",
				Available:      true,
				LastQuotaError: time.Time{},
				RecoveryDelay:  0,
			},
		},
		Config: cfg,
	}
	// Initialize mutex
	cliManager.mu = sync.RWMutex{}

	// Create status manager
	statusManager := NewStatusManager()
	statusManager.SetTotalScripts(1)
	statusManager.InitializeWorker("claude")

	// Create scheduler with debug logging
	debugLogger := logger.New(logger.DEBUG, nil, nil)
	scheduler := NewScheduler(cfg, []string{"test1.sh"}, cliManager, statusManager, debugLogger)

	t.Run("RegularErrorIncreasesFailed", func(t *testing.T) {
		// Queue a script
		scheduler.queueScript("test1.sh", "claude")

		// Record script start
		statusManager.RecordScriptStart("test1.sh", "claude")

		// Get initial status
		initialStatus := statusManager.GetStatus()
		initialFailed := initialStatus.Queue.Failed
		initialProcessing := initialStatus.Queue.Processing

		// Simulate regular error (not quota error)
		result := WorkerResult{
			Script:       "test1.sh",
			CLI:          "claude",
			Success:      false,
			IsQuotaError: false, // Regular error
			IsRetryable:  true,
			Duration:     100 * time.Millisecond,
		}

		t.Logf("Before handleWorkerResult - IsQuotaError: %v, Success: %v", result.IsQuotaError, result.Success)

		scheduler.handleWorkerResult(result)

		// Get status after regular error
		finalStatus := statusManager.GetStatus()
		finalFailed := finalStatus.Queue.Failed
		finalProcessing := finalStatus.Queue.Processing
		finalCompleted := finalStatus.Queue.Completed

		t.Logf("After handleWorkerResult - Failed: %d->%d, Processing: %d->%d, Completed: %d->%d",
			initialFailed, finalFailed, initialProcessing, finalProcessing, initialStatus.Queue.Completed, finalCompleted)

		// In the new implementation, regular errors also go to retrying state
		// Failed count should NOT increase until retry limit is exceeded
		if finalFailed != initialFailed {
			t.Errorf("Expected failed count to remain %d (retrying state), but got %d", initialFailed, finalFailed)
		}

		// But retrying count should increase
		finalRetrying := finalStatus.Queue.Retrying
		if finalRetrying == 0 {
			t.Errorf("Expected retrying count to increase for regular error, but got %d", finalRetrying)
		}
	})
}
