package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"comemo/internal/config"
	"comemo/internal/logger"
)

// Task represents a script execution task
type Task struct {
	Script  string
	CLI     string
	AddedAt time.Time
}

// WorkerResult represents the result of a worker execution
type WorkerResult struct {
	Script       string
	CLI          string
	Success      bool
	IsQuotaError bool
	IsRetryable  bool
	IsCritical   bool
	Error        error
	Output       string
	Duration     time.Duration
}

// Scheduler manages all script assignments and execution decisions
type Scheduler struct {
	config         *config.Config       // 設定
	scripts        []string             // 処理対象スクリプト
	cliManager     *CLIManager          // CLI状態管理
	workers        map[string]chan Task // ワーカー名 -> タスクチャネル
	results        chan WorkerResult    // ワーカーからの結果チャネル
	queueManager   *QueueManager        // キュー管理
	completed      map[string]bool      // 完了したスクリプト（既存互換性のため）
	failed         map[string]int       // 失敗回数（既存互換性のため）
	retryLimit     int                  // リトライ回数上限
	mu             sync.Mutex
	logger         *logger.Logger
	wg             sync.WaitGroup
	stopCh         chan struct{}
	statusManager  *StatusManager
	scriptStateMgr *ScriptStateManager          // 新しいスクリプト状態管理
	ctx            context.Context              // Context for cancellation
	resultWaiters  map[string]chan WorkerResult // スクリプト名 -> 結果待機チャネル
	activeCLIs     []string                     // 実行対象のCLIリスト
	pendingScripts []string                     // キュー待ちスクリプトリスト
	assignAttempts int                          // 割り当て試行回数（ログ抑制用）
}

// NewScheduler creates a new scheduler instance
func NewScheduler(cfg *config.Config, scripts []string, cliManager *CLIManager, statusManager *StatusManager, logger *logger.Logger) *Scheduler {
	// Extract CLI names from cliManager
	var cliNames []string
	for cliName := range cliManager.CLIs {
		cliNames = append(cliNames, cliName)
	}

	s := &Scheduler{
		config:         cfg,
		scripts:        scripts,
		cliManager:     cliManager,
		workers:        make(map[string]chan Task),
		results:        make(chan WorkerResult, cfg.ResultChannelSize),
		queueManager:   NewQueueManager(cliNames, cfg.QueueCapacityPerCLI, logger),
		completed:      make(map[string]bool),
		failed:         make(map[string]int),
		retryLimit:     cfg.MaxRetries,
		logger:         logger,
		stopCh:         make(chan struct{}),
		statusManager:  statusManager,
		scriptStateMgr: NewScriptStateManager(cfg),
		resultWaiters:  make(map[string]chan WorkerResult),
		pendingScripts: make([]string, 0),
	}

	// Initialize script states
	for _, script := range scripts {
		s.scriptStateMgr.InitializeScript(script, cfg.MaxRetries)
	}

	return s
}

// Run starts the scheduler main loop
func (s *Scheduler) Run(ctx context.Context, cliNames []string) error {
	s.ctx = ctx             // Store context for workers
	s.activeCLIs = cliNames // Store active CLIs for selection
	s.logger.Info("=== スケジューラー起動 ===")
	s.logger.Info("対象スクリプト数: %d", len(s.scripts))
	s.logger.Info("使用CLI: %v", cliNames)

	// Start workers
	for _, cliName := range cliNames {
		taskChan := make(chan Task, s.config.WorkerChannelSize)
		s.workers[cliName] = taskChan
		s.wg.Add(1)
		go s.runWorker(cliName, taskChan)
	}

	// Start result handler
	s.wg.Add(1)
	go s.handleResults()

	// Initial script assignment
	s.logger.Debug("=== 初期スクリプト割り当て開始 ===")

	// If no scripts, complete immediately
	if len(s.scripts) == 0 {
		s.logger.Info("=== スクリプトなし - 即座に完了 ===")
		s.Stop()
		return nil
	}

	// Initialize all scripts as pending
	s.pendingScripts = make([]string, len(s.scripts))
	copy(s.pendingScripts, s.scripts)

	// Try to assign as many scripts as possible to available queue slots
	s.assignPendingScripts()

	// Periodic reevaluation
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("=== コンテキストキャンセル - スケジューラー停止 ===")
			s.Stop()
			return ctx.Err()
		case <-ticker.C:
			// Check context first
			if ctx.Err() != nil {
				s.logger.Info("=== コンテキストキャンセル検出 - スケジューラー停止 ===")
				s.Stop()
				return ctx.Err()
			}

			s.reevaluateQueuedScripts()
			s.assignPendingScripts() // 待機中スクリプトの割り当て試行
			if s.isAllCompleted() {
				s.logger.Info("=== 全スクリプト処理完了 ===")
				s.Stop()
				return nil
			}
		case <-s.stopCh:
			s.logger.Info("=== スケジューラー停止 ===")
			return nil
		}
	}
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	// Close stop channel (non-blocking)
	select {
	case <-s.stopCh:
		// Already closed
	default:
		close(s.stopCh)
	}

	// Close all worker channels
	for name, ch := range s.workers {
		select {
		case <-ch:
			// Already closed
		default:
			s.logger.Debug("Closing worker channel: %s", name)
			close(ch)
		}
	}

	// Close results channel
	select {
	case <-s.results:
		// Already closed
	default:
		close(s.results)
	}

	// Wait for all workers to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Debug("All workers stopped gracefully")
	case <-time.After(5 * time.Second):
		s.logger.Warn("Force stopping workers after timeout")
	}
}

