package verifier

import (
	"fmt"
	"io"
	"os"
	"strings"

	"comemo/internal/config"
	"comemo/internal/git"
)

// VerifierOptions provides configuration for verifier functions
type VerifierOptions struct {
	Output io.Writer
	Error  io.Writer
}

// Verify checks the consistency of generated files
func Verify(cfg *config.Config) error {
	return VerifyWithOptions(cfg, &VerifierOptions{
		Output: os.Stdout,
		Error:  os.Stderr,
	})
}

// VerifyWithOptions checks the consistency of generated files with configurable output
func VerifyWithOptions(cfg *config.Config, opts *VerifierOptions) error {
	if opts == nil {
		opts = &VerifierOptions{
			Output: os.Stdout,
			Error:  os.Stderr,
		}
	}

	fmt.Fprintln(opts.Output, "--- Verification Started ---")

	// 1. コミット数を取得
	allHashes, err := git.GetCommitHashes(cfg.GoRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}
	commitCount := len(allHashes)
	fmt.Fprintf(opts.Output, "Total commits: %d\n", commitCount)

	// 2. commit_dataディレクトリ内のファイル数を取得
	commitDataFiles, err := os.ReadDir(cfg.CommitDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(opts.Output, "commit_data directory does not exist: %s\n", cfg.CommitDataDir)
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
	fmt.Fprintf(opts.Output, "commit_data files: %d\n", commitDataCount)

	// 3. promptsディレクトリ内のファイル数を取得
	promptFiles, err := os.ReadDir(cfg.PromptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(opts.Output, "prompts directory does not exist: %s\n", cfg.PromptsDir)
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
	fmt.Fprintf(opts.Output, "prompt scripts: %d\n", promptCount)

	// 4. srcディレクトリ内の説明ファイル数を取得
	outputFiles, err := os.ReadDir(cfg.OutputDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(opts.Output, "src directory does not exist: %s\n", cfg.OutputDir)
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
	fmt.Fprintf(opts.Output, "explanation files: %d\n", outputCount)

	// 5. 検証結果の表示
	fmt.Fprintln(opts.Output, "\n--- Verification Results ---")

	if commitDataCount != commitCount {
		fmt.Fprintf(opts.Output, "❌ Mismatch: commit_data files (%d) != total commits (%d)\n", commitDataCount, commitCount)
		missing := commitCount - commitDataCount
		if missing > 0 {
			fmt.Fprintf(opts.Output, "   Missing %d commit data files. Run 'collect' command.\n", missing)
		} else {
			fmt.Fprintf(opts.Output, "   Extra %d commit data files found.\n", -missing)
		}
	} else {
		fmt.Fprintf(opts.Output, "✅ commit_data files match total commits (%d)\n", commitCount)
	}

	expectedPrompts := commitCount - promptCount
	if promptCount > 0 {
		fmt.Fprintf(opts.Output, "✅ Found %d prompt scripts\n", promptCount)
		if expectedPrompts > 0 {
			fmt.Fprintf(opts.Output, "   %d prompts may have been executed already\n", expectedPrompts)
		}
	} else if commitDataCount > 0 {
		fmt.Fprintf(opts.Output, "⚠️  No prompt scripts found. Run 'generate' command to create them.\n")
	}

	if outputCount > 0 {
		fmt.Fprintf(opts.Output, "✅ Found %d explanation files\n", outputCount)
		remaining := commitCount - outputCount
		if remaining > 0 {
			fmt.Fprintf(opts.Output, "   %d explanations remaining to be generated\n", remaining)
		}
	} else if commitCount > 0 {
		fmt.Fprintf(opts.Output, "⚠️  No explanation files found. Run 'execute' command after generating prompts.\n")
	}

	// 6. 進捗サマリー
	fmt.Fprintln(opts.Output, "\n--- Progress Summary ---")
	if commitCount == 0 {
		fmt.Fprintln(opts.Output, "⚠️  No commits found in the repository")
	} else {
		collectProgress := float64(commitDataCount) / float64(commitCount) * 100
		generateProgress := float64(outputCount) / float64(commitCount) * 100

		fmt.Fprintf(opts.Output, "Data Collection: %.1f%% (%d/%d)\n", collectProgress, commitDataCount, commitCount)
		fmt.Fprintf(opts.Output, "Explanation Generation: %.1f%% (%d/%d)\n", generateProgress, outputCount, commitCount)

		if collectProgress == 100 && generateProgress == 100 {
			fmt.Fprintln(opts.Output, "🎉 All commits have been processed!")
		} else if collectProgress == 100 {
			fmt.Fprintln(opts.Output, "📝 Ready for explanation generation")
		} else {
			fmt.Fprintln(opts.Output, "📥 Need to collect more commit data")
		}
	}

	fmt.Fprintln(opts.Output, "--- Verification Complete ---")
	return nil
}
