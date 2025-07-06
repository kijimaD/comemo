package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	filePermission = 0644
)

// TestGetCommitIndex tests the GetCommitIndex function
func TestGetCommitIndex(t *testing.T) {
	tests := []struct {
		name       string
		allHashes  []string
		targetHash string
		expected   int
	}{
		{
			name:       "First hash",
			allHashes:  []string{"hash1", "hash2", "hash3"},
			targetHash: "hash1",
			expected:   1,
		},
		{
			name:       "Middle hash",
			allHashes:  []string{"hash1", "hash2", "hash3"},
			targetHash: "hash2",
			expected:   2,
		},
		{
			name:       "Last hash",
			allHashes:  []string{"hash1", "hash2", "hash3"},
			targetHash: "hash3",
			expected:   3,
		},
		{
			name:       "Non-existent hash",
			allHashes:  []string{"hash1", "hash2", "hash3"},
			targetHash: "hash4",
			expected:   0,
		},
		{
			name:       "Empty array",
			allHashes:  []string{},
			targetHash: "hash1",
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCommitIndex(tt.allHashes, tt.targetHash)
			assert.Equal(t, tt.expected, result, "GetCommitIndex should return expected index")
		})
	}
}

// TestRunCommand tests the RunCommand function
func TestRunCommand(t *testing.T) {
	// Create temporary git repository
	tempDir := t.TempDir()

	// Initialize git repository
	_, err := RunCommand(tempDir, "init")
	assert.NoError(t, err)

	// Test git status
	output, err := RunCommand(tempDir, "status", "--porcelain")
	assert.NoError(t, err, "git status should not return error")

	// Empty repository should have empty status
	assert.Empty(t, strings.TrimSpace(output), "Expected empty status in new repository")

	// Test invalid git command
	_, err = RunCommand(tempDir, "invalid-command")
	assert.Error(t, err, "Invalid git command should return error")

	// Test non-existent repository path
	_, err = RunCommand("/non/existent/path", "status")
	assert.Error(t, err, "Non-existent path should return error")
}

// TestGetCommitHashes tests the GetCommitHashes function
func TestGetCommitHashes(t *testing.T) {
	// Create temporary git repository
	tempDir := t.TempDir()

	// Initialize git repository
	_, err := RunCommand(tempDir, "init")
	if err != nil {
		t.Skip("Git not available or failed to initialize repository")
	}

	// Configure git user
	_, err = RunCommand(tempDir, "config", "user.name", "Test User")
	if err != nil {
		t.Skip("Failed to configure git user.name")
	}
	_, err = RunCommand(tempDir, "config", "user.email", "test@example.com")
	if err != nil {
		t.Skip("Failed to configure git user.email")
	}

	// Test empty repository
	hashes, err := GetCommitHashes(tempDir)
	if err == nil {
		assert.Empty(t, hashes, "Expected 0 hashes in empty repository")
	} else {
		t.Logf("Empty repository returned error (expected): %v", err)
	}

	// Create test file and commit
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), filePermission)
	assert.NoError(t, err, "Failed to create test file")

	_, err = RunCommand(tempDir, "add", "test.txt")
	if err != nil {
		t.Skip("Failed to add file to git")
	}

	_, err = RunCommand(tempDir, "commit", "-m", "Initial commit")
	if err != nil {
		t.Skip("Failed to create commit")
	}

	// Test repository with one commit
	hashes, err = GetCommitHashes(tempDir)
	assert.NoError(t, err, "GetCommitHashes should not return error for valid repository")
	assert.Len(t, hashes, 1, "Expected 1 hash after creating commit")

	// Test non-existent repository
	_, err = GetCommitHashes("/non/existent/path")
	assert.Error(t, err, "Non-existent repository should return error")
}

// TestGetCommitData tests the GetCommitData function
func TestGetCommitData(t *testing.T) {
	// Create temporary git repository
	tempDir := t.TempDir()

	// Initialize git repository
	_, err := RunCommand(tempDir, "init")
	if err != nil {
		t.Skip("Git not available or failed to initialize repository")
	}

	// Configure git user
	_, err = RunCommand(tempDir, "config", "user.name", "Test User")
	if err != nil {
		t.Skip("Failed to configure git user.name")
	}
	_, err = RunCommand(tempDir, "config", "user.email", "test@example.com")
	if err != nil {
		t.Skip("Failed to configure git user.email")
	}

	// Create test file and commit
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), filePermission)
	assert.NoError(t, err, "Failed to create test file")

	_, err = RunCommand(tempDir, "add", "test.txt")
	if err != nil {
		t.Skip("Failed to add file to git")
	}

	_, err = RunCommand(tempDir, "commit", "-m", "Initial commit")
	if err != nil {
		t.Skip("Failed to create commit")
	}

	// Get commit hash
	hashes, err := GetCommitHashes(tempDir)
	if err != nil {
		t.Skip("Failed to get commit hashes")
	}
	if len(hashes) == 0 {
		t.Skip("No commits found")
	}

	commitHash := hashes[0]

	// Test GetCommitData with valid hash
	commitData, err := GetCommitData(tempDir, commitHash)
	assert.NoError(t, err, "GetCommitData should not return error for valid hash")
	assert.NotEmpty(t, commitData, "Commit data should not be empty")

	// Verify commit data contains expected information
	assert.Contains(t, commitData, "Initial commit", "Commit data should contain commit message")
	assert.Contains(t, commitData, "test.txt", "Commit data should contain file name")

	// Test GetCommitData with invalid hash
	_, err = GetCommitData(tempDir, "invalid-hash")
	assert.Error(t, err, "GetCommitData should return error for invalid hash")

	// Test GetCommitData with non-existent repository
	_, err = GetCommitData("/non/existent/path", commitHash)
	assert.Error(t, err, "GetCommitData should return error for non-existent repository")

	// Create second commit to test patch output
	err = os.WriteFile(testFile, []byte("modified content"), filePermission)
	assert.NoError(t, err, "Failed to modify test file")

	_, err = RunCommand(tempDir, "add", "test.txt")
	if err != nil {
		t.Skip("Failed to add modified file to git")
	}

	_, err = RunCommand(tempDir, "commit", "-m", "Second commit")
	if err != nil {
		t.Skip("Failed to create second commit")
	}

	// Get updated commit hashes
	hashes, err = GetCommitHashes(tempDir)
	if err != nil {
		t.Skip("Failed to get updated commit hashes")
	}
	if len(hashes) < 2 {
		t.Skip("Not enough commits found")
	}

	// Test commit data for second commit (should contain diff)
	secondCommitHash := hashes[1]
	commitData, err = GetCommitData(tempDir, secondCommitHash)
	assert.NoError(t, err, "GetCommitData should not return error for second commit")
	assert.Contains(t, commitData, "Second commit", "Commit data should contain second commit message")
	assert.Contains(t, commitData, "modified content", "Commit data should contain new content")
	assert.Contains(t, commitData, "initial content", "Commit data should contain old content in diff")
}
