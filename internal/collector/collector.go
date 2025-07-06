package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"comemo/internal/config"
	"comemo/internal/git"
)

// CollectCommits collects commit data from the Go repository
func CollectCommits(cfg *config.Config) error {
	// 必要なディレクトリを作成
	if err := os.MkdirAll(cfg.CommitDataDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", cfg.CommitDataDir, err)
	}

	fmt.Println("--- Collecting Commits from Go Repository ---")

	allHashes, err := git.GetCommitHashes(cfg.GoRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}

	fmt.Printf("Found %d commits in the repository.\n", len(allHashes))

	// 並行処理用のsemaphoreとWaitGroup
	sem := make(chan struct{}, cfg.MaxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	skipped := 0
	processed := 0

	for _, hash := range allHashes {
		index := git.GetCommitIndex(allHashes, hash)
		if index == 0 {
			fmt.Fprintf(os.Stderr, "Warning: Could not find index for hash %s\n", hash)
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
				fmt.Fprintf(os.Stderr, "Error preparing commit data for %s (index %d): %v\n", h, idx, err)
			} else {
				mu.Lock()
				processed++
				if processed%100 == 0 {
					fmt.Printf("Progress: %d commits processed\n", processed)
				}
				mu.Unlock()
			}
		}(hash, index)
	}

	wg.Wait()

	fmt.Printf("\n--- Commit Collection Complete ---\n")
	fmt.Printf("Total commits: %d\n", len(allHashes))
	fmt.Printf("Newly collected: %d\n", processed)
	fmt.Printf("Already existed: %d\n", skipped)

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