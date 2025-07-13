package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	// DEBUG level for detailed diagnostic information
	DEBUG LogLevel = iota
	// INFO level for general information
	INFO
	// WARN level for warning messages
	WARN
	// ERROR level for error messages
	ERROR
	// SILENT level for no output
	SILENT
)

// String returns the string representation of LogLevel
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case SILENT:
		return "SILENT"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel parses a string into LogLevel
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN", "WARNING":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	case "SILENT":
		return SILENT, nil
	default:
		return INFO, fmt.Errorf("invalid log level: %s", level)
	}
}

// Logger provides structured logging with levels
type Logger struct {
	level  LogLevel
	output io.Writer
	error  io.Writer
}

// New creates a new Logger with the specified level and writers
func New(level LogLevel, output, errorOutput io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}
	if errorOutput == nil {
		errorOutput = os.Stderr
	}

	return &Logger{
		level:  level,
		output: output,
		error:  errorOutput,
	}
}

// Default creates a logger with INFO level and standard outputs
func Default() *Logger {
	return New(INFO, os.Stdout, os.Stderr)
}

// Silent creates a logger that outputs nothing
func Silent() *Logger {
	return New(SILENT, io.Discard, io.Discard)
}

// SetLevel changes the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// GetLevel returns the current logging level
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// IsLevel checks if the given level would be logged
func (l *Logger) IsLevel(level LogLevel) bool {
	return l.level <= level
}

// log writes a message at the specified level
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if !l.IsLevel(level) {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)

	var writer io.Writer
	switch level {
	case ERROR:
		writer = l.error
	default:
		writer = l.output
	}

	_, _ = fmt.Fprintf(writer, "[%s] %s: %s\n", timestamp, level.String(), message) // ログ出力エラーは無視
}

// logf writes a formatted message at the specified level without timestamp/level prefix
func (l *Logger) logf(level LogLevel, format string, args ...interface{}) {
	if !l.IsLevel(level) {
		return
	}

	var writer io.Writer
	switch level {
	case ERROR:
		writer = l.error
	default:
		writer = l.output
	}

	_, _ = fmt.Fprintf(writer, format, args...) // ログ出力エラーは無視
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Debugf logs a formatted debug message without prefix
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logf(DEBUG, format, args...)
}

// Infof logs a formatted info message without prefix
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logf(INFO, format, args...)
}

// Warnf logs a formatted warning message without prefix
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logf(WARN, format, args...)
}

// Errorf logs a formatted error message without prefix
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logf(ERROR, format, args...)
}

// Printf logs a formatted message at DEBUG level (for backward compatibility)
func (l *Logger) Printf(format string, args ...interface{}) {
	l.Debugf(format, args...)
}

// Println logs a message at DEBUG level (for backward compatibility)
func (l *Logger) Println(args ...interface{}) {
	l.Debugf("%s\n", fmt.Sprint(args...))
}
