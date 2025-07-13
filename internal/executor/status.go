package executor

import (
	"sync"
	"time"
)

// ExecutionStatus holds the complete status of script execution
type ExecutionStatus struct {
	Workers     map[string]*WorkerStatus `json:"workers"`
	Queue       *QueueStatus             `json:"queue"`
	Performance *PerformanceMetrics      `json:"performance"`
	Errors      *ErrorStatus             `json:"errors"`
}

// WorkerStatus represents the status of a single CLI worker
type WorkerStatus struct {
	Name              string        `json:"name"`
	Available         bool          `json:"available"`
	QuotaRecoveryTime time.Duration `json:"quota_recovery_time"`
	CurrentScript     string        `json:"current_script"`
	ProcessedCount    int           `json:"processed_count"`
	ProcessingCount   int           `json:"processing_count"` // Current processing count
	LastActivity      time.Time     `json:"last_activity"`
	LastFailureReason string        `json:"last_failure_reason"`
}

// QueueStatus represents the current queue state
type QueueStatus struct {
	Waiting    int `json:"waiting"`
	Processing int `json:"processing"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
	Retrying   int `json:"retrying"`
	Total      int `json:"total"`
}

// PerformanceMetrics tracks execution performance
type PerformanceMetrics struct {
	StartTime        time.Time     `json:"start_time"`
	ElapsedTime      time.Duration `json:"elapsed_time"`
	ScriptsPerMinute float64       `json:"scripts_per_minute"`
	EstimatedETA     time.Duration `json:"estimated_eta"`
	TotalScripts     int           `json:"total_scripts"`
}

// ErrorStatus tracks recent errors and retry information
type ErrorStatus struct {
	RecentFailures []FailureInfo `json:"recent_failures"`
	RetryQueue     []string      `json:"retry_queue"`
	LastError      string        `json:"last_error"`
	ErrorCount     int           `json:"error_count"`
}

// FailureInfo represents a single failure
type FailureInfo struct {
	ScriptName string    `json:"script_name"`
	Reason     string    `json:"reason"`
	Timestamp  time.Time `json:"timestamp"`
	RetryCount int       `json:"retry_count"`
}

// StatusManager manages the execution status and provides thread-safe updates
type StatusManager struct {
	status   *ExecutionStatus
	ticker   *time.Ticker
	done     chan bool
	updating bool
	mu       sync.RWMutex
}

// NewStatusManager creates a new status manager
func NewStatusManager() *StatusManager {
	return &StatusManager{
		status: &ExecutionStatus{
			Workers: make(map[string]*WorkerStatus),
			Queue: &QueueStatus{
				Total: 0,
			},
			Performance: &PerformanceMetrics{
				StartTime: time.Now(),
			},
			Errors: &ErrorStatus{
				RecentFailures: make([]FailureInfo, 0),
				RetryQueue:     make([]string, 0),
			},
		},
		done: make(chan bool),
	}
}

// GetStatus returns a copy of the current status
func (sm *StatusManager) GetStatus() *ExecutionStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Deep copy to avoid race conditions
	status := &ExecutionStatus{
		Workers:     make(map[string]*WorkerStatus),
		Queue:       &QueueStatus{},
		Performance: &PerformanceMetrics{},
		Errors: &ErrorStatus{
			RecentFailures: make([]FailureInfo, len(sm.status.Errors.RecentFailures)),
			RetryQueue:     make([]string, len(sm.status.Errors.RetryQueue)),
		},
	}

	// Copy workers
	for name, worker := range sm.status.Workers {
		status.Workers[name] = &WorkerStatus{
			Name:              worker.Name,
			Available:         worker.Available,
			QuotaRecoveryTime: worker.QuotaRecoveryTime,
			CurrentScript:     worker.CurrentScript,
			ProcessedCount:    worker.ProcessedCount,
			ProcessingCount:   worker.ProcessingCount,
			LastActivity:      worker.LastActivity,
			LastFailureReason: worker.LastFailureReason,
		}
	}

	// Copy queue status
	*status.Queue = *sm.status.Queue

	// Copy performance metrics
	*status.Performance = *sm.status.Performance

	// Copy error status
	*status.Errors = *sm.status.Errors
	copy(status.Errors.RecentFailures, sm.status.Errors.RecentFailures)
	copy(status.Errors.RetryQueue, sm.status.Errors.RetryQueue)

	return status
}

// InitializeWorker initializes a worker in the status
func (sm *StatusManager) InitializeWorker(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.status.Workers[name] = &WorkerStatus{
		Name:         name,
		Available:    true,
		LastActivity: time.Now(),
	}
}

// UpdateWorkerStatus updates a worker's status
func (sm *StatusManager) UpdateWorkerStatus(name string, available bool, currentScript string, quotaRecoveryTime time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if worker, exists := sm.status.Workers[name]; exists {
		worker.Available = available
		worker.CurrentScript = currentScript
		worker.QuotaRecoveryTime = quotaRecoveryTime
		worker.LastActivity = time.Now()
	}
}

// RecordScriptStart records when a script starts processing
func (sm *StatusManager) RecordScriptStart(scriptName, workerName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.status.Queue.Processing++
	sm.status.Queue.Waiting--

	if worker, exists := sm.status.Workers[workerName]; exists {
		worker.CurrentScript = scriptName
		worker.ProcessingCount++
		worker.LastActivity = time.Now()
	}
}

// RecordScriptComplete records when a script completes (success or failure)
func (sm *StatusManager) RecordScriptComplete(scriptName, workerName string, success bool, duration time.Duration, errorMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.status.Queue.Processing--

	if success {
		sm.status.Queue.Completed++
	} else {
		sm.status.Queue.Failed++
		// Record failure
		sm.status.Errors.RecentFailures = append(sm.status.Errors.RecentFailures, FailureInfo{
			ScriptName: scriptName,
			Reason:     errorMsg,
			Timestamp:  time.Now(),
		})
		sm.status.Errors.LastError = errorMsg
		sm.status.Errors.ErrorCount++

		// Keep only last 10 failures
		if len(sm.status.Errors.RecentFailures) > 10 {
			sm.status.Errors.RecentFailures = sm.status.Errors.RecentFailures[1:]
		}
	}

	if worker, exists := sm.status.Workers[workerName]; exists {
		worker.CurrentScript = ""
		worker.ProcessedCount++
		worker.ProcessingCount--
		worker.LastActivity = time.Now()

		// Update worker's last failure reason
		if !success && errorMsg != "" {
			worker.LastFailureReason = errorMsg
		}
	}

	// Update performance metrics
	sm.updatePerformanceMetrics()
}

// SetTotalScripts sets the total number of scripts to process
func (sm *StatusManager) SetTotalScripts(total int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.status.Queue.Total = total
	sm.status.Queue.Waiting = total
	sm.status.Performance.TotalScripts = total
}

// AddRetryScript adds a script to the retry queue (prevents duplicates)
func (sm *StatusManager) AddRetryScript(scriptName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if already in retry queue
	for _, existingName := range sm.status.Errors.RetryQueue {
		if existingName == scriptName {
			return // Already in retry queue, don't add again
		}
	}

	sm.status.Errors.RetryQueue = append(sm.status.Errors.RetryQueue, scriptName)
	sm.status.Queue.Retrying++
	// Remove the logic that decrements failed count
	// The failed count should only be managed by RecordScriptComplete
}

// RemoveRetryScript removes a script from the retry queue
func (sm *StatusManager) RemoveRetryScript(scriptName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i, name := range sm.status.Errors.RetryQueue {
		if name == scriptName {
			sm.status.Errors.RetryQueue = append(sm.status.Errors.RetryQueue[:i], sm.status.Errors.RetryQueue[i+1:]...)
			sm.status.Queue.Retrying--
			sm.status.Queue.Waiting++
			break
		}
	}
}

// RecordQuotaError records a quota error without marking script as failed
func (sm *StatusManager) RecordQuotaError(scriptName, workerName string, duration time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Decrease processing count
	sm.status.Queue.Processing--
	// Script goes back to waiting (in CLI queue)
	sm.status.Queue.Waiting++

	// Update worker status but don't increment failed count
	if worker, exists := sm.status.Workers[workerName]; exists {
		worker.CurrentScript = ""
		worker.ProcessingCount--
		worker.LastActivity = time.Now()
		worker.LastFailureReason = "Quota limit reached - waiting for recovery"
	}

	// Update performance metrics
	sm.updatePerformanceMetrics()
}

// updatePerformanceMetrics calculates performance metrics (must be called with lock held)
func (sm *StatusManager) updatePerformanceMetrics() {
	now := time.Now()
	sm.status.Performance.ElapsedTime = now.Sub(sm.status.Performance.StartTime)

	if sm.status.Performance.ElapsedTime.Minutes() > 0 {
		sm.status.Performance.ScriptsPerMinute = float64(sm.status.Queue.Completed) / sm.status.Performance.ElapsedTime.Minutes()
	}

	// Calculate ETA
	remaining := sm.status.Queue.Total - sm.status.Queue.Completed
	if sm.status.Performance.ScriptsPerMinute > 0 && remaining > 0 {
		etaMinutes := float64(remaining) / sm.status.Performance.ScriptsPerMinute
		sm.status.Performance.EstimatedETA = time.Duration(etaMinutes * float64(time.Minute))
	}
}

// Start begins the status manager's update loop
func (sm *StatusManager) Start() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.updating {
		return
	}

	sm.updating = true
	sm.ticker = time.NewTicker(500 * time.Millisecond) // Update every 500ms

	go func() {
		for {
			select {
			case <-sm.ticker.C:
				sm.updatePerformanceMetrics()
			case <-sm.done:
				return
			}
		}
	}()
}

// Stop stops the status manager
func (sm *StatusManager) Stop() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.updating {
		return
	}

	sm.updating = false
	sm.ticker.Stop()
	close(sm.done)
}

// RecordRetryError records a non-quota retryable error without marking script as failed
func (sm *StatusManager) RecordRetryError(scriptName, workerName string, duration time.Duration, errorMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Decrease processing count
	sm.status.Queue.Processing--
	// Script goes back to waiting (will be retried)
	sm.status.Queue.Waiting++

	// Update worker status but don't increment failed count
	if worker, exists := sm.status.Workers[workerName]; exists {
		worker.CurrentScript = ""
		worker.ProcessingCount--
		worker.LastActivity = time.Now()
		if errorMsg != "" {
			worker.LastFailureReason = "Retrying: " + errorMsg
		} else {
			worker.LastFailureReason = "Retrying after error"
		}
	}

	// Update performance metrics
	sm.updatePerformanceMetrics()
}
