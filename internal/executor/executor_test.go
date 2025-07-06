package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"comemo/internal/config"
)

// TestIsQuotaError tests the IsQuotaError function
func TestIsQuotaError(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "Quota exceeded error",
			output:   "Error: Quota exceeded for the day",
			expected: true,
		},
		{
			name:     "Quota metric error",
			output:   "quota metric limit reached",
			expected: true,
		},
		{
			name:     "Resource exhausted error",
			output:   "Error: RESOURCE_EXHAUSTED",
			expected: true,
		},
		{
			name:     "Rate limit error",
			output:   "rateLimitExceeded: Too many requests",
			expected: true,
		},
		{
			name:     "Per day per user error",
			output:   "Limit: 100 per day per user",
			expected: true,
		},
		{
			name:     "Claude AI limit error",
			output:   "Claude AI usage limit reached. Please try again later.",
			expected: true,
		},
		{
			name:     "Normal output",
			output:   "Processing completed successfully",
			expected: false,
		},
		{
			name:     "Empty output",
			output:   "",
			expected: false,
		},
		{
			name:     "Case insensitive check",
			output:   "ERROR: QUOTA EXCEEDED",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsQuotaError(tt.output)
			assert.Equal(t, tt.expected, result, "IsQuotaError should return expected result")
		})
	}
}

// TestCLIManager tests the CLIManager functionality
func TestCLIManager(t *testing.T) {
	cfg := &config.Config{
		QuotaRetryDelay: 1 * time.Hour,
		MaxRetries:      3,
	}

	manager := NewCLIManager(cfg)

	// Test initial state
	assert.NotNil(t, manager.CLIs)
	assert.NotNil(t, manager.RetryQueue)
	assert.NotNil(t, manager.RetryInfo)

	// Test CLI initialization
	for name, cmd := range SupportedCLIs {
		cli, exists := manager.CLIs[name]
		assert.True(t, exists, "CLI %s should exist", name)
		assert.Equal(t, name, cli.Name)
		assert.Equal(t, cmd, cli.Command)
		assert.True(t, cli.Available)
		assert.True(t, cli.LastQuotaError.IsZero())
		assert.Empty(t, cli.PendingScripts)
	}

	// Test IsAvailable
	assert.True(t, manager.IsAvailable("claude"))
	assert.True(t, manager.IsAvailable("gemini"))
	assert.False(t, manager.IsAvailable("nonexistent"))

	// Test MarkUnavailable
	manager.MarkUnavailable("claude")
	assert.False(t, manager.IsAvailable("claude"))
	assert.False(t, manager.CLIs["claude"].Available)
	assert.False(t, manager.CLIs["claude"].LastQuotaError.IsZero())

	// Test GetCLICommand
	cmd, exists := manager.GetCLICommand("claude")
	assert.True(t, exists)
	assert.Equal(t, SupportedCLIs["claude"], cmd)

	_, exists = manager.GetCLICommand("nonexistent")
	assert.False(t, exists)

	// Test retry info
	manager.UpdateRetryInfo("test.sh", "quota_error")
	assert.Equal(t, 1, manager.GetRetryCount("test.sh"))
	assert.True(t, manager.ShouldRetry("test.sh"))

	// Update retry count
	manager.UpdateRetryInfo("test.sh", "quality_check_failed")
	assert.Equal(t, 2, manager.GetRetryCount("test.sh"))
	assert.True(t, manager.ShouldRetry("test.sh"))

	// Exceed max retries
	manager.UpdateRetryInfo("test.sh", "quota_error")
	assert.Equal(t, 3, manager.GetRetryCount("test.sh"))
	assert.False(t, manager.ShouldRetry("test.sh"))

	// Test non-existent file
	assert.Equal(t, 0, manager.GetRetryCount("nonexistent.sh"))
	assert.True(t, manager.ShouldRetry("nonexistent.sh"))
}

// TestSupportedCLIs tests the supported CLI commands
func TestSupportedCLIs(t *testing.T) {
	assert.Contains(t, SupportedCLIs, "claude")
	assert.Contains(t, SupportedCLIs, "gemini")

	// Check Claude configuration
	claude := SupportedCLIs["claude"]
	assert.Equal(t, "claude", claude.Name)
	assert.Contains(t, claude.Command, "claude")

	// Check Gemini configuration
	gemini := SupportedCLIs["gemini"]
	assert.Equal(t, "gemini", gemini.Name)
	assert.Contains(t, gemini.Command, "gemini")
}