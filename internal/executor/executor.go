package executor

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"comemo/internal/config"
)

// ExecutePrompts executes generated prompt scripts
func ExecutePrompts(cfg *config.Config, cliCommand string) error {
	fmt.Println("\n--- Executing Prompt Scripts ---")

	files, err := os.ReadDir(cfg.PromptsDir)
	if err != nil {
		return fmt.Errorf("error reading prompts directory: %w", err)
	}

	var shFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") {
			shFiles = append(shFiles, file.Name())
		}
	}

	if len(shFiles) == 0 {
		fmt.Println("No .sh files found in the prompts directory.")
		return nil
	}

	fmt.Printf("Found %d scripts to execute\n", len(shFiles))

	// Create CLI manager
	manager := NewCLIManager(cfg)

	// Determine which CLIs to use
	var cliTools []string
	if cliCommand == "all" {
		for name := range SupportedCLIs {
			cliTools = append(cliTools, name)
		}
	} else {
		cliTools = []string{cliCommand}
	}

	// Create channels for script distribution
	scriptQueues := make(map[string]chan string)
	for _, cliName := range cliTools {
		scriptQueues[cliName] = make(chan string, len(shFiles))
	}

	// Start workers
	var wg sync.WaitGroup
	for cliName, queue := range scriptQueues {
		wg.Add(1)
		go func(name string, q chan string) {
			defer wg.Done()
			Worker(name, q, manager)
		}(cliName, queue)
	}

	// Start quota monitor
	monitorQueue := make(chan string, len(shFiles))
	go quotaMonitor(manager, monitorQueue)

	// Distribute scripts round-robin among CLIs
	for i, fileName := range shFiles {
		cliIndex := i % len(cliTools)
		cliName := cliTools[cliIndex]
		scriptQueues[cliName] <- fileName
		monitorQueue <- fileName
	}

	// Close queues
	for _, queue := range scriptQueues {
		close(queue)
	}
	close(monitorQueue)

	// Wait for all workers to complete
	wg.Wait()

	// Count remaining scripts
	remainingFiles, _ := os.ReadDir(cfg.PromptsDir)
	remainingCount := 0
	for _, file := range remainingFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sh") {
			remainingCount++
		}
	}

	if remainingCount > 0 {
		fmt.Printf("\n⚠️  %d scripts remain unprocessed. They may have failed or hit quota limits.\n", remainingCount)
	} else {
		fmt.Println("\nAll prompt scripts executed successfully and were deleted.")
	}

	return nil
}

// quotaMonitor monitors quota status and provides status updates
func quotaMonitor(manager *CLIManager, scriptQueue <-chan string) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	totalScripts := 0
	startTime := time.Now()

	for {
		select {
		case _, ok := <-scriptQueue:
			if !ok {
				return
			}
			totalScripts++

		case <-ticker.C:
			elapsed := time.Since(startTime)
			fmt.Printf("\n📊 Status Update:\n")
			fmt.Printf("   Total scripts queued: %d\n", totalScripts)
			fmt.Printf("   Time elapsed: %v\n", elapsed)

			for name, cli := range manager.CLIs {
				status := "Available"
				if !cli.Available {
					timeUntilAvailable := manager.Config.QuotaRetryDelay - time.Since(cli.LastQuotaError)
					status = fmt.Sprintf("Quota limit (available in %v)", timeUntilAvailable.Round(time.Minute))
				}
				fmt.Printf("   %s: %s\n", name, status)
			}
			fmt.Println()
		}
	}
}