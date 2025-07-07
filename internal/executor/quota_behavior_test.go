package executor

import (
	"testing"
	"time"

	"comemo/internal/config"
)

func TestQuotaErrorDoesNotIncrementRetryCount(t *testing.T) {
	cfg := &config.Config{
		MaxRetries: 3,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 5 * time.Minute,
			OtherError:   2 * time.Minute,
		},
	}

	mgr := NewScriptStateManager(cfg)

	// Add a script
	mgr.InitializeScript("test1.sh", 3)

	// Check initial state
	script := mgr.GetScript("test1.sh")
	if script.RetryCount != 0 {
		t.Errorf("Expected initial retry count 0, got %d", script.RetryCount)
	}

	// Simulate quota error - should NOT increment retry count
	mgr.SetScriptQuotaExceeded("test1.sh", "quota exceeded")

	script = mgr.GetScript("test1.sh")
	if script.RetryCount != 0 {
		t.Errorf("Expected retry count to remain 0 after quota error, got %d", script.RetryCount)
	}
	if script.RetryReason != RetryReasonQuotaError {
		t.Errorf("Expected retry reason to be QuotaError, got %s", script.RetryReason.String())
	}
	if script.State != StateRetrying {
		t.Errorf("Expected state to be Retrying, got %s", script.State.String())
	}

	// Simulate regular error - should increment retry count
	mgr.SetScriptRetrying("test1.sh", RetryReasonOtherError, "other error")

	script = mgr.GetScript("test1.sh")
	if script.RetryCount != 1 {
		t.Errorf("Expected retry count to be 1 after regular error, got %d", script.RetryCount)
	}
	if script.RetryReason != RetryReasonOtherError {
		t.Errorf("Expected retry reason to be OtherError, got %s", script.RetryReason.String())
	}

	// Another quota error - should still not increment retry count
	mgr.SetScriptQuotaExceeded("test1.sh", "quota exceeded again")

	script = mgr.GetScript("test1.sh")
	if script.RetryCount != 1 {
		t.Errorf("Expected retry count to remain 1 after second quota error, got %d", script.RetryCount)
	}
	if script.RetryReason != RetryReasonQuotaError {
		t.Errorf("Expected retry reason to be QuotaError, got %s", script.RetryReason.String())
	}
}

func TestSetScriptRetryingWithQuotaReason(t *testing.T) {
	cfg := &config.Config{
		MaxRetries: 3,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 5 * time.Minute,
			OtherError:   2 * time.Minute,
		},
	}

	mgr := NewScriptStateManager(cfg)

	// Add a script
	mgr.InitializeScript("test2.sh", 3)

	// Test SetScriptRetrying with quota reason - should NOT increment
	mgr.SetScriptRetrying("test2.sh", RetryReasonQuotaError, "quota error")

	script := mgr.GetScript("test2.sh")
	if script.RetryCount != 0 {
		t.Errorf("Expected retry count to remain 0 with quota reason, got %d", script.RetryCount)
	}

	// Test SetScriptRetrying with other reason - should increment
	mgr.SetScriptRetrying("test2.sh", RetryReasonOtherError, "other error")

	script = mgr.GetScript("test2.sh")
	if script.RetryCount != 1 {
		t.Errorf("Expected retry count to be 1 with other reason, got %d", script.RetryCount)
	}

	// Test SetScriptRetrying with quality reason - should increment
	mgr.SetScriptRetrying("test2.sh", RetryReasonQualityError, "quality error")

	script = mgr.GetScript("test2.sh")
	if script.RetryCount != 2 {
		t.Errorf("Expected retry count to be 2 with quality reason, got %d", script.RetryCount)
	}
}