package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Config holds application configuration
type Config struct {
	GoRepoPath       string
	PromptsDir       string
	OutputDir        string
	CommitDataDir    string
	MaxConcurrency   int
	ExecutionTimeout time.Duration
	QuotaRetryDelay  time.Duration
	MaxRetries       int
	RetryDelay       time.Duration
}

// QuotaErrors contains patterns that indicate quota limits
type QuotaErrors []string

// CLICommand represents supported AI CLI commands
type CLICommand struct {
	Name    string
	Command string
}

// ScriptRetryInfo tracks retry information for scripts
type ScriptRetryInfo struct {
	FileName    string
	RetryCount  int
	LastAttempt time.Time
	FailReason  string
}

// CLIState manages the state of a CLI execution
type CLIState struct {
	Name           string
	Command        CLICommand
	Available      bool
	LastQuotaError time.Time
	PendingScripts []string
}

// CLIManager manages multiple CLI states
type CLIManager struct {
	CLIs       map[string]*CLIState
	Config     *Config
	RetryQueue chan string
	RetryInfo  map[string]*ScriptRetryInfo
	mu         sync.RWMutex
	retryMu    sync.RWMutex
}

var (
	config = Config{
		GoRepoPath:       "go",
		PromptsDir:       "prompts",
		OutputDir:        "src",
		CommitDataDir:    "commit_data",
		MaxConcurrency:   20,
		ExecutionTimeout: 10 * time.Minute,
		QuotaRetryDelay:  1 * time.Hour,
		MaxRetries:       3,
		RetryDelay:       5 * time.Minute,
	}

	quotaErrors = QuotaErrors{
		"Quota exceeded",
		"quota metric",
		"RESOURCE_EXHAUSTED",
		"rateLimitExceeded",
		"per day per user",
		"Claude AI usage limit reached",
	}

	supportedCLIs = map[string]CLICommand{
		"claude": {"claude", "claude --model sonnet"},
		"gemini": {"gemini", "gemini -m gemini-2.5-flash -p"},
	}
)

// runGitCommand は指定されたディレクトリでgitコマンドを実行します。
func runGitCommand(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git command failed with %w. Stderr: %s", err, stderr.String())
	}
	return stdout.String(), nil
}

// isQuotaError checks if the output contains quota-related error messages
func isQuotaError(output string) bool {
	for _, pattern := range quotaErrors {
		if strings.Contains(output, pattern) {
			return true
		}
	}
	return false
}

// NewCLIManager creates a new CLI manager
func NewCLIManager(config *Config) *CLIManager {
	manager := &CLIManager{
		CLIs:   make(map[string]*CLIState),
		Config: config,
	}

	// Initialize all supported CLIs
	for name, cmd := range supportedCLIs {
		manager.CLIs[name] = &CLIState{
			Name:           name,
			Command:        cmd,
			Available:      true,
			PendingScripts: make([]string, 0),
		}
	}

	return manager
}

// IsAvailable checks if a CLI is currently available (not in quota timeout)
func (cm *CLIManager) IsAvailable(cliName string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	cli, exists := cm.CLIs[cliName]
	if !exists {
		return false
	}

	if !cli.Available {
		// Check if enough time has passed since quota error
		if time.Since(cli.LastQuotaError) >= cm.Config.QuotaRetryDelay {
			cli.Available = true
			fmt.Printf("CLI %s is now available again after quota timeout\n", cliName)
		}
	}

	return cli.Available
}

// MarkQuotaError marks a CLI as unavailable due to quota error
func (cm *CLIManager) MarkQuotaError(cliName string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cli, exists := cm.CLIs[cliName]; exists {
		cli.Available = false
		cli.LastQuotaError = time.Now()
		fmt.Printf("CLI %s marked as unavailable due to quota error. Will retry after %v\n", cliName, cm.Config.QuotaRetryDelay)
	}
}

