package executor

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"comemo/internal/config"
	"comemo/internal/logger"
)

// ExecutePromptsWithProgressScheduler executes scripts using scheduler with progress display
func ExecutePromptsWithProgressScheduler(cfg *config.Config, cliCommand string) error {
	// Check if we're in a terminal
	if !isTerminalSupported() {
		opts := &ExecutorOptions{
			Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
		}
		return ExecutePromptsWithScheduler(cfg, cliCommand, opts)
	}

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
		fmt.Println("No .sh files found in prompts directory")
		return nil
	}

	// Create silent logger for scheduler (progress display handles output)
	opts := &ExecutorOptions{
		Logger: logger.Silent(),
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

	// Create status manager
	statusManager := NewStatusManager()
	statusManager.SetTotalScripts(len(shFiles))
	statusManager.Start()
	defer statusManager.Stop()

	// Initialize workers in status manager
	for _, cliName := range cliTools {
		statusManager.InitializeWorker(cliName)
	}

	// Create and start progress display
	progressDisplay := NewProgressDisplay(statusManager)
	progressDisplay.Start()
	defer progressDisplay.Stop()

	// Print initial header
	fmt.Printf("🚀 Comemo Execution Started\n")
	fmt.Printf("├── Scripts: %d files\n", len(shFiles))
	fmt.Printf("├── CLI: %s\n", cliCommand)
	fmt.Printf("└── Workers: %s\n", strings.Join(cliTools, ", "))
	fmt.Println()

	// Create scheduler with context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupts
	handleInterrupt(ctx, cancel)

	// Create and run scheduler
	scheduler := NewScheduler(cfg, shFiles, manager, statusManager, opts.Logger)

	// Handle panics from critical errors
	var criticalError error
	schedulerDone := make(chan struct{})

	go func() {
		defer close(schedulerDone)
		defer func() {
			if r := recover(); r != nil {
				if execErr, ok := r.(*ExecutionError); ok && execErr.Type == ErrorTypeCritical {
					criticalError = execErr
					cancel() // Cancel context to stop scheduler
				} else {
					panic(r) // Re-panic for unexpected errors
				}
			}
		}()

		// Run the scheduler
		if err := scheduler.Run(ctx, cliTools); err != nil {
			criticalError = err
		}
	}()

	// Wait for scheduler to complete or context to be cancelled
	select {
	case <-schedulerDone:
		// Scheduler completed normally
	case <-ctx.Done():
		// Context was cancelled (Ctrl+C)
		fmt.Println("\n🛑 Cancelling execution...")
		// Wait a bit for graceful shutdown
		select {
		case <-schedulerDone:
			// Scheduler stopped gracefully
		case <-time.After(3 * time.Second):
			// Force stop after timeout
			fmt.Println("🛑 Force stopping...")
		}
	}

	// Final status summary
	status := statusManager.GetStatus()
	fmt.Printf("\n🏁 Execution Summary\n")
	fmt.Printf("├── ✅ Completed: %d scripts\n", status.Queue.Completed)
	if status.Queue.Failed > 0 {
		fmt.Printf("├── ❌ Failed: %d scripts\n", status.Queue.Failed)
	}
	if status.Queue.Retrying > 0 {
		fmt.Printf("├── 🔄 Retrying: %d scripts\n", status.Queue.Retrying)
	}

	elapsed := status.Performance.ElapsedTime.Round(time.Second)
	fmt.Printf("├── ⏱️ Total time: %v\n", elapsed)

	if status.Performance.ScriptsPerMinute > 0 {
		fmt.Printf("├── ⚡ Average speed: %.1f scripts/min\n", status.Performance.ScriptsPerMinute)
	}

	// Worker summary
	fmt.Printf("└── 🤖 Workers:\n")
	for _, name := range cliTools {
		if worker, exists := status.Workers[name]; exists {
			fmt.Printf("    ├── %s: %d scripts processed\n", name, worker.ProcessedCount)
		}
	}

	if criticalError != nil {
		if execErr, ok := criticalError.(*ExecutionError); ok && execErr.Type == ErrorTypeCritical {
			return fmt.Errorf("実行が重要なエラーにより停止されました")
		}
		return criticalError
	}

	return nil
}

// handleInterrupt sets up signal handling for graceful shutdown
func handleInterrupt(ctx context.Context, cancel context.CancelFunc) {
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		select {
		case <-sigChan:
			fmt.Println("\n🛑 Interrupt received, shutting down gracefully...")
			cancel()
		case <-ctx.Done():
			return
		}
	}()
}
