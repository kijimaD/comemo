package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultConfig tests the DefaultConfig function
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, "go", cfg.GoRepoPath)
	assert.Equal(t, "prompts", cfg.PromptsDir)
	assert.Equal(t, "src", cfg.OutputDir)
	assert.Equal(t, "commit_data", cfg.CommitDataDir)
	assert.Equal(t, 1, cfg.MaxConcurrency)
	assert.Equal(t, 10*time.Minute, cfg.ExecutionTimeout)
	assert.Equal(t, 1*time.Hour, cfg.QuotaRetryDelay)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 5*time.Minute, cfg.RetryDelay)
}

// TestQuotaErrors tests the quota error patterns
func TestQuotaErrors(t *testing.T) {
	assert.Contains(t, QuotaErrors, "Quota exceeded")
	assert.Contains(t, QuotaErrors, "quota metric")
	assert.Contains(t, QuotaErrors, "RESOURCE_EXHAUSTED")
	assert.Contains(t, QuotaErrors, "Resource has been exhausted")
	assert.Contains(t, QuotaErrors, "rateLimitExceeded")
	assert.Contains(t, QuotaErrors, "per day per user")
	assert.Contains(t, QuotaErrors, "Claude AI usage limit reached")
	assert.Contains(t, QuotaErrors, "GaxiosError:")
}