// selectBestCLIWithCapacity selects the best available CLI with queue capacity
func (s *Scheduler) selectBestCLIWithCapacity() string {
	var availableCLIs []string

	// Only consider active CLIs (those specified in the command)
	for _, name := range s.activeCLIs {
		// Use IsAvailable to check current availability (includes recovery logic)
		if s.cliManager.IsAvailable(name) {
			// Check if this CLI has queue capacity using QueueManager
			if s.queueManager.HasCapacity(name) {
				availableCLIs = append(availableCLIs, name)
			}
		}
	}

	if len(availableCLIs) == 0 {
		return ""
	}

	// Select CLI with least queue length for load balancing
	bestCLI := availableCLIs[0]
	minQueueLen := s.queueManager.Length(bestCLI)
	for _, cliName := range availableCLIs[1:] {
		queueLen := s.queueManager.Length(cliName)
		if queueLen < minQueueLen {
			bestCLI = cliName
			minQueueLen = queueLen
		}
	}

	return bestCLI
}

// reevaluateQueuedScripts checks queued scripts and retryable scripts, then tries to execute them
func (s *Scheduler) reevaluateQueuedScripts() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug("=== キューイングスクリプト再評価 ===")

	// Check retryable scripts first (from both old and new state management)
	retryableScripts := s.scriptStateMgr.GetRetryableScripts()

	// Also check EventStatusManager for retry-ready scripts
	if eventMgr := s.getEventStatusManager(); eventMgr != nil {
		eventRetryScripts := eventMgr.GetRetryReadyScripts()
		if len(eventRetryScripts) > 0 {
			s.logger.Debug("  イベントステータス管理からのリトライ可能スクリプト数: %d", len(eventRetryScripts))
			for _, scriptName := range eventRetryScripts {
				s.logger.Debug("    → %s がリトライ準備完了", scriptName)
				s.assignScriptForRetry(scriptName)
			}
		}
	}

	if len(retryableScripts) > 0 {
		s.logger.Debug("  リトライ可能なスクリプト数: %d", len(retryableScripts))
		for _, script := range retryableScripts {
			s.logger.Debug("    → %s がリトライ可能 (理由: %s)", script.Name, script.RetryReason.String())
			// Try to find an available CLI and add script to its queue
			s.assignScriptForRetry(script.Name)
		}
	}

	totalQueued := s.queueManager.TotalLength()

	if totalQueued == 0 {
		s.logger.Debug("  キューイングスクリプトなし")
		return
	}

	s.logger.Debug("  総キューイング数: %d", totalQueued)

	// Check each CLI's queued scripts
	queueStatus := s.queueManager.GetQueueStatus()
	for cliName, queueLen := range queueStatus {
		if queueLen == 0 {
			continue
		}

		// Check if CLI is available and worker has capacity
		if s.cliManager.IsAvailable(cliName) {
			s.logger.Debug("  CLI %s が利用可能 (キュー数: %d)", cliName, queueLen)

			// Try to execute first script in queue
			if firstScript, exists := s.queueManager.Peek(cliName); exists {
				// Check new state management
				scriptState := s.scriptStateMgr.GetScript(firstScript)
				if scriptState == nil {
					s.logger.Debug("    → %s の状態が見つからない", firstScript)
					s.queueManager.Remove(cliName, firstScript)
					continue
				}

				// Skip if completed or failed
				if scriptState.State == StateCompleted || scriptState.State == StateFailed {
					s.logger.Debug("    → %s はスキップ (状態: %s)", firstScript, scriptState.State.String())
					s.queueManager.Remove(cliName, firstScript)
					continue
				}

				// Skip if in retrying state and not ready yet
				if scriptState.State == StateRetrying && !scriptState.CanRetryNow() {
					remaining := time.Until(scriptState.RetryAfter)
					s.logger.Debug("    → %s はリトライ待ち中 (残り: %v)", firstScript, remaining)
					continue
				}

				// Execute the script
				if task := s.queueManager.ProcessQueue(cliName); task != nil {
					select {
					case s.workers[cliName] <- *task:
						s.logger.Debug("    → %s を実行開始", task.Script)
						s.scriptStateMgr.SetScriptProcessing(task.Script, cliName)
						s.statusManager.RecordScriptStart(task.Script, cliName)
					default:
						s.logger.Debug("    → %s のワーカーがビジー", cliName)
					}
				}
			}
		} else {
			cli := s.cliManager.CLIs[cliName]
			recoveryDelay := cli.RecoveryDelay
			if recoveryDelay == 0 {
				recoveryDelay = s.cliManager.Config.QuotaRetryDelay
			}
			remaining := time.Until(cli.LastQuotaError.Add(recoveryDelay))
			s.logger.Debug("  CLI %s はまだ利用不可 (キュー数: %d, 回復まで: %v)",
				cliName, queueLen, remaining)
		}
	}
}