// GetAvailableCLIs returns list of currently available CLIs
func (cm *CLIManager) GetAvailableCLIs() []string {
	available := make([]string, 0)
	for name := range cm.CLIs {
		if cm.IsAvailable(name) {
			available = append(available, name)
		}
	}
	return available
}

// GetCLICommand returns the command for a CLI
func (cm *CLIManager) GetCLICommand(cliName string) (CLICommand, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cli, exists := cm.CLIs[cliName]; exists {
		return cli.Command, true
	}
	return CLICommand{}, false
}

// handleQuotaError handles quota limit errors with retry logic
func handleQuotaError(scriptPath, output, cliName string, manager *CLIManager) bool {
	fmt.Fprintf(os.Stderr, "\n!!! Quota limit reached for %s !!!\n", cliName)
	fmt.Fprintf(os.Stderr, "Script: %s\n", scriptPath)
	fmt.Fprintf(os.Stderr, "Output:\n%s\n", output)

	manager.MarkQuotaError(cliName)
	return true // Indicate quota error was handled
}

// getCommitHashes はコミットハッシュを古い順に取得します。
func getCommitHashes() ([]string, error) {
	return getCommitHashesFromRepo(config.GoRepoPath)
}

// getCommitHashesFromRepo は指定されたリポジトリからコミットハッシュを取得します。
func getCommitHashesFromRepo(repoPath string) ([]string, error) {
	output, err := runGitCommand(repoPath, "log", "--reverse", "--pretty=format:%H")
	if err != nil {
		return nil, err
	}
	hashes := strings.TrimSpace(string(output))
	if hashes == "" {
		return []string{}, nil
	}
	return strings.Split(hashes, "\n"), nil
}

// getCommitIndex はハッシュリスト内のハッシュの位置（インデックス）を返します。
func getCommitIndex(allHashes []string, targetHash string) int {
	for i, h := range allHashes {
		if h == targetHash {
			return i + 1 // 1-based index
		}
	}
	return 0
}

// prepareCommitData は `git show` の結果をファイルに保存します。
func prepareCommitData(hash string, index int) (string, error) {
	filePath := filepath.Join(config.CommitDataDir, fmt.Sprintf("%d.txt", index))
	commitData, err := runGitCommand(config.GoRepoPath, "show", "--patch-with-stat", hash)
	if err != nil {
		return "", fmt.Errorf("failed to get commit data for %s: %w", hash, err)
	}
	if err := os.WriteFile(filePath, []byte(commitData), 0644); err != nil {
		return "", err
	}
	return filePath, nil
}

// generatePromptScript は解説生成を指示するシェルスクリプトを作成します。
func generatePromptScript(hash string, index int, commitDataPath string) error {
	scriptPath := filepath.Join(config.PromptsDir, fmt.Sprintf("%d.sh", index))
	githubURL := fmt.Sprintf("https://github.com/golang/go/commit/%s", hash)

	readCmd := fmt.Sprintf("@%s", commitDataPath)
	prompt := `これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、` + readCmd + ` を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
`
	prompt += fmt.Sprintf("- **コミットインデックス**: %d\n", index)
	prompt += fmt.Sprintf("- **コミットハッシュ**: %s\n", hash)
	prompt += fmt.Sprintf("- **GitHub URL**: %s\n", githubURL)
	prompt += `
### 章構成
`
	prompt += fmt.Sprintf("\n# [インデックス %d] ファイルの概要\n", index)
	prompt += `
## コミット

## GitHub上でのコミットページへのリンク

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク
`

	scriptContent := fmt.Sprintf(`#!/bin/bash
# Index %d: %s

echo "🚀 Generating explanation for commit %d..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
%s
EOF
`, index, hash, index, prompt)

	return os.WriteFile(scriptPath, []byte(scriptContent), 0755)
}

// executePrompts は prompts ディレクトリ内のスクリプトを並列実行します。
func executePrompts(cliCommand string) error {
	return executePromptsWithManager(cliCommand, NewCLIManager(&config))
}

