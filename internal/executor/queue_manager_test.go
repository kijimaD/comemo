package executor

import (
	"comemo/internal/logger"
	"testing"
)

func TestQueueManager_BasicOperations(t *testing.T) {
	logger := logger.Silent()
	qm := NewQueueManager([]string{"claude", "gemini"}, 3, logger)

	// Test enqueue
	t.Run("Enqueue", func(t *testing.T) {
		success := qm.Enqueue("claude", "test1.sh")
		if !success {
			t.Errorf("Expected enqueue to succeed")
		}

		if qm.Length("claude") != 1 {
			t.Errorf("Expected queue length to be 1, got %d", qm.Length("claude"))
		}
	})

	// Test capacity
	t.Run("Capacity", func(t *testing.T) {
		// Fill queue to capacity
		qm.Enqueue("claude", "test2.sh")
		qm.Enqueue("claude", "test3.sh")

		if qm.Length("claude") != 3 {
			t.Errorf("Expected queue length to be 3, got %d", qm.Length("claude"))
		}

		// Try to exceed capacity
		success := qm.Enqueue("claude", "test4.sh")
		if success {
			t.Errorf("Expected enqueue to fail when capacity exceeded")
		}
	})

	// Test dequeue
	t.Run("Dequeue", func(t *testing.T) {
		script, ok := qm.Dequeue("claude")
		if !ok {
			t.Errorf("Expected dequeue to succeed")
		}

		if script != "test1.sh" {
			t.Errorf("Expected script to be 'test1.sh', got '%s'", script)
		}

		if qm.Length("claude") != 2 {
			t.Errorf("Expected queue length to be 2, got %d", qm.Length("claude"))
		}
	})

	// Test peek
	t.Run("Peek", func(t *testing.T) {
		script, ok := qm.Peek("claude")
		if !ok {
			t.Errorf("Expected peek to succeed")
		}

		if script != "test2.sh" {
			t.Errorf("Expected script to be 'test2.sh', got '%s'", script)
		}

		// Queue length should remain same after peek
		if qm.Length("claude") != 2 {
			t.Errorf("Expected queue length to remain 2, got %d", qm.Length("claude"))
		}
	})

	// Test remove
	t.Run("Remove", func(t *testing.T) {
		success := qm.Remove("claude", "test3.sh")
		if !success {
			t.Errorf("Expected remove to succeed")
		}

		if qm.Length("claude") != 1 {
			t.Errorf("Expected queue length to be 1, got %d", qm.Length("claude"))
		}
	})
}

func TestQueueManager_CapacityOperations(t *testing.T) {
	logger := logger.Silent()
	qm := NewQueueManager([]string{"claude", "gemini"}, 2, logger)

	// Test HasCapacity
	t.Run("HasCapacity", func(t *testing.T) {
		if !qm.HasCapacity("claude") {
			t.Errorf("Expected claude to have capacity")
		}

		qm.Enqueue("claude", "test1.sh")
		qm.Enqueue("claude", "test2.sh")

		if qm.HasCapacity("claude") {
			t.Errorf("Expected claude to not have capacity")
		}
	})

	// Test GetAvailableSlots
	t.Run("GetAvailableSlots", func(t *testing.T) {
		slots := qm.GetAvailableSlots("gemini")
		if slots != 2 {
			t.Errorf("Expected 2 available slots, got %d", slots)
		}

		qm.Enqueue("gemini", "test1.sh")
		slots = qm.GetAvailableSlots("gemini")
		if slots != 1 {
			t.Errorf("Expected 1 available slot, got %d", slots)
		}
	})

	// Test TotalLength
	t.Run("TotalLength", func(t *testing.T) {
		total := qm.TotalLength()
		if total != 3 { // 2 in claude + 1 in gemini
			t.Errorf("Expected total length to be 3, got %d", total)
		}
	})
}

func TestQueueManager_SearchOperations(t *testing.T) {
	logger := logger.Silent()
	qm := NewQueueManager([]string{"claude", "gemini"}, 3, logger)

	qm.Enqueue("claude", "test1.sh")
	qm.Enqueue("gemini", "test2.sh")

	// Test IsScriptInQueue
	t.Run("IsScriptInQueue", func(t *testing.T) {
		cliName, found := qm.IsScriptInQueue("test1.sh")
		if !found {
			t.Errorf("Expected script to be found")
		}
		if cliName != "claude" {
			t.Errorf("Expected script to be in claude, got %s", cliName)
		}

		_, found = qm.IsScriptInQueue("nonexistent.sh")
		if found {
			t.Errorf("Expected script to not be found")
		}
	})

	// Test IsScriptInSpecificQueue
	t.Run("IsScriptInSpecificQueue", func(t *testing.T) {
		found := qm.IsScriptInSpecificQueue("claude", "test1.sh")
		if !found {
			t.Errorf("Expected script to be found in claude queue")
		}

		found = qm.IsScriptInSpecificQueue("gemini", "test1.sh")
		if found {
			t.Errorf("Expected script to not be found in gemini queue")
		}
	})
}

