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

const (
	goRepoPath    = "go"
	promptsDir    = "prompts"
	outputDir     = "src"
	commitDataDir = "commit_data" // git showの結果を保存する一時ディレクトリ
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

// getCommitHashes はコミットハッシュを古い順に取得します。
func getCommitHashes(repoPath string) ([]string, error) {
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
	filePath := filepath.Join(commitDataDir, fmt.Sprintf("%d.txt", index))
	commitData, err := runGitCommand(goRepoPath, "show", "--stat", hash) // --statを追加して統計情報も表示
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
	scriptPath := filepath.Join(promptsDir, fmt.Sprintf("%d.sh", index))
	githubURL := fmt.Sprintf("https://github.com/golang/go/commit/%s", hash)

	// 絶対パスを生成
	absCommitDataPath, err := filepath.Abs(commitDataPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", commitDataPath, err)
	}

	// `read_file("...")` という文字列をプロンプトに含めるための正しい方法
	readCmd := fmt.Sprintf("`read_file(\"%s\")`", absCommitDataPath)

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
	files, err := os.ReadDir(promptsDir)
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

	// CLIコマンドの決定
	var cliCommandLine string
	switch cliCommand {
	case "gemini":
		cliCommandLine = "gemini -m gemini-2.5-flash -p"
	case "claude":
		cliCommandLine = "claude"
	case "claude-haiku":
		cliCommandLine = "claude --model claude-3-haiku-20240307"
	case "claude-sonnet":
		cliCommandLine = "claude --model claude-3-5-sonnet-20241022"
	default:
		cliCommandLine = cliCommand
	}

	fmt.Printf("\n--- Executing %d Prompt Scripts with %s ---\n", len(shFiles), cliCommand)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 1) // 同時実行数を1に制限

	for _, fileName := range shFiles {
		wg.Add(1)
		sem <- struct{}{}

		go func(fName string) {
			defer wg.Done()
			defer func() { <-sem }()

			scriptPath := filepath.Join(promptsDir, fName)

			// ファイル名から出力ファイルのパスを決定
			baseName := strings.TrimSuffix(fName, ".sh")
			outputPath := filepath.Join(outputDir, baseName+".md")

			// スクリプト内のプレースホルダーを置換
			content, err := os.ReadFile(scriptPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading script %s: %v\n", scriptPath, err)
				return
			}

			// プレースホルダーを実際のCLIコマンドに置換（メモリ上でのみ）
			modifiedContent := strings.ReplaceAll(string(content), "{{AI_CLI_COMMAND}}", cliCommandLine)

			fmt.Printf("Executing script: %s\n", scriptPath)

			// 修正されたスクリプトをstdinから実行（タイムアウト付き）
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			cmd := exec.CommandContext(ctx, "/bin/bash")
			cmd.Stdin = strings.NewReader(modifiedContent)
			output, cmdErr := cmd.CombinedOutput() // stdoutとstderrを結合して取得

			// Always check for token/quota related errors in the output
			outputStr := string(output)

			// タイムアウトエラーをチェック
			if ctx.Err() == context.DeadlineExceeded {
				fmt.Fprintf(os.Stderr, "\n!!! Script execution timed out after 10 minutes. Terminating. !!!\n")
				fmt.Fprintf(os.Stderr, "Script: %s\n", scriptPath)
				return
			}

			// 一日のクォータ制限に達した場合はプログラム全体を終了
			if strings.Contains(outputStr, "Quota exceeded") ||
				strings.Contains(outputStr, "quota metric") ||
				strings.Contains(outputStr, "RESOURCE_EXHAUSTED") ||
				strings.Contains(outputStr, "rateLimitExceeded") ||
				strings.Contains(outputStr, "per day per user") {
				fmt.Fprintf(os.Stderr, "\n!!! Daily quota limit reached. Terminating program. !!!\n")
				fmt.Fprintf(os.Stderr, "Script: %s\n", scriptPath)
				fmt.Fprintf(os.Stderr, "Please try again tomorrow or switch to a different API.\n")
				fmt.Fprintf(os.Stderr, "Output:\n%s\n", outputStr) // Print output for debugging
				os.Exit(1)                                         // プログラム全体を終了
			}

			// その他のAPI エラーパターンをチェック
			if strings.Contains(outputStr, "quota") ||
				strings.Contains(outputStr, "token") ||
				strings.Contains(outputStr, "rate limit") ||
				strings.Contains(outputStr, "URL_RETRIEVAL_STATUS_ERROR") ||
				strings.Contains(outputStr, "unable to access the content") {
				fmt.Fprintf(os.Stderr, "\n!!! Detected potential API error. Script may have failed. !!!\n")
				fmt.Fprintf(os.Stderr, "Script: %s\n", scriptPath)
				fmt.Fprintf(os.Stderr, "Output:\n%s\n", outputStr) // Print output for debugging
				return                                             // スクリプトを削除せずに終了
			}

			if cmdErr != nil {
				// 実行に失敗した場合 (token/quota エラー以外)
				fmt.Fprintf(os.Stderr, "--- ❌ Error executing script: %s ---\n", scriptPath)
				fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
				fmt.Fprintf(os.Stderr, "Output:\n%s\n", outputStr)
				fmt.Fprintf(os.Stderr, "--- End of Error for %s ---\n", scriptPath)
				return // スクリプトは削除しない
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
				len(strings.TrimSpace(aiOutputContent)) > 100 && // 最小文字数
				strings.Contains(aiOutputContent, "## コミット") && // 必須セクションの存在
				!strings.Contains(outputStr, "GaxiosError") && // エラー出力がない
				!strings.Contains(outputStr, "API Error") // APIエラーがない

			if outputValid {
				// 成功した場合のみファイルに保存
				if err := os.WriteFile(outputPath, []byte(aiOutputContent), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Error saving output to %s: %v\n", outputPath, err)
					return
				}
				fmt.Printf("--- ✅ Successfully executed script: %s ---\n", scriptPath)
				fmt.Printf("Saved output to: %s\n", outputPath)
			} else {
				// 出力が不完全または無効な場合
				fmt.Fprintf(os.Stderr, "--- ⚠️ Script executed but output is incomplete or invalid: %s ---\n", scriptPath)
				fmt.Fprintf(os.Stderr, "Output length: %d characters\n", len(aiOutputContent))
				fmt.Fprintf(os.Stderr, "Found valid content: %v\n", foundValidContent)
				if len(outputStr) > 500 {
					fmt.Fprintf(os.Stderr, "Output preview:\n%s...\n", outputStr[:500])
				} else {
					fmt.Fprintf(os.Stderr, "Full output:\n%s\n", outputStr)
				}
				return // スクリプトを削除せずに終了
			}

			if err := os.Remove(scriptPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to delete script %s: %v\n", scriptPath, err)
			} else {
				fmt.Printf("Deleted script: %s\n", scriptPath)
			}

		}(fileName)
	}

	wg.Wait()
	fmt.Println("\n--- All script executions complete ---")

	// 最終確認
	remainingFiles, _ := os.ReadDir(promptsDir)
	if len(remainingFiles) > 0 {
		fmt.Printf("\n%d scripts failed to execute and remain in the '%s' directory.\n", len(remainingFiles), promptsDir)
		fmt.Println("Please check the error messages above, fix the issues, and run the program again.")
	} else {
		fmt.Println("\nAll prompt scripts executed successfully and were deleted.")
	}
	return nil
}

// collectCommits は goRepoPath からコミットデータを収集し、commitDataDir に保存します。
func collectCommits() error {
	// 必要なディレクトリを作成
	if err := os.MkdirAll(commitDataDir, 0755); err != nil {
		return fmt.Errorf("error creating directory %s: %w", commitDataDir, err)
	}

	allHashes, err := getCommitHashes(goRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}

	fmt.Printf("Found %d total commits. Collecting commit data...\n", len(allHashes))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // 同時実行数を20に制限

	for _, hash := range allHashes {
		index := getCommitIndex(allHashes, hash)
		if index == 0 {
			continue
		}

		// commit_data にファイルが既に存在する場合はスキップ
		commitDataFile := filepath.Join(commitDataDir, fmt.Sprintf("%d.txt", index))
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
	for _, dir := range []string{promptsDir, outputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", dir, err)
		}
	}

	// commit_data ディレクトリからコミットハッシュとインデックスの対応を取得
	// または、再度 getCommitHashes を実行して、commit_data のファイルと突き合わせる
	// ここでは、getCommitHashes を再度実行し、commit_data の存在を確認する方式を採用
	allHashes, err := getCommitHashes(goRepoPath)
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

		outputFile := filepath.Join(outputDir, fmt.Sprintf("%d.md", index))
		if _, err := os.Stat(outputFile); err == nil {
			// fmt.Printf("Explanation for index %d already exists. Skipping prompt generation.\n", index)
			continue // 既に解説ファイルが存在する場合はスキップ
		}

		// commit_data が存在しない場合はスキップ
		commitDataPath := filepath.Join(commitDataDir, fmt.Sprintf("%d.txt", index))
		if _, err := os.Stat(commitDataPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: Commit data for index %d (%s) not found in %s. Skipping prompt generation.\n", index, hash, commitDataDir)
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
	allHashes, err := getCommitHashes(goRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}
	commitCount := len(allHashes)
	fmt.Printf("Total commits: %d\n", commitCount)

	// 2. commit_dataディレクトリ内のファイル数を取得
	commitDataFiles, err := os.ReadDir(commitDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("commit_data directory does not exist: %s\n", commitDataDir)
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
	promptFiles, err := os.ReadDir(promptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("prompts directory does not exist: %s\n", promptsDir)
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
	outputFiles, err := os.ReadDir(outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("src directory does not exist: %s\n", outputDir)
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
		fmt.Println("                            Supported: claude, claude-haiku, claude-sonnet, gemini")
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

		// サポートされているCLIかチェック
		supportedCLIs := map[string]bool{
			"claude":        true,
			"claude-haiku":  true,
			"claude-sonnet": true,
			"gemini":        true,
		}
		if !supportedCLIs[cliCommand] {
			fmt.Fprintf(os.Stderr, "Error: Unsupported CLI command '%s'. Supported: claude, claude-haiku, claude-sonnet, gemini\n", cliCommand)
			os.Exit(1)
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
		fmt.Println("                            Supported: claude, claude-haiku, claude-sonnet, gemini")
		fmt.Println("                            Only available with execute command")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
