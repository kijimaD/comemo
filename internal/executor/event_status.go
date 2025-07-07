package executor

import (
	"sync"
	"time"
)

// EventStatus represents the detailed status of a script execution event
type EventStatus int

const (
	// EventStatusRunning indicates the script is currently executing
	EventStatusRunning EventStatus = iota
	// EventStatusRetryWaiting indicates the script is waiting for retry
	EventStatusRetryWaiting
	// EventStatusFailed indicates the script has failed permanently
	EventStatusFailed
	// EventStatusSuccess indicates the script has completed successfully
	EventStatusSuccess
)

// String returns the string representation of EventStatus
func (e EventStatus) String() string {
	switch e {
	case EventStatusRunning:
		return "実行中"
	case EventStatusRetryWaiting:
		return "リトライ待ち"
	case EventStatusFailed:
		return "失敗"
	case EventStatusSuccess:
		return "成功"
	default:
		return "不明"
	}
}

// RetryDelayType represents different types of retry delays
type RetryDelayType int

const (
	// RetryDelayQuota for quota errors (1 hour)
	RetryDelayQuota RetryDelayType = iota
	// RetryDelayQuality for quality test errors (10 seconds)
	RetryDelayQuality
	// RetryDelayTimeout for timeout errors (5 minutes)
	RetryDelayTimeout
	// RetryDelayOther for other retryable errors (30 seconds)
	RetryDelayOther
)

// GetRetryDelay returns the appropriate delay duration for the retry type
func (r RetryDelayType) GetRetryDelay() time.Duration {
	switch r {
	case RetryDelayQuota:
		return 1 * time.Hour
	case RetryDelayQuality:
		return 10 * time.Second
	case RetryDelayTimeout:
		return 5 * time.Minute
	case RetryDelayOther:
		return 30 * time.Second
	default:
		return 30 * time.Second
	}
}

// String returns the string representation of RetryDelayType
func (r RetryDelayType) String() string {
	switch r {
	case RetryDelayQuota:
		return "quota error"
	case RetryDelayQuality:
		return "quality error"
	case RetryDelayTimeout:
		return "timeout error"
	case RetryDelayOther:
		return "other error"
	default:
		return "unknown error"
	}
}

// EventStatusEntry represents a single event status entry with timing information
type EventStatusEntry struct {
	ScriptName    string         `json:"script_name"`
	CLI           string         `json:"cli"`
	Status        EventStatus    `json:"status"`
	StartTime     time.Time      `json:"start_time"`
	LastUpdate    time.Time      `json:"last_update"`
	RetryCount    int            `json:"retry_count"`
	RetryDelayType RetryDelayType `json:"retry_delay_type,omitempty"`
	NextRetryTime time.Time      `json:"next_retry_time,omitempty"`
	ErrorMessage  string         `json:"error_message,omitempty"`
	Duration      time.Duration  `json:"duration,omitempty"`
}

// IsRetryReady checks if the entry is ready for retry
func (e *EventStatusEntry) IsRetryReady() bool {
	if e.Status != EventStatusRetryWaiting {
		return false
	}
	return time.Now().After(e.NextRetryTime)
}

// GetWaitingTime returns how long the entry has been in the current status
func (e *EventStatusEntry) GetWaitingTime() time.Duration {
	return time.Since(e.LastUpdate)
}

// GetTimeUntilRetry returns the time remaining until retry
func (e *EventStatusEntry) GetTimeUntilRetry() time.Duration {
	if e.Status != EventStatusRetryWaiting {
		return 0
	}
	remaining := e.NextRetryTime.Sub(time.Now())
	if remaining < 0 {
		return 0
	}
	return remaining
}

// EventStatusManager manages event status for all scripts
type EventStatusManager struct {
	entries map[string]*EventStatusEntry
	mu      sync.RWMutex
	maxRetries int
}

// GetRetryInfo returns retry information for a script
func (m *EventStatusManager) GetRetryInfo(scriptName string) (retryCount int, retryReason string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if entry, exists := m.entries[scriptName]; exists {
		return entry.RetryCount, entry.RetryDelayType.String()
	}
	return 0, ""
}

