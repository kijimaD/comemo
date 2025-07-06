package logger

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{SILENT, "SILENT"},
		{LogLevel(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.level.String())
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
		hasError bool
	}{
		{"DEBUG", DEBUG, false},
		{"debug", DEBUG, false},
		{"INFO", INFO, false},
		{"info", INFO, false},
		{"WARN", WARN, false},
		{"WARNING", WARN, false},
		{"warn", WARN, false},
		{"ERROR", ERROR, false},
		{"error", ERROR, false},
		{"SILENT", SILENT, false},
		{"silent", SILENT, false},
		{"INVALID", INFO, true},
		{"", INFO, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLogLevel(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, level)
			}
		})
	}
}

func TestLogger_New(t *testing.T) {
	var output, errorOutput bytes.Buffer
	logger := New(DEBUG, &output, &errorOutput)

	assert.Equal(t, DEBUG, logger.GetLevel())
	assert.Equal(t, &output, logger.output)
	assert.Equal(t, &errorOutput, logger.error)
}

func TestLogger_Default(t *testing.T) {
	logger := Default()
	assert.Equal(t, INFO, logger.GetLevel())
}

func TestLogger_Silent(t *testing.T) {
	logger := Silent()
	assert.Equal(t, SILENT, logger.GetLevel())
	assert.Equal(t, io.Discard, logger.output)
	assert.Equal(t, io.Discard, logger.error)
}

func TestLogger_SetLevel(t *testing.T) {
	logger := Default()
	logger.SetLevel(DEBUG)
	assert.Equal(t, DEBUG, logger.GetLevel())
}

func TestLogger_IsLevel(t *testing.T) {
	logger := New(INFO, nil, nil)

	assert.False(t, logger.IsLevel(DEBUG))
	assert.True(t, logger.IsLevel(INFO))
	assert.True(t, logger.IsLevel(WARN))
	assert.True(t, logger.IsLevel(ERROR))
}

func TestLogger_LogLevels(t *testing.T) {
	var output, errorOutput bytes.Buffer
	logger := New(DEBUG, &output, &errorOutput)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warning message")
	logger.Error("error message")

	outputStr := output.String()
	errorStr := errorOutput.String()

	// Check that debug, info, warn messages go to output
	assert.Contains(t, outputStr, "DEBUG: debug message")
	assert.Contains(t, outputStr, "INFO: info message")
	assert.Contains(t, outputStr, "WARN: warning message")

	// Check that error messages go to error output
	assert.Contains(t, errorStr, "ERROR: error message")
}

func TestLogger_LogLevelFiltering(t *testing.T) {
	var output, errorOutput bytes.Buffer
	logger := New(WARN, &output, &errorOutput)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warning message")
	logger.Error("error message")

	outputStr := output.String()
	errorStr := errorOutput.String()

	// Debug and Info should not appear
	assert.NotContains(t, outputStr, "debug message")
	assert.NotContains(t, outputStr, "info message")

	// Warn should appear in output
	assert.Contains(t, outputStr, "warning message")

	// Error should appear in error output
	assert.Contains(t, errorStr, "error message")
}

func TestLogger_FormattedMessages(t *testing.T) {
	var output bytes.Buffer
	logger := New(DEBUG, &output, nil)

	logger.Debugf("Hello %s, number: %d\n", "world", 42)
	logger.Infof("Formatted %s\n", "message")

	outputStr := output.String()
	assert.Contains(t, outputStr, "Hello world, number: 42")
	assert.Contains(t, outputStr, "Formatted message")
}

func TestLogger_SilentLevel(t *testing.T) {
	var output, errorOutput bytes.Buffer
	logger := New(SILENT, &output, &errorOutput)

	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	assert.Empty(t, output.String())
	assert.Empty(t, errorOutput.String())
}

func TestLogger_BackwardCompatibility(t *testing.T) {
	var output bytes.Buffer
	logger := New(DEBUG, &output, nil)

	logger.Printf("formatted %s", "message")
	logger.Println("line message")

	outputStr := output.String()
	assert.Contains(t, outputStr, "formatted message")
	assert.Contains(t, outputStr, "line message")
}

func TestLogger_TimestampFormat(t *testing.T) {
	var output bytes.Buffer
	logger := New(INFO, &output, nil)

	logger.Info("test message")

	outputStr := output.String()
	// Check that timestamp is present (format: [YYYY-MM-DD HH:MM:SS])
	assert.Regexp(t, `\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\]`, outputStr)
	assert.Contains(t, outputStr, "INFO: test message")
}
