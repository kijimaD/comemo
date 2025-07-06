package executor

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"comemo/internal/config"
	"comemo/internal/logger"
)

// ExecutePrompts executes generated prompt scripts
func ExecutePrompts(cfg *config.Config, cliCommand string) error {
	return ExecutePromptsWithOptions(cfg, cliCommand, &ExecutorOptions{
		Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
	})
}

// ExecutePromptsWithOptions executes generated prompt scripts with configurable output
func ExecutePromptsWithOptions(cfg *config.Config, cliCommand string, opts *ExecutorOptions) error {
	if opts == nil {
		opts = &ExecutorOptions{
			Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
		}
	}
	if opts.Logger == nil {
		opts.Logger = logger.New(cfg.LogLevel, os.Stdout, os.Stderr)
	}

	opts.Logger.Debug("実行開始: プロンプトスクリプトの実行")

	files, err := os.ReadDir(cfg.PromptsDir)
	if err != nil {
		return fmt.Errorf("error reading prompts directory: %w", err)
	}

	var shFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") {
			shFiles = append(shFiles, file.Name())
		}
	}

	if len(shFiles) == 0 {
		opts.Logger.Debug("プロンプトディレクトリに.shファイルが見つかりませんでした")
		return nil
	}

	opts.Logger.Debug("実行対象スクリプト数: %d", len(shFiles))

	// Create CLI manager
	manager := NewCLIManagerWithOptions(cfg, opts)

	// Determine which CLIs to use
	var cliTools []string
	if cliCommand == "all" {
		for name := range SupportedCLIs {
			cliTools = append(cliTools, name)
		}
	} else {
		cliTools = []string{cliCommand}
	}

	// Create channels for script distribution
	scriptQueues := make(map[string]chan string)
	for _, cliName := range cliTools {
		scriptQueues[cliName] = make(chan string, len(shFiles))
	}

	// Start workers with panic recovery
	var wg sync.WaitGroup
	var criticalError error
	var criticalErrorMu sync.Mutex

	for cliName, queue := range scriptQueues {
		wg.Add(1)
		go func(name string, q chan string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					if execErr, ok := r.(*ExecutionError); ok && execErr.Type == ErrorTypeCritical {
						criticalErrorMu.Lock()
						criticalError = execErr
						criticalErrorMu.Unlock()

						// Close all queues to stop other workers
						for _, queue := range scriptQueues {
							select {
							case <-queue:
							default:
								close(queue)
							}
						}
					} else {
						// Re-panic for unexpected panics
						panic(r)
					}
				}
			}()
			WorkerWithOptions(name, q, manager, opts)
		}(cliName, queue)
	}

	// Start quota monitor
	monitorQueue := make(chan string, len(shFiles))
	go quotaMonitorWithOptions(manager, monitorQueue, opts)

	// Distribute scripts round-robin among CLIs
	for i, fileName := range shFiles {
		cliIndex := i % len(cliTools)
		cliName := cliTools[cliIndex]
		scriptQueues[cliName] <- fileName
		monitorQueue <- fileName
	}

	// Close queues
	for _, queue := range scriptQueues {
		close(queue)
	}
	close(monitorQueue)

	// Wait for all workers to complete
	wg.Wait()

	// Check if a critical error occurred
	criticalErrorMu.Lock()
	if criticalError != nil {
		criticalErrorMu.Unlock()
		return fmt.Errorf("実行が重要なエラーにより停止されました")
	}
	criticalErrorMu.Unlock()

	// Count remaining scripts
	remainingFiles, _ := os.ReadDir(cfg.PromptsDir)
	remainingCount := 0
	for _, file := range remainingFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") {
			remainingCount++
		}
	}

	if remainingCount > 0 {
		opts.Logger.Warn("未処理のスクリプトが残っています: %d個", remainingCount)
		opts.Logger.Debug("%d scripts remain unprocessed. They may have failed or hit quota limits.", remainingCount)
	} else {
		opts.Logger.Debug("すべてのプロンプトスクリプトが正常に実行され、削除されました")
	}

	return nil
}

// quotaMonitor monitors quota status and provides status updates
func quotaMonitor(manager *CLIManager, scriptQueue <-chan string) {
	quotaMonitorWithOptions(manager, scriptQueue, &ExecutorOptions{
		Logger: logger.Default(),
	})
}

// quotaMonitorWithOptions monitors quota status and provides status updates with configurable output
func quotaMonitorWithOptions(manager *CLIManager, scriptQueue <-chan string, opts *ExecutorOptions) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	totalScripts := 0
	startTime := time.Now()

	for {
		select {
		case _, ok := <-scriptQueue:
			if !ok {
				return
			}
			totalScripts++

		case <-ticker.C:
			elapsed := time.Since(startTime)
			opts.Logger.Debug("📊 Status Update:")
			opts.Logger.Debug("   Total scripts queued: %d", totalScripts)
			opts.Logger.Debug("   Time elapsed: %v", elapsed)

			for name, cli := range manager.CLIs {
				status := "Available"
				if !cli.Available {
					timeUntilAvailable := manager.Config.QuotaRetryDelay - time.Since(cli.LastQuotaError)
					status = fmt.Sprintf("Quota limit (available in %v)", timeUntilAvailable.Round(time.Minute))
				}
				opts.Logger.Debug("   %s: %s", name, status)
			}
		}
	}
}