// assignScriptForRetry tries to assign a retryable script to an available CLI
func (s *Scheduler) assignScriptForRetry(scriptName string) {
	// Find best available CLI with capacity
	bestCLI := s.selectBestCLIWithCapacity()
	if bestCLI == "" {
		s.logger.Debug("    → %s のリトライ: 利用可能なCLIなし", scriptName)
		return
	}

	// Check if script is already in any queue
	if cliName, exists := s.queueManager.IsScriptInQueue(scriptName); exists {
		s.logger.Debug("    → %s は既に %s のキューに存在", scriptName, cliName)
		return
	}

	// Add to queue
	if s.queueManager.Enqueue(bestCLI, scriptName) {
		s.logger.Debug("    → %s をリトライ用に %s にキューイング", scriptName, bestCLI)
	} else {
		s.logger.Debug("    → %s のキューイングに失敗", scriptName)
		return
	}

	// 一元的なタスク状態管理: キューイング（リトライ）
	if taskStateManager := s.getTaskStateManager(); taskStateManager != nil {
		taskStateManager.TransitionToQueued(scriptName, bestCLI)
	} else {
		// Fallback to direct event logging
		if eventLogger := s.getTaskEventLogger(); eventLogger != nil {
			eventLogger.LogQueued(scriptName, bestCLI)
		}
	}
}

// handleResults processes results from workers
func (s *Scheduler) handleResults() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debug("=== 結果ハンドラー - コンテキストキャンセル ===")
			return
		case result, ok := <-s.results:
			if !ok {
				s.logger.Debug("=== 結果ハンドラー - チャネルクローズ ===")
				return
			}
			s.handleWorkerResult(result)
		}
	}
}