func TestQueueManager_StatusOperations(t *testing.T) {
	logger := logger.Silent()
	qm := NewQueueManager([]string{"claude", "gemini"}, 3, logger)

	qm.Enqueue("claude", "test1.sh")
	qm.Enqueue("claude", "test2.sh")
	qm.Enqueue("gemini", "test3.sh")

	// Test GetQueueStatus
	t.Run("GetQueueStatus", func(t *testing.T) {
		status := qm.GetQueueStatus()
		if status["claude"] != 2 {
			t.Errorf("Expected claude queue to have 2 scripts, got %d", status["claude"])
		}
		if status["gemini"] != 1 {
			t.Errorf("Expected gemini queue to have 1 script, got %d", status["gemini"])
		}
	})

	// Test GetQueueCopy
	t.Run("GetQueueCopy", func(t *testing.T) {
		queue := qm.GetQueueCopy("claude")
		if len(queue) != 2 {
			t.Errorf("Expected queue copy to have 2 scripts, got %d", len(queue))
		}
		if queue[0] != "test1.sh" || queue[1] != "test2.sh" {
			t.Errorf("Expected queue copy to have correct scripts")
		}
	})

	// Test GetAllQueues
	t.Run("GetAllQueues", func(t *testing.T) {
		allQueues := qm.GetAllQueues()
		if len(allQueues["claude"]) != 2 {
			t.Errorf("Expected claude queue to have 2 scripts, got %d", len(allQueues["claude"]))
		}
		if len(allQueues["gemini"]) != 1 {
			t.Errorf("Expected gemini queue to have 1 script, got %d", len(allQueues["gemini"]))
		}
	})
}

func TestQueueManager_ClearOperations(t *testing.T) {
	logger := logger.Silent()
	qm := NewQueueManager([]string{"claude", "gemini"}, 3, logger)

	qm.Enqueue("claude", "test1.sh")
	qm.Enqueue("gemini", "test2.sh")

	// Test Clear
	t.Run("Clear", func(t *testing.T) {
		qm.Clear("claude")
		if qm.Length("claude") != 0 {
			t.Errorf("Expected claude queue to be empty after clear")
		}
		if qm.Length("gemini") != 1 {
			t.Errorf("Expected gemini queue to still have 1 script")
		}
	})

	// Test ClearAll
	t.Run("ClearAll", func(t *testing.T) {
		qm.Enqueue("claude", "test3.sh")
		qm.ClearAll()
		if qm.TotalLength() != 0 {
			t.Errorf("Expected all queues to be empty after clear all")
		}
	})
}

func TestQueueManager_ProcessQueue(t *testing.T) {
	logger := logger.Silent()
	qm := NewQueueManager([]string{"claude"}, 3, logger)

	// Test empty queue
	t.Run("EmptyQueue", func(t *testing.T) {
		task := qm.ProcessQueue("claude")
		if task != nil {
			t.Errorf("Expected nil task for empty queue")
		}
	})

	// Test queue with scripts
	t.Run("QueueWithScripts", func(t *testing.T) {
		qm.Enqueue("claude", "test1.sh")
		qm.Enqueue("claude", "test2.sh")

		task := qm.ProcessQueue("claude")
		if task == nil {
			t.Errorf("Expected task for non-empty queue")
		}
		if task.Script != "test1.sh" {
			t.Errorf("Expected script to be 'test1.sh', got '%s'", task.Script)
		}
		if task.CLI != "claude" {
			t.Errorf("Expected CLI to be 'claude', got '%s'", task.CLI)
		}
	})

	// Test MarkScriptProcessed
	t.Run("MarkScriptProcessed", func(t *testing.T) {
		success := qm.MarkScriptProcessed("claude", "test1.sh")
		if !success {
			t.Errorf("Expected mark script processed to succeed")
		}
		if qm.Length("claude") != 1 {
			t.Errorf("Expected queue length to be 1 after processing, got %d", qm.Length("claude"))
		}

		// Test wrong script
		success = qm.MarkScriptProcessed("claude", "wrong.sh")
		if success {
			t.Errorf("Expected mark script processed to fail for wrong script")
		}
	})
}
