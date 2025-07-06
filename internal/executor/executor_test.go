package executor

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestExecutePrompts tests the ExecutePrompts function
func TestExecutePrompts(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	outputDir := filepath.Join(tmpDir, "output")
	
	// Create directories
	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.MkdirAll(outputDir, 0755))
	
	// Create test configuration
	cfg := &config.Config{
		PromptsDir:       promptsDir,
		OutputDir:        outputDir,
		MaxConcurrency:   2,
		ExecutionTimeout: 30 * time.Second,
		QuotaRetryDelay:  1 * time.Minute,
		MaxRetries:       2,
	}
	
	t.Run("empty prompts directory", func(t *testing.T) {
		var output, errOutput bytes.Buffer
		opts := &ExecutorOptions{
			Output: &output,
			Error:  &errOutput,
		}
		
		err := ExecutePromptsWithOptions(cfg, "claude", opts)
		assert.NoError(t, err)
		assert.Contains(t, output.String(), "No .sh files found in the prompts directory")
	})
	
	t.Run("invalid CLI command validation", func(t *testing.T) {
		var output, errOutput bytes.Buffer
		opts := &ExecutorOptions{
			Output: &output,
			Error:  &errOutput,
		}
		
		// Create test script
		scriptPath := filepath.Join(promptsDir, "test.sh")
		scriptContent := `#!/bin/bash
echo "Test output"
`
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))
		
		// ExecutePrompts will run but worker will find CLI unavailable
		err := ExecutePromptsWithOptions(cfg, "invalid-cli", opts)
		assert.NoError(t, err) // Function completes without error even with invalid CLI
		assert.Contains(t, output.String(), "Found 1 scripts to execute")
		
		// Clean up
		os.Remove(scriptPath)
	})
	
	t.Run("successful execution with mock", func(t *testing.T) {
		var output, errOutput bytes.Buffer
		opts := &ExecutorOptions{
			Output: &output,
			Error:  &errOutput,
		}
		
		// Create test script that doesn't depend on external CLI
		scriptPath := filepath.Join(promptsDir, "test-mock.sh")
		scriptContent := `#!/bin/bash
# Mock script for testing
echo "🚀 Generating explanation for commit abc123"
echo ""
echo "## コアとなるコードの解説"
echo "This is a test explanation with sufficient content to pass the length check."
echo "This content should be long enough to satisfy the 1000 character minimum requirement."
echo "Additional content to ensure we meet the minimum length requirement for testing purposes."
echo "More content to make sure this passes the validation checks in the processing logic."
echo "Even more content to ensure comprehensive testing of the output validation functionality."
echo "Additional explanatory text to provide thorough coverage of the validation requirements."
echo "Final additional content to ensure we definitely exceed the 1000 character threshold."
echo ""
echo "## 技術的詳細"
echo "Technical details section for comprehensive testing of the validation logic."
echo "This section provides additional content to ensure proper test coverage."
echo "More technical details to satisfy the content validation requirements."
echo "Final technical details to complete the comprehensive test content."
`
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))
		
		// Execute with 'all' option
		err := ExecutePromptsWithOptions(cfg, "all", opts)
		assert.NoError(t, err)
		assert.Contains(t, output.String(), "Found 1 scripts to execute")
		
		// Note: The script might not be deleted because it doesn't use real CLI
		// But we can check that the function completes without error
	})
}

