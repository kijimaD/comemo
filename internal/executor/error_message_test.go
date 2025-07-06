package executor

import (
	"comemo/internal/config"
	"comemo/internal/logger"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestDifferentErrorMessages(t *testing.T) {
	// Create test configuration
	cfg := &config.Config{
		MaxRetries:          3,
		PromptsDir:          "/tmp/test_prompts",
		QuotaRetryDelay:     5 * time.Minute,
		QueueCapacityPerCLI: 3,
		WorkerChannelSize:   10,
		ResultChannelSize:   100,
		RetryDelays: config.RetryDelayConfig{
			QuotaError:   1 * time.Hour,
			QualityError: 10 * time.Second,
			OtherError:   5 * time.Minute,
		},
	}

	// Create test CLI manager
	cliManager := &CLIManager{
		CLIs: map[string]*CLIState{
			"claude": {
				Name:           "claude",
				Available:      true,
				LastQuotaError: time.Time{},
				RecoveryDelay:  0,
			},
			"gemini": {
				Name:           "gemini",
				Available:      true,
				LastQuotaError: time.Time{},
				RecoveryDelay:  0,
			},
		},
		Config: cfg,
	}
	cliManager.mu = sync.RWMutex{}

	// Create status manager
	statusManager := NewStatusManager()
	statusManager.SetTotalScripts(2)
	statusManager.InitializeWorker("claude")
	statusManager.InitializeWorker("gemini")

	// Create scheduler
	scheduler := NewScheduler(cfg, []string{"test1.sh", "test2.sh"}, cliManager, statusManager, logger.Silent())

	t.Run("QuotaErrorVsRegularError", func(t *testing.T) {
		// Queue scripts
		scheduler.queueScript("test1.sh", "claude")
		scheduler.queueScript("test2.sh", "gemini")
		
		// Record script starts
		statusManager.RecordScriptStart("test1.sh", "claude")
		statusManager.RecordScriptStart("test2.sh", "gemini")
		
		// Simulate quota error on claude
		quotaResult := WorkerResult{
			Script:       "test1.sh",
			CLI:          "claude",
			Success:      false,
			IsQuotaError: true,
			Duration:     100 * time.Millisecond,
		}
		
		scheduler.handleWorkerResult(quotaResult)
		
		// Simulate regular error on gemini
		regularResult := WorkerResult{
			Script:      "test2.sh",
			CLI:         "gemini",
			Success:     false,
			IsQuotaError: false,
			IsRetryable: true,
			Error:       errors.New("connection timeout"),
			Duration:    100 * time.Millisecond,
		}
		
		scheduler.handleWorkerResult(regularResult)
		
		// Check status after errors
		status := statusManager.GetStatus()
		
		// Check claude worker (quota error)
		claudeWorker := status.Workers["claude"]
		if claudeWorker.LastFailureReason != "Quota limit reached - waiting for recovery" {
			t.Errorf("Expected claude failure reason to be quota error, got: %s", claudeWorker.LastFailureReason)
		}
		
		// Check gemini worker (regular error)
		geminiWorker := status.Workers["gemini"]
		expectedGeminiError := "Retrying: connection timeout"
		if geminiWorker.LastFailureReason != expectedGeminiError {
			t.Errorf("Expected gemini failure reason to be '%s', got: %s", expectedGeminiError, geminiWorker.LastFailureReason)
		}
		
		// Verify they have different error messages
		if claudeWorker.LastFailureReason == geminiWorker.LastFailureReason {
			t.Error("Claude and Gemini should have different error messages")
		}
		
		t.Logf("Claude error: %s", claudeWorker.LastFailureReason)
		t.Logf("Gemini error: %s", geminiWorker.LastFailureReason)
	})
}

func TestRecordRetryError(t *testing.T) {
	statusManager := NewStatusManager()
	statusManager.InitializeWorker("test-cli")
	
	// Record script start to set processing state
	statusManager.RecordScriptStart("test.sh", "test-cli")
	
	// Get initial status
	initialStatus := statusManager.GetStatus()
	initialProcessing := initialStatus.Queue.Processing
	initialWaiting := initialStatus.Queue.Waiting
	
	// Record retry error
	errorMsg := "test error message"
	statusManager.RecordRetryError("test.sh", "test-cli", 100*time.Millisecond, errorMsg)
	
	// Get status after retry error
	finalStatus := statusManager.GetStatus()
	
	// Check processing count decreased
	if finalStatus.Queue.Processing != initialProcessing-1 {
		t.Errorf("Expected processing to decrease by 1, got %d (was %d)", 
			finalStatus.Queue.Processing, initialProcessing)
	}
	
	// Check waiting count increased
	if finalStatus.Queue.Waiting != initialWaiting+1 {
		t.Errorf("Expected waiting to increase by 1, got %d (was %d)", 
			finalStatus.Queue.Waiting, initialWaiting)
	}
	
	// Check worker status
	worker := finalStatus.Workers["test-cli"]
	expectedReason := "Retrying: " + errorMsg
	if worker.LastFailureReason != expectedReason {
		t.Errorf("Expected LastFailureReason to be '%s', got '%s'", 
			expectedReason, worker.LastFailureReason)
	}
	
	// Check current script is cleared
	if worker.CurrentScript != "" {
		t.Errorf("Expected CurrentScript to be empty, got '%s'", worker.CurrentScript)
	}
}