package executor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"comemo/internal/config"
	"comemo/internal/logger"
)

// ExecutePromptsWithProgressSchedulerAndOptions executes scripts using the new scheduler architecture with custom options
func ExecutePromptsWithProgressSchedulerAndOptions(cfg *config.Config, cliCommand string, opts *ExecutorOptions) error {
	return ExecutePromptsWithScheduler(cfg, cliCommand, opts)
}

// ExecutePromptsWithScheduler executes scripts using the new scheduler architecture
func ExecutePromptsWithScheduler(cfg *config.Config, cliCommand string, opts *ExecutorOptions) error {
	if opts == nil {
		opts = &ExecutorOptions{
			Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
		}
	}
	if opts.Logger == nil {
		opts.Logger = logger.New(cfg.LogLevel, os.Stdout, os.Stderr)
	}

	opts.Logger.Info("=== 実行開始: スケジューラー方式 ===")

	// Read script files
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
		opts.Logger.Info("プロンプトディレクトリに.shファイルが見つかりませんでした")
		return nil
	}

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

	// Create status manager if not in progress mode
	statusManager := NewStatusManager()
	statusManager.SetTotalScripts(len(shFiles))
	statusManager.Start()
	defer statusManager.Stop()

	// Initialize workers in status manager
	for _, cliName := range cliTools {
		statusManager.InitializeWorker(cliName)
	}

	// Create and run scheduler
	scheduler := NewScheduler(cfg, shFiles, manager, statusManager, opts.Logger)

	// Handle panics from critical errors
	defer func() {
		if r := recover(); r != nil {
			if execErr, ok := r.(*ExecutionError); ok && execErr.Type == ErrorTypeCritical {
				opts.Logger.Error("=== 実行が重要なエラーにより停止されました ===")
				panic(execErr) // Re-panic to propagate the error
			}
			panic(r) // Re-panic for unexpected errors
		}
	}()

	// Create context for cancellation
	ctx := context.Background()

	// Run the scheduler
	if err := scheduler.Run(ctx, cliTools); err != nil {
		return err
	}

	// Final status report
	status := statusManager.GetStatus()
	opts.Logger.Info("=== 実行完了 ===")
	opts.Logger.Info("完了: %d スクリプト", status.Queue.Completed)

	if status.Queue.Failed > 0 {
		opts.Logger.Warn("失敗: %d スクリプト", status.Queue.Failed)
	}

	if status.Queue.Retrying > 0 {
		opts.Logger.Warn("ペンディング: %d スクリプト", status.Queue.Retrying)
	}

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
		return fmt.Errorf("%d scripts remain unprocessed", remainingCount)
	}

	opts.Logger.Info("すべてのプロンプトスクリプトが正常に実行されました")
	return nil
}
