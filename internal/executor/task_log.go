package executor

import (
	"fmt"
	"io"
	"time"
)

// TaskLogEntry represents a single task execution log entry
type TaskLogEntry struct {
	Timestamp time.Time
	Status    string // START, SUCCESS, FAIL
	Script    string
	CLI       string
	Output    string // for SUCCESS
	Error     string // for FAIL
	Retry     int    // for FAIL
}

// LogTaskStart logs the start of a task execution
func LogTaskStart(w io.Writer, script, cli string) {
	if w == nil {
		return
	}
	entry := TaskLogEntry{
		Timestamp: time.Now(),
		Status:    "START",
		Script:    script,
		CLI:       cli,
	}
	fmt.Fprintf(w, "[%s] %s script: %s, cli: %s\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Status,
		entry.Script,
		entry.CLI)
}

// LogTaskSuccess logs the successful completion of a task
func LogTaskSuccess(w io.Writer, script, cli, output string) {
	if w == nil {
		return
	}
	entry := TaskLogEntry{
		Timestamp: time.Now(),
		Status:    "SUCCESS",
		Script:    script,
		CLI:       cli,
		Output:    output,
	}
	fmt.Fprintf(w, "[%s] %s script: %s, cli: %s, output: %s\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Status,
		entry.Script,
		entry.CLI,
		entry.Output)
}

// LogTaskFailure logs the failure of a task
func LogTaskFailure(w io.Writer, script, cli, errorMsg string, retryCount int) {
	if w == nil {
		return
	}
	entry := TaskLogEntry{
		Timestamp: time.Now(),
		Status:    "FAIL",
		Script:    script,
		CLI:       cli,
		Error:     errorMsg,
		Retry:     retryCount,
	}
	fmt.Fprintf(w, "[%s] %s script: %s, cli: %s, error: %s, retry: %d\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Status,
		entry.Script,
		entry.CLI,
		entry.Error,
		entry.Retry)
}