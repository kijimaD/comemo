package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%d.md", index))
	githubURL := fmt.Sprintf("https://github.com/golang/go/commit/%s", hash)

	// `read_file("...")` という文字列をプロンプトに含めるための正しい方法
	readCmd := fmt.Sprintf("`read_file(\"%s\")`", commitDataPath)

	prompt := `これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、` + readCmd + ` を実行して、コミット情報を取得してください。
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

# Gemini CLIにプロンプトを渡す (実際のCLIコマンド名に要変更)
# ヒアドキュメントを使い、プロンプトを安全に渡す
gemini -p <<'EOF'
%s
EOF

echo -e "\n✅ Done. Copy the output above and save it as: %s"
`, index, hash, index, prompt, outputPath)

	return os.WriteFile(scriptPath, []byte(scriptContent), 0755)
}

// executePrompts は prompts ディレクトリ内のスクリプトを並列実行します。
func executePrompts() {
	files, err := os.ReadDir(promptsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading prompts directory: %v\n", err)
		return
	}

	shFiles := []string{}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") {
			shFiles = append(shFiles, file.Name())
		}
	}

	if len(shFiles) == 0 {
		fmt.Println("No prompt scripts to execute.")
		return
	}

	fmt.Printf("\n--- Executing %d Prompt Scripts ---\n", len(shFiles))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // 同時実行数を20に制限

	for _, fileName := range shFiles {
		wg.Add(1)
		sem <- struct{}{}

		go func(fName string) {
			defer wg.Done()
			defer func() { <-sem }()

			scriptPath := filepath.Join(promptsDir, fName)
			fmt.Printf("Executing script: %s\n", scriptPath)

			// スクリプトを実行し、出力をキャプチャ
			cmd := exec.Command("/bin/bash", scriptPath)
			output, cmdErr := cmd.CombinedOutput() // stdoutとstderrを結合して取得

			// Always check for token/quota related errors in the output
			outputStr := string(output)
			if strings.Contains(outputStr, "quota") || strings.Contains(outputStr, "token") || strings.Contains(outputStr, "rate limit") {
				fmt.Fprintf(os.Stderr, "\n!!! Detected potential token/quota error. Terminating immediately. !!!\n")
				fmt.Fprintf(os.Stderr, "Output:\n%s\n", outputStr) // Print output for debugging
				os.Exit(1) // Terminate the program immediately
			}

			if cmdErr != nil {
				// 実行に失敗した場合 (token/quota エラー以外)
				fmt.Fprintf(os.Stderr, "--- ❌ Error executing script: %s ---\n", scriptPath)
				fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
				fmt.Fprintf(os.Stderr, "Output:\n%s\n", outputStr)
				fmt.Fprintf(os.Stderr, "--- End of Error for %s ---\n", scriptPath)
				return // スクリプトは削除しない
			}

			// 成功した場合、標準出力を表示し、スクリプトを削除
			fmt.Printf("--- ✅ Successfully executed script: %s ---\n", scriptPath)
			fmt.Printf("Output:\n%s\n", outputStr)

			if err := os.Remove(scriptPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to delete script %s: %v\n", scriptPath, err)
			} else {
				fmt.Printf("Deleted script: %s\n", scriptPath)
			}
			fmt.Printf("--- End of Output for %s ---\n", scriptPath)

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
		fmt.Println("\nAll prompt scripts executed successfully and were deleted.\n")
	}
}

// collectCommits は goRepoPath からコミットデータを収集し、commitDataDir に保存します。
func collectCommits() {
	// 必要なディレクトリを作成
	if err := os.MkdirAll(commitDataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", commitDataDir, err)
		os.Exit(1)
	}

	allHashes, err := getCommitHashes(goRepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting commit hashes: %v\n", err)
		os.Exit(1)
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
}

// generatePrompts は収集されたコミットデータに基づいてプロンプトスクリプトを生成します。
func generatePrompts() {
	// 必要なディレクトリを作成
	for _, dir := range []string{promptsDir, outputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	// commit_data ディレクトリからコミットハッシュとインデックスの対応を取得
	// または、再度 getCommitHashes を実行して、commit_data のファイルと突き合わせる
	// ここでは、getCommitHashes を再度実行し、commit_data の存在を確認する方式を採用
	allHashes, err := getCommitHashes(goRepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting commit hashes: %v\n", err)
		os.Exit(1)
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
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <command>")
		fmt.Println("Commands:")
		fmt.Println("  collect   - Collects commit data from the 'go' repository.")
		fmt.Println("  generate  - Generates prompt scripts for missing explanations.")
		fmt.Println("  execute   - Executes generated prompt scripts in parallel.")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "collect":
		collectCommits()
	case "generate":
		generatePrompts()
	case "execute":
		executePrompts()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Println("Usage: go run main.go <command>")
		fmt.Println("Commands:")
		fmt.Println("  collect   - Collects commit data from the 'go' repository.")
		fmt.Println("  generate  - Generates prompt scripts for missing explanations.")
		fmt.Println("  execute   - Executes generated prompt scripts in parallel.")
		os.Exit(1)
	}
}
