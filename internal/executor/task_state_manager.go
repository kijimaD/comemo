package executor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"comemo/internal/config"
)

// TaskStateManager manages task state transitions, event status, and automatically emits event logs
type TaskStateManager struct {
	mu          sync.RWMutex
	states      map[string]*TaskStateEntry
	eventLogger *TaskEventLogger
	config      *config.Config
	maxRetries  int
}

// TaskState represents the current state of a task
type TaskState string

const (
	TaskStateQueued        TaskState = "QUEUED"
	TaskStateStarted       TaskState = "STARTED"
	TaskStateCompleted     TaskState = "COMPLETED"
	TaskStateFailed        TaskState = "FAILED"
	TaskStateRetrying      TaskState = "RETRYING"
	TaskStateTimeout       TaskState = "TIMEOUT"
	TaskStateQuotaError    TaskState = "QUOTA_EXCEEDED"
	TaskStateQualityFailed TaskState = "QUALITY_FAILED"
)

// TaskStateEntry represents a task's state information with event status integration
type TaskStateEntry struct {
	TaskID string
	CLI    string
	State  TaskState

	// Event Status integration
	EventStatus    EventStatus // Current event status (Running, Success, RetryWaiting, Failed)
	RetryCount     int
	RetryReason    string
	RetryDelayType RetryDelayType // Type of retry delay (quota, quality, timeout, other)
	NextRetryTime  time.Time      // When this task can be retried

	// Task execution details
	Error      string
	Output     string
	OutputPath string
	Duration   time.Duration
	StartTime  time.Time
	LastUpdate time.Time
	Metadata   map[string]interface{}
}

// NewTaskStateManager creates a new task state manager with event status integration
func NewTaskStateManager(eventLogger *TaskEventLogger, config *config.Config, maxRetries int) *TaskStateManager {
	return &TaskStateManager{
		states:      make(map[string]*TaskStateEntry),
		eventLogger: eventLogger,
		config:      config,
		maxRetries:  maxRetries,
	}
}

// TransitionToQueued transitions a task to queued state
func (m *TaskStateManager) TransitionToQueued(taskID, cli string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getOrCreateEntry(taskID)

	// Update TaskState
	entry.CLI = cli
	entry.State = TaskStateQueued
	entry.LastUpdate = time.Now()

	// Update EventStatus (queued tasks are typically in retry waiting or initial state)
	if entry.EventStatus == 0 {
		entry.EventStatus = EventStatusRetryWaiting // For new tasks
	}

	// Record state transition
	m.recordStateTransition(entry, "queued", "Task queued for execution")

	// Emit event log
	if m.eventLogger != nil {
		m.eventLogger.LogQueued(taskID, cli)
	}
}

// TransitionToStarted transitions a task to started state
func (m *TaskStateManager) TransitionToStarted(taskID, cli string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getOrCreateEntry(taskID)

	// Update TaskState
	entry.CLI = cli
	entry.State = TaskStateStarted
	entry.StartTime = time.Now()
	entry.LastUpdate = time.Now()

	// Update EventStatus
	entry.EventStatus = EventStatusRunning

	// Record state transition
	if entry.RetryCount > 0 {
		m.recordStateTransition(entry, "started", fmt.Sprintf("Task started (retry %d, reason: %s)", entry.RetryCount, entry.RetryReason))
	} else {
		m.recordStateTransition(entry, "started", "Task started for first time")
	}

	// Emit event log with retry information if applicable
	if m.eventLogger != nil {
		if entry.RetryCount > 0 {
			m.eventLogger.LogStartedWithRetry(taskID, cli, entry.RetryCount, entry.RetryReason)
		} else {
			m.eventLogger.LogStarted(taskID, cli)
		}
	}
}

// TransitionToCompleted transitions a task to completed state
func (m *TaskStateManager) TransitionToCompleted(taskID, cli string, duration time.Duration, outputPath, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getOrCreateEntry(taskID)

	// Update TaskState
	entry.CLI = cli
	entry.State = TaskStateCompleted
	entry.Duration = duration
	entry.OutputPath = outputPath
	entry.Output = output
	entry.LastUpdate = time.Now()

	// Update EventStatus
	entry.EventStatus = EventStatusSuccess

	// Clear retry information
	entry.NextRetryTime = time.Time{}
	entry.RetryDelayType = 0

	// Record state transition
	m.recordStateTransition(entry, "completed",
		fmt.Sprintf("Task completed successfully (duration: %v, output: %s)", duration, outputPath))

	// Emit event log
	if m.eventLogger != nil {
		m.eventLogger.LogCompletedWithOutput(taskID, cli, duration, outputPath, output)
	}
}

