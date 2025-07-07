package executor

import (
	"fmt"
	"io"
	"strings"
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
	LogTaskSuccessWithDetails(w, script, cli, output, "", 0)
}

// LogTaskSuccessWithDetails logs the successful completion of a task with execution details
func LogTaskSuccessWithDetails(w io.Writer, script, cli, output, executionOutput string, duration time.Duration) {
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

	logLine := fmt.Sprintf("[%s] %s script: %s, cli: %s, output: %s",
		entry.Timestamp.Format(time.RFC3339),
		entry.Status,
		entry.Script,
		entry.CLI,
		entry.Output)

	if duration > 0 {
		logLine += fmt.Sprintf(", duration: %v", duration)
	}

	if executionOutput != "" {
		// Limit output length and sanitize
		sanitizedOutput := sanitizeOutput(executionOutput)
		if len(sanitizedOutput) > 200 {
			sanitizedOutput = sanitizedOutput[:200] + "..."
		}
		logLine += fmt.Sprintf(", result: %s", sanitizedOutput)
	}

	fmt.Fprintln(w, logLine)
}

// LogTaskFailure logs the failure of a task
func LogTaskFailure(w io.Writer, script, cli, errorMsg string, retryCount int) {
	LogTaskFailureWithDetails(w, script, cli, errorMsg, retryCount, "", 0)
}

// LogTaskFailureWithDetails logs the failure of a task with execution details
func LogTaskFailureWithDetails(w io.Writer, script, cli, errorMsg string, retryCount int, executionOutput string, duration time.Duration) {
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

	logLine := fmt.Sprintf("[%s] %s script: %s, cli: %s, error: %s, retry: %d",
		entry.Timestamp.Format(time.RFC3339),
		entry.Status,
		entry.Script,
		entry.CLI,
		entry.Error,
		entry.Retry)

	if duration > 0 {
		logLine += fmt.Sprintf(", duration: %v", duration)
	}

	if executionOutput != "" {
		// Limit output length and sanitize
		sanitizedOutput := sanitizeOutput(executionOutput)
		if len(sanitizedOutput) > 200 {
			sanitizedOutput = sanitizedOutput[:200] + "..."
		}
		logLine += fmt.Sprintf(", output: %s", sanitizedOutput)
	}

	fmt.Fprintln(w, logLine)
}

// sanitizeOutput removes newlines and controls characters for single-line logging
func sanitizeOutput(output string) string {
	// Replace newlines with spaces
	sanitized := strings.ReplaceAll(output, "\n", " ")
	sanitized = strings.ReplaceAll(sanitized, "\r", " ")
	sanitized = strings.ReplaceAll(sanitized, "\t", " ")

	// Remove multiple spaces
	for strings.Contains(sanitized, "  ") {
		sanitized = strings.ReplaceAll(sanitized, "  ", " ")
	}

	return strings.TrimSpace(sanitized)
}
