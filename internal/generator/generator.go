package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"comemo/internal/config"
	"comemo/internal/git"
	"comemo/internal/logger"
)

// GeneratorOptions provides configuration for generator functions
type GeneratorOptions struct {
	Logger *logger.Logger
}

// GeneratePrompts generates prompt scripts for missing explanations
func GeneratePrompts(cfg *config.Config) error {
	return GeneratePromptsWithOptions(cfg, &GeneratorOptions{
		Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
	})
}

// GeneratePromptsWithOptions generates prompt scripts for missing explanations with configurable output
func GeneratePromptsWithOptions(cfg *config.Config, opts *GeneratorOptions) error {
	if opts == nil {
		opts = &GeneratorOptions{
			Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
		}
	}
	if opts.Logger == nil {
		opts.Logger = logger.New(cfg.LogLevel, os.Stdout, os.Stderr)
	}
	// 必要なディレクトリを作成
	for _, dir := range []string{cfg.PromptsDir, cfg.OutputDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", dir, err)
		}
	}

	opts.Logger.Debug("--- Generating Prompt Scripts ---")

	allHashes, err := git.GetCommitHashes(cfg.GoRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}

	opts.Logger.Debug("Total commits in repository: %d", len(allHashes))

	// 並行処理用のsemaphoreとWaitGroup
	sem := make(chan struct{}, cfg.MaxConcurrency)
	var wg sync.WaitGroup

	for _, hash := range allHashes {
		index := git.GetCommitIndex(allHashes, hash)
		if index == 0 {
			opts.Logger.Warn("Could not find index for hash %s", hash)
			continue
		}

		outputFile := filepath.Join(cfg.OutputDir, fmt.Sprintf("%d.md", index))
		if _, err := os.Stat(outputFile); err == nil {
			continue // 既に説明が存在する場合はスキップ
		}

		// コミットデータが存在するかチェック
		commitDataPath := filepath.Join(cfg.CommitDataDir, fmt.Sprintf("%d.txt", index))
		if _, err := os.Stat(commitDataPath); os.IsNotExist(err) {
			opts.Logger.Warn("Commit data for index %d (%s) not found in %s. Skipping prompt generation.", index, hash, cfg.CommitDataDir)
			continue
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(h string, idx int, cdp string) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := generatePromptScript(cfg, h, idx, cdp); err != nil {
				opts.Logger.Error("Error generating script for %s (index %d): %v", h, idx, err)
			} else {
				opts.Logger.Debug("Generated prompt script for index %d (%s)", idx, h)
			}
		}(hash, index, commitDataPath)
	}
	wg.Wait()
	opts.Logger.Debug("--- Prompt script generation complete ---")
	return nil
}

// generatePromptScript generates a prompt script for a specific commit
func generatePromptScript(cfg *config.Config, hash string, index int, commitDataPath string) error {
	scriptPath := filepath.Join(cfg.PromptsDir, fmt.Sprintf("%d.sh", index))
	githubURL := fmt.Sprintf("https://github.com/golang/go/commit/%s", hash)

	prompt := `これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/%d.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/%d.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: %d
- **コミットハッシュ**: %s
- **GitHub URL**: %s

### 章構成

# [インデックス %d] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[%s](%s)

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
`, index, hash, index, fmt.Sprintf(prompt, index, index, index, hash, githubURL, index, githubURL, githubURL))

	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to write script file: %w", err)
	}

	return nil
}
