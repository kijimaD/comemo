package executor

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// TaskEventType represents the type of task event
type TaskEventType string

const (
	// TaskEventQueued indicates a task was added to queue
	TaskEventQueued TaskEventType = "QUEUED"
	// TaskEventStarted indicates a task execution started
	TaskEventStarted TaskEventType = "STARTED"
	// TaskEventCompleted indicates a task completed successfully
	TaskEventCompleted TaskEventType = "COMPLETED"
	// TaskEventFailed indicates a task failed
	TaskEventFailed TaskEventType = "FAILED"
	// TaskEventRetrying indicates a task is being retried
	TaskEventRetrying TaskEventType = "RETRYING"
	// TaskEventTimeout indicates a task timed out
	TaskEventTimeout TaskEventType = "TIMEOUT"
	// TaskEventQuotaExceeded indicates quota was exceeded
	TaskEventQuotaExceeded TaskEventType = "QUOTA_EXCEEDED"
	// TaskEventQualityFailed indicates quality check failed
	TaskEventQualityFailed TaskEventType = "QUALITY_FAILED"
)

// TaskEvent represents a single task state change event
type TaskEvent struct {
	Timestamp   time.Time     `json:"timestamp"`
	EventType   TaskEventType `json:"event_type"`
	TaskID      string        `json:"task_id"`
	CLI         string        `json:"cli"`
	Message     string        `json:"message,omitempty"`
	Error       string        `json:"error,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
	RetryCount  int           `json:"retry_count,omitempty"`
	RetryReason string        `json:"retry_reason,omitempty"`
	OutputPath  string        `json:"output_path,omitempty"`
	Output      string        `json:"output,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ToJSON converts the event to JSON string
func (e *TaskEvent) ToJSON() string {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to marshal event: %v"}`, err)
	}
	return string(data)
}


// TaskEventLogger handles task event logging in JSON format
type TaskEventLogger struct {
	writer io.Writer
}

// NewTaskEventLogger creates a new task event logger (JSON format only)
func NewTaskEventLogger(writer io.Writer) *TaskEventLogger {
	return &TaskEventLogger{
		writer: writer,
	}
}

// LogEvent logs a task event in JSON format
func (l *TaskEventLogger) LogEvent(event *TaskEvent) {
	if l.writer == nil {
		return
	}

	output := event.ToJSON()
	fmt.Fprintln(l.writer, output)
}

// LogQueued logs a task queued event
func (l *TaskEventLogger) LogQueued(taskID, cli string) {
	event := &TaskEvent{
		Timestamp: time.Now(),
		EventType: TaskEventQueued,
		TaskID:    taskID,
		CLI:       cli,
	}
	l.LogEvent(event)
}

// LogStarted logs a task started event
func (l *TaskEventLogger) LogStarted(taskID, cli string) {
	event := &TaskEvent{
		Timestamp: time.Now(),
		EventType: TaskEventStarted,
		TaskID:    taskID,
		CLI:       cli,
	}
	l.LogEvent(event)
}

// LogStartedWithRetry logs a task started event with retry information
func (l *TaskEventLogger) LogStartedWithRetry(taskID, cli string, retryCount int, retryReason string) {
	event := &TaskEvent{
		Timestamp:   time.Now(),
		EventType:   TaskEventStarted,
		TaskID:      taskID,
		CLI:         cli,
		RetryCount:  retryCount,
		RetryReason: retryReason,
	}
	l.LogEvent(event)
}

// LogCompleted logs a task completed event
func (l *TaskEventLogger) LogCompleted(taskID, cli string, duration time.Duration, outputPath string) {
	l.LogCompletedWithOutput(taskID, cli, duration, outputPath, "")
}

// LogCompletedWithOutput logs a task completed event with output details
func (l *TaskEventLogger) LogCompletedWithOutput(taskID, cli string, duration time.Duration, outputPath, output string) {
	event := &TaskEvent{
		Timestamp:  time.Now(),
		EventType:  TaskEventCompleted,
		TaskID:     taskID,
		CLI:        cli,
		Duration:   duration,
		OutputPath: outputPath,
		Output:     sanitizeOutputForEvent(output),
	}
	l.LogEvent(event)
}

// LogFailed logs a task failed event
func (l *TaskEventLogger) LogFailed(taskID, cli string, errorMsg string, retryCount int) {
	l.LogFailedWithOutput(taskID, cli, errorMsg, retryCount, "")
}

