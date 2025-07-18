package executor

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"comemo/internal/config"
	"comemo/internal/logger"
)

// TestQualityErrorHandling tests that quality errors are handled with normal retry limits
func TestQualityErrorHandling(t *testing.T) {
	cfg := &config.Config{
		MaxRetries:       5,
		ExecutionTimeout: 30 * time.Second,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 10 * time.Second,
			OtherError:   5 * time.Minute,
		},
		PromptsDir:        "/tmp/test-prompts",
		OutputDir:         "/tmp/test-output",
		WorkerChannelSize: 10,
		ResultChannelSize: 100,
	}

	scripts := []string{"quality_error_test.sh"}

	// Create CLI manager with test configuration
	cliManager := NewCLIManager(cfg)
	statusManager := NewStatusManager()

	// Initialize test CLI manually
	cliManager.CLIs = map[string]*CLIState{
		"test-cli": {
			Available: true,
		},
	}

	// Create logger for scheduler
	// Create logger for scheduler (silent mode)
	testLogger := logger.New(0, nil, nil)

	// Create scheduler
	scheduler := NewScheduler(cfg, scripts, cliManager, statusManager, testLogger)

	// Test that quality errors are handled with normal retry limits
	t.Run("QualityErrorRetryLimit", func(t *testing.T) {
		scriptName := "quality_error_test.sh"

		// Initialize script
		scheduler.scriptStateMgr.InitializeScript(scriptName, cfg.MaxRetries)

		// Get initial state
		initialState := scheduler.scriptStateMgr.GetScript(scriptName)
		if initialState.RetryCount != 0 {
			t.Errorf("Expected initial retry count to be 0, got %d", initialState.RetryCount)
		}

		// Simulate first quality error
		result1 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result1)

		// Check state after first quality error
		state1 := scheduler.scriptStateMgr.GetScript(scriptName)
		if state1.RetryCount != 1 {
			t.Errorf("Expected retry count to be 1, got %d", state1.RetryCount)
		}
		if state1.State != StateRetrying {
			t.Errorf("Expected state to be Retrying, got %s", state1.State.String())
		}

		// Simulate second quality error
		result2 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result2)

		// Check state after second quality error
		state2 := scheduler.scriptStateMgr.GetScript(scriptName)
		if state2.RetryCount != 2 {
			t.Errorf("Expected retry count to be 2, got %d", state2.RetryCount)
		}
		if state2.State != StateRetrying {
			t.Errorf("Expected state to be Retrying, got %s", state2.State.String())
		}

		// Simulate third quality error
		result3 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result3)

		// Check state after third quality error
		state3 := scheduler.scriptStateMgr.GetScript(scriptName)
		if state3.RetryCount != 3 {
			t.Errorf("Expected retry count to be 3, got %d", state3.RetryCount)
		}
		if state3.State != StateRetrying {
			t.Errorf("Expected state to be Retrying, got %s", state3.State.String())
		}

		// Simulate fourth quality error
		result4 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result4)

		// Check state after fourth quality error
		state4 := scheduler.scriptStateMgr.GetScript(scriptName)
		if state4.RetryCount != 4 {
			t.Errorf("Expected retry count to be 4, got %d", state4.RetryCount)
		}
		if state4.State != StateRetrying {
			t.Errorf("Expected state to be Retrying, got %s", state4.State.String())
		}

		// Simulate fifth quality error (should exceed limit and mark as failed)
		result5 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result5)

		// Check state after fifth quality error - should be failed (RetryCount=5 >= MaxRetries=5)
		state5 := scheduler.scriptStateMgr.GetScript(scriptName)
		if state5.State != StateFailed {
			t.Errorf("Expected state to be Failed after exceeding retry limit, got %s", state5.State.String())
		}

		// Check that script is marked as processed (removed from queue)
		_, exists := scheduler.queueManager.IsScriptInQueue(scriptName)
		if exists {
			t.Errorf("Expected script to be removed from queue after failing, but it still exists")
		}
	})
}

// TestQualityErrorWithSuccess tests that quality errors work normally with success
func TestQualityErrorWithSuccess(t *testing.T) {
	cfg := &config.Config{
		MaxRetries: 5,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 10 * time.Second,
			OtherError:   5 * time.Minute,
		},
	}

	scripts := []string{"quality_error_reset_test.sh"}

	cliManager := NewCLIManager(cfg)
	statusManager := NewStatusManager()

	// Initialize test CLI manually
	cliManager.CLIs = map[string]*CLIState{
		"test-cli": {
			Available: true,
		},
	}

	// Create logger for scheduler (silent mode)
	testLogger := logger.New(0, nil, nil)
	scheduler := NewScheduler(cfg, scripts, cliManager, statusManager, testLogger)

	t.Run("QualityErrorWithSuccess", func(t *testing.T) {
		scriptName := "quality_error_reset_test.sh"

		// Initialize script
		scheduler.scriptStateMgr.InitializeScript(scriptName, cfg.MaxRetries)

		// Simulate two quality errors
		result1 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result1)

		result2 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result2)

		// Check that retry count is 2
		state := scheduler.scriptStateMgr.GetScript(scriptName)
		if state.RetryCount != 2 {
			t.Errorf("Expected retry count to be 2, got %d", state.RetryCount)
		}

		// Simulate success
		resultSuccess := WorkerResult{
			Script:   scriptName,
			CLI:      "test-cli",
			Success:  true,
			Duration: time.Second,
		}

		scheduler.handleWorkerResult(resultSuccess)

		// Check that script is completed (retry count doesn't matter for completed scripts)
		stateAfterSuccess := scheduler.scriptStateMgr.GetScript(scriptName)
		if stateAfterSuccess.RetryCount != 2 {
			t.Errorf("Expected retry count to remain 2 after success, got %d", stateAfterSuccess.RetryCount)
		}
		if stateAfterSuccess.State != StateCompleted {
			t.Errorf("Expected state to be Completed after success, got %s", stateAfterSuccess.State.String())
		}
	})
}

