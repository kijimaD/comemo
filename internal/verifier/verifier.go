package verifier

import (
	"fmt"
	"os"
	"strings"

	"comemo/internal/config"
	"comemo/internal/git"
	"comemo/internal/logger"
)

// VerifierOptions provides configuration for verifier functions
type VerifierOptions struct {
	Logger *logger.Logger
}

// Verify checks the consistency of generated files
func Verify(cfg *config.Config) error {
	return VerifyWithOptions(cfg, &VerifierOptions{
		Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
	})
}

// VerifyWithOptions checks the consistency of generated files with configurable output
func VerifyWithOptions(cfg *config.Config, opts *VerifierOptions) error {
	if opts == nil {
		opts = &VerifierOptions{
			Logger: logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
		}
	}
	if opts.Logger == nil {
		opts.Logger = logger.New(cfg.LogLevel, os.Stdout, os.Stderr)
	}

	opts.Logger.Debug("--- Verification Started ---")

	// 1. コミット数を取得
	allHashes, err := git.GetCommitHashes(cfg.GoRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}
	commitCount := len(allHashes)
	opts.Logger.Debug("Total commits: %d", commitCount)

	// 2. commit_dataディレクトリ内のファイル数を取得
	commitDataFiles, err := os.ReadDir(cfg.CommitDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			opts.Logger.Debug("commit_data directory does not exist: %s", cfg.CommitDataDir)
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
	opts.Logger.Debug("commit_data files: %d", commitDataCount)

	// 3. promptsディレクトリ内のファイル数を取得
	promptFiles, err := os.ReadDir(cfg.PromptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			opts.Logger.Debug("prompts directory does not exist: %s", cfg.PromptsDir)
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
	opts.Logger.Debug("prompt scripts: %d", promptCount)

	// 4. srcディレクトリ内の説明ファイル数を取得
	outputFiles, err := os.ReadDir(cfg.OutputDir)
	if err != nil {
		if os.IsNotExist(err) {
			opts.Logger.Debug("src directory does not exist: %s", cfg.OutputDir)
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
	opts.Logger.Debug("explanation files: %d", outputCount)

	// 5. 検証結果の表示
	opts.Logger.Debug("--- Verification Results ---")

	if commitDataCount != commitCount {
		opts.Logger.Debug("❌ Mismatch: commit_data files (%d) != total commits (%d)", commitDataCount, commitCount)
		missing := commitCount - commitDataCount
		if missing > 0 {
			opts.Logger.Debug("   Missing %d commit data files. Run 'collect' command.", missing)
		} else {
			opts.Logger.Debug("   Extra %d commit data files found.", -missing)
		}
	} else {
		opts.Logger.Debug("✅ commit_data files match total commits (%d)", commitCount)
	}

	expectedPrompts := commitCount - promptCount
	if promptCount > 0 {
		opts.Logger.Debug("✅ Found %d prompt scripts", promptCount)
		if expectedPrompts > 0 {
			opts.Logger.Debug("   %d prompts may have been executed already", expectedPrompts)
		}
	} else if commitDataCount > 0 {
		opts.Logger.Debug("⚠️  No prompt scripts found. Run 'generate' command to create them.")
	}

	if outputCount > 0 {
		opts.Logger.Debug("✅ Found %d explanation files", outputCount)
		remaining := commitCount - outputCount
		if remaining > 0 {
			opts.Logger.Debug("   %d explanations remaining to be generated", remaining)
		}
	} else if commitCount > 0 {
		opts.Logger.Debug("⚠️  No explanation files found. Run 'execute' command after generating prompts.")
	}

	// 6. 進捗サマリー
	opts.Logger.Debug("--- Progress Summary ---")
	if commitCount == 0 {
		opts.Logger.Debug("⚠️  No commits found in the repository")
	} else {
		collectProgress := float64(commitDataCount) / float64(commitCount) * 100
		generateProgress := float64(outputCount) / float64(commitCount) * 100

		opts.Logger.Debug("Data Collection: %.1f%% (%d/%d)", collectProgress, commitDataCount, commitCount)
		opts.Logger.Debug("Explanation Generation: %.1f%% (%d/%d)", generateProgress, outputCount, commitCount)

		if collectProgress == 100 && generateProgress == 100 {
			opts.Logger.Debug("🎉 All commits have been processed!")
		} else if collectProgress == 100 {
			opts.Logger.Debug("📝 Ready for explanation generation")
		} else {
			opts.Logger.Debug("📥 Need to collect more commit data")
		}
	}

	opts.Logger.Debug("--- Verification Complete ---")
	return nil
}
