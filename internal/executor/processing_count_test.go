package executor

import (
	"testing"
	"time"
)

func TestProcessingCount(t *testing.T) {
	statusManager := NewStatusManager()
	statusManager.InitializeWorker("test-cli")

	// Initial state - no processing
	status := statusManager.GetStatus()
	worker := status.Workers["test-cli"]
	if worker.ProcessingCount != 0 {
		t.Errorf("Expected initial ProcessingCount to be 0, got %d", worker.ProcessingCount)
	}

	// Start processing a script
	statusManager.RecordScriptStart("test1.sh", "test-cli")
	status = statusManager.GetStatus()
	worker = status.Workers["test-cli"]
	if worker.ProcessingCount != 1 {
		t.Errorf("Expected ProcessingCount to be 1 after start, got %d", worker.ProcessingCount)
	}

	// Start processing another script
	statusManager.RecordScriptStart("test2.sh", "test-cli")
	status = statusManager.GetStatus()
	worker = status.Workers["test-cli"]
	if worker.ProcessingCount != 2 {
		t.Errorf("Expected ProcessingCount to be 2 after second start, got %d", worker.ProcessingCount)
	}

	// Complete one script successfully
	statusManager.RecordScriptComplete("test1.sh", "test-cli", true, 100*time.Millisecond, "")
	status = statusManager.GetStatus()
	worker = status.Workers["test-cli"]
	if worker.ProcessingCount != 1 {
		t.Errorf("Expected ProcessingCount to be 1 after successful completion, got %d", worker.ProcessingCount)
	}
	if worker.ProcessedCount != 1 {
		t.Errorf("Expected ProcessedCount to be 1 after successful completion, got %d", worker.ProcessedCount)
	}

	// Record a quota error for the remaining script
	statusManager.RecordQuotaError("test2.sh", "test-cli", 100*time.Millisecond)
	status = statusManager.GetStatus()
	worker = status.Workers["test-cli"]
	if worker.ProcessingCount != 0 {
		t.Errorf("Expected ProcessingCount to be 0 after quota error, got %d", worker.ProcessingCount)
	}

	// Start processing again
	statusManager.RecordScriptStart("test3.sh", "test-cli")
	status = statusManager.GetStatus()
	worker = status.Workers["test-cli"]
	if worker.ProcessingCount != 1 {
		t.Errorf("Expected ProcessingCount to be 1 after restart, got %d", worker.ProcessingCount)
	}

	// Record a retry error
	statusManager.RecordRetryError("test3.sh", "test-cli", 100*time.Millisecond, "connection timeout")
	status = statusManager.GetStatus()
	worker = status.Workers["test-cli"]
	if worker.ProcessingCount != 0 {
		t.Errorf("Expected ProcessingCount to be 0 after retry error, got %d", worker.ProcessingCount)
	}
}
