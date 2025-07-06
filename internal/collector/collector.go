package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"comemo/internal/config"
	"comemo/internal/git"
	"comemo/internal/logger"
)

// CollectorOptions provides configuration for collector functions
type CollectorOptions struct {
	Logger *logger.Logger
}

// CollectCommits collects commit data from the Go repository
func CollectCommits(cfg *config.Config) error {
	return CollectCommitsWithOptions(cfg, &CollectorOptions{
		Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
	})
}

// CollectCommitsWithOptions collects commit data from the Go repository with configurable output
func CollectCommitsWithOptions(cfg *config.Config, opts *CollectorOptions) error {
	if opts == nil {
		opts = &CollectorOptions{
			Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
		}
	}
	if opts.Logger == nil {
		opts.Logger = logger.New(cfg.LogLevel, os.Stdout, os.Stderr)
	}
	// 必要なディレクトリを作成
	if err := os.MkdirAll(cfg.CommitDataDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", cfg.CommitDataDir, err)
	}

	opts.Logger.Debug("--- Collecting Commits from Go Repository ---")

	allHashes, err := git.GetCommitHashes(cfg.GoRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}

	opts.Logger.Debug("Found %d commits in the repository.", len(allHashes))

	// 並行処理用のsemaphoreとWaitGroup
	sem := make(chan struct{}, cfg.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	skipped := 0
	processed := 0

	for _, hash := range allHashes {
		index := git.GetCommitIndex(allHashes, hash)
		if index == 0 {
			opts.Logger.Warn("Could not find index for hash %s", hash)
			continue
		}

		// コミットデータファイルが既に存在するかチェック
		commitDataFile := filepath.Join(cfg.CommitDataDir, fmt.Sprintf("%d.txt", index))
		if _, err := os.Stat(commitDataFile); err == nil {
			mu.Lock()
			skipped++
			mu.Unlock()
			continue // 既に存在する場合はスキップ
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(h string, idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := prepareCommitData(cfg, h, idx); err != nil {
				opts.Logger.Error("Error preparing commit data for %s (index %d): %v", h, idx, err)
			} else {
				mu.Lock()
				processed++
				if processed%100 == 0 {
					opts.Logger.Debug("Progress: %d commits processed", processed)
				}
				mu.Unlock()
			}
		}(hash, index)
	}

	wg.Wait()

	opts.Logger.Debug("--- Commit Collection Complete ---")
	opts.Logger.Debug("Total commits: %d", len(allHashes))
	opts.Logger.Debug("Newly collected: %d", processed)
	opts.Logger.Debug("Already existed: %d", skipped)

	return nil
}

// prepareCommitData prepares commit data for a specific commit
func prepareCommitData(cfg *config.Config, hash string, index int) error {
	commitData, err := git.GetCommitData(cfg.GoRepoPath, hash)
	if err != nil {
		return fmt.Errorf("failed to get commit data: %w", err)
	}

	filePath := filepath.Join(cfg.CommitDataDir, fmt.Sprintf("%d.txt", index))
	if err := os.WriteFile(filePath, []byte(commitData), 0644); err != nil {
		return fmt.Errorf("failed to write commit data: %w", err)
	}

	return nil
}