// NewEventStatusManager creates a new event status manager
func NewEventStatusManager(maxRetries int) *EventStatusManager {
	return &EventStatusManager{
		entries:    make(map[string]*EventStatusEntry),
		maxRetries: maxRetries,
	}
}

// StartExecution marks a script as running
func (m *EventStatusManager) StartExecution(scriptName, cli string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, exists := m.entries[scriptName]
	if !exists {
		entry = &EventStatusEntry{
			ScriptName: scriptName,
			CLI:        cli,
			RetryCount: 0,
		}
		m.entries[scriptName] = entry
	}

	entry.Status = EventStatusRunning
	entry.StartTime = time.Now()
	entry.LastUpdate = time.Now()
	entry.CLI = cli
}

// CompleteSuccess marks a script as successfully completed
func (m *EventStatusManager) CompleteSuccess(scriptName string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.entries[scriptName]; exists {
		entry.Status = EventStatusSuccess
		entry.LastUpdate = time.Now()
		entry.Duration = duration
	}
}

// SetRetryWaiting marks a script as waiting for retry
func (m *EventStatusManager) SetRetryWaiting(scriptName string, retryDelayType RetryDelayType, errorMessage string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.entries[scriptName]; exists {
		entry.RetryCount++
		
		// Check if we've exceeded max retries
		if entry.RetryCount > m.maxRetries {
			entry.Status = EventStatusFailed
			entry.LastUpdate = time.Now()
			entry.ErrorMessage = errorMessage
			return
		}

		// Set retry waiting status
		entry.Status = EventStatusRetryWaiting
		entry.LastUpdate = time.Now()
		entry.RetryDelayType = retryDelayType
		entry.NextRetryTime = time.Now().Add(retryDelayType.GetRetryDelay())
		entry.ErrorMessage = errorMessage
	}
}

// SetFailed marks a script as permanently failed
func (m *EventStatusManager) SetFailed(scriptName string, errorMessage string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry, exists := m.entries[scriptName]; exists {
		entry.Status = EventStatusFailed
		entry.LastUpdate = time.Now()
		entry.ErrorMessage = errorMessage
	}
}

// GetEntry returns the status entry for a script
func (m *EventStatusManager) GetEntry(scriptName string) (*EventStatusEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.entries[scriptName]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	entryCopy := *entry
	return &entryCopy, true
}

// GetAllEntries returns all status entries
func (m *EventStatusManager) GetAllEntries() map[string]*EventStatusEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*EventStatusEntry)
	for name, entry := range m.entries {
		entryCopy := *entry
		result[name] = &entryCopy
	}
	return result
}

// GetRetryReadyScripts returns scripts that are ready for retry
func (m *EventStatusManager) GetRetryReadyScripts() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var readyScripts []string
	for name, entry := range m.entries {
		if entry.IsRetryReady() {
			readyScripts = append(readyScripts, name)
		}
	}
	return readyScripts
}

// GetStatusCounts returns counts for each status
func (m *EventStatusManager) GetStatusCounts() map[EventStatus]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[EventStatus]int)
	for _, entry := range m.entries {
		counts[entry.Status]++
	}
	return counts
}

// GetRetryWaitingSummary returns summary of scripts waiting for retry
func (m *EventStatusManager) GetRetryWaitingSummary() map[RetryDelayType]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := make(map[RetryDelayType]int)
	for _, entry := range m.entries {
		if entry.Status == EventStatusRetryWaiting {
			summary[entry.RetryDelayType]++
		}
	}
	return summary
}

// GetRetryDelayTypeFromErrorType returns the appropriate retry delay type based on error type
func GetRetryDelayTypeFromErrorType(errorType ErrorType) RetryDelayType {
	switch errorType {
	case ErrorTypeQuota:
		return RetryDelayQuota
	case ErrorTypeQuality:
		return RetryDelayQuality
	case ErrorTypeTimeout:
		return RetryDelayTimeout
	case ErrorTypeRetryable:
		return RetryDelayOther
	default:
		return RetryDelayOther
	}
}