// executePromptsWithManager は CLIマネージャーを使用してプロンプトを実行します。
func executePromptsWithManager(cliCommand string, manager *CLIManager) error {
	files, err := os.ReadDir(config.PromptsDir)
	if err != nil {
		return fmt.Errorf("error reading prompts directory: %w", err)
	}

	shFiles := []string{}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") {
			shFiles = append(shFiles, file.Name())
		}
	}

	if len(shFiles) == 0 {
		fmt.Println("No prompt scripts to execute.")
		return nil
	}

	// 単一CLIの場合も特定のCLIのみで並列実行
	if cliCommand != "all" {
		return executeWithSpecificCLI(cliCommand, shFiles, manager)
	}

	// すべてのCLIを並列実行
	return executeMultipleCLIs(shFiles, manager)
}

// executeWithSpecificCLI executes scripts with a specific CLI in parallel
func executeWithSpecificCLI(cliCommand string, shFiles []string, manager *CLIManager) error {
	fmt.Printf("\n--- Executing %d Prompt Scripts with %s ---\n", len(shFiles), cliCommand)

	// Create script queue
	scriptQueue := make(chan string, len(shFiles))
	for _, file := range shFiles {
		scriptQueue <- file
	}
	close(scriptQueue)

	var wg sync.WaitGroup

	// Start worker for specific CLI
	wg.Add(1)
	go func() {
		defer wg.Done()
		cliWorker(cliCommand, scriptQueue, manager)
	}()

	wg.Wait()

	// 最終確認
	remainingFiles, _ := os.ReadDir(config.PromptsDir)
	if len(remainingFiles) > 0 {
		fmt.Printf("\n%d scripts failed to execute and remain in the '%s' directory.\n", len(remainingFiles), config.PromptsDir)
		fmt.Println("Please check the error messages above, fix the issues, and run the program again.")
	} else {
		fmt.Println("\nAll prompt scripts executed successfully and were deleted.")
	}

	return nil
}

// executeMultipleCLIs executes scripts with multiple CLIs in parallel
func executeMultipleCLIs(shFiles []string, manager *CLIManager) error {
	fmt.Printf("\n--- Executing %d Prompt Scripts with multiple CLIs in parallel ---\n", len(shFiles))

	// Create script queue
	scriptQueue := make(chan string, len(shFiles))
	for _, file := range shFiles {
		scriptQueue <- file
	}
	close(scriptQueue)

	var wg sync.WaitGroup

	// Start workers for each CLI
	for cliName := range supportedCLIs {
		wg.Add(1)
		go func(cliName string) {
			defer wg.Done()
			cliWorker(cliName, scriptQueue, manager)
		}(cliName)
	}

	// Monitor and retry logic
	go quotaMonitor(manager, scriptQueue)

	wg.Wait()

	// 最終確認
	remainingFiles, _ := os.ReadDir(config.PromptsDir)
	if len(remainingFiles) > 0 {
		fmt.Printf("\n%d scripts failed to execute and remain in the '%s' directory.\n", len(remainingFiles), config.PromptsDir)
		fmt.Println("Please check the error messages above, fix the issues, and run the program again.")
	} else {
		fmt.Println("\nAll prompt scripts executed successfully and were deleted.")
	}

	return nil
}

