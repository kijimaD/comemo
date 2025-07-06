package executor

import (
	"sync"
	"time"

	"comemo/internal/config"
)

// ScriptState represents the current state of a script
type ScriptState int

const (
	StateWaiting    ScriptState = iota // 待機中
	StateProcessing                    // 実行中
	StateRetrying                      // リトライ待ち
	StateFailed                        // 失敗
	StateCompleted                     // 完了
)

// String returns the string representation of ScriptState
func (s ScriptState) String() string {
	switch s {
	case StateWaiting:
		return "waiting"
	case StateProcessing:
		return "processing"
	case StateRetrying:
		return "retrying"
	case StateFailed:
		return "failed"
	case StateCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

// RetryReason represents the reason for retry
type RetryReason int

const (
	RetryReasonQuotaError   RetryReason = iota // quota error - 1時間待機
	RetryReasonQualityError                    // 品質テストエラー - 10秒待機
	RetryReasonOtherError                      // その他のエラー - 5分待機
)

// String returns the string representation of RetryReason
func (r RetryReason) String() string {
	switch r {
	case RetryReasonQuotaError:
		return "quota_error"
	case RetryReasonQualityError:
		return "quality_error"
	case RetryReasonOtherError:
		return "other_error"
	default:
		return "unknown"
	}
}

// GetRetryDelay returns the delay duration for the retry reason using config
func (r RetryReason) GetRetryDelay(cfg *config.Config) time.Duration {
	switch r {
	case RetryReasonQuotaError:
		return cfg.RetryDelays.QuotaError
	case RetryReasonQualityError:
		return cfg.RetryDelays.QualityError
	case RetryReasonOtherError:
		return cfg.RetryDelays.OtherError
	default:
		return time.Minute // デフォルト1分
	}
}

// GetRetryDelayDefault returns the default delay duration for the retry reason
func (r RetryReason) GetRetryDelayDefault() time.Duration {
	switch r {
	case RetryReasonQuotaError:
		return time.Hour // 1時間
	case RetryReasonQualityError:
		return 10 * time.Second // 10秒
	case RetryReasonOtherError:
		return 5 * time.Minute // 5分
	default:
		return time.Minute // デフォルト1分
	}
}

// ScriptStatus represents the detailed status of a script
type ScriptStatus struct {
	Name        string        `json:"name"`
	State       ScriptState   `json:"state"`
	RetryCount  int           `json:"retry_count"`
	MaxRetries  int           `json:"max_retries"`
	RetryReason RetryReason   `json:"retry_reason,omitempty"`
	RetryAfter  time.Time     `json:"retry_after,omitempty"`
	AssignedCLI string        `json:"assigned_cli,omitempty"`
	LastError   string        `json:"last_error,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
}

// IsRetryable returns true if the script can be retried
func (s *ScriptStatus) IsRetryable() bool {
	return s.State == StateRetrying && s.RetryCount < s.MaxRetries
}

// CanRetryNow returns true if the script can be retried now
func (s *ScriptStatus) CanRetryNow() bool {
	return s.IsRetryable() && time.Now().After(s.RetryAfter)
}

// ScriptStateManager manages the state of all scripts
type ScriptStateManager struct {
	scripts map[string]*ScriptStatus
	config  *config.Config
	mu      sync.RWMutex
}

// NewScriptStateManager creates a new script state manager
func NewScriptStateManager(cfg *config.Config) *ScriptStateManager {
	return &ScriptStateManager{
		scripts: make(map[string]*ScriptStatus),
		config:  cfg,
	}
}

// InitializeScript initializes a script with waiting state
func (sm *ScriptStateManager) InitializeScript(scriptName string, maxRetries int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	sm.scripts[scriptName] = &ScriptStatus{
		Name:       scriptName,
		State:      StateWaiting,
		RetryCount: 0,
		MaxRetries: maxRetries,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// GetScript returns the script status
func (sm *ScriptStateManager) GetScript(scriptName string) *ScriptStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if script, exists := sm.scripts[scriptName]; exists {
		// Return a copy to avoid race conditions
		scriptCopy := *script
		return &scriptCopy
	}
	return nil
}

// GetAllScripts returns all script statuses
func (sm *ScriptStateManager) GetAllScripts() map[string]*ScriptStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]*ScriptStatus)
	for name, script := range sm.scripts {
		scriptCopy := *script
		result[name] = &scriptCopy
	}
	return result
}

// GetScriptsByState returns scripts in the specified state
func (sm *ScriptStateManager) GetScriptsByState(state ScriptState) []*ScriptStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*ScriptStatus
	for _, script := range sm.scripts {
		if script.State == state {
			scriptCopy := *script
			result = append(result, &scriptCopy)
		}
	}
	return result
}

// GetRetryableScripts returns scripts that can be retried now
func (sm *ScriptStateManager) GetRetryableScripts() []*ScriptStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*ScriptStatus
	for _, script := range sm.scripts {
		if script.State == StateRetrying && script.CanRetryNow() {
			scriptCopy := *script
			result = append(result, &scriptCopy)
		}
	}
	return result
}

// SetScriptProcessing sets a script to processing state
func (sm *ScriptStateManager) SetScriptProcessing(scriptName string, cliName string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if script, exists := sm.scripts[scriptName]; exists {
		script.State = StateProcessing
		script.AssignedCLI = cliName
		script.UpdatedAt = time.Now()
	}
}

// SetScriptCompleted sets a script to completed state
func (sm *ScriptStateManager) SetScriptCompleted(scriptName string, duration time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if script, exists := sm.scripts[scriptName]; exists {
		now := time.Now()
		script.State = StateCompleted
		script.Duration = duration
		script.CompletedAt = &now
		script.UpdatedAt = now
	}
}

// SetScriptRetrying sets a script to retrying state
func (sm *ScriptStateManager) SetScriptRetrying(scriptName string, reason RetryReason, errorMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if script, exists := sm.scripts[scriptName]; exists {
		script.RetryCount++
		script.State = StateRetrying
		script.RetryReason = reason
		script.RetryAfter = time.Now().Add(reason.GetRetryDelay(sm.config))
		script.LastError = errorMsg
		script.UpdatedAt = time.Now()
	}
}

// SetScriptFailed sets a script to failed state
func (sm *ScriptStateManager) SetScriptFailed(scriptName string, errorMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if script, exists := sm.scripts[scriptName]; exists {
		script.State = StateFailed
		script.LastError = errorMsg
		script.UpdatedAt = time.Now()
	}
}

// GetStateCounts returns the count of scripts in each state
func (sm *ScriptStateManager) GetStateCounts() map[ScriptState]int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	counts := make(map[ScriptState]int)
	for _, script := range sm.scripts {
		counts[script.State]++
	}
	return counts
}

// GetRetryStats returns retry statistics
func (sm *ScriptStateManager) GetRetryStats() map[RetryReason]int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := make(map[RetryReason]int)
	for _, script := range sm.scripts {
		if script.State == StateRetrying {
			stats[script.RetryReason]++
		}
	}
	return stats
}