// TransitionToFailed transitions a task to failed state
func (m *TaskStateManager) TransitionToFailed(taskID, cli, errorMsg, output string, retryCount int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getOrCreateEntry(taskID)
	entry.CLI = cli
	entry.State = TaskStateFailed
	entry.Error = errorMsg
	entry.Output = output
	entry.RetryCount = retryCount
	entry.Duration = duration
	entry.LastUpdate = time.Now()

	// Emit event log
	if m.eventLogger != nil {
		m.eventLogger.LogFailedWithOutput(taskID, cli, errorMsg, retryCount, output)
	}
}

// TransitionToRetrying transitions a task to retrying state with retry scheduling
func (m *TaskStateManager) TransitionToRetrying(taskID, cli string, retryCount int, retryReason, errorMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getOrCreateEntry(taskID)

	// Update TaskState
	entry.CLI = cli
	entry.State = TaskStateRetrying
	entry.RetryCount = retryCount
	entry.RetryReason = retryReason
	entry.Error = errorMsg
	entry.LastUpdate = time.Now()

	// Update EventStatus and set retry timing
	entry.EventStatus = EventStatusRetryWaiting

	// Determine retry delay type and schedule next retry
	entry.RetryDelayType = m.getRetryDelayType(retryReason)
	entry.NextRetryTime = time.Now().Add(entry.RetryDelayType.GetRetryDelay())

	// Record state transition
	m.recordStateTransition(entry, "retrying",
		fmt.Sprintf("Task set to retry (count: %d, reason: %s, next retry: %v)",
			retryCount, retryReason, entry.NextRetryTime))

	// Emit event log with detailed reason
	if m.eventLogger != nil {
		detailedReason := fmt.Sprintf("%s: %s", retryReason, errorMsg)
		m.eventLogger.LogRetryingWithCLI(taskID, cli, retryCount, detailedReason)
	}
}

// TransitionToTimeout transitions a task to timeout state
func (m *TaskStateManager) TransitionToTimeout(taskID, cli, output string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getOrCreateEntry(taskID)
	entry.CLI = cli
	entry.State = TaskStateTimeout
	entry.Output = output
	entry.Duration = duration
	entry.LastUpdate = time.Now()

	// Emit event log
	if m.eventLogger != nil {
		m.eventLogger.LogTimeoutWithOutput(taskID, cli, duration, output)
	}
}

// TransitionToQuotaExceeded transitions a task to quota exceeded state
func (m *TaskStateManager) TransitionToQuotaExceeded(taskID, cli string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getOrCreateEntry(taskID)
	entry.CLI = cli
	entry.State = TaskStateQuotaError
	entry.LastUpdate = time.Now()

	// Emit event log
	if m.eventLogger != nil {
		m.eventLogger.LogQuotaExceeded(taskID, cli)
	}
}

// TransitionToQualityFailed transitions a task to quality check failed state
func (m *TaskStateManager) TransitionToQualityFailed(taskID, cli string, retryCount int, failureDetail, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.getOrCreateEntry(taskID)
	entry.CLI = cli
	entry.State = TaskStateQualityFailed
	entry.RetryCount = retryCount
	entry.Error = failureDetail
	entry.Output = output
	entry.LastUpdate = time.Now()

	// Emit event log
	if m.eventLogger != nil {
		m.eventLogger.LogQualityFailedWithDetails(taskID, cli, retryCount, failureDetail, output)
	}
}

// GetTaskState returns the current state of a task
func (m *TaskStateManager) GetTaskState(taskID string) *TaskStateEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entry, exists := m.states[taskID]; exists {
		// Return a copy to avoid concurrent access issues
		return &TaskStateEntry{
			TaskID:      entry.TaskID,
			CLI:         entry.CLI,
			State:       entry.State,
			RetryCount:  entry.RetryCount,
			RetryReason: entry.RetryReason,
			Error:       entry.Error,
			Output:      entry.Output,
			OutputPath:  entry.OutputPath,
			Duration:    entry.Duration,
			LastUpdate:  entry.LastUpdate,
			Metadata:    copyMetadata(entry.Metadata),
		}
	}
	return nil
}

