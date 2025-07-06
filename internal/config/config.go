package config

import (
	"os"
	"path/filepath"
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

// GetWorkingDir returns the current working directory
func GetWorkingDir() (string, error) {
	return os.Getwd()
}

// ResolvePath converts a relative path to an absolute path relative to the current working directory
func ResolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	wd, err := GetWorkingDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(wd, path), nil
}

// ResolveConfigPaths converts all relative paths in the config to absolute paths
func (c *Config) ResolveConfigPaths() error {
	var err error

	c.GoRepoPath, err = ResolvePath(c.GoRepoPath)
	if err != nil {
		return err
	}

	c.PromptsDir, err = ResolvePath(c.PromptsDir)
	if err != nil {
		return err
	}

	c.OutputDir, err = ResolvePath(c.OutputDir)
	if err != nil {
		return err
	}

	c.CommitDataDir, err = ResolvePath(c.CommitDataDir)
	if err != nil {
		return err
	}

	return nil
}
