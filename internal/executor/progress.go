package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"comemo/internal/config"
)

// ProgressDisplay handles real-time progress display using carriage return
type ProgressDisplay struct {
	statusManager *StatusManager
	ticker        *time.Ticker
	done          chan bool
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	displayLines  int            // Track number of lines currently displayed
	started       bool           // Track if started to avoid double start
	stopped       bool           // Track if stopped to avoid double stop
	wg            sync.WaitGroup // Wait for goroutine completion
}

// IsRunning returns true if the progress display is currently running
func (pd *ProgressDisplay) IsRunning() bool {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return pd.started && !pd.stopped
}

// NewProgressDisplay creates a new progress display
func NewProgressDisplay(statusManager *StatusManager) *ProgressDisplay {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProgressDisplay{
		statusManager: statusManager,
		done:          make(chan bool),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the progress display
func (pd *ProgressDisplay) Start() {
	pd.mu.Lock()
	defer pd.mu.Unlock()

	if pd.started {
		return // Already started
	}

	pd.started = true
	pd.ticker = time.NewTicker(500 * time.Millisecond)

	pd.wg.Add(1)
	go func() {
		defer pd.wg.Done()
		// Create a local ticker to avoid race conditions
		localTicker := time.NewTicker(500 * time.Millisecond)
		defer localTicker.Stop()

		for {
			select {
			case <-pd.ctx.Done():
				return
			case <-localTicker.C:
				pd.updateDisplay()
			case <-pd.done:
				return
			}
		}
	}()
}

// Stop stops the progress display
func (pd *ProgressDisplay) Stop() {
	pd.mu.Lock()
	if pd.stopped {
		pd.mu.Unlock()
		return // Already stopped
	}
	pd.stopped = true

	if pd.ticker != nil {
		pd.ticker.Stop()
		pd.ticker = nil
	}

	pd.cancel()

	// Close done channel safely
	select {
	case <-pd.done:
		// Already closed
	default:
		close(pd.done)
	}
	pd.mu.Unlock()

	// Wait for goroutine to finish
	pd.wg.Wait()

	// Clear all displayed lines and move cursor to final position
	pd.mu.Lock()
	if pd.displayLines > 0 {
		pd.clearPreviousDisplay(pd.displayLines)
	}
	pd.mu.Unlock()
	fmt.Println() // Add final newline
}

// updateDisplay updates the display with multi-line worker status
func (pd *ProgressDisplay) updateDisplay() {
	status := pd.statusManager.GetStatus()

	// Build multi-line display
	var lines []string

	// Overall progress header
	if status.Queue.Total > 0 {
		progress := float64(status.Queue.Completed) / float64(status.Queue.Total) * 100
		progressBar := buildProgressBar(progress, 40)

		headerLine := fmt.Sprintf("📊 Progress: %d/%d (%.1f%%) %s",
			status.Queue.Completed, status.Queue.Total, progress, progressBar)
		lines = append(lines, headerLine)
	}

	// Worker status lines - separate line for each worker
	workerNames := []string{"claude", "gemini"} // Ensure consistent order
	for _, name := range workerNames {
		if worker, exists := status.Workers[name]; exists {
			workerLine := buildWorkerStatusLine(name, worker)
			lines = append(lines, workerLine)
		}
	}

	// Queue details line
	var queueParts []string
	if status.Queue.Processing > 0 {
		queueParts = append(queueParts, fmt.Sprintf("🔄 Processing: %d", status.Queue.Processing))
	}
	if status.Queue.Waiting > 0 {
		queueParts = append(queueParts, fmt.Sprintf("⏳ Waiting: %d", status.Queue.Waiting))
	}
	if status.Queue.Failed > 0 {
		queueParts = append(queueParts, fmt.Sprintf("❌ Failed: %d", status.Queue.Failed))
	}
	if status.Queue.Retrying > 0 {
		queueParts = append(queueParts, fmt.Sprintf("🔄 Retrying: %d", status.Queue.Retrying))
	}

	if len(queueParts) > 0 {
		lines = append(lines, strings.Join(queueParts, " | "))
	}

	// Performance and time line
	var perfParts []string
	elapsed := status.Performance.ElapsedTime.Round(time.Second)
	perfParts = append(perfParts, fmt.Sprintf("⏱️ Elapsed: %v", elapsed))

	if status.Performance.ScriptsPerMinute > 0 {
		perfParts = append(perfParts, fmt.Sprintf("⚡ Speed: %.1f/min", status.Performance.ScriptsPerMinute))

		if status.Performance.EstimatedETA > 0 {
			eta := status.Performance.EstimatedETA.Round(time.Second)
			perfParts = append(perfParts, fmt.Sprintf("🎯 ETA: %v", eta))
		}
	}

	lines = append(lines, strings.Join(perfParts, " | "))

	// Clear previous display and show new content
	if pd.displayLines > 0 {
		pd.clearPreviousDisplay(pd.displayLines)
	}

	// Update display lines count
	pd.displayLines = len(lines)

	// Print all lines
	for i, line := range lines {
		if i > 0 {
			fmt.Print("\n")
		}
		fmt.Print(line)
	}
}

// buildWorkerStatusLine creates a detailed status line for a single worker
func buildWorkerStatusLine(name string, worker *WorkerStatus) string {
	var parts []string

	// Worker name and basic status
	if worker.Available {
		if worker.CurrentScript != "" {
			parts = append(parts, fmt.Sprintf("🤖 %s: 📝 Processing %s", name, worker.CurrentScript))
		} else {
			parts = append(parts, fmt.Sprintf("🤖 %s: ✅ Available", name))
		}
	} else {
		if worker.QuotaRecoveryTime > 0 {
			recovery := worker.QuotaRecoveryTime.Round(time.Second)
			parts = append(parts, fmt.Sprintf("🤖 %s: ⏳ Quota limit (recovery: %v)", name, recovery))
		} else {
			parts = append(parts, fmt.Sprintf("🤖 %s: ❌ Unavailable", name))
		}
	}

	// Processed count and processing count
	parts = append(parts, fmt.Sprintf("(Processed: %d, Processing: %d)", worker.ProcessedCount, worker.ProcessingCount))

	// Last activity
	if !worker.LastActivity.IsZero() {
		timeSince := time.Since(worker.LastActivity).Round(time.Second)
		if timeSince < time.Minute {
			parts = append(parts, fmt.Sprintf("(Last active: %v ago)", timeSince))
		} else {
			parts = append(parts, fmt.Sprintf("(Last active: %v ago)", timeSince))
		}
	}

	// Last failure reason
	if worker.LastFailureReason != "" {
		// Truncate long error messages to keep display manageable
		failureReason := worker.LastFailureReason
		if len(failureReason) > 50 {
			failureReason = failureReason[:47] + "..."
		}
		parts = append(parts, fmt.Sprintf("(Last failure: %s)", failureReason))
	}

	return strings.Join(parts, " ")
}

// buildProgressBar creates a visual progress bar
func buildProgressBar(progress float64, width int) string {
	if width <= 0 {
		return ""
	}

	filled := int(progress * float64(width) / 100)
	if filled > width {
		filled = width
	}

	bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
	return bar
}

// clearPreviousDisplay clears the specified number of lines
func (pd *ProgressDisplay) clearPreviousDisplay(lineCount int) {
	for i := 0; i < lineCount; i++ {
		if i > 0 {
			fmt.Print("\033[A") // Move cursor up one line
		}
		fmt.Print("\r" + strings.Repeat(" ", 120) + "\r") // Clear line
	}
}

// ExecutePromptsWithProgress executes prompts with carriage return progress display
func ExecutePromptsWithProgress(cfg *config.Config, cliCommand string) error {
	// Use the new scheduler-based implementation with progress
	return ExecutePromptsWithProgressScheduler(cfg, cliCommand)
}

// executePromptsWithStatusManagerAndProgress executes prompts with progress tracking
func executePromptsWithStatusManagerAndProgress(ctx context.Context, cfg *config.Config, cliCommand string, opts *ExecutorOptions, statusManager *StatusManager, shFiles []string) error {
	// Create a context that can be cancelled in case of critical errors
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create CLI manager
	manager := NewCLIManagerWithOptions(cfg, opts)

	// Initialize workers in status manager
	for name := range SupportedCLIs {
		statusManager.InitializeWorker(name)
	}

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

	// Start workers with context cancellation and panic recovery
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

						// Cancel context to stop other workers
						cancel()
					} else {
						// Re-panic for unexpected panics
						panic(r)
					}
				}
			}()
			WorkerWithStatusManagerAndProgress(ctx, name, q, manager, opts, statusManager)
		}(cliName, queue)
	}

	// Distribute scripts round-robin among CLIs
	for i, fileName := range shFiles {
		select {
		case <-ctx.Done():
			goto cleanup
		default:
			cliIndex := i % len(cliTools)
			cliName := cliTools[cliIndex]
			scriptQueues[cliName] <- fileName
		}
	}