// handleWorkerResult processes a single worker result with new state management
func (s *Scheduler) handleWorkerResult(result WorkerResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Debug("=== ワーカー結果処理: %s (CLI: %s) ===", result.Script, result.CLI)

	// Notify result waiters if any
	if waiter, exists := s.resultWaiters[result.Script]; exists {
		select {
		case waiter <- result:
			s.logger.Debug("  → 結果を待機中のゴルーチンに送信")
		default:
			s.logger.Debug("  → 結果待機チャネルの送信に失敗")
		}
		delete(s.resultWaiters, result.Script)
	}

	// Handle success
	if result.Success {
		s.logger.Debug("  → 成功")
		// Update new state management
		s.scriptStateMgr.SetScriptCompleted(result.Script, result.Duration)
		// Update old state management for compatibility
		s.completed[result.Script] = true
		delete(s.failed, result.Script)
		s.queueManager.MarkScriptProcessed(result.CLI, result.Script)
		s.statusManager.RecordScriptComplete(result.Script, result.CLI, true, result.Duration, "")
		s.statusManager.RemoveRetryScript(result.Script)
		return
	}

	// Handle failures based on error type
	var errorMsg string
	if result.Error != nil {
		errorMsg = result.Error.Error()
	}

	// Critical errors - stop execution immediately
	if result.IsCritical {
		s.logger.Error("  → 致命的エラー: %v", result.Error)
		s.scriptStateMgr.SetScriptFailed(result.Script, errorMsg)
		s.queueManager.MarkScriptProcessed(result.CLI, result.Script)
		panic(&ExecutionError{
			Type:     ErrorTypeCritical,
			Message:  errorMsg,
			Output:   result.Output,
			Script:   result.Script,
			CLIName:  result.CLI,
			Original: result.Error,
		})
	}

	// Get current script state for retry logic
	scriptState := s.scriptStateMgr.GetScript(result.Script)
	if scriptState == nil {
		s.logger.Error("  → スクリプト状態が見つからない: %s", result.Script)
		return
	}

	// Determine retry reason first to decide whether to increment count
	var retryReason RetryReason
	var willIncrementRetryCount bool

	if result.IsQuotaError {
		s.logger.Debug("  → Quotaエラー: CLI %s を設定時間スリープ（リトライカウント増加なし）", result.CLI)
		retryReason = RetryReasonQuotaError
		willIncrementRetryCount = false
		// Set CLI unavailable for configured duration
		retryDelay := s.config.RetryDelays.QuotaError
		s.cliManager.MarkUnavailableForDuration(result.CLI, retryDelay)
		s.statusManager.UpdateWorkerStatus(result.CLI, false, "", retryDelay)
		s.statusManager.RecordQuotaError(result.Script, result.CLI, result.Duration)
	} else {
		// Classify error to determine retry reason
		errorType := ClassifyError(result.Error, result.Output)
		switch errorType {
		case ErrorTypeQuality:
			s.logger.Debug("  → 品質エラー: 短時間待機後リトライ")
			retryReason = RetryReasonQualityError
		default:
			s.logger.Debug("  → その他のエラー: 通常待機後リトライ")
			retryReason = RetryReasonOtherError
		}
		willIncrementRetryCount = true
		// Record as retry error with specific error message
		s.statusManager.RecordRetryError(result.Script, result.CLI, result.Duration, errorMsg)
	}

	// Check if retry limit will be exceeded after this attempt
	futureRetryCount := scriptState.RetryCount
	if willIncrementRetryCount {
		futureRetryCount++
	}

	if futureRetryCount >= scriptState.MaxRetries {
		s.logger.Error("  → リトライ上限到達: %s (現在: %d, 上限: %d)", result.Script, futureRetryCount, scriptState.MaxRetries)
		s.scriptStateMgr.SetScriptFailed(result.Script, errorMsg)
		s.queueManager.MarkScriptProcessed(result.CLI, result.Script)
		// Update old state management for compatibility
		s.failed[result.Script] = s.retryLimit
		s.statusManager.RecordScriptComplete(result.Script, result.CLI, false, result.Duration, errorMsg)
		return
	}

	// Set script to retrying state with appropriate delay
	if result.IsQuotaError {
		// Use special quota error handling that doesn't increment retry count
		s.scriptStateMgr.SetScriptQuotaExceeded(result.Script, errorMsg)
	} else {
		// Set script to retrying state with appropriate delay (increments retry count)
		s.scriptStateMgr.SetScriptRetrying(result.Script, retryReason, errorMsg)
	}
	s.logger.Debug("  → スクリプト %s をリトライ待ち状態に設定 (理由: %s, 待機時間: %v)",
		result.Script, retryReason.String(), retryReason.GetRetryDelay(s.config))

	// 一元的なタスク状態管理: リトライ状態
	if taskStateManager := s.getTaskStateManager(); taskStateManager != nil {
		// Get updated retry count (quota errors don't increment count)
		updatedState := s.scriptStateMgr.GetScript(result.Script)
		retryCount := updatedState.RetryCount
		if result.IsQuotaError {
			taskStateManager.TransitionToQuotaExceeded(result.Script, result.CLI)
		} else {
			taskStateManager.TransitionToRetrying(result.Script, result.CLI, retryCount, retryReason.String(), errorMsg)
		}
	} else {
		// Fallback to direct event logging
		if eventLogger := s.getTaskEventLogger(); eventLogger != nil {
			// Get updated retry count (quota errors don't increment count)
			updatedState := s.scriptStateMgr.GetScript(result.Script)
			retryCount := updatedState.RetryCount
			if result.IsQuotaError {
				eventLogger.LogQuotaExceeded(result.Script, result.CLI)
			} else {
				// Include both retry reason type and actual error message
				detailedReason := fmt.Sprintf("%s: %s", retryReason.String(), errorMsg)
				eventLogger.LogRetryingWithCLI(result.Script, result.CLI, retryCount, detailedReason)
			}
		}
	}

	// Update old state management for compatibility - but don't increase failed count for retrying scripts
	s.statusManager.AddRetryScript(result.Script)
}