// TestWorker tests the Worker function
func TestWorker(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	outputDir := filepath.Join(tmpDir, "output")
	
	// Create directories
	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.MkdirAll(outputDir, 0755))
	
	// Create test configuration
	cfg := &config.Config{
		PromptsDir:       promptsDir,
		OutputDir:        outputDir,
		MaxConcurrency:   2,
		ExecutionTimeout: 5 * time.Second,
		QuotaRetryDelay:  1 * time.Minute,
		MaxRetries:       2,
	}
	
	var output, errOutput bytes.Buffer
	opts := &ExecutorOptions{
		Output: &output,
		Error:  &errOutput,
	}
	manager := NewCLIManagerWithOptions(cfg, opts)
	
	t.Run("worker with empty queue", func(t *testing.T) {
		scriptQueue := make(chan string)
		close(scriptQueue)
		
		// This should exit immediately
		WorkerWithOptions("claude", scriptQueue, manager, opts)
		// Test passes if Worker doesn't hang
	})
	
	t.Run("worker with unavailable CLI", func(t *testing.T) {
		scriptQueue := make(chan string, 1)
		
		// Mark CLI as unavailable
		manager.MarkUnavailable("claude")
		
		// Add a script to the queue
		scriptQueue <- "test.sh"
		close(scriptQueue)
		
		// Worker should handle unavailable CLI gracefully
		WorkerWithOptions("claude", scriptQueue, manager, opts)
		
		// Test passes if Worker doesn't hang
	})
	
	t.Run("worker with simple successful script", func(t *testing.T) {
		// Create test script
		scriptPath := filepath.Join(promptsDir, "worker-test.sh")
		scriptContent := `#!/bin/bash
echo "🚀 Generating explanation for commit abc123"
echo ""
echo "## コアとなるコードの解説"
echo "This is a comprehensive test explanation with sufficient content to pass all validation checks."
echo "This content is specifically designed to satisfy the 1000 character minimum requirement."
echo "Additional detailed content to ensure we meet the minimum length requirement for testing purposes."
echo "More comprehensive content to make sure this passes all the validation checks in the processing logic."
echo "Even more detailed content to ensure comprehensive testing of the output validation functionality."
echo "Additional explanatory text to provide thorough coverage of all the validation requirements."
echo "Final additional comprehensive content to ensure we definitely exceed the 1000 character threshold."
echo ""
echo "## 技術的詳細"
echo "Technical details section for comprehensive testing of the validation logic and processing."
echo "This section provides additional detailed content to ensure proper test coverage of all functionality."
echo "More comprehensive technical details to satisfy all the content validation requirements completely."
echo "Final comprehensive technical details to complete the thorough test content validation."
`
		require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))
		
		scriptQueue := make(chan string, 1)
		scriptQueue <- "worker-test.sh"
		close(scriptQueue)
		
		// Make sure CLI is available
		if !manager.IsAvailable("claude") {
			// Reset CLI availability for testing
			manager.CLIs["claude"].Available = true
			manager.CLIs["claude"].LastQuotaError = time.Time{}
		}
		
		// Run worker
		WorkerWithOptions("claude", scriptQueue, manager, opts)
		
		// Test passes if Worker completes without hanging
		// The script processing success depends on external CLI availability
	})
	
	t.Run("worker with nonexistent CLI", func(t *testing.T) {
		scriptQueue := make(chan string, 1)
		scriptQueue <- "test.sh"
		close(scriptQueue)
		
		// Worker should handle nonexistent CLI gracefully
		WorkerWithOptions("nonexistent-cli", scriptQueue, manager, opts)
		
		// Test passes if Worker doesn't crash
	})
}

// Helper function to create silent executor options for testing
func silentExecutorOptions() *ExecutorOptions {
	return &ExecutorOptions{
		Output: io.Discard,
		Error:  io.Discard,
	}
}

// TestExecutePromptsWithSilentOutput tests that ExecutePrompts works without output
func TestExecutePromptsWithSilentOutput(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := t.TempDir()
	promptsDir := filepath.Join(tmpDir, "prompts")
	outputDir := filepath.Join(tmpDir, "output")
	
	// Create directories
	require.NoError(t, os.MkdirAll(promptsDir, 0755))
	require.NoError(t, os.MkdirAll(outputDir, 0755))
	
	// Create test configuration
	cfg := &config.Config{
		PromptsDir:       promptsDir,
		OutputDir:        outputDir,
		MaxConcurrency:   1,
		ExecutionTimeout: 5 * time.Second,
		QuotaRetryDelay:  1 * time.Minute,
		MaxRetries:       1,
	}
	
	// Test with silent output
	err := ExecutePromptsWithOptions(cfg, "claude", silentExecutorOptions())
	assert.NoError(t, err)
}