cleanup:
	// Close queues
	for _, queue := range scriptQueues {
		close(queue)
	}

	// Wait for all workers to complete or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All workers completed normally
	case <-ctx.Done():
		// Context cancelled, wait a bit for graceful shutdown
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			// Force quit after 3 seconds
		}
	}

	// Check if a critical error occurred
	criticalErrorMu.Lock()
	if criticalError != nil {
		criticalErrorMu.Unlock()
		return fmt.Errorf("実行が重要なエラーにより停止されました")
	}
	criticalErrorMu.Unlock()

	return ctx.Err()
}

// WorkerWithStatusManagerAndProgress is a context-aware worker for progress display
func WorkerWithStatusManagerAndProgress(ctx context.Context, cliName string, scriptQueue <-chan string, manager *CLIManager, opts *ExecutorOptions, statusManager *StatusManager) {
	pendingScripts := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return

		case fileName, ok := <-scriptQueue:
			if !ok {
				// Channel closed, process pending scripts if context allows
				if ctx.Err() == nil {
					processPendingScriptsWithProgress(ctx, pendingScripts, cliName, manager, opts, statusManager)
				}
				return
			}

			if ctx.Err() != nil {
				return
			}

			if manager.IsAvailable(cliName) {
				executeScriptWithProgress(ctx, fileName, cliName, manager, opts, statusManager)
				processPendingScriptsWithProgress(ctx, pendingScripts, cliName, manager, opts, statusManager)
			} else {
				pendingScripts[fileName] = true
				updateWorkerUnavailableStatus(cliName, manager, statusManager)
			}

		case <-time.After(5 * time.Second):
			if ctx.Err() != nil {
				return
			}
			processPendingScriptsWithProgress(ctx, pendingScripts, cliName, manager, opts, statusManager)
			updateWorkerUnavailableStatus(cliName, manager, statusManager)
		}
	}
}

