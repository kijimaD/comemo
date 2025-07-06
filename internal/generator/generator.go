package generator

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"comemo/internal/config"
	"comemo/internal/git"
)

// GeneratorOptions provides configuration for generator functions
type GeneratorOptions struct {
	Output io.Writer
	Error  io.Writer
}

// GeneratePrompts generates prompt scripts for missing explanations
func GeneratePrompts(cfg *config.Config) error {
	return GeneratePromptsWithOptions(cfg, &GeneratorOptions{
		Output: os.Stdout,
		Error:  os.Stderr,
	})
}

// GeneratePromptsWithOptions generates prompt scripts for missing explanations with configurable output
func GeneratePromptsWithOptions(cfg *config.Config, opts *GeneratorOptions) error {
	if opts == nil {
		opts = &GeneratorOptions{
			Output: os.Stdout,
			Error:  os.Stderr,
		}
	}
	// 必要なディレクトリを作成
	for _, dir := range []string{cfg.PromptsDir, cfg.OutputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", dir, err)
		}
	}

	fmt.Fprintln(opts.Output, "\n--- Generating Prompt Scripts ---")

	allHashes, err := git.GetCommitHashes(cfg.GoRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}

	fmt.Fprintf(opts.Output, "Total commits in repository: %d\n", len(allHashes))

	// 並行処理用のsemaphoreとWaitGroup
	sem := make(chan struct{}, cfg.MaxConcurrency)
	var wg sync.WaitGroup

	for _, hash := range allHashes {
		index := git.GetCommitIndex(allHashes, hash)
		if index == 0 {
			fmt.Fprintf(opts.Error, "Warning: Could not find index for hash %s\n", hash)
			continue
		}

		outputFile := filepath.Join(cfg.OutputDir, fmt.Sprintf("%d.md", index))
		if _, err := os.Stat(outputFile); err == nil {
			continue // 既に説明が存在する場合はスキップ
		}

		// コミットデータが存在するかチェック
		commitDataPath := filepath.Join(cfg.CommitDataDir, fmt.Sprintf("%d.txt", index))
		if _, err := os.Stat(commitDataPath); os.IsNotExist(err) {
			fmt.Fprintf(opts.Error, "Warning: Commit data for index %d (%s) not found in %s. Skipping prompt generation.\n", index, hash, cfg.CommitDataDir)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(h string, idx int, cdp string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := generatePromptScript(cfg, h, idx, cdp); err != nil {
				fmt.Fprintf(opts.Error, "Error generating script for %s (index %d): %v\n", h, idx, err)
			} else {
				fmt.Fprintf(opts.Output, "Generated prompt script for index %d (%s)\n", idx, h)
			}
		}(hash, index, commitDataPath)
	}
	wg.Wait()
	fmt.Fprintln(opts.Output, "\n--- Prompt script generation complete ---")
	return nil
}

// generatePromptScript generates a prompt script for a specific commit
func generatePromptScript(cfg *config.Config, hash string, index int, commitDataPath string) error {
	scriptPath := filepath.Join(cfg.PromptsDir, fmt.Sprintf("%d.sh", index))
	githubURL := fmt.Sprintf("https://github.com/golang/go/commit/%s", hash)

	prompt := `これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、@commit_data/%d.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: %d
- **コミットハッシュ**: %s
- **GitHub URL**: %s

### 章構成

# [インデックス %d] ファイルの概要

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
`, index, hash, index, fmt.Sprintf(prompt, index, index, hash, githubURL, index))

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write script file: %w", err)
	}

	return nil
}
