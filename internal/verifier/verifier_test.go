package verifier

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
	maxTestCommits   = 3
)

// TestVerify tests the Verify function
func TestVerify(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	commitDataDir := filepath.Join(tempDir, "commit_data")
	promptsDir := filepath.Join(tempDir, "prompts")
	outputDir := filepath.Join(tempDir, "src")
	repoDir := t.TempDir()

	cfg := &config.Config{
		GoRepoPath:    repoDir,
		CommitDataDir: commitDataDir,
		PromptsDir:    promptsDir,
		OutputDir:     outputDir,
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

	// Create test commits
	for i := 1; i <= maxTestCommits; i++ {
		testFile := filepath.Join(repoDir, fmt.Sprintf("test%d.txt", i))
		err = os.WriteFile(testFile, []byte(fmt.Sprintf("test content %d", i)), filePermission)
		assert.NoError(t, err, "Failed to create test file")

		_, err = git.RunCommand(repoDir, "add", fmt.Sprintf("test%d.txt", i))
		assert.NoError(t, err, "Failed to add file")

		_, err = git.RunCommand(repoDir, "commit", "-m", fmt.Sprintf("Test commit %d", i))
		assert.NoError(t, err, "Failed to create commit")
	}

	// Create directories
	err = os.MkdirAll(commitDataDir, 0755)
	assert.NoError(t, err, "Failed to create commit data directory")
	err = os.MkdirAll(promptsDir, 0755)
	assert.NoError(t, err, "Failed to create prompts directory")
	err = os.MkdirAll(outputDir, 0755)
	assert.NoError(t, err, "Failed to create output directory")

	// Create commit data files
	for i := 1; i <= maxTestCommits; i++ {
		commitDataFile := filepath.Join(commitDataDir, fmt.Sprintf("%d.txt", i))
		err = os.WriteFile(commitDataFile, []byte(fmt.Sprintf("commit data %d", i)), filePermission)
		assert.NoError(t, err, "Failed to create commit data file")
	}

	// Create some prompt scripts (simulate incomplete processing)
	for i := 1; i <= 2; i++ {
		promptFile := filepath.Join(promptsDir, fmt.Sprintf("%d.sh", i))
		err = os.WriteFile(promptFile, []byte(fmt.Sprintf("#!/bin/bash\necho 'prompt %d'", i)), 0755)
		assert.NoError(t, err, "Failed to create prompt script")
	}

	// Create some output files (simulate completed processing)
	outputFile := filepath.Join(outputDir, "1.md")
	err = os.WriteFile(outputFile, []byte("# Output 1\nSome content"), filePermission)
	assert.NoError(t, err, "Failed to create output file")

	// Test Verify function
	err = Verify(cfg)
	assert.NoError(t, err, "Verify should not return error")
}

// TestVerifyNonExistentDirectories tests Verify with non-existent directories
func TestVerifyNonExistentDirectories(t *testing.T) {
	cfg := &config.Config{
		GoRepoPath:    "/non/existent/repo",
		CommitDataDir: "/non/existent/commit_data",
		PromptsDir:    "/non/existent/prompts",
		OutputDir:     "/non/existent/output",
	}

	// This should handle non-existent directories gracefully
	err := Verify(cfg)
	assert.Error(t, err, "Verify should return error for non-existent repository")
}

// TestVerifyEmptyDirectories tests Verify with empty directories
func TestVerifyEmptyDirectories(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	commitDataDir := filepath.Join(tempDir, "commit_data")
	promptsDir := filepath.Join(tempDir, "prompts")
	outputDir := filepath.Join(tempDir, "src")
	repoDir := t.TempDir()

	cfg := &config.Config{
		GoRepoPath:    repoDir,
		CommitDataDir: commitDataDir,
		PromptsDir:    promptsDir,
		OutputDir:     outputDir,
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

	// Create one commit
	testFile := filepath.Join(repoDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), filePermission)
	assert.NoError(t, err, "Failed to create test file")

	_, err = git.RunCommand(repoDir, "add", "test.txt")
	assert.NoError(t, err, "Failed to add file")

	_, err = git.RunCommand(repoDir, "commit", "-m", "Test commit")
	assert.NoError(t, err, "Failed to create commit")

	// Create empty directories
	err = os.MkdirAll(commitDataDir, 0755)
	assert.NoError(t, err, "Failed to create commit data directory")
	err = os.MkdirAll(promptsDir, 0755)
	assert.NoError(t, err, "Failed to create prompts directory")
	err = os.MkdirAll(outputDir, 0755)
	assert.NoError(t, err, "Failed to create output directory")

	// Test Verify with empty directories
	err = Verify(cfg)
	assert.NoError(t, err, "Verify should not return error for empty directories")
}