// Helper functions
func processPendingScriptsWithProgress(ctx context.Context, pendingScripts map[string]bool, cliName string, manager *CLIManager, opts *ExecutorOptions, statusManager *StatusManager) {
	for fileName := range pendingScripts {
		if ctx.Err() != nil || !manager.IsAvailable(cliName) {
			break
		}
		executeScriptWithProgress(ctx, fileName, cliName, manager, opts, statusManager)
		delete(pendingScripts, fileName)
	}
}

func executeScriptWithProgress(ctx context.Context, fileName, cliName string, manager *CLIManager, opts *ExecutorOptions, statusManager *StatusManager) {
	if ctx.Err() != nil {
		return
	}

	statusManager.RecordScriptStart(fileName, cliName)
	startTime := time.Now()

	err := executeScriptWithContext(ctx, fileName, cliName, manager, opts)
	duration := time.Since(startTime)

	success := err == nil && ctx.Err() == nil
	errorMsg := ""
	if err != nil && ctx.Err() == nil {
		errorMsg = err.Error()

		// Create execution error for proper classification
		scriptPath := filepath.Join(manager.Config.PromptsDir, fileName)
		execError := CreateExecutionError(err, "", scriptPath, cliName)

		switch execError.Type {
		case ErrorTypeQuota:
			manager.MarkUnavailable(cliName)
			statusManager.AddRetryScript(fileName)

		case ErrorTypeCritical:
			// Display critical error and panic to stop execution
			fmt.Printf("\n💥 CRITICAL ERROR DETECTED\n")
			fmt.Printf("Script: %s\n", scriptPath)
			fmt.Printf("CLI: %s\n", cliName)
			fmt.Printf("Error: %v\n", err)
			fmt.Printf("Execution has been stopped due to a critical error.\n")
			fmt.Printf("Please resolve the issue before retrying.\n")
			panic(execError)

		case ErrorTypeTimeout:
			statusManager.AddRetryScript(fileName)

		case ErrorTypeRetryable:
			statusManager.AddRetryScript(fileName)
		}
	}

	statusManager.RecordScriptComplete(fileName, cliName, success, duration, errorMsg)
}

