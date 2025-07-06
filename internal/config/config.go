package config

import (
	"time"

	"comemo/internal/logger"
)

// Config holds application configuration
type Config struct {
	GoRepoPath       string
	PromptsDir       string
	OutputDir        string
	CommitDataDir    string
	MaxConcurrency   int
	ExecutionTimeout time.Duration
	QuotaRetryDelay  time.Duration
	MaxRetries       int
	RetryDelay       time.Duration
	LogLevel         logger.LogLevel
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		GoRepoPath:       "go",
		PromptsDir:       "prompts",
		OutputDir:        "src",
		CommitDataDir:    "commit_data",
		MaxConcurrency:   20,
		ExecutionTimeout: 10 * time.Minute,
		QuotaRetryDelay:  1 * time.Hour,
		MaxRetries:       3,
		RetryDelay:       5 * time.Minute,
		LogLevel:         logger.INFO,
	}
}

// QuotaErrors contains patterns that indicate quota limits
var QuotaErrors = []string{
	"Quota exceeded",
	"quota metric",
	"RESOURCE_EXHAUSTED",
	"rateLimitExceeded",
	"per day per user",
	"Claude AI usage limit reached",
}
