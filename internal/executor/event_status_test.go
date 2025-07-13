package executor

import (
	"testing"
	"time"
)

func TestEventStatusManager_StartExecution(t *testing.T) {
	manager := NewEventStatusManager(3)

	manager.StartExecution("test1.sh", "claude")

	entry, exists := manager.GetEntry("test1.sh")
	if !exists {
		t.Fatal("Entry should exist after StartExecution")
	}

	if entry.Status != EventStatusRunning {
		t.Errorf("Expected status %v, got %v", EventStatusRunning, entry.Status)
	}

	if entry.CLI != "claude" {
		t.Errorf("Expected CLI 'claude', got %v", entry.CLI)
	}

	if entry.RetryCount != 0 {
		t.Errorf("Expected retry count 0, got %v", entry.RetryCount)
	}
}

func TestEventStatusManager_CompleteSuccess(t *testing.T) {
	manager := NewEventStatusManager(3)

	manager.StartExecution("test1.sh", "claude")
	manager.CompleteSuccess("test1.sh", 5*time.Second)

	entry, exists := manager.GetEntry("test1.sh")
	if !exists {
		t.Fatal("Entry should exist after CompleteSuccess")
	}

	if entry.Status != EventStatusSuccess {
		t.Errorf("Expected status %v, got %v", EventStatusSuccess, entry.Status)
	}

	if entry.Duration != 5*time.Second {
		t.Errorf("Expected duration 5s, got %v", entry.Duration)
	}
}

func TestEventStatusManager_SetRetryWaiting(t *testing.T) {
	manager := NewEventStatusManager(3)

	manager.StartExecution("test1.sh", "claude")
	manager.SetRetryWaiting("test1.sh", RetryDelayQuality, "quality check failed")

	entry, exists := manager.GetEntry("test1.sh")
	if !exists {
		t.Fatal("Entry should exist after SetRetryWaiting")
	}

	if entry.Status != EventStatusRetryWaiting {
		t.Errorf("Expected status %v, got %v", EventStatusRetryWaiting, entry.Status)
	}

	if entry.RetryCount != 1 {
		t.Errorf("Expected retry count 1, got %v", entry.RetryCount)
	}

	if entry.RetryDelayType != RetryDelayQuality {
		t.Errorf("Expected retry delay type %v, got %v", RetryDelayQuality, entry.RetryDelayType)
	}

	if entry.ErrorMessage != "quality check failed" {
		t.Errorf("Expected error message 'quality check failed', got %v", entry.ErrorMessage)
	}

	// NextRetryTime should be in the future
	if entry.NextRetryTime.Before(time.Now()) {
		t.Error("NextRetryTime should be in the future")
	}

	// Should be approximately 10 seconds from now (quality error delay)
	expectedTime := time.Now().Add(RetryDelayQuality.GetRetryDelay())
	diff := entry.NextRetryTime.Sub(expectedTime)
	if diff > time.Second || diff < -time.Second {
		t.Errorf("NextRetryTime should be approximately %v, got %v (diff: %v)",
			expectedTime, entry.NextRetryTime, diff)
	}
}

func TestEventStatusManager_MaxRetriesExceeded(t *testing.T) {
	manager := NewEventStatusManager(2) // Max 2 retries

	manager.StartExecution("test1.sh", "claude")

	// First retry
	manager.SetRetryWaiting("test1.sh", RetryDelayOther, "first error")
	entry, _ := manager.GetEntry("test1.sh")
	if entry.Status != EventStatusRetryWaiting {
		t.Error("Should be in retry waiting after first failure")
	}

	// Second retry
	manager.SetRetryWaiting("test1.sh", RetryDelayOther, "second error")
	entry, _ = manager.GetEntry("test1.sh")
	if entry.Status != EventStatusRetryWaiting {
		t.Error("Should be in retry waiting after second failure")
	}

	// Third retry should result in failure (exceeds max retries)
	manager.SetRetryWaiting("test1.sh", RetryDelayOther, "third error")
	entry, _ = manager.GetEntry("test1.sh")
	if entry.Status != EventStatusFailed {
		t.Errorf("Should be failed after exceeding max retries, got %v", entry.Status)
	}
}

func TestEventStatusManager_IsRetryReady(t *testing.T) {
	manager := NewEventStatusManager(3)

	manager.StartExecution("test1.sh", "claude")
	manager.SetRetryWaiting("test1.sh", RetryDelayQuality, "quality error")

	entry, _ := manager.GetEntry("test1.sh")

	// Should not be ready immediately
	if entry.IsRetryReady() {
		t.Error("Should not be ready for retry immediately")
	}

	// Simulate time passing by manually setting NextRetryTime to the past
	entry.NextRetryTime = time.Now().Add(-1 * time.Second)
	if !entry.IsRetryReady() {
		t.Error("Should be ready for retry after delay time has passed")
	}
}

