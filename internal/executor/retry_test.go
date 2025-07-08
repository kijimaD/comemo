package executor

import (
	"comemo/internal/logger"
	"testing"
)

func TestStatusManager_AddRetryScript_NoDuplicates(t *testing.T) {
	sm := NewStatusManager()
	sm.SetTotalScripts(10)

	// Add a script to retry queue
	sm.AddRetryScript("test1.sh")

	// Verify initial state
	status := sm.GetStatus()
	if status.Queue.Retrying != 1 {
		t.Errorf("Expected Retrying count to be 1, got %d", status.Queue.Retrying)
	}
	if len(status.Errors.RetryQueue) != 1 {
		t.Errorf("Expected RetryQueue length to be 1, got %d", len(status.Errors.RetryQueue))
	}

	// Try to add the same script again - should not increase count
	sm.AddRetryScript("test1.sh")

	// Verify no duplicate was added
	status = sm.GetStatus()
	if status.Queue.Retrying != 1 {
		t.Errorf("Expected Retrying count to remain 1, got %d", status.Queue.Retrying)
	}
	if len(status.Errors.RetryQueue) != 1 {
		t.Errorf("Expected RetryQueue length to remain 1, got %d", len(status.Errors.RetryQueue))
	}

	// Add a different script - should increase count
	sm.AddRetryScript("test2.sh")

	// Verify second script was added
	status = sm.GetStatus()
	if status.Queue.Retrying != 2 {
		t.Errorf("Expected Retrying count to be 2, got %d", status.Queue.Retrying)
	}
	if len(status.Errors.RetryQueue) != 2 {
		t.Errorf("Expected RetryQueue length to be 2, got %d", len(status.Errors.RetryQueue))
	}
}

func TestStatusManager_RemoveRetryScript(t *testing.T) {
	sm := NewStatusManager()
	sm.SetTotalScripts(10)

	// Add scripts to retry queue
	sm.AddRetryScript("test1.sh")
	sm.AddRetryScript("test2.sh")

	// Verify initial state
	status := sm.GetStatus()
	if status.Queue.Retrying != 2 {
		t.Errorf("Expected Retrying count to be 2, got %d", status.Queue.Retrying)
	}

	// Remove one script
	sm.RemoveRetryScript("test1.sh")

	// Verify script was removed
	status = sm.GetStatus()
	if status.Queue.Retrying != 1 {
		t.Errorf("Expected Retrying count to be 1, got %d", status.Queue.Retrying)
	}
	if len(status.Errors.RetryQueue) != 1 {
		t.Errorf("Expected RetryQueue length to be 1, got %d", len(status.Errors.RetryQueue))
	}
	if status.Errors.RetryQueue[0] != "test2.sh" {
		t.Errorf("Expected remaining script to be 'test2.sh', got %s", status.Errors.RetryQueue[0])
	}
}

func TestScheduler_QueueScript_WithCapacity(t *testing.T) {
	// Create a minimal scheduler for testing
	statusManager := NewStatusManager()
	queueManager := NewQueueManager([]string{"claude"}, 2, logger.Silent())
	scheduler := &Scheduler{
		queueManager:  queueManager,
		statusManager: statusManager,
		logger:        logger.Silent(),
	}

	// Add first script to queue
	success := scheduler.queueScript("test1.sh", "claude")

	// Verify initial state
	if !success {
		t.Errorf("Expected queueScript to succeed")
	}
	if scheduler.queueManager.Length("claude") != 1 {
		t.Errorf("Expected queue length to be 1, got %d", scheduler.queueManager.Length("claude"))
	}

	// Add second script - should succeed (within capacity)
	success = scheduler.queueScript("test2.sh", "claude")

	// Verify second script was added
	if !success {
		t.Errorf("Expected queueScript to succeed for second script")
	}
	if scheduler.queueManager.Length("claude") != 2 {
		t.Errorf("Expected queue length to be 2, got %d", scheduler.queueManager.Length("claude"))
	}

	// Try to add third script - should fail (exceeds capacity)
	success = scheduler.queueScript("test3.sh", "claude")

	// Verify third script was rejected
	if success {
		t.Errorf("Expected queueScript to fail when exceeding capacity")
	}
	if scheduler.queueManager.Length("claude") != 2 {
		t.Errorf("Expected queue length to remain 2, got %d", scheduler.queueManager.Length("claude"))
	}
}