// queueScript queues a script for execution on a specific CLI
func (s *Scheduler) queueScript(script string, cliName string) bool {
	return s.queueManager.Enqueue(cliName, script)
}

// isAllCompleted checks if all scripts have been processed
func (s *Scheduler) isAllCompleted() bool {
	completedCount := len(s.completed)
	totalCount := len(s.scripts)

	// Check if there are any queued scripts
	totalQueued := s.queueManager.TotalLength()

	// Check if there are any failed scripts beyond retry limit
	totalFailed := 0
	for _, failCount := range s.failed {
		if failCount >= s.retryLimit {
			totalFailed++
		}
	}

	// Check pending scripts count
	pendingCount := len(s.pendingScripts)

	s.logger.Debug("完了: %d/%d, キュー中: %d, 失敗: %d, 待機中: %d", completedCount, totalCount, totalQueued, totalFailed, pendingCount)

	// All completed when: completed + failed beyond retry limit = total scripts AND no queued or pending scripts
	return completedCount+totalFailed >= totalCount && totalQueued == 0 && pendingCount == 0
}

// runWorker runs a worker for a specific CLI
func (s *Scheduler) runWorker(cliName string, tasks <-chan Task) {
	defer s.wg.Done()

	// Create dedicated worker instance
	worker := NewWorker(fmt.Sprintf("Worker-%s", cliName), s.cliManager)

	// Use Worker.Run method for execution
	worker.Run(s.ctx, tasks, s.results)
}

// ExecuteScriptSync executes a script synchronously and returns the result
func (s *Scheduler) ExecuteScriptSync(script string) (WorkerResult, error) {
	s.mu.Lock()

	// Skip if already completed
	if s.completed[script] {
		s.mu.Unlock()
		return WorkerResult{Script: script, Success: true}, nil
	}

	// Find best available CLI with capacity
	bestCLI := s.selectBestCLIWithCapacity()
	if bestCLI == "" {
		s.mu.Unlock()
		return WorkerResult{}, fmt.Errorf("no available CLI with capacity")
	}

	// Queue the script
	if !s.queueScript(script, bestCLI) {
		s.mu.Unlock()
		return WorkerResult{}, fmt.Errorf("failed to queue script")
	}

	// Create result waiter
	resultChan := make(chan WorkerResult, 1)
	s.resultWaiters[script] = resultChan

	s.mu.Unlock()

	// Try to execute immediately
	task := Task{
		Script:  script,
		CLI:     bestCLI,
		AddedAt: time.Now(),
	}

	select {
	case s.workers[bestCLI] <- task:
		s.statusManager.RecordScriptStart(script, bestCLI)
		s.logger.Debug("Script %s queued and started on %s", script, bestCLI)
	default:
		s.logger.Debug("Script %s queued but worker busy: %s", script, bestCLI)
	}

	// Wait for result
	select {
	case result := <-resultChan:
		return result, nil
	case <-s.ctx.Done():
		return WorkerResult{}, s.ctx.Err()
	case <-time.After(10 * time.Minute): // Timeout after 10 minutes
		return WorkerResult{}, fmt.Errorf("script execution timeout")
	}
}

// getEventStatusManager returns the EventStatusManager from ExecutorOptions if available
func (s *Scheduler) getEventStatusManager() *EventStatusManager {
	// This assumes we have access to the executor options through the CLI manager
	// For now, we'll check if the manager has ExecutorOptions with EventStatusManager
	if s.cliManager != nil && s.cliManager.Options != nil {
		return s.cliManager.Options.EventStatusManager
	}
	return nil
}