// cliWorker processes scripts for a specific CLI
func cliWorker(cliName string, scriptQueue <-chan string, manager *CLIManager) {
	pendingScripts := make(map[string]bool) // Use map to prevent duplicates
	lastUnavailableLogTime := time.Time{}
	unavailableLogInterval := 30 * time.Second
	wasUnavailable := false

	for {
		select {
		case fileName, ok := <-scriptQueue:
			if !ok {
				// Queue is closed, process any pending scripts
				if len(pendingScripts) > 0 && manager.IsAvailable(cliName) {
					processPendingScripts(pendingScripts, cliName, manager)
				}
				return
			}

			// Check if CLI is available
			if !manager.IsAvailable(cliName) {
				// Add to pending scripts map (prevents duplicates)
				if !pendingScripts[fileName] {
					pendingScripts[fileName] = true

					// Log unavailability message only periodically to reduce spam
					now := time.Now()
					if now.Sub(lastUnavailableLogTime) > unavailableLogInterval {
						fmt.Printf("CLI %s is not available, queuing %d scripts for retry\n", cliName, len(pendingScripts))
						lastUnavailableLogTime = now
					}
				}
				wasUnavailable = true
				continue
			}

			// Log when CLI becomes available again after being unavailable
			if wasUnavailable && len(pendingScripts) > 0 {
				fmt.Printf("CLI %s is now available, processing %d pending scripts\n", cliName, len(pendingScripts))
				wasUnavailable = false
			}

			cli, exists := manager.GetCLICommand(cliName)
			if !exists {
				fmt.Printf("CLI %s not found\n", cliName)
				continue
			}

			// Try to process the script
			success := processScriptWithRetry(fileName, cli, cliName, manager)
			if !success {
				// If processing failed (quota error), add to pending
				if !pendingScripts[fileName] {
					pendingScripts[fileName] = true
					fmt.Printf("Script %s failed, added to pending queue\n", fileName)
				}
			} else {
				// Remove from pending if it was there
				delete(pendingScripts, fileName)
			}

		case <-time.After(30 * time.Second):
			// Periodically check for pending scripts and CLI availability
			if len(pendingScripts) > 0 && manager.IsAvailable(cliName) {
				fmt.Printf("CLI %s is now available, processing %d pending scripts\n", cliName, len(pendingScripts))
				processPendingScripts(pendingScripts, cliName, manager)
			} else if len(pendingScripts) > 0 {
				fmt.Printf("CLI %s still not available, %d scripts pending\n", cliName, len(pendingScripts))
			}
		}
	}
}

// processPendingScripts processes all pending scripts and removes successful ones
func processPendingScripts(pendingScripts map[string]bool, cliName string, manager *CLIManager) {
	cli, exists := manager.GetCLICommand(cliName)
	if !exists {
		return
	}

	// Process pending scripts
	for fileName := range pendingScripts {
		success := processScriptWithRetry(fileName, cli, cliName, manager)
		if success {
			// Remove successful scripts from pending
			delete(pendingScripts, fileName)
			fmt.Printf("Successfully processed pending script: %s\n", fileName)
		} else {
			fmt.Printf("Pending script %s failed again, keeping in queue\n", fileName)
			// Keep failed scripts in pending for next retry cycle
		}
	}
}

// processScriptWithRetry wraps processScript and returns success status
func processScriptWithRetry(fileName string, cli CLICommand, cliName string, manager *CLIManager) bool {
	// Store original availability state
	originalAvailable := manager.IsAvailable(cliName)

	// Store paths
	scriptPath := filepath.Join(config.PromptsDir, fileName)
	baseName := strings.TrimSuffix(fileName, ".sh")
	outputPath := filepath.Join(config.OutputDir, baseName+".md")

	// Check if files exist before processing
	scriptExistsBefore := true
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptExistsBefore = false
	}

	processScript(fileName, cli, cliName, manager)

	// Check if CLI became unavailable (indicating quota error)
	newAvailable := manager.IsAvailable(cliName)
	if originalAvailable && !newAvailable {
		return false // Quota error occurred
	}

	// Check if script file still exists (successful scripts are deleted)
	scriptExistsAfter := true
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		scriptExistsAfter = false
	}

	// Check if output file was created and is valid
	outputExists := false
	if _, err := os.Stat(outputPath); err == nil {
		outputExists = true
	}

	// Success criteria: script was deleted AND valid output was created
	if scriptExistsBefore && !scriptExistsAfter && outputExists {
		return true // Complete success
	}

	// If script still exists but output was created (quality check failed)
	if scriptExistsBefore && scriptExistsAfter && outputExists {
		fmt.Printf("Quality check failed for %s, output file removed, script kept for retry\n", fileName)
		return false // Quality check failure
	}

	return false // Other failure cases
}