func TestEventStatusManager_GetRetryReadyScripts(t *testing.T) {
	manager := NewEventStatusManager(3)

	// Set up multiple scripts in different states
	manager.StartExecution("ready1.sh", "claude")
	manager.SetRetryWaiting("ready1.sh", RetryDelayQuality, "error")

	manager.StartExecution("ready2.sh", "gemini")
	manager.SetRetryWaiting("ready2.sh", RetryDelayOther, "error")

	manager.StartExecution("not_ready.sh", "claude")
	manager.SetRetryWaiting("not_ready.sh", RetryDelayQuota, "quota error") // 1 hour delay

	// Manually set retry times to test readiness
	manager.entries["ready1.sh"].NextRetryTime = time.Now().Add(-1 * time.Second)
	manager.entries["ready2.sh"].NextRetryTime = time.Now().Add(-1 * time.Second)
	// not_ready.sh should have NextRetryTime in the future (1 hour)

	readyScripts := manager.GetRetryReadyScripts()

	if len(readyScripts) != 2 {
		t.Errorf("Expected 2 ready scripts, got %v", len(readyScripts))
	}

	// Check that the correct scripts are returned
	foundReady1, foundReady2 := false, false
	for _, script := range readyScripts {
		switch script {
		case "ready1.sh":
			foundReady1 = true
		case "ready2.sh":
			foundReady2 = true
		case "not_ready.sh":
			t.Errorf("not_ready.sh should not be in ready scripts list")
		}
	}

	if !foundReady1 {
		t.Error("ready1.sh should be in ready scripts list")
	}
	if !foundReady2 {
		t.Error("ready2.sh should be in ready scripts list")
	}
}

func TestEventStatusManager_GetStatusCounts(t *testing.T) {
	manager := NewEventStatusManager(3)

	// Create scripts in different states
	manager.StartExecution("running1.sh", "claude")
	manager.StartExecution("running2.sh", "gemini")

	manager.StartExecution("success1.sh", "claude")
	manager.CompleteSuccess("success1.sh", time.Second)

	manager.StartExecution("retry1.sh", "claude")
	manager.SetRetryWaiting("retry1.sh", RetryDelayQuality, "error")

	manager.StartExecution("failed1.sh", "gemini")
	manager.SetFailed("failed1.sh", "critical error")

	counts := manager.GetStatusCounts()

	expectedCounts := map[EventStatus]int{
		EventStatusRunning:      2,
		EventStatusSuccess:      1,
		EventStatusRetryWaiting: 1,
		EventStatusFailed:       1,
	}

	for status, expectedCount := range expectedCounts {
		if actualCount, exists := counts[status]; !exists || actualCount != expectedCount {
			t.Errorf("Expected %v for status %v, got %v", expectedCount, status, actualCount)
		}
	}
}

func TestRetryDelayType_GetRetryDelay(t *testing.T) {
	tests := []struct {
		delayType     RetryDelayType
		expectedDelay time.Duration
	}{
		{RetryDelayQuota, 1 * time.Hour},
		{RetryDelayQuality, 10 * time.Second},
		{RetryDelayTimeout, 5 * time.Minute},
		{RetryDelayOther, 30 * time.Second},
	}

	for _, test := range tests {
		actualDelay := test.delayType.GetRetryDelay()
		if actualDelay != test.expectedDelay {
			t.Errorf("Expected delay %v for type %v, got %v",
				test.expectedDelay, test.delayType, actualDelay)
		}
	}
}

func TestGetRetryDelayTypeFromErrorType(t *testing.T) {
	tests := []struct {
		errorType         ErrorType
		expectedRetryType RetryDelayType
	}{
		{ErrorTypeQuota, RetryDelayQuota},
		{ErrorTypeQuality, RetryDelayQuality},
		{ErrorTypeTimeout, RetryDelayTimeout},
		{ErrorTypeRetryable, RetryDelayOther},
		{ErrorTypeCritical, RetryDelayOther}, // Critical errors map to Other
	}

	for _, test := range tests {
		actualRetryType := GetRetryDelayTypeFromErrorType(test.errorType)
		if actualRetryType != test.expectedRetryType {
			t.Errorf("Expected retry type %v for error type %v, got %v",
				test.expectedRetryType, test.errorType, actualRetryType)
		}
	}
}
