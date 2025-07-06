package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"comemo/internal/config"
	"comemo/internal/git"
	"comemo/internal/logger"
)

const (
	filePermission = 0644
	maxTestCommits = 3
)

// TestPrepareCommitData tests the prepareCommitData function
func TestPrepareCommitData(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	repoDir := t.TempDir()

	cfg := &config.Config{
		GoRepoPath:    repoDir,
		CommitDataDir: tempDir,
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

	// Create test file and commit
	testFile := filepath.Join(repoDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), filePermission)
	if err != nil {
		t.Fatal("Failed to create test file")
	}

	_, err = git.RunCommand(repoDir, "add", "test.txt")
	if err != nil {
		t.Skip("Failed to add file to git")
	}

	_, err = git.RunCommand(repoDir, "commit", "-m", "Test commit")
	if err != nil {
		t.Skip("Failed to create commit")
	}

	// Get commit hash
	hashes, err := git.GetCommitHashes(repoDir)
	if err != nil {
		t.Skip("Failed to get commit hashes")
	}
	if len(hashes) == 0 {
		t.Skip("No commits found")
	}

	// Test prepareCommitData
	hash := hashes[0]
	index := 1
	err = prepareCommitData(cfg, hash, index)
	assert.NoError(t, err, "prepareCommitData should not return error")

	// Check if file was created
	expectedPath := filepath.Join(tempDir, "1.txt")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err, "Expected file to be created")

	// Check file content
	content, err := os.ReadFile(expectedPath)
	assert.NoError(t, err, "Failed to read created file")
	assert.NotEmpty(t, content, "Expected non-empty file content")
}

// TestCollectCommits tests the CollectCommits function with limited data
func TestCollectCommits(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	commitDataDir := filepath.Join(tempDir, "commit_data")
	repoDir := t.TempDir()

	cfg := &config.Config{
		GoRepoPath:     repoDir,
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

	// Create several test commits
	for i := 1; i <= maxTestCommits; i++ {
		testFile := filepath.Join(repoDir, fmt.Sprintf("test%d.txt", i))
		err = os.WriteFile(testFile, []byte(fmt.Sprintf("test content %d", i)), filePermission)
		assert.NoError(t, err, "Failed to create test file")

		_, err = git.RunCommand(repoDir, "add", fmt.Sprintf("test%d.txt", i))
		assert.NoError(t, err, "Failed to add file")

		_, err = git.RunCommand(repoDir, "commit", "-m", fmt.Sprintf("Test commit %d", i))
		assert.NoError(t, err, "Failed to create commit")
	}

	// Test CollectCommits with silent output
	err = CollectCommitsWithOptions(cfg, &CollectorOptions{
		Logger: logger.Silent(),
	})
	assert.NoError(t, err, "CollectCommits should not return error")

	// Check if commit data directory was created
	_, err = os.Stat(commitDataDir)
	assert.NoError(t, err, "Expected commit data directory to be created")

	// Check if commit data files were created
	for i := 1; i <= maxTestCommits; i++ {
		commitDataFile := filepath.Join(commitDataDir, fmt.Sprintf("%d.txt", i))
		_, err = os.Stat(commitDataFile)
		assert.NoError(t, err, fmt.Sprintf("Expected commit data file %d to be created", i))

		// Check file content
		content, err := os.ReadFile(commitDataFile)
		assert.NoError(t, err, "Failed to read commit data file")
		assert.NotEmpty(t, content, "Expected non-empty commit data")
	}
}
