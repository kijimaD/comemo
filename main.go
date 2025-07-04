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
	output, err := runGitCommand(repoPath, "log", "--reverse", "--pretty=format:%H", "--first-parent")
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

func main() {
	// 必要なディレクトリを作成
	for _, dir := range []string{promptsDir, outputDir, commitDataDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	allHashes, err := getCommitHashes(goRepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting commit hashes: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d total commits. Checking for missing explanations...\n", len(allHashes))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // 同時実行数を20に制限
	var generatedCount int32

	for _, hash := range allHashes {
		index := getCommitIndex(allHashes, hash)
		if index == 0 {
			continue
		}

		outputFile := filepath.Join(outputDir, fmt.Sprintf("%d.md", index))
		if _, err := os.Stat(outputFile); err == nil {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(h string, idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			commitDataPath, err := prepareCommitData(h, idx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error preparing data for %s: %v\n", h, err)
				return
			}

			if err := generatePromptScript(h, idx, commitDataPath); err != nil {
				fmt.Fprintf(os.Stderr, "Error generating script for %s: %v\n", h, err)
			}
		}(hash, index)
	}

	wg.Wait()

	// スクリプトが実際に生成されたかを確認するためにディレクトリを読む
	files, err := os.ReadDir(promptsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read prompts directory: %v\n", err)
	} else {
		generatedCount = int32(len(files))
	}

	fmt.Println("\n--- Preparation Complete ---")
	// Check against the number of existing md files
	mdFiles, _ := os.ReadDir(outputDir)
	if int(generatedCount) > (len(allHashes) - len(mdFiles)) {
		generatedCount = int32(len(allHashes) - len(mdFiles))
	}

	if generatedCount > 0 {
		fmt.Printf("Generated %d new prompt scripts in '%s/' directory.\n", generatedCount, promptsDir)
		fmt.Println("\nNext Steps:")
		fmt.Printf("1. Open multiple terminals and `cd %s`.\n", promptsDir)
		fmt.Println("2. Run scripts in parallel (e.g., `./1.sh`, `./2.sh`, ...).")
		fmt.Printf("3. Save the Markdown output from Gemini into the '%s/' directory with the correct filename.\n", outputDir)
	} else {
		fmt.Println("All explanations seem to be up-to-date.")
	}
}