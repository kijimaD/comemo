package executor

import (
	"testing"
	"time"

	"comemo/internal/config"
)

func TestScriptStateManager(t *testing.T) {
	cfg := &config.Config{
		MaxRetries: 3,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 10 * time.Second,
			OtherError:   5 * time.Minute,
		},
	}

	sm := NewScriptStateManager(cfg)

	t.Run("InitializeScript", func(t *testing.T) {
		sm.InitializeScript("test1.sh", 3)

		script := sm.GetScript("test1.sh")
		if script == nil {
			t.Fatal("Script not found after initialization")
		}

		if script.State != StateWaiting {
			t.Errorf("Expected state %v, got %v", StateWaiting, script.State)
		}

		if script.RetryCount != 0 {
			t.Errorf("Expected retry count 0, got %d", script.RetryCount)
		}

		if script.MaxRetries != 3 {
			t.Errorf("Expected max retries 3, got %d", script.MaxRetries)
		}
	})

	t.Run("SetScriptProcessing", func(t *testing.T) {
		sm.SetScriptProcessing("test1.sh", "claude")

		script := sm.GetScript("test1.sh")
		if script.State != StateProcessing {
			t.Errorf("Expected state %v, got %v", StateProcessing, script.State)
		}

		if script.AssignedCLI != "claude" {
			t.Errorf("Expected CLI 'claude', got '%s'", script.AssignedCLI)
		}
	})

	t.Run("SetScriptCompleted", func(t *testing.T) {
		duration := 5 * time.Second
		sm.SetScriptCompleted("test1.sh", duration)

		script := sm.GetScript("test1.sh")
		if script.State != StateCompleted {
			t.Errorf("Expected state %v, got %v", StateCompleted, script.State)
		}

		if script.Duration != duration {
			t.Errorf("Expected duration %v, got %v", duration, script.Duration)
		}

		if script.CompletedAt == nil {
			t.Error("CompletedAt should not be nil")
		}
	})

	t.Run("SetScriptRetrying", func(t *testing.T) {
		sm.InitializeScript("test2.sh", 3)
		sm.SetScriptProcessing("test2.sh", "claude")

		sm.SetScriptRetrying("test2.sh", RetryReasonQualityError, "quality check failed")

		script := sm.GetScript("test2.sh")
		if script.State != StateRetrying {
			t.Errorf("Expected state %v, got %v", StateRetrying, script.State)
		}

		if script.RetryCount != 1 {
			t.Errorf("Expected retry count 1, got %d", script.RetryCount)
		}

		if script.RetryReason != RetryReasonQualityError {
			t.Errorf("Expected retry reason %v, got %v", RetryReasonQualityError, script.RetryReason)
		}

		if script.LastError != "quality check failed" {
			t.Errorf("Expected error message 'quality check failed', got '%s'", script.LastError)
		}

		// Check retry delay
		expectedDelay := cfg.RetryDelays.QualityError
		expectedRetryAfter := time.Now().Add(expectedDelay)
		if script.RetryAfter.Before(expectedRetryAfter.Add(-time.Second)) ||
			script.RetryAfter.After(expectedRetryAfter.Add(time.Second)) {
			t.Errorf("RetryAfter should be around %v, got %v", expectedRetryAfter, script.RetryAfter)
		}
	})

	t.Run("SetScriptFailed", func(t *testing.T) {
		sm.InitializeScript("test3.sh", 3)
		sm.SetScriptFailed("test3.sh", "critical error")

		script := sm.GetScript("test3.sh")
		if script.State != StateFailed {
			t.Errorf("Expected state %v, got %v", StateFailed, script.State)
		}

		if script.LastError != "critical error" {
			t.Errorf("Expected error message 'critical error', got '%s'", script.LastError)
		}
	})

	t.Run("GetRetryableScripts", func(t *testing.T) {
		sm.InitializeScript("retry1.sh", 3)
		sm.InitializeScript("retry2.sh", 3)
		sm.InitializeScript("completed.sh", 3)

		// Set retry1 to retrying with past retry time (should be retryable)
		sm.SetScriptRetrying("retry1.sh", RetryReasonQualityError, "error")
		// Manually update retry time to past for testing
		sm.mu.Lock()
		if script, exists := sm.scripts["retry1.sh"]; exists {
			script.RetryAfter = time.Now().Add(-time.Second) // Past time
		}
		sm.mu.Unlock()

		// Set retry2 to retrying with future retry time (should not be retryable)
		sm.SetScriptRetrying("retry2.sh", RetryReasonQuotaError, "quota error")

		// Set completed script
		sm.SetScriptCompleted("completed.sh", time.Second)

		retryable := sm.GetRetryableScripts()

		// Should only have retry1
		if len(retryable) != 1 {
			t.Errorf("Expected 1 retryable script, got %d", len(retryable))
		}

		if len(retryable) > 0 && retryable[0].Name != "retry1.sh" {
			t.Errorf("Expected 'retry1.sh', got '%s'", retryable[0].Name)
		}
	})

	t.Run("GetStateCounts", func(t *testing.T) {
		// Clear previous scripts
		sm = NewScriptStateManager(cfg)

		sm.InitializeScript("waiting.sh", 3)
		sm.InitializeScript("processing.sh", 3)
		sm.InitializeScript("retrying.sh", 3)
		sm.InitializeScript("failed.sh", 3)
		sm.InitializeScript("completed.sh", 3)

		sm.SetScriptProcessing("processing.sh", "claude")
		sm.SetScriptRetrying("retrying.sh", RetryReasonOtherError, "error")
		sm.SetScriptFailed("failed.sh", "error")
		sm.SetScriptCompleted("completed.sh", time.Second)

		counts := sm.GetStateCounts()

		expected := map[ScriptState]int{
			StateWaiting:    1,
			StateProcessing: 1,
			StateRetrying:   1,
			StateFailed:     1,
			StateCompleted:  1,
		}

		for state, expectedCount := range expected {
			if counts[state] != expectedCount {
				t.Errorf("Expected %d scripts in state %v, got %d", expectedCount, state, counts[state])
			}
		}
	})
}

