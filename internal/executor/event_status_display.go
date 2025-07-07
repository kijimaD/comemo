package executor

import (
	"fmt"
	"strings"
	"time"
)

// EventStatusSummary represents a summary of current event statuses
type EventStatusSummary struct {
	Total          int                    `json:"total"`
	Running        int                    `json:"running"`
	RetryWaiting   int                    `json:"retry_waiting"`
	Failed         int                    `json:"failed"`
	Success        int                    `json:"success"`
	RetryBreakdown map[RetryDelayType]int `json:"retry_breakdown"`
	TopFailures    []EventStatusEntry     `json:"top_failures"`
	LongestRunning []EventStatusEntry     `json:"longest_running"`
}

// GetEventStatusSummary returns a comprehensive summary of event statuses
func (m *EventStatusManager) GetEventStatusSummary() EventStatusSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := EventStatusSummary{
		RetryBreakdown: make(map[RetryDelayType]int),
	}

	var failures []EventStatusEntry
	var runningScripts []EventStatusEntry

	for _, entry := range m.entries {
		summary.Total++

		switch entry.Status {
		case EventStatusRunning:
			summary.Running++
			runningScripts = append(runningScripts, *entry)
		case EventStatusRetryWaiting:
			summary.RetryWaiting++
			summary.RetryBreakdown[entry.RetryDelayType]++
		case EventStatusFailed:
			summary.Failed++
			failures = append(failures, *entry)
		case EventStatusSuccess:
			summary.Success++
		}
	}

	// Sort and limit failures by retry count (top failures)
	if len(failures) > 0 {
		// Simple sort by retry count (descending)
		for i := 0; i < len(failures)-1; i++ {
			for j := i + 1; j < len(failures); j++ {
				if failures[i].RetryCount < failures[j].RetryCount {
					failures[i], failures[j] = failures[j], failures[i]
				}
			}
		}
		if len(failures) > 5 {
			summary.TopFailures = failures[:5]
		} else {
			summary.TopFailures = failures
		}
	}

	// Sort running scripts by duration (longest running)
	if len(runningScripts) > 0 {
		now := time.Now()
		for i := 0; i < len(runningScripts)-1; i++ {
			for j := i + 1; j < len(runningScripts); j++ {
				duration1 := now.Sub(runningScripts[i].StartTime)
				duration2 := now.Sub(runningScripts[j].StartTime)
				if duration1 < duration2 {
					runningScripts[i], runningScripts[j] = runningScripts[j], runningScripts[i]
				}
			}
		}
		if len(runningScripts) > 5 {
			summary.LongestRunning = runningScripts[:5]
		} else {
			summary.LongestRunning = runningScripts
		}
	}

	return summary
}

// FormatEventStatusSummary returns a human-readable string representation of the summary
func FormatEventStatusSummary(summary EventStatusSummary) string {
	var sb strings.Builder

	sb.WriteString("=== イベントステータス サマリー ===\n")
	sb.WriteString(fmt.Sprintf("総計: %d スクリプト\n", summary.Total))
	sb.WriteString(fmt.Sprintf("✅ 成功: %d\n", summary.Success))
	sb.WriteString(fmt.Sprintf("🔄 実行中: %d\n", summary.Running))
	sb.WriteString(fmt.Sprintf("⏳ リトライ待ち: %d\n", summary.RetryWaiting))
	sb.WriteString(fmt.Sprintf("❌ 失敗: %d\n", summary.Failed))

	if len(summary.RetryBreakdown) > 0 {
		sb.WriteString("\n--- リトライ待ち内訳 ---\n")
		for delayType, count := range summary.RetryBreakdown {
			sb.WriteString(fmt.Sprintf("  %s: %d スクリプト (待機時間: %v)\n",
				delayType.String(), count, delayType.GetRetryDelay()))
		}
	}

	if len(summary.LongestRunning) > 0 {
		sb.WriteString("\n--- 実行時間が長いスクリプト TOP5 ---\n")
		now := time.Now()
		for i, entry := range summary.LongestRunning {
			duration := now.Sub(entry.StartTime)
			sb.WriteString(fmt.Sprintf("  %d. %s (%s) - %v 実行中\n",
				i+1, entry.ScriptName, entry.CLI, duration.Round(time.Second)))
		}
	}

	if len(summary.TopFailures) > 0 {
		sb.WriteString("\n--- 失敗回数が多いスクリプト TOP5 ---\n")
		for i, entry := range summary.TopFailures {
			sb.WriteString(fmt.Sprintf("  %d. %s - %d 回失敗 (最終エラー: %s)\n",
				i+1, entry.ScriptName, entry.RetryCount, entry.ErrorMessage))
		}
	}

	return sb.String()
}

// FormatRetryWaitingDetails returns detailed information about scripts waiting for retry
func (m *EventStatusManager) FormatRetryWaitingDetails() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("=== リトライ待ちスクリプト詳細 ===\n")

	var retryWaitingEntries []EventStatusEntry
	for _, entry := range m.entries {
		if entry.Status == EventStatusRetryWaiting {
			retryWaitingEntries = append(retryWaitingEntries, *entry)
		}
	}

	if len(retryWaitingEntries) == 0 {
		sb.WriteString("リトライ待ちのスクリプトはありません\n")
		return sb.String()
	}

	// Sort by next retry time
	for i := 0; i < len(retryWaitingEntries)-1; i++ {
		for j := i + 1; j < len(retryWaitingEntries); j++ {
			if retryWaitingEntries[i].NextRetryTime.After(retryWaitingEntries[j].NextRetryTime) {
				retryWaitingEntries[i], retryWaitingEntries[j] = retryWaitingEntries[j], retryWaitingEntries[i]
			}
		}
	}

	for _, entry := range retryWaitingEntries {
		timeUntilRetry := entry.GetTimeUntilRetry()
		status := "準備完了"
		if timeUntilRetry > 0 {
			status = fmt.Sprintf("あと %v", timeUntilRetry.Round(time.Second))
		}

		sb.WriteString(fmt.Sprintf("📄 %s (%s)\n", entry.ScriptName, entry.CLI))
		sb.WriteString(fmt.Sprintf("   エラー種別: %s\n", entry.RetryDelayType.String()))
		sb.WriteString(fmt.Sprintf("   リトライ回数: %d/%d\n", entry.RetryCount, m.maxRetries))
		sb.WriteString(fmt.Sprintf("   リトライまで: %s\n", status))
		sb.WriteString(fmt.Sprintf("   最終エラー: %s\n", entry.ErrorMessage))
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatRunningScripts returns information about currently running scripts
func (m *EventStatusManager) FormatRunningScripts() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("=== 実行中スクリプト ===\n")

	var runningEntries []EventStatusEntry
	for _, entry := range m.entries {
		if entry.Status == EventStatusRunning {
			runningEntries = append(runningEntries, *entry)
		}
	}

	if len(runningEntries) == 0 {
		sb.WriteString("実行中のスクリプトはありません\n")
		return sb.String()
	}

	for _, entry := range runningEntries {
		duration := time.Since(entry.StartTime)
		sb.WriteString(fmt.Sprintf("🔄 %s (%s) - %v 実行中\n",
			entry.ScriptName, entry.CLI, duration.Round(time.Second)))
	}

	return sb.String()
}
