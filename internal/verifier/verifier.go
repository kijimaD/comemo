package verifier

import (
	"fmt"
	"os"
	"strings"

	"comemo/internal/config"
	"comemo/internal/git"
)

// Verify checks the consistency of generated files
func Verify(cfg *config.Config) error {
	fmt.Println("--- Verification Started ---")

	// 1. コミット数を取得
	allHashes, err := git.GetCommitHashes(cfg.GoRepoPath)
	if err != nil {
		return fmt.Errorf("error getting commit hashes: %w", err)
	}
	commitCount := len(allHashes)
	fmt.Printf("Total commits: %d\n", commitCount)

	// 2. commit_dataディレクトリ内のファイル数を取得
	commitDataFiles, err := os.ReadDir(cfg.CommitDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("commit_data directory does not exist: %s\n", cfg.CommitDataDir)
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
	promptFiles, err := os.ReadDir(cfg.PromptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("prompts directory does not exist: %s\n", cfg.PromptsDir)
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
	outputFiles, err := os.ReadDir(cfg.OutputDir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("src directory does not exist: %s\n", cfg.OutputDir)
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