// quotaMonitor monitors quota status and provides status updates
func quotaMonitor(manager *CLIManager, scriptQueue <-chan string) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			availableCLIs := manager.GetAvailableCLIs()
			fmt.Printf("[Monitor] Available CLIs: %v\n", availableCLIs)

			// Check quota status for each CLI
			for cliName, cli := range manager.CLIs {
				if !cli.Available {
					timeRemaining := manager.Config.QuotaRetryDelay - time.Since(cli.LastQuotaError)
					if timeRemaining > 0 {
						fmt.Printf("[Monitor] CLI %s: quota limit, %v remaining until retry\n", cliName, timeRemaining.Round(time.Minute))
					} else {
						fmt.Printf("[Monitor] CLI %s: quota limit expired, should be available\n", cliName)
					}
				}
			}

			// Check for remaining scripts
			files, err := os.ReadDir(config.PromptsDir)
			if err == nil {
				remainingScripts := 0
				for _, file := range files {
					if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") {
						remainingScripts++
					}
				}
				if remainingScripts > 0 {
					fmt.Printf("[Monitor] Remaining scripts: %d\n", remainingScripts)
				}
			}
		}
	}
}

// processScript processes a single script with the given CLI
func processScript(fileName string, cli CLICommand, cliName string, manager *CLIManager) {
	scriptPath := filepath.Join(config.PromptsDir, fileName)

	// ファイル名から出力ファイルのパスを決定
	baseName := strings.TrimSuffix(fileName, ".sh")
	outputPath := filepath.Join(config.OutputDir, baseName+".md")

	// スクリプト内のプレースホルダーを置換
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading script %s: %v\n", scriptPath, err)
		return
	}

	// プレースホルダーを実際のCLIコマンドに置換（メモリ上でのみ）
	modifiedContent := strings.ReplaceAll(string(content), "{{AI_CLI_COMMAND}}", cli.Command)

	fmt.Printf("Executing script %s with %s\n", scriptPath, cliName)

	// 修正されたスクリプトをstdinから実行（タイムアウト付き）
	ctx, cancel := context.WithTimeout(context.Background(), config.ExecutionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/bash")
	cmd.Stdin = strings.NewReader(modifiedContent)
	output, cmdErr := cmd.CombinedOutput()

	outputStr := string(output)

	// タイムアウトエラーをチェック
	if ctx.Err() == context.DeadlineExceeded {
		fmt.Fprintf(os.Stderr, "\n!!! Script execution timed out after %v. Terminating. !!!\n", config.ExecutionTimeout)
		fmt.Fprintf(os.Stderr, "Script: %s\n", scriptPath)
		return
	}

	// クォータ制限に達した場合の処理
	if isQuotaError(outputStr) {
		handleQuotaError(scriptPath, outputStr, cliName, manager)
		return
	}

	if cmdErr != nil {
		// 実行に失敗した場合 (token/quota エラー以外)
		fmt.Fprintf(os.Stderr, "--- ❌ Error executing script: %s ---\n", scriptPath)
		fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
		fmt.Fprintf(os.Stderr, "Output:\n%s\n", outputStr)
		fmt.Fprintf(os.Stderr, "--- End of Error for %s ---\n", scriptPath)
		return
	}

	// AI の出力部分のみを抽出
	lines := strings.Split(outputStr, "\n")
	var aiOutput []string
	capturing := false
	foundValidContent := false

	for _, line := range lines {
		// AI の実際の出力開始を検出
		if strings.Contains(line, "# [インデックス") {
			capturing = true
			foundValidContent = true
		}

		// "✅ Done" メッセージが出たら終了
		if strings.Contains(line, "✅ Done") {
			break
		}

		if capturing {
			aiOutput = append(aiOutput, line)
		}
	}

	// 出力の品質をチェック
	aiOutputContent := strings.Join(aiOutput, "\n")
	outputValid := foundValidContent &&
		len(strings.TrimSpace(aiOutputContent)) > 100 &&
		strings.Contains(aiOutputContent, "## コミット") &&
		!strings.Contains(outputStr, "GaxiosError") &&
		!strings.Contains(outputStr, "API Error")

	if outputValid {
		// 成功した場合のみファイルに保存
		if err := os.WriteFile(outputPath, []byte(aiOutputContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving output to %s: %v\n", outputPath, err)
			return
		}
		fmt.Printf("--- ✅ Successfully executed script: %s with %s ---\n", scriptPath, cliName)
		fmt.Printf("Saved output to: %s\n", outputPath)

		// 成功時のみスクリプトを削除
		if err := os.Remove(scriptPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete script %s: %v\n", scriptPath, err)
		} else {
			fmt.Printf("Deleted script: %s\n", scriptPath)
		}
	} else {
		// 出力が不完全または無効な場合
		fmt.Fprintf(os.Stderr, "--- ⚠️ Script executed but output is incomplete or invalid: %s ---\n", scriptPath)
		fmt.Fprintf(os.Stderr, "Output length: %d characters\n", len(aiOutputContent))
		fmt.Fprintf(os.Stderr, "Found valid content: %v\n", foundValidContent)

		// 不完全な出力ファイルが既に存在する場合は削除
		if _, err := os.Stat(outputPath); err == nil {
			if removeErr := os.Remove(outputPath); removeErr != nil {
				fmt.Fprintf(os.Stderr, "Failed to remove incomplete output file %s: %v\n", outputPath, removeErr)
			} else {
				fmt.Printf("Removed incomplete output file: %s\n", outputPath)
			}
		}

		if len(outputStr) > 500 {
			fmt.Fprintf(os.Stderr, "Output preview:\n%s...\n", outputStr[:500])
		} else {
			fmt.Fprintf(os.Stderr, "Full output:\n%s\n", outputStr)
		}

		// スクリプトは削除せずにリトライ用に保持
		fmt.Printf("Script %s kept for retry\n", scriptPath)
	}
}

// collectCommits は Go リポジトリからコミットデータを収集します。
func collectCommits() error {
	// 必要なディレクトリを作成
	if err := os.MkdirAll(config.CommitDataDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", config.CommitDataDir, err)
	}

	allHashes, err := getCommitHashes()
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}

	fmt.Printf("Found %d total commits. Collecting commit data...\n", len(allHashes))

	var wg sync.WaitGroup
	sem := make(chan struct{}, config.MaxConcurrency)

	for _, hash := range allHashes {
		index := getCommitIndex(allHashes, hash)
		if index == 0 {
			continue
		}

		// commit_data にファイルが既に存在する場合はスキップ
		commitDataFile := filepath.Join(config.CommitDataDir, fmt.Sprintf("%d.txt", index))
		if _, err := os.Stat(commitDataFile); err == nil {
			// fmt.Printf("Commit data for index %d already exists. Skipping.\n", index)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(h string, idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			_, err := prepareCommitData(h, idx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing data for %s (index %d): %v\n", h, idx, err)
			} else {
				fmt.Printf("Collected commit data for index %d (%s)\n", idx, h)
			}
		}(hash, index)
	}
	wg.Wait()
	fmt.Println("\n--- Commit data collection complete ---")
	return nil
}