// LogFailedWithOutput logs a task failed event with output details
func (l *TaskEventLogger) LogFailedWithOutput(taskID, cli string, errorMsg string, retryCount int, output string) {
	event := &TaskEvent{
		Timestamp:  time.Now(),
		EventType:  TaskEventFailed,
		TaskID:     taskID,
		CLI:        cli,
		Error:      errorMsg,
		RetryCount: retryCount,
		Output:     sanitizeOutputForEvent(output),
	}
	l.LogEvent(event)
}

// LogRetrying logs a task retrying event
func (l *TaskEventLogger) LogRetrying(taskID string, retryCount int, reason string) {
	event := &TaskEvent{
		Timestamp:  time.Now(),
		EventType:  TaskEventRetrying,
		TaskID:     taskID,
		RetryCount: retryCount,
		Error:      reason,
	}
	l.LogEvent(event)
}

// LogRetryingWithCLI logs a task retrying event with CLI information
func (l *TaskEventLogger) LogRetryingWithCLI(taskID, cli string, retryCount int, reason string) {
	event := &TaskEvent{
		Timestamp:   time.Now(),
		EventType:   TaskEventRetrying,
		TaskID:      taskID,
		CLI:         cli,
		RetryCount:  retryCount,
		RetryReason: reason,
	}
	l.LogEvent(event)
}

// LogTimeout logs a task timeout event
func (l *TaskEventLogger) LogTimeout(taskID, cli string, duration time.Duration) {
	l.LogTimeoutWithOutput(taskID, cli, duration, "")
}

// LogTimeoutWithOutput logs a task timeout event with output details
func (l *TaskEventLogger) LogTimeoutWithOutput(taskID, cli string, duration time.Duration, output string) {
	event := &TaskEvent{
		Timestamp: time.Now(),
		EventType: TaskEventTimeout,
		TaskID:    taskID,
		CLI:       cli,
		Duration:  duration,
		Output:    sanitizeOutputForEvent(output),
	}
	l.LogEvent(event)
}

// LogQuotaExceeded logs a quota exceeded event
func (l *TaskEventLogger) LogQuotaExceeded(taskID, cli string) {
	event := &TaskEvent{
		Timestamp: time.Now(),
		EventType: TaskEventQuotaExceeded,
		TaskID:    taskID,
		CLI:       cli,
	}
	l.LogEvent(event)
}

// LogQualityFailed logs a quality check failed event
func (l *TaskEventLogger) LogQualityFailed(taskID, cli string, retryCount int) {
	l.LogQualityFailedWithOutput(taskID, cli, retryCount, "")
}

// LogQualityFailedWithOutput logs a quality check failed event with output details
func (l *TaskEventLogger) LogQualityFailedWithOutput(taskID, cli string, retryCount int, output string) {
	event := &TaskEvent{
		Timestamp:  time.Now(),
		EventType:  TaskEventQualityFailed,
		TaskID:     taskID,
		CLI:        cli,
		RetryCount: retryCount,
		Output:     sanitizeOutputForEvent(output),
	}
	l.LogEvent(event)
}

// LogQualityFailedWithDetails logs a quality check failed event with detailed failure reason and output
func (l *TaskEventLogger) LogQualityFailedWithDetails(taskID, cli string, retryCount int, failureDetail, output string) {
	event := &TaskEvent{
		Timestamp:  time.Now(),
		EventType:  TaskEventQualityFailed,
		TaskID:     taskID,
		CLI:        cli,
		Error:      failureDetail,
		RetryCount: retryCount,
		Output:     sanitizeOutputForEvent(output),
	}
	l.LogEvent(event)
}


// sanitizeOutputForEvent sanitizes output for event logging (truncate to 1000 chars for JSON)
func sanitizeOutputForEvent(output string) string {
	if output == "" {
		return ""
	}
	
	// Remove control characters for JSON compatibility
	sanitized := strings.ReplaceAll(output, "\n", " ")
	sanitized = strings.ReplaceAll(sanitized, "\r", " ")
	sanitized = strings.ReplaceAll(sanitized, "\t", " ")
	
	// Remove multiple spaces
	for strings.Contains(sanitized, "  ") {
		sanitized = strings.ReplaceAll(sanitized, "  ", " ")
	}
	
	sanitized = strings.TrimSpace(sanitized)
	
	// Truncate to 1000 characters for events (longer than task logs for detailed analysis)
	if len(sanitized) > 1000 {
		sanitized = sanitized[:1000] + "..."
	}
	
	return sanitized
}