// getTaskEventLogger returns the TaskEventLogger from ExecutorOptions if available
func (s *Scheduler) getTaskEventLogger() *TaskEventLogger {
	if s.cliManager != nil && s.cliManager.Options != nil {
		return s.cliManager.Options.TaskEventLogger
	}
	return nil
}

// getTaskStateManager returns the TaskStateManager from ExecutorOptions if available
func (s *Scheduler) getTaskStateManager() *TaskStateManager {
	if s.cliManager != nil && s.cliManager.Options != nil {
		return s.cliManager.Options.TaskStateManager
	}
	return nil
}

// assignPendingScripts tries to assign pending scripts to available queue slots
func (s *Scheduler) assignPendingScripts() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pendingScripts) == 0 {
		return
	}

	// Calculate total available queue capacity
	totalAvailableSlots := 0
	for _, cliName := range s.activeCLIs {
		if s.cliManager.IsAvailable(cliName) {
			availableSlots := s.queueManager.GetAvailableSlots(cliName)
			if availableSlots > 0 {
				totalAvailableSlots += availableSlots
			}
		}
	}

	// If no capacity, skip assignment
	if totalAvailableSlots == 0 {
		s.assignAttempts++
		if s.assignAttempts%10 == 1 {
			s.logger.Debug("待機中スクリプト割り当て: 全CLIのキューが満杯 (待機中: %d, 試行回数: %d)", len(s.pendingScripts), s.assignAttempts)
		}
		return
	}

	s.logger.Debug("待機中スクリプト割り当て: %d スロット利用可能, %d スクリプト待機中", totalAvailableSlots, len(s.pendingScripts))

	// Process only as many scripts as we have available slots
	maxToProcess := totalAvailableSlots
	if maxToProcess > len(s.pendingScripts) {
		maxToProcess = len(s.pendingScripts)
	}

	var remainingScripts []string
	assignedCount := 0
	processedCount := 0

	for _, script := range s.pendingScripts {
		// Stop if we've processed enough scripts for available slots
		if processedCount >= maxToProcess {
			remainingScripts = append(remainingScripts, script)
			continue
		}

		// Check if script state allows assignment
		scriptState := s.scriptStateMgr.GetScript(script)
		if scriptState == nil {
			remainingScripts = append(remainingScripts, script)
			continue
		}

		// Skip if already completed or failed
		if scriptState.State == StateCompleted || scriptState.State == StateFailed {
			processedCount++
			continue
		}

		// Skip if already processing
		if scriptState.State == StateProcessing {
			processedCount++
			continue
		}

		// Try to find available CLI with capacity
		bestCLI := s.selectBestCLIWithCapacity()
		if bestCLI != "" {
			// Add script to queue
			if s.queueManager.Enqueue(bestCLI, script) {
				assignedCount++

				// 一元的なタスク状態管理: キューイング
				if taskStateManager := s.getTaskStateManager(); taskStateManager != nil {
					taskStateManager.TransitionToQueued(script, bestCLI)
				} else {
					// Fallback to direct event logging
					if eventLogger := s.getTaskEventLogger(); eventLogger != nil {
						eventLogger.LogQueued(script, bestCLI)
					}
				}

				// Try to execute immediately if worker has capacity
				if task := s.queueManager.ProcessQueue(bestCLI); task != nil {
					// Non-blocking send
					select {
					case s.workers[bestCLI] <- *task:
						s.scriptStateMgr.SetScriptProcessing(task.Script, bestCLI)
						s.statusManager.RecordScriptStart(task.Script, bestCLI)
						s.logger.Debug("  → %s で実行開始: %s", bestCLI, task.Script)
					default:
						s.logger.Debug("  → %s のワーカーがビジー、キューに待機", bestCLI)
					}
				}
			} else {
				// Failed to enqueue, keep in pending
				remainingScripts = append(remainingScripts, script)
			}
		} else {
			// No available CLI with capacity, keep in pending
			remainingScripts = append(remainingScripts, script)
		}

		processedCount++
	}

	// Update pending scripts list
	s.pendingScripts = remainingScripts

	// Log summary
	if assignedCount > 0 {
		s.logger.Debug("割り当て完了: %d スクリプトを割り当て, %d スクリプトが待機中", assignedCount, len(s.pendingScripts))
	}
}
