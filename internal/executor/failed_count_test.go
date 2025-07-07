package executor

import (
	"comemo/internal/config"
	"comemo/internal/logger"
	"sync"
	"testing"
	"time"
)

func TestFailedCountOnlyAfterRetryLimit(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MaxRetries:          2, // Set low for testing
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
	cliManager.mu = sync.RWMutex{}

	// Create status manager
	statusManager := NewStatusManager()
	statusManager.SetTotalScripts(1)
	statusManager.InitializeWorker("claude")

	// Create scheduler
	scheduler := NewScheduler(cfg, []string{"test1.sh"}, cliManager, statusManager, logger.Silent())

	t.Run("RetryableErrorDoesNotIncreaseFailed", func(t *testing.T) {
		// Queue a script
		scheduler.queueScript("test1.sh", "claude")

		// Record script start
		statusManager.RecordScriptStart("test1.sh", "claude")

		// Get initial status
		initialStatus := statusManager.GetStatus()
		initialFailed := initialStatus.Queue.Failed

		t.Logf("Initial failed count: %d", initialFailed)

		// Simulate first retryable error
		result := WorkerResult{
			Script:       "test1.sh",
			CLI:          "claude",
			Success:      false,
			IsQuotaError: false,
			IsRetryable:  true,
			Duration:     100 * time.Millisecond,
		}

		scheduler.handleWorkerResult(result)

		// Check status after first error
		afterFirstError := statusManager.GetStatus()
		t.Logf("After first error - Failed: %d, Retrying: %d", afterFirstError.Queue.Failed, afterFirstError.Queue.Retrying)

		// Failed count should not increase for retryable errors
		if afterFirstError.Queue.Failed != initialFailed {
			t.Errorf("Expected failed count to remain %d after first error, but got %d", initialFailed, afterFirstError.Queue.Failed)
		}

		// But retrying count should increase
		if afterFirstError.Queue.Retrying != 1 {
			t.Errorf("Expected retrying count to be 1, but got %d", afterFirstError.Queue.Retrying)
		}

		// Queue script again for second attempt
		scheduler.queueScript("test1.sh", "claude")
		statusManager.RecordScriptStart("test1.sh", "claude")

		// Simulate second retryable error (should reach retry limit)
		scheduler.handleWorkerResult(result)

		// Check status after second error
		afterSecondError := statusManager.GetStatus()
		t.Logf("After second error - Failed: %d, Retrying: %d", afterSecondError.Queue.Failed, afterSecondError.Queue.Retrying)

		// Failed count should still not increase until retry limit is exceeded
		if afterSecondError.Queue.Failed != initialFailed {
			t.Errorf("Expected failed count to remain %d after second error, but got %d", initialFailed, afterSecondError.Queue.Failed)
		}

		// Queue script again for third attempt (exceeds retry limit)
		scheduler.queueScript("test1.sh", "claude")
		statusManager.RecordScriptStart("test1.sh", "claude")

		// Simulate third retryable error (exceeds retry limit)
		scheduler.handleWorkerResult(result)

		// Check status after retry limit exceeded
		afterRetryLimit := statusManager.GetStatus()
		t.Logf("After retry limit exceeded - Failed: %d, Retrying: %d", afterRetryLimit.Queue.Failed, afterRetryLimit.Queue.Retrying)

		// Now failed count should increase
		if afterRetryLimit.Queue.Failed != initialFailed+1 {
			t.Errorf("Expected failed count to increase to %d after retry limit exceeded, but got %d", initialFailed+1, afterRetryLimit.Queue.Failed)
		}

		// Check that script state is failed
		scriptState := scheduler.scriptStateMgr.GetScript("test1.sh")
		if scriptState.State != StateFailed {
			t.Errorf("Expected script state to be Failed, but got %v", scriptState.State)
		}
	})
}

func TestQuotaErrorStillDoesNotIncreaseFailed(t *testing.T) {
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
	cliManager.mu = sync.RWMutex{}

	// Create status manager
	statusManager := NewStatusManager()
	statusManager.SetTotalScripts(1)
	statusManager.InitializeWorker("claude")

	// Create scheduler
	scheduler := NewScheduler(cfg, []string{"test2.sh"}, cliManager, statusManager, logger.Silent())

	t.Run("QuotaErrorDoesNotIncreaseFailed", func(t *testing.T) {
		// Queue a script
		scheduler.queueScript("test2.sh", "claude")

		// Record script start
		statusManager.RecordScriptStart("test2.sh", "claude")

		// Get initial status
		initialStatus := statusManager.GetStatus()
		initialFailed := initialStatus.Queue.Failed

		// Simulate quota error
		result := WorkerResult{
			Script:       "test2.sh",
			CLI:          "claude",
			Success:      false,
			IsQuotaError: true,
			Duration:     100 * time.Millisecond,
		}

		scheduler.handleWorkerResult(result)

		// Check status after quota error
		finalStatus := statusManager.GetStatus()

		// Failed count should not increase for quota errors
		if finalStatus.Queue.Failed != initialFailed {
			t.Errorf("Expected failed count to remain %d after quota error, but got %d", initialFailed, finalStatus.Queue.Failed)
		}

		// Check that script state is retrying
		scriptState := scheduler.scriptStateMgr.GetScript("test2.sh")
		if scriptState.State != StateRetrying {
			t.Errorf("Expected script state to be Retrying, but got %v", scriptState.State)
		}

		if scriptState.RetryReason != RetryReasonQuotaError {
			t.Errorf("Expected retry reason to be QuotaError, but got %v", scriptState.RetryReason)
		}
	})
}
