package executor

import (
	"comemo/internal/config"
	"comemo/internal/logger"
	"sync"
	"testing"
	"time"
)

func TestQuotaErrorOneHourRecovery(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MaxRetries:          3,
		PromptsDir:          "/tmp/test_prompts",
		QuotaRetryDelay:     5 * time.Minute, // Default shorter delay
		QueueCapacityPerCLI: 3,
		WorkerChannelSize:   10,
		ResultChannelSize:   100,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,    // quota error - 1時間待機
			QualityError: 10 * time.Second, // 品質テストエラー - 10秒待機
			OtherError:   5 * time.Minute,  // その他のエラー - 5分待機
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

	// Create scheduler
	scheduler := NewScheduler(cfg, []string{"test1.sh"}, cliManager, statusManager, logger.Silent())

	t.Run("QuotaErrorSetsOneHourRecovery", func(t *testing.T) {
		// Queue a script
		scheduler.queueScript("test1.sh", "claude")

		// Verify CLI is initially available
		if !cliManager.CLIs["claude"].Available {
			t.Errorf("CLI should be initially available")
		}

		// Simulate quota error
		result := WorkerResult{
			Script:       "test1.sh",
			CLI:          "claude",
			Success:      false,
			IsQuotaError: true,
		}

		beforeTime := time.Now()
		scheduler.handleWorkerResult(result)

		// Check that CLI is marked unavailable
		if cliManager.CLIs["claude"].Available {
			t.Errorf("Expected CLI to be marked unavailable after quota error")
		}

		// Check that recovery delay is set to 1 hour
		if cliManager.CLIs["claude"].RecoveryDelay != time.Hour {
			t.Errorf("Expected recovery delay to be 1 hour, got %v", cliManager.CLIs["claude"].RecoveryDelay)
		}

		// Check that LastQuotaError is set to approximately now
		if time.Since(cliManager.CLIs["claude"].LastQuotaError) > time.Second {
			t.Errorf("LastQuotaError should be set to approximately now")
		}

		// Check that script remains in queue
		if len(scheduler.queued["claude"]) != 1 || scheduler.queued["claude"][0] != "test1.sh" {
			t.Errorf("Expected script to remain in queue after quota error")
		}

		// Verify that IsAvailable returns false immediately after quota error
		if cliManager.IsAvailable("claude") {
			t.Errorf("IsAvailable should return false immediately after quota error")
		}

		afterTime := time.Now()

		// Verify timing
		expectedRecoveryTime := beforeTime.Add(time.Hour)
		actualRecoveryTime := cliManager.CLIs["claude"].LastQuotaError.Add(time.Hour)

		if actualRecoveryTime.Before(expectedRecoveryTime.Add(-time.Second)) ||
			actualRecoveryTime.After(afterTime.Add(time.Hour)) {
			t.Errorf("Recovery time not set correctly. Expected around %v, got %v",
				expectedRecoveryTime, actualRecoveryTime)
		}
	})

	t.Run("CLIBecomesAvailableAfterRecovery", func(t *testing.T) {
		// Simulate time passing (1 hour + 1 second)
		pastTime := time.Now().Add(-time.Hour - time.Second)
		cliManager.CLIs["claude"].LastQuotaError = pastTime
		cliManager.CLIs["claude"].RecoveryDelay = time.Hour
		cliManager.CLIs["claude"].Available = false

		// Check that CLI becomes available after recovery time
		if !cliManager.IsAvailable("claude") {
			t.Errorf("CLI should become available after recovery time has passed")
		}

		// Check that CLI state is reset
		if !cliManager.CLIs["claude"].Available {
			t.Errorf("CLI Available flag should be reset to true")
		}

		if cliManager.CLIs["claude"].RecoveryDelay != 0 {
			t.Errorf("RecoveryDelay should be reset to 0, got %v", cliManager.CLIs["claude"].RecoveryDelay)
		}
	})

	t.Run("SelectCLIRespectsRecoveryTime", func(t *testing.T) {
		// Set CLI as unavailable with 1-hour recovery
		cliManager.CLIs["claude"].Available = false
		cliManager.CLIs["claude"].LastQuotaError = time.Now()
		cliManager.CLIs["claude"].RecoveryDelay = time.Hour

		// Ensure scheduler has "claude" in active CLIs
		scheduler.activeCLIs = []string{"claude"}

		// Should not select CLI with active recovery period
		bestCLI := scheduler.selectBestCLIWithCapacity()
		if bestCLI != "" {
			t.Errorf("Should not select CLI during recovery period, got %s", bestCLI)
		}

		// Simulate recovery time passed
		cliManager.CLIs["claude"].LastQuotaError = time.Now().Add(-time.Hour - time.Second)

		// Clear queue for selection
		scheduler.queued["claude"] = []string{}

		// Should select CLI after recovery
		bestCLI = scheduler.selectBestCLIWithCapacity()
		if bestCLI != "claude" {
			t.Errorf("Should select CLI after recovery period, got %s", bestCLI)
		}
	})
}
