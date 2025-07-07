package executor

import (
	"strings"
	"testing"
	"time"
)

func TestBuildWorkerStatusLineWithProcessingCount(t *testing.T) {
	// Create a worker with both processed and processing counts
	worker := &WorkerStatus{
		Name:              "claude",
		Available:         true,
		ProcessedCount:    5,
		ProcessingCount:   2,
		LastActivity:      time.Now().Add(-30 * time.Second),
		CurrentScript:     "current.sh",
		LastFailureReason: "",
	}

	line := buildWorkerStatusLine("claude", worker)

	// Check that the line contains both counts
	if !strings.Contains(line, "Processing: 2") {
		t.Errorf("Expected line to contain 'Processing: 2', got: %s", line)
	}
	if !strings.Contains(line, "Processed: 5") {
		t.Errorf("Expected line to contain 'Processed: 5', got: %s", line)
	}
	if !strings.Contains(line, "📝 Processing current.sh") {
		t.Errorf("Expected line to show current script being processed, got: %s", line)
	}

	t.Logf("Generated line: %s", line)

	// Test with unavailable worker
	worker.Available = false
	worker.CurrentScript = ""
	worker.QuotaRecoveryTime = 45 * time.Minute
	worker.LastFailureReason = "Quota limit reached"

	line = buildWorkerStatusLine("claude", worker)
	if !strings.Contains(line, "⏳ Quota limit") {
		t.Errorf("Expected line to show quota limit status, got: %s", line)
	}
	if !strings.Contains(line, "Processing: 2") {
		t.Errorf("Expected line to still contain 'Processing: 2', got: %s", line)
	}

	t.Logf("Generated unavailable line: %s", line)
}
