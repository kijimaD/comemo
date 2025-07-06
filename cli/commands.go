package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"

	"comemo/internal/collector"
	"comemo/internal/config"
	"comemo/internal/executor"
	"comemo/internal/generator"
	"comemo/internal/git"
	"comemo/internal/logger"
	"comemo/internal/verifier"
)

// CreateApp creates the CLI application
func CreateApp() *cli.Command {
	cfg := config.DefaultConfig()

	return &cli.Command{
		Name:    "comemo",
		Usage:   "Go repository commit explanation generator",
		Version: "1.0.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "repo",
				Aliases: []string{"r"},
				Value:   cfg.GoRepoPath,
				Usage:   "Path to Go repository",
			},
			&cli.StringFlag{
				Name:    "prompts-dir",
				Aliases: []string{"p"},
				Value:   cfg.PromptsDir,
				Usage:   "Directory for prompt scripts",
			},
			&cli.StringFlag{
				Name:    "output-dir",
				Aliases: []string{"o"},
				Value:   cfg.OutputDir,
				Usage:   "Directory for output markdown files",
			},
			&cli.StringFlag{
				Name:    "commit-data-dir",
				Aliases: []string{"c"},
				Value:   cfg.CommitDataDir,
				Usage:   "Directory for commit data files",
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Aliases: []string{"j"},
				Value:   cfg.MaxConcurrency,
				Usage:   "Maximum concurrent AI CLI executions",
			},
			&cli.DurationFlag{
				Name:    "timeout",
				Aliases: []string{"t"},
				Value:   cfg.ExecutionTimeout,
				Usage:   "Execution timeout for each script",
			},
			&cli.DurationFlag{
				Name:  "quota-retry-delay",
				Value: cfg.QuotaRetryDelay,
				Usage: "Delay before retrying after quota limit",
			},
			&cli.IntFlag{
				Name:  "max-retries",
				Value: cfg.MaxRetries,
				Usage: "Maximum number of retries for failed scripts",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Value:   "info",
				Usage:   "Log level (debug, info, warn, error, silent)",
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "collect",
				Aliases: []string{"c"},
				Usage:   "Collect commit data from Go repository",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					updateConfig(cfg, cmd)
					return collector.CollectCommits(cfg)
				},
			},
			{
				Name:    "generate",
				Aliases: []string{"g"},
				Usage:   "Generate prompt scripts for missing explanations",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					updateConfig(cfg, cmd)
					return generator.GeneratePrompts(cfg)
				},
			},
			{
				Name:    "execute",
				Aliases: []string{"e"},
				Usage:   "Execute prompt scripts to generate explanations",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "cli",
						Usage: "AI CLI command to use (claude, gemini, all)",
						Value: "claude",
					},
					&cli.StringFlag{
						Name:  "task-log",
						Usage: "File path to write task execution logs",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					updateConfig(cfg, cmd)
					cliCommand := cmd.String("cli")

					if cliCommand != "all" {
						_, exists := executor.SupportedCLIs[cliCommand]
						if !exists {
							return fmt.Errorf("unsupported CLI command '%s'. Supported: claude, gemini, all", cliCommand)
						}
					}

					// タスクログファイルの設定
					taskLogPath := cmd.String("task-log")
					var taskLogWriter *os.File
					if taskLogPath != "" {
						f, err := os.OpenFile(taskLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
						if err != nil {
							return fmt.Errorf("failed to open task log file: %w", err)
						}
						defer f.Close()
						taskLogWriter = f
					}

					opts := &executor.ExecutorOptions{
						Logger:             logger.New(cfg.LogLevel, os.Stdout, os.Stderr),
						TaskLogWriter:      taskLogWriter,
						EventStatusManager: executor.NewEventStatusManager(cfg.MaxRetries),
					}

					return executor.ExecutePromptsWithProgressAndOptions(cfg, cliCommand, opts)
				},
			},
			{
				Name:    "verify",
				Aliases: []string{"v"},
				Usage:   "Verify the consistency of generated files",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					updateConfig(cfg, cmd)
					return verifier.Verify(cfg)
				},
			},
			{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Run all steps: collect, generate, and execute",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "cli",
						Usage: "AI CLI command to use (claude, gemini, all)",
						Value: "claude",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					updateConfig(cfg, cmd)

					fmt.Printf("🎯 Running all steps...\n\n")

					fmt.Printf("📦 Step 1/3: Collecting commits...\n")
					if err := collector.CollectCommits(cfg); err != nil {
						return fmt.Errorf("collect failed: %w", err)
					}

					fmt.Printf("\n📝 Step 2/3: Generating prompts...\n")
					if err := generator.GeneratePrompts(cfg); err != nil {
						return fmt.Errorf("generate failed: %w", err)
					}

					fmt.Printf("\n🚀 Step 3/3: Executing scripts...\n")
					cliCommand := cmd.String("cli")
					if cliCommand != "all" {
						_, exists := executor.SupportedCLIs[cliCommand]
						if !exists {
							return fmt.Errorf("unsupported CLI command '%s'. Supported: claude, gemini, all", cliCommand)
						}
					}

					if err := executor.ExecutePromptsWithProgress(cfg, cliCommand); err != nil {
						return fmt.Errorf("execute failed: %w", err)
					}

					fmt.Printf("\n✅ All steps completed successfully!\n")
					return nil
				},
			},
			{
				Name:    "status",
				Aliases: []string{"s"},
				Usage:   "Show current status and statistics",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "events",
						Usage: "Show detailed event status information",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					updateConfig(cfg, cmd)

					fmt.Printf("📊 Comemo Status\n")
					fmt.Printf("================\n\n")

					allHashes, err := git.GetCommitHashes(cfg.GoRepoPath)
					if err != nil {
						return fmt.Errorf("error getting commit hashes: %w", err)
					}
					commitCount := len(allHashes)

					commitDataFiles, _ := filepath.Glob(filepath.Join(cfg.CommitDataDir, "*.txt"))
					scriptFiles, _ := filepath.Glob(filepath.Join(cfg.PromptsDir, "*.sh"))
					outputFiles, _ := filepath.Glob(filepath.Join(cfg.OutputDir, "*.md"))

					outputCount := 0
					for _, f := range outputFiles {
						if filepath.Base(f) != "SUMMARY.md" {
							outputCount++
						}
					}

					fmt.Printf("📊 Repository commits:    %d\n", commitCount)
					fmt.Printf("📦 Collected data files:  %d\n", len(commitDataFiles))
					fmt.Printf("📝 Generated scripts:     %d\n", len(scriptFiles))
					fmt.Printf("✅ Completed outputs:     %d\n", outputCount)

					if commitCount > 0 {
						collectProgress := float64(len(commitDataFiles)) / float64(commitCount) * 100
						generateProgress := float64(outputCount) / float64(commitCount) * 100

						fmt.Printf("\n📈 Progress:\n")
						fmt.Printf("   Data collection:      %.1f%%\n", collectProgress)
						fmt.Printf("   Output generation:    %.1f%%\n", generateProgress)

						remaining := commitCount - outputCount
						if remaining > 0 {
							fmt.Printf("\n⏳ Remaining: %d commits to process\n", remaining)
						} else {
							fmt.Printf("\n🎉 All commits have been processed!\n")
						}
					}

					return nil
				},
			},
		},
	}
}

// updateConfig updates the configuration from CLI flags
func updateConfig(cfg *config.Config, cmd *cli.Command) {
	cfg.GoRepoPath = cmd.String("repo")
	cfg.PromptsDir = cmd.String("prompts-dir")
	cfg.OutputDir = cmd.String("output-dir")
	cfg.CommitDataDir = cmd.String("commit-data-dir")
	cfg.MaxConcurrency = cmd.Int("concurrency")
	cfg.ExecutionTimeout = cmd.Duration("timeout")
	cfg.QuotaRetryDelay = cmd.Duration("quota-retry-delay")
	cfg.MaxRetries = cmd.Int("max-retries")

	// Parse log level
	logLevelStr := cmd.String("log-level")
	if logLevel, err := logger.ParseLogLevel(logLevelStr); err == nil {
		cfg.LogLevel = logLevel
	}

	// Resolve all paths to absolute paths from current working directory
	if err := cfg.ResolveConfigPaths(); err != nil {
		fmt.Printf("Warning: Failed to resolve config paths: %v\n", err)
	}
}