func updateWorkerUnavailableStatus(cliName string, manager *CLIManager, statusManager *StatusManager) {
	if !manager.IsAvailable(cliName) {
		cli := manager.CLIs[cliName]
		quotaRecovery := manager.Config.QuotaRetryDelay - time.Since(cli.LastQuotaError)
		if quotaRecovery < 0 {
			quotaRecovery = 0
		}
		statusManager.UpdateWorkerStatus(cliName, false, "", quotaRecovery)
	}
}

// executeScriptWithContext executes a single script file with context cancellation
func executeScriptWithContext(ctx context.Context, fileName, cliName string, manager *CLIManager, opts *ExecutorOptions) error {
	scriptPath := filepath.Join(manager.Config.PromptsDir, fileName)

	// Get CLI command
	cliCmd, exists := manager.GetCLICommand(cliName)
	if !exists {
		return fmt.Errorf("CLI command %s not found", cliName)
	}

	// Read script content and replace placeholder
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("error reading script %s: %w", scriptPath, err)
	}

	modifiedContent := strings.ReplaceAll(string(content), "{{AI_CLI_COMMAND}}", cliCmd.Command)

	// Create context with timeout, respecting parent context
	timeoutCtx, cancel := context.WithTimeout(ctx, manager.Config.ExecutionTimeout)
	defer cancel()

	// Execute the script with modified content
	cmd := exec.CommandContext(timeoutCtx, "bash", "-c", modifiedContent)
	// Set working directory to project root instead of prompts directory
	if wd, err := os.Getwd(); err == nil {
		cmd.Dir = wd
	}

	// Set environment variables if needed
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("CLI_COMMAND=%s", cliCmd.Command),
		fmt.Sprintf("OUTPUT_DIR=%s", manager.Config.OutputDir),
	)

	output, err := cmd.Output()
	if err != nil {
		// Check if it was cancelled by context
		if timeoutCtx.Err() != nil {
			return timeoutCtx.Err()
		}

		// Check if it's a quota error
		errorOutput := string(output)
		if exitErr, ok := err.(*exec.ExitError); ok {
			errorOutput = string(exitErr.Stderr)
		}

		fullError := fmt.Sprintf("%v: %s", err, errorOutput)

		if IsQuotaError(fullError) {
			manager.MarkUnavailable(cliName)
			return fmt.Errorf("quota error: %s", fullError)
		}

		return fmt.Errorf("execution failed: %s", fullError)
	}

	return nil
}

// isTerminalSupported checks if the current environment supports progress display
func isTerminalSupported() bool {
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	term := os.Getenv("TERM")
	return term != "" && term != "dumb"
}
