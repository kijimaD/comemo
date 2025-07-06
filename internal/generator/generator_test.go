package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"comemo/internal/config"
	"comemo/internal/git"
)

const (
	filePermission   = 0644
	scriptPermission = 0755
	dirPermission    = 0755
	maxTestCommits   = 3
)

// TestGeneratePromptScript tests the generatePromptScript function
func TestGeneratePromptScript(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	promptsDir := filepath.Join(tempDir, "prompts")
	outputDir := filepath.Join(tempDir, "src")
	commitDataDir := filepath.Join(tempDir, "commit_data")

	// Create directories
	err := os.MkdirAll(promptsDir, dirPermission)
	assert.NoError(t, err, "Failed to create prompts directory")
	err = os.MkdirAll(outputDir, dirPermission)
	assert.NoError(t, err, "Failed to create output directory")
	err = os.MkdirAll(commitDataDir, dirPermission)
	assert.NoError(t, err, "Failed to create commit data directory")

	cfg := &config.Config{
		PromptsDir:    promptsDir,
		OutputDir:     outputDir,
		CommitDataDir: commitDataDir,
	}

	// Create test commit data file
	commitDataPath := filepath.Join(commitDataDir, "1.txt")
	err = os.WriteFile(commitDataPath, []byte("test commit data"), filePermission)
	assert.NoError(t, err, "Failed to create commit data file")

	// Test generatePromptScript
	hash := "test-hash-123"
	index := 1
	err = generatePromptScript(cfg, hash, index, commitDataPath)
	assert.NoError(t, err, "generatePromptScript should not return error")

	// Check if script file was created
	scriptPath := filepath.Join(promptsDir, "1.sh")
	_, err = os.Stat(scriptPath)
	assert.NoError(t, err, "Expected script file to be created")

	// Check script content
	content, err := os.ReadFile(scriptPath)
	assert.NoError(t, err, "Failed to read script file")

	scriptContent := string(content)

	// Check expected elements
	expectedElements := []string{
		"#!/bin/bash",
		hash,
		fmt.Sprintf("Index %d", index),
		"{{AI_CLI_COMMAND}}",
		"@commit_data/1.txt",
		"https://github.com/golang/go/commit/" + hash,
	}

	for _, element := range expectedElements {
		assert.Contains(t, scriptContent, element, fmt.Sprintf("Script should contain %v", element))
	}
}

// TestGeneratePrompts tests the GeneratePrompts function with limited data
func TestGeneratePrompts(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	promptsDir := filepath.Join(tempDir, "prompts")
	outputDir := filepath.Join(tempDir, "src")
	commitDataDir := filepath.Join(tempDir, "commit_data")
	repoDir := t.TempDir()

	cfg := &config.Config{
		GoRepoPath:     repoDir,
		PromptsDir:     promptsDir,
		OutputDir:      outputDir,
		CommitDataDir:  commitDataDir,
		MaxConcurrency: 5,
	}

	// Initialize git repository
	_, err := git.RunCommand(repoDir, "init")
	if err != nil {
		t.Skip("Git not available")
	}

	// Configure git user
	_, err = git.RunCommand(repoDir, "config", "user.name", "Test User")
	if err != nil {
		t.Skip("Failed to configure git user.name")
	}
	_, err = git.RunCommand(repoDir, "config", "user.email", "test@example.com")
	if err != nil {
		t.Skip("Failed to configure git user.email")
	}

	// Create test commits and commit data
	for i := 1; i <= maxTestCommits; i++ {
		testFile := filepath.Join(repoDir, fmt.Sprintf("test%d.txt", i))
		err = os.WriteFile(testFile, []byte(fmt.Sprintf("test content %d", i)), filePermission)
		assert.NoError(t, err, "Failed to create test file")

		_, err = git.RunCommand(repoDir, "add", fmt.Sprintf("test%d.txt", i))
		assert.NoError(t, err, "Failed to add file")

		_, err = git.RunCommand(repoDir, "commit", "-m", fmt.Sprintf("Test commit %d", i))
		assert.NoError(t, err, "Failed to create commit")
	}

	// Create commit data directory and files
	err = os.MkdirAll(commitDataDir, dirPermission)
	assert.NoError(t, err, "Failed to create commit data directory")

	hashes, err := git.GetCommitHashes(repoDir)
	assert.NoError(t, err, "Failed to get commit hashes")

	for i, hash := range hashes {
		index := i + 1
		commitData, err := git.GetCommitData(repoDir, hash)
		assert.NoError(t, err, "Failed to get commit data")

		commitDataPath := filepath.Join(commitDataDir, fmt.Sprintf("%d.txt", index))
		err = os.WriteFile(commitDataPath, []byte(commitData), filePermission)
		assert.NoError(t, err, "Failed to write commit data")
	}

	// Test GeneratePrompts
	err = GeneratePrompts(cfg)
	assert.NoError(t, err, "GeneratePrompts should not return error")

	// Check if directories were created
	_, err = os.Stat(promptsDir)
	assert.NoError(t, err, "Expected prompts directory to be created")
	_, err = os.Stat(outputDir)
	assert.NoError(t, err, "Expected output directory to be created")

	// Check if prompt scripts were created
	for i := 1; i <= maxTestCommits; i++ {
		scriptFile := filepath.Join(promptsDir, fmt.Sprintf("%d.sh", i))
		_, err = os.Stat(scriptFile)
		assert.NoError(t, err, fmt.Sprintf("Expected prompt script %d to be created", i))

		// Check script content
		content, err := os.ReadFile(scriptFile)
		assert.NoError(t, err, "Failed to read script file")
		assert.NotEmpty(t, content, "Expected non-empty script content")
		assert.Contains(t, string(content), "#!/bin/bash", "Script should contain shebang")
		assert.Contains(t, string(content), "{{AI_CLI_COMMAND}}", "Script should contain AI CLI command placeholder")
	}
}