// generatePrompts は収集されたコミットデータに基づいてプロンプトスクリプトを生成します。
func generatePrompts() error {
	// 必要なディレクトリを作成
	for _, dir := range []string{config.PromptsDir, config.OutputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", dir, err)
		}
	}

	// commit_data ディレクトリからコミットハッシュとインデックスの対応を取得
	// または、再度 getCommitHashes を実行して、commit_data のファイルと突き合わせる
	// ここでは、getCommitHashes を再度実行し、commit_data の存在を確認する方式を採用
	allHashes, err := getCommitHashes()
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}

	fmt.Printf("Found %d total commits. Generating prompt scripts for missing explanations...\n", len(allHashes))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // 同時実行数を20に制限

	for _, hash := range allHashes {
		index := getCommitIndex(allHashes, hash)
		if index == 0 {
			continue
		}

		outputFile := filepath.Join(config.OutputDir, fmt.Sprintf("%d.md", index))
		if _, err := os.Stat(outputFile); err == nil {
			// fmt.Printf("Explanation for index %d already exists. Skipping prompt generation.\n", index)
			continue // 既に解説ファイルが存在する場合はスキップ
		}

		// commit_data が存在しない場合はスキップ
		commitDataPath := filepath.Join(config.CommitDataDir, fmt.Sprintf("%d.txt", index))
		if _, err := os.Stat(commitDataPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: Commit data for index %d (%s) not found in %s. Skipping prompt generation.\n", index, hash, config.CommitDataDir)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(h string, idx int, cdp string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := generatePromptScript(h, idx, cdp); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating script for %s (index %d): %v\n", h, idx, err)
			} else {
				fmt.Printf("Generated prompt script for index %d (%s)\n", idx, h)
			}
		}(hash, index, commitDataPath)
	}
	wg.Wait()
	fmt.Println("\n--- Prompt script generation complete ---")
	return nil
}

// verify は生成されたファイル数とコミット数の一致を検証します。
func verify() error {
	fmt.Println("--- Verification Started ---")

	// 1. コミット数を取得
	allHashes, err := getCommitHashes()
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}
	commitCount := len(allHashes)
	fmt.Printf("Total commits: %d\n", commitCount)

	// 2. commit_dataディレクトリ内のファイル数を取得
	commitDataFiles, err := os.ReadDir(config.CommitDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("commit_data directory does not exist: %s\n", config.CommitDataDir)
			commitDataFiles = []os.DirEntry{}
		} else {
			return fmt.Errorf("error reading commit_data directory: %w", err)
		}
	}

	commitDataCount := 0
	for _, file := range commitDataFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".txt") {
			commitDataCount++
		}
	}
	fmt.Printf("commit_data files: %d\n", commitDataCount)

	// 3. promptsディレクトリ内のファイル数を取得
	promptFiles, err := os.ReadDir(config.PromptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("prompts directory does not exist: %s\n", config.PromptsDir)
			promptFiles = []os.DirEntry{}
		} else {
			return fmt.Errorf("error reading prompts directory: %w", err)
		}
	}

	promptCount := 0
	for _, file := range promptFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") {
			promptCount++
		}
	}
	fmt.Printf("prompt scripts: %d\n", promptCount)

	// 4. srcディレクトリ内の説明ファイル数を取得
	outputFiles, err := os.ReadDir(config.OutputDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("src directory does not exist: %s\n", config.OutputDir)
			outputFiles = []os.DirEntry{}
		} else {
			return fmt.Errorf("error reading src directory: %w", err)
		}
	}

	outputCount := 0
	for _, file := range outputFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".md") && file.Name() != "SUMMARY.md" {
			outputCount++
		}
	}
	fmt.Printf("explanation files: %d\n", outputCount)

	// 5. 検証結果の表示
	fmt.Println("\n--- Verification Results ---")

	if commitDataCount != commitCount {
		fmt.Printf("❌ Mismatch: commit_data files (%d) != total commits (%d)\n", commitDataCount, commitCount)
		missing := commitCount - commitDataCount
		if missing > 0 {
			fmt.Printf("   Missing %d commit data files. Run 'collect' command.\n", missing)
		} else {
			fmt.Printf("   Extra %d commit data files found.\n", -missing)
		}
	} else {
		fmt.Printf("✅ commit_data files match total commits (%d)\n", commitCount)
	}

	expectedPrompts := commitCount - promptCount
	if promptCount > 0 {
		fmt.Printf("✅ Found %d prompt scripts\n", promptCount)
		if expectedPrompts > 0 {
			fmt.Printf("   %d prompts may have been executed already\n", expectedPrompts)
		}
	} else if commitDataCount > 0 {
		fmt.Printf("⚠️  No prompt scripts found. Run 'generate' command to create them.\n")
	}

	if outputCount > 0 {
		fmt.Printf("✅ Found %d explanation files\n", outputCount)
		remaining := commitCount - outputCount
		if remaining > 0 {
			fmt.Printf("   %d explanations remaining to be generated\n", remaining)
		}
	} else if commitCount > 0 {
		fmt.Printf("⚠️  No explanation files found. Run 'execute' command after generating prompts.\n")
	}

	// 6. 進捗サマリー
	fmt.Println("\n--- Progress Summary ---")
	if commitCount == 0 {
		fmt.Println("⚠️  No commits found in the repository")
	} else {
		collectProgress := float64(commitDataCount) / float64(commitCount) * 100
		generateProgress := float64(outputCount) / float64(commitCount) * 100

		fmt.Printf("Data Collection: %.1f%% (%d/%d)\n", collectProgress, commitDataCount, commitCount)
		fmt.Printf("Explanation Generation: %.1f%% (%d/%d)\n", generateProgress, outputCount, commitCount)

		if collectProgress == 100 && generateProgress == 100 {
			fmt.Println("🎉 All commits have been processed!")
		} else if collectProgress == 100 {
			fmt.Println("📝 Ready for explanation generation")
		} else {
			fmt.Println("📥 Need to collect more commit data")
		}
	}

	fmt.Println("--- Verification Complete ---")
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  collect                 - Collects commit data from the 'go' repository.")
		fmt.Println("  generate                - Generates prompt scripts for missing explanations.")
		fmt.Println("  execute [--cli=CMD]     - Executes generated prompt scripts in parallel.")
		fmt.Println("  verify                  - Verifies the consistency of generated files.")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --cli=CMD               - AI CLI command to use (default: claude)")
		fmt.Println("                            Supported: claude, gemini, all")
		fmt.Println("                            Use 'all' to run both CLIs in parallel")
		fmt.Println("                            Only available with execute command")
		os.Exit(1)
	}

	command := os.Args[1]

	var err error
	switch command {
	case "collect":
		err = collectCommits()
	case "generate":
		err = generatePrompts()
	case "execute":
		// executeコマンドの場合のみCLIオプションをパース
		cliCommand := "claude"
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			if strings.HasPrefix(arg, "--cli=") {
				cliCommand = strings.TrimPrefix(arg, "--cli=")
			}
		}

		// サポートされているCLIかチェック (allは特別扱い)
		if cliCommand != "all" {
			_, exists := supportedCLIs[cliCommand]
			if !exists {
				fmt.Fprintf(os.Stderr, "Error: Unsupported CLI command '%s'. Supported: claude, gemini, all\n", cliCommand)
				os.Exit(1)
			}
		}

		err = executePrompts(cliCommand)
	case "verify":
		err = verify()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Usage: go run main.go <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  collect                 - Collects commit data from the 'go' repository.")
		fmt.Println("  generate                - Generates prompt scripts for missing explanations.")
		fmt.Println("  execute [--cli=CMD]     - Executes generated prompt scripts in parallel.")
		fmt.Println("  verify                  - Verifies the consistency of generated files.")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --cli=CMD               - AI CLI command to use (default: claude)")
		fmt.Println("                            Supported: claude, gemini, all")
		fmt.Println("                            Use 'all' to run both CLIs in parallel")
		fmt.Println("                            Only available with execute command")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
