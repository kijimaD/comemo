package executor

import (
	"sync"
	"time"

	"comemo/internal/logger"
)

// QueueManager manages script queues for different CLIs
type QueueManager struct {
	queues   map[string][]string // CLI名 -> キューイング中のスクリプトリスト
	capacity int                 // 各CLIのキュー容量
	mu       sync.RWMutex
	logger   *logger.Logger
}

// NewQueueManager creates a new queue manager
func NewQueueManager(cliNames []string, capacity int, logger *logger.Logger) *QueueManager {
	qm := &QueueManager{
		queues:   make(map[string][]string),
		capacity: capacity,
		logger:   logger,
	}

	// 各CLIのキューを初期化
	for _, cliName := range cliNames {
		qm.queues[cliName] = make([]string, 0, capacity)
	}

	return qm
}

// Enqueue adds a script to the specified CLI's queue
func (qm *QueueManager) Enqueue(cliName string, script string) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// キューの容量チェック
	if len(qm.queues[cliName]) >= qm.capacity {
		qm.logger.Debug("CLI %s のキューが満杯: %d/%d", cliName, len(qm.queues[cliName]), qm.capacity)
		return false
	}

	// スクリプトをキューに追加
	qm.queues[cliName] = append(qm.queues[cliName], script)
	qm.logger.Debug("スクリプト %s を %s にキューイング (%d/%d)", script, cliName, len(qm.queues[cliName]), qm.capacity)
	return true
}

// Dequeue removes and returns the first script from the specified CLI's queue
func (qm *QueueManager) Dequeue(cliName string) (string, bool) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if len(qm.queues[cliName]) == 0 {
		return "", false
	}

	script := qm.queues[cliName][0]
	qm.queues[cliName] = qm.queues[cliName][1:]
	qm.logger.Debug("スクリプト %s を %s のキューから除去 (残り: %d)", script, cliName, len(qm.queues[cliName]))
	return script, true
}

// Remove removes a specific script from the specified CLI's queue
func (qm *QueueManager) Remove(cliName string, script string) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	queue := qm.queues[cliName]
	for i, queuedScript := range queue {
		if queuedScript == script {
			// スクリプトをキューから削除
			qm.queues[cliName] = append(queue[:i], queue[i+1:]...)
			qm.logger.Debug("スクリプト %s を %s のキューから除去 (残り: %d)", script, cliName, len(qm.queues[cliName]))
			return true
		}
	}
	qm.logger.Debug("スクリプト %s が %s のキューに見つからない", script, cliName)
	return false
}

// Peek returns the first script in the specified CLI's queue without removing it
func (qm *QueueManager) Peek(cliName string) (string, bool) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	if len(qm.queues[cliName]) == 0 {
		return "", false
	}

	return qm.queues[cliName][0], true
}

// Length returns the number of scripts in the specified CLI's queue
func (qm *QueueManager) Length(cliName string) int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	return len(qm.queues[cliName])
}

// HasCapacity checks if the specified CLI's queue has available capacity
func (qm *QueueManager) HasCapacity(cliName string) bool {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	return len(qm.queues[cliName]) < qm.capacity
}

// TotalLength returns the total number of scripts across all queues
func (qm *QueueManager) TotalLength() int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	total := 0
	for _, queue := range qm.queues {
		total += len(queue)
	}
	return total
}

// GetAvailableSlots returns the number of available slots for the specified CLI
func (qm *QueueManager) GetAvailableSlots(cliName string) int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	return qm.capacity - len(qm.queues[cliName])
}

// GetQueueStatus returns the current status of all queues
func (qm *QueueManager) GetQueueStatus() map[string]int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	status := make(map[string]int)
	for cliName, queue := range qm.queues {
		status[cliName] = len(queue)
	}
	return status
}

// IsScriptInQueue checks if a script is in any queue
func (qm *QueueManager) IsScriptInQueue(script string) (string, bool) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	for cliName, queue := range qm.queues {
		for _, queuedScript := range queue {
			if queuedScript == script {
				return cliName, true
			}
		}
	}
	return "", false
}

// IsScriptInSpecificQueue checks if a script is in a specific CLI's queue
func (qm *QueueManager) IsScriptInSpecificQueue(cliName string, script string) bool {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	for _, queuedScript := range qm.queues[cliName] {
		if queuedScript == script {
			return true
		}
	}
	return false
}

// Clear removes all scripts from the specified CLI's queue
func (qm *QueueManager) Clear(cliName string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	qm.queues[cliName] = qm.queues[cliName][:0]
	qm.logger.Debug("CLI %s のキューをクリア", cliName)
}

// ClearAll removes all scripts from all queues
func (qm *QueueManager) ClearAll() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	for cliName := range qm.queues {
		qm.queues[cliName] = qm.queues[cliName][:0]
	}
	qm.logger.Debug("全てのキューをクリア")
}

// GetQueueCopy returns a copy of the specified CLI's queue
func (qm *QueueManager) GetQueueCopy(cliName string) []string {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	queue := qm.queues[cliName]
	copy := make([]string, len(queue))
	for i, script := range queue {
		copy[i] = script
	}
	return copy
}

// GetAllQueues returns a copy of all queues
func (qm *QueueManager) GetAllQueues() map[string][]string {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	result := make(map[string][]string)
	for cliName, queue := range qm.queues {
		result[cliName] = make([]string, len(queue))
		copy(result[cliName], queue)
	}
	return result
}

// ProcessQueue processes the queue for a specific CLI and returns the next task if available
func (qm *QueueManager) ProcessQueue(cliName string) *Task {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if len(qm.queues[cliName]) == 0 {
		return nil
	}

	script := qm.queues[cliName][0]
	return &Task{
		Script:  script,
		CLI:     cliName,
		AddedAt: time.Now(),
	}
}

// MarkScriptProcessed removes the first script from the queue after processing
func (qm *QueueManager) MarkScriptProcessed(cliName string, script string) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if len(qm.queues[cliName]) == 0 {
		return false
	}

	// 最初のスクリプトが処理対象と一致するかチェック
	if qm.queues[cliName][0] == script {
		qm.queues[cliName] = qm.queues[cliName][1:]
		qm.logger.Debug("処理済みスクリプト %s を %s のキューから除去 (残り: %d)", script, cliName, len(qm.queues[cliName]))
		return true
	}

	return false
}