func TestRetryReason(t *testing.T) {
	cfg := &config.Config{
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   2 * time.Hour,
			QualityError: 30 * time.Second,
			OtherError:   10 * time.Minute,
		},
	}

	t.Run("GetRetryDelay", func(t *testing.T) {
		testCases := []struct {
			reason   RetryReason
			expected time.Duration
		}{
			{RetryReasonQuotaError, 2 * time.Hour},
			{RetryReasonQualityError, 30 * time.Second},
			{RetryReasonOtherError, 10 * time.Minute},
		}

		for _, tc := range testCases {
			actual := tc.reason.GetRetryDelay(cfg)
			if actual != tc.expected {
				t.Errorf("For reason %v, expected delay %v, got %v", tc.reason, tc.expected, actual)
			}
		}
	})

	t.Run("String", func(t *testing.T) {
		testCases := []struct {
			reason   RetryReason
			expected string
		}{
			{RetryReasonQuotaError, "quota_error"},
			{RetryReasonQualityError, "quality_error"},
			{RetryReasonOtherError, "other_error"},
		}

		for _, tc := range testCases {
			actual := tc.reason.String()
			if actual != tc.expected {
				t.Errorf("For reason %v, expected string %v, got %v", tc.reason, tc.expected, actual)
			}
		}
	})
}

func TestScriptStatus(t *testing.T) {
	t.Run("IsRetryable", func(t *testing.T) {
		status := &ScriptStatus{
			State:      StateRetrying,
			RetryCount: 2,
			MaxRetries: 3,
		}

		if !status.IsRetryable() {
			t.Error("Script should be retryable")
		}

		status.RetryCount = 3
		if status.IsRetryable() {
			t.Error("Script should not be retryable when retry count equals max retries")
		}

		status.State = StateFailed
		status.RetryCount = 1
		if status.IsRetryable() {
			t.Error("Failed script should not be retryable")
		}
	})

	t.Run("CanRetryNow", func(t *testing.T) {
		status := &ScriptStatus{
			State:      StateRetrying,
			RetryCount: 1,
			MaxRetries: 3,
			RetryAfter: time.Now().Add(-time.Second), // Past time
		}

		if !status.CanRetryNow() {
			t.Error("Script should be retryable now")
		}

		status.RetryAfter = time.Now().Add(time.Hour) // Future time
		if status.CanRetryNow() {
			t.Error("Script should not be retryable yet")
		}
	})
}