// GetAllTaskStates returns all current task states
func (m *TaskStateManager) GetAllTaskStates() map[string]*TaskStateEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*TaskStateEntry)
	for taskID, entry := range m.states {
		result[taskID] = &TaskStateEntry{
			TaskID:      entry.TaskID,
			CLI:         entry.CLI,
			State:       entry.State,
			RetryCount:  entry.RetryCount,
			RetryReason: entry.RetryReason,
			Error:       entry.Error,
			Output:      entry.Output,
			OutputPath:  entry.OutputPath,
			Duration:    entry.Duration,
			LastUpdate:  entry.LastUpdate,
			Metadata:    copyMetadata(entry.Metadata),
		}
	}
	return result
}

// getOrCreateEntry gets an existing entry or creates a new one
func (m *TaskStateManager) getOrCreateEntry(taskID string) *TaskStateEntry {
	if entry, exists := m.states[taskID]; exists {
		return entry
	}

	entry := &TaskStateEntry{
		TaskID:   taskID,
		Metadata: make(map[string]interface{}),
	}
	m.states[taskID] = entry
	return entry
}

// copyMetadata creates a deep copy of metadata map
func copyMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range metadata {
		result[k] = v
	}
	return result
}

// recordStateTransition records a state transition for debugging and monitoring
func (m *TaskStateManager) recordStateTransition(entry *TaskStateEntry, transition, reason string) {
	// Add to metadata for history tracking
	if entry.Metadata == nil {
		entry.Metadata = make(map[string]interface{})
	}

	// Initialize transitions history if not exists
	if _, exists := entry.Metadata["transitions"]; !exists {
		entry.Metadata["transitions"] = []map[string]interface{}{}
	}

	// Add new transition record
	transitions := entry.Metadata["transitions"].([]map[string]interface{})
	newTransition := map[string]interface{}{
		"timestamp":    time.Now(),
		"transition":   transition,
		"from_state":   entry.State,
		"event_status": entry.EventStatus,
		"reason":       reason,
	}

	entry.Metadata["transitions"] = append(transitions, newTransition)
}

// getRetryDelayType determines the retry delay type based on retry reason
func (m *TaskStateManager) getRetryDelayType(retryReason string) RetryDelayType {
	switch {
	case strings.Contains(strings.ToLower(retryReason), "quota"):
		return RetryDelayQuota
	case strings.Contains(strings.ToLower(retryReason), "quality"):
		return RetryDelayQuality
	case strings.Contains(strings.ToLower(retryReason), "timeout"):
		return RetryDelayTimeout
	default:
		return RetryDelayOther
	}
}

// EventStatusManager compatibility methods

// GetRetryInfo returns retry information for a script (EventStatusManager compatibility)
func (m *TaskStateManager) GetRetryInfo(taskID string) (retryCount int, retryReason string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entry, exists := m.states[taskID]; exists {
		return entry.RetryCount, entry.RetryReason
	}
	return 0, ""
}

// IsRetryReady checks if a task is ready for retry (EventStatusManager compatibility)
func (m *TaskStateManager) IsRetryReady(taskID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if entry, exists := m.states[taskID]; exists {
		return entry.EventStatus == EventStatusRetryWaiting && time.Now().After(entry.NextRetryTime)
	}
	return false
}

// GetRetryReadyTasks returns all tasks ready for retry (EventStatusManager compatibility)
func (m *TaskStateManager) GetRetryReadyTasks() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var readyTasks []string
	for taskID, entry := range m.states {
		if entry.EventStatus == EventStatusRetryWaiting && time.Now().After(entry.NextRetryTime) {
			readyTasks = append(readyTasks, taskID)
		}
	}
	return readyTasks
}

// GetStatusCounts returns counts of tasks by event status (EventStatusManager compatibility)
func (m *TaskStateManager) GetStatusCounts() map[EventStatus]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[EventStatus]int)
	for _, entry := range m.states {
		counts[entry.EventStatus]++
	}
	return counts
}
