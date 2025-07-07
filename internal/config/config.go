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
	// キュー関連設定
	QueueCapacityPerCLI int // 各CLIのキュー容量
	WorkerChannelSize   int // ワーカーチャネルサイズ
	ResultChannelSize   int // 結果チャネルサイズ
	// リトライ待機時間設定
	RetryDelays RetryDelayConfig // エラータイプ別のリトライ待機時間
}

// RetryDelayConfig holds retry delay settings for different error types
type RetryDelayConfig struct {
	QuotaError   time.Duration // quota error時の待機時間
	QualityError time.Duration // 品質テストエラー時の待機時間
	OtherError   time.Duration // その他のエラー時の待機時間
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		GoRepoPath:       "go",
		PromptsDir:       "prompts",
		OutputDir:        "src",
		CommitDataDir:    "commit_data",
		MaxConcurrency:   1,
		ExecutionTimeout: 10 * time.Minute,
		QuotaRetryDelay:  1 * time.Hour,
		MaxRetries:       3,
		RetryDelay:       5 * time.Minute,
		LogLevel:         logger.INFO,
		// キュー設定のデフォルト値
		QueueCapacityPerCLI: 1,   // 各CLIに1つまでキュー可能
		WorkerChannelSize:   1,   // ワーカーチャネルサイズ
		ResultChannelSize:   100, // 結果チャネルサイズ
		// リトライ待機時間のデフォルト値
		RetryDelays: RetryDelayConfig{
			QuotaError:   1 * time.Hour,    // quota error - 1時間待機
			QualityError: 10 * time.Second, // 品質テストエラー - 10秒待機
			OtherError:   5 * time.Minute,  // その他のエラー - 5分待機
		},
	}
}

// QuotaErrors contains patterns that indicate quota limits
var QuotaErrors = []string{
	"Quota exceeded",
	"quota metric",
	"RESOURCE_EXHAUSTED",
	"Resource has been exhausted",
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
