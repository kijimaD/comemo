package executor

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTaskEvent_ToJSON(t *testing.T) {
	event := &TaskEvent{
		Timestamp: time.Date(2025, 1, 6, 12, 0, 0, 0, time.UTC),
		EventType: TaskEventStarted,
		TaskID:    "test.sh",
		CLI:       "claude",
		Message:   "Task started",
	}

	jsonStr := event.ToJSON()

	// Parse JSON to verify it's valid
	var parsed TaskEvent
	err := json.Unmarshal([]byte(jsonStr), &parsed)
	if err != nil {
		t.Errorf("Failed to parse JSON: %v", err)
	}

	if parsed.EventType != TaskEventStarted {
		t.Errorf("Expected EventType %v, got %v", TaskEventStarted, parsed.EventType)
	}
	if parsed.TaskID != "test.sh" {
		t.Errorf("Expected TaskID 'test.sh', got %v", parsed.TaskID)
	}
}

func TestTaskEventLogger_JSONFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTaskEventLogger(buf)

	logger.LogStarted("test.sh", "claude")

	output := strings.TrimSpace(buf.String())

	var event TaskEvent
	if err := json.Unmarshal([]byte(output), &event); err != nil {
		t.Errorf("Expected valid JSON, got error: %v", err)
	}

	if event.EventType != TaskEventStarted {
		t.Errorf("Expected EventType %v, got %v", TaskEventStarted, event.EventType)
	}
	if event.TaskID != "test.sh" {
		t.Errorf("Expected TaskID 'test.sh', got %v", event.TaskID)
	}
	if event.CLI != "claude" {
		t.Errorf("Expected CLI 'claude', got %v", event.CLI)
	}
}

func TestTaskEventLogger_AllEventTypes(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTaskEventLogger(buf)

	// Test all event types
	logger.LogQueued("test.sh", "claude")
	logger.LogStarted("test.sh", "claude")
	logger.LogCompleted("test.sh", "claude", 5*time.Second, "output/test.md")
	logger.LogFailed("test.sh", "claude", "error message", 1)
	logger.LogRetrying("test.sh", 2, "retry reason")
	logger.LogTimeout("test.sh", "claude", 30*time.Second)
	logger.LogQuotaExceeded("test.sh", "claude")
	logger.LogQualityFailed("test.sh", "claude", 1)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	expectedEventTypes := []TaskEventType{
		TaskEventQueued,
		TaskEventStarted,
		TaskEventCompleted,
		TaskEventFailed,
		TaskEventRetrying,
		TaskEventTimeout,
		TaskEventQuotaExceeded,
		TaskEventQualityFailed,
	}

	if len(lines) != len(expectedEventTypes) {
		t.Errorf("Expected %d JSON lines, got %d", len(expectedEventTypes), len(lines))
	}

	for i, expectedEventType := range expectedEventTypes {
		if i >= len(lines) {
			t.Errorf("Missing JSON line for event type: %s", expectedEventType)
			continue
		}

		var event TaskEvent
		if err := json.Unmarshal([]byte(lines[i]), &event); err != nil {
			t.Errorf("Failed to parse JSON line %d: %v", i, err)
			continue
		}

		if event.EventType != expectedEventType {
			t.Errorf("Line %d should have event type '%s', got: %s", i, expectedEventType, event.EventType)
		}
	}
}

func TestTaskEventLogger_NilWriter(t *testing.T) {
	// Should not panic with nil writer
	logger := NewTaskEventLogger(nil)
	logger.LogStarted("test.sh", "claude")
	logger.LogCompleted("test.sh", "claude", time.Second, "output.md")
}

func TestTaskEventLogger_JSONOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTaskEventLogger(buf)

	logger.LogStarted("test.sh", "claude")

	output := strings.TrimSpace(buf.String())

	var event TaskEvent
	err := json.Unmarshal([]byte(output), &event)
	if err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}

	if event.EventType != TaskEventStarted {
		t.Errorf("Expected EventType %v, got %v", TaskEventStarted, event.EventType)
	}
	if event.TaskID != "test.sh" {
		t.Errorf("Expected TaskID 'test.sh', got %v", event.TaskID)
	}
	if event.CLI != "claude" {
		t.Errorf("Expected CLI 'claude', got %v", event.CLI)
	}
}

func TestSanitizeOutputForEvent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "新行とタブの除去",
			input:    "line1\nline2\tline3\r\nline4",
			expected: "line1 line2 line3 line4",
		},
		{
			name:     "複数スペースの除去",
			input:    "word1    word2   word3",
			expected: "word1 word2 word3",
		},
		{
			name:     "前後の空白除去",
			input:    "  \t  text with spaces  \n  ",
			expected: "text with spaces",
		},
		{
			name:     "長い出力の切り詰め",
			input:    strings.Repeat("This is a long line. ", 100),                                                         // 約2100文字
			expected: strings.Repeat("This is a long line. ", 47) + "This is a long line. This is a long line. This is...", // 1000文字 + "..."
		},
		{
			name:     "空文字列",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeOutputForEvent(tt.input)
			if tt.name == "長い出力の切り詰め" {
				// 長い出力の場合は長さと末尾を確認
				if len(result) != 1003 { // 1000 + "..."
					t.Errorf("Expected length 1003, got %d", len(result))
				}
				if !strings.HasSuffix(result, "...") {
					t.Errorf("Expected result to end with '...', got: %s", result[len(result)-10:])
				}
			} else {
				if result != tt.expected {
					t.Errorf("Expected '%s', got '%s'", tt.expected, result)
				}
			}
		})
	}
}

func TestTaskEventLogger_WithOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := NewTaskEventLogger(buf)

	// Test event with output
	logger.LogFailedWithOutput("test.sh", "claude", "test error", 1, "Some output\nwith newlines\tand tabs")

	output := strings.TrimSpace(buf.String())

	var event TaskEvent
	err := json.Unmarshal([]byte(output), &event)
	if err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}

	if event.EventType != TaskEventFailed {
		t.Errorf("Expected EventType %v, got %v", TaskEventFailed, event.EventType)
	}
	if event.Output != "Some output with newlines and tabs" {
		t.Errorf("Expected sanitized output, got: %s", event.Output)
	}
}