// TestQualityErrorWithOtherError tests that quality errors work normally with other errors
func TestQualityErrorWithOtherError(t *testing.T) {
	cfg := &config.Config{
		MaxRetries: 5,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 10 * time.Second,
			OtherError:   5 * time.Minute,
		},
	}

	scripts := []string{"quality_error_reset_other_test.sh"}

	cliManager := NewCLIManager(cfg)
	statusManager := NewStatusManager()

	// Initialize test CLI manually
	cliManager.CLIs = map[string]*CLIState{
		"test-cli": {
			Available: true,
		},
	}

	// Create logger for scheduler (silent mode)
	testLogger := logger.New(0, nil, nil)
	scheduler := NewScheduler(cfg, scripts, cliManager, statusManager, testLogger)

	t.Run("QualityErrorWithOtherError", func(t *testing.T) {
		scriptName := "quality_error_reset_other_test.sh"

		// Initialize script
		scheduler.scriptStateMgr.InitializeScript(scriptName, cfg.MaxRetries)

		// Simulate two quality errors
		result1 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result1)

		result2 := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("quality check failed: insufficient content"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(result2)

		// Check that retry count is 2
		state := scheduler.scriptStateMgr.GetScript(scriptName)
		if state.RetryCount != 2 {
			t.Errorf("Expected retry count to be 2, got %d", state.RetryCount)
		}

		// Simulate a different (non-quality) error
		resultOtherError := WorkerResult{
			Script:      scriptName,
			CLI:         "test-cli",
			Success:     false,
			IsRetryable: true,
			Error:       errors.New("connection timeout"),
			Duration:    time.Second,
		}

		scheduler.handleWorkerResult(resultOtherError)

		// Check that retry count continues to increment normally
		stateAfterOtherError := scheduler.scriptStateMgr.GetScript(scriptName)
		if stateAfterOtherError.RetryCount != 3 {
			t.Errorf("Expected retry count to be 3 after non-quality error, got %d", stateAfterOtherError.RetryCount)
		}
		if stateAfterOtherError.State != StateRetrying {
			t.Errorf("Expected state to be Retrying after other error, got %s", stateAfterOtherError.State.String())
		}
	})
}

// TestRetryLimitFixedCalculation tests that retry limit is checked correctly after the fix
func TestRetryLimitFixedCalculation(t *testing.T) {
	cfg := &config.Config{
		MaxRetries: 3, // Set low limit for testing
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 10 * time.Second,
			OtherError:   5 * time.Minute,
		},
	}

	scripts := []string{"retry_limit_test.sh"}

	cliManager := NewCLIManager(cfg)
	statusManager := NewStatusManager()

	// Initialize test CLI manually
	cliManager.CLIs = map[string]*CLIState{
		"test-cli": {
			Available: true,
		},
	}

	// Create logger for scheduler (silent mode)
	testLogger := logger.New(0, nil, nil)
	scheduler := NewScheduler(cfg, scripts, cliManager, statusManager, testLogger)

	t.Run("RetryLimitCorrectlyCalculated", func(t *testing.T) {
		scriptName := "retry_limit_test.sh"

		// Initialize script
		scheduler.scriptStateMgr.InitializeScript(scriptName, cfg.MaxRetries)

		// Simulate errors until retry limit is reached
		for i := 1; i <= cfg.MaxRetries; i++ {
			result := WorkerResult{
				Script:      scriptName,
				CLI:         "test-cli",
				Success:     false,
				IsRetryable: true,
				Error:       fmt.Errorf("connection error %d", i),
				Duration:    time.Second,
			}

			scheduler.handleWorkerResult(result)

			state := scheduler.scriptStateMgr.GetScript(scriptName)

			if i < cfg.MaxRetries {
				// Should still be retrying
				if state.State != StateRetrying {
					t.Errorf("After %d errors (limit: %d), expected state to be Retrying, got %s", i, cfg.MaxRetries, state.State.String())
				}
				if state.RetryCount != i {
					t.Errorf("After %d errors, expected retry count to be %d, got %d", i, i, state.RetryCount)
				}
			} else {
				// Should be failed at exactly the limit
				if state.State != StateFailed {
					t.Errorf("After %d errors (limit: %d), expected state to be Failed, got %s", i, cfg.MaxRetries, state.State.String())
				}
			}
		}

		// Verify script is removed from queue
		_, exists := scheduler.queueManager.IsScriptInQueue(scriptName)
		if exists {
			t.Errorf("Expected script to be removed from queue after exceeding retry limit")
		}
	})
}
