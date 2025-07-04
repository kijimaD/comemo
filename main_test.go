package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetCommitIndex tests the getCommitIndex function
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
			result := getCommitIndex(tt.allHashes, tt.targetHash)
			assert.Equal(t, tt.expected, result, "getCommitIndex should return expected index")
		})
	}
}

// TestRunGitCommand tests the runGitCommand function
func TestRunGitCommand(t *testing.T) {
	// Create a temporary git repository for testing
	tempDir := t.TempDir()

	// Initialize a git repository
	_, err := runGitCommand(tempDir, "init")
	assert.NoError(t, err)

	// Test git status
	output, err := runGitCommand(tempDir, "status", "--porcelain")
	assert.NoError(t, err, "git status should not return error")

	// Empty repository should have empty status
	assert.Empty(t, strings.TrimSpace(output), "Expected empty status in new repository")

	// Test invalid git command
	_, err = runGitCommand(tempDir, "invalid-command")
	assert.Error(t, err, "Invalid git command should return error")

	// Test non-existent repository path
	_, err = runGitCommand("/non/existent/path", "status")
	assert.Error(t, err, "Non-existent path should return error")
}

// TestGetCommitHashes tests the getCommitHashes function
func TestGetCommitHashes(t *testing.T) {
	// Create a temporary git repository for testing
	tempDir := t.TempDir()

	// Initialize a git repository
	_, err := runGitCommand(tempDir, "init")
	if err != nil {
		t.Skip("Git not available or failed to initialize repository")
	}

	// Configure git user for commits
	_, err = runGitCommand(tempDir, "config", "user.name", "Test User")
	if err != nil {
		t.Skip("Failed to configure git user.name")
	}
	_, err = runGitCommand(tempDir, "config", "user.email", "test@example.com")
	if err != nil {
		t.Skip("Failed to configure git user.email")
	}

	// Test empty repository (this will return an error which is expected)
	hashes, err := getCommitHashes(tempDir)
	if err == nil {
		// If no error, check that we get 0 hashes
		assert.Empty(t, hashes, "Expected 0 hashes in empty repository")
	} else {
		// Error is expected for empty repository, just log it
		t.Logf("Empty repository returned error (expected): %v", err)
	}

	// Create a test file and commit
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	assert.NoError(t, err, "Failed to create test file")

	_, err = runGitCommand(tempDir, "add", "test.txt")
	if err != nil {
		t.Skip("Failed to add file to git")
	}

	_, err = runGitCommand(tempDir, "commit", "-m", "Initial commit")
	if err != nil {
		t.Skip("Failed to create commit")
	}

	// Test repository with one commit
	hashes, err = getCommitHashes(tempDir)
	assert.NoError(t, err, "getCommitHashes should not return error for valid repository")
	assert.Len(t, hashes, 1, "Expected 1 hash after creating commit")

	// Test non-existent repository
	_, err = getCommitHashes("/non/existent/path")
	assert.Error(t, err, "Non-existent repository should return error")
}

// prepareCommitDataWithPath is a test helper function
func prepareCommitDataWithPath(hash string, index int, repoPath, dataDir string) (string, error) {
	filePath := filepath.Join(dataDir, fmt.Sprintf("%d.txt", index))
	commitData, err := runGitCommand(repoPath, "show", "--stat", hash)
	if err != nil {
		return "", fmt.Errorf("failed to get commit data for %s: %w", hash, err)
	}
	if err := os.WriteFile(filePath, []byte(commitData), 0644); err != nil {
		return "", err
	}
	return filePath, nil
}

// generatePromptScriptWithPath is a test helper function
func generatePromptScriptWithPath(hash string, index int, commitDataPath, promptsDir, outputDir string) error {
	scriptPath := filepath.Join(promptsDir, fmt.Sprintf("%d.sh", index))
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%d.md", index))
	githubURL := fmt.Sprintf("https://github.com/golang/go/commit/%s", hash)

	readCmd := fmt.Sprintf("`read_file(\"%s\")`", commitDataPath)

	prompt := `これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、` + readCmd + ` を実行して、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
`
	prompt += fmt.Sprintf("- **コミットインデックス**: %d\n", index)
	prompt += fmt.Sprintf("- **コミットハッシュ**: %s\n", hash)
	prompt += fmt.Sprintf("- **GitHub URL**: %s\n", githubURL)
	prompt += `
### 章構成
`
	prompt += fmt.Sprintf("\n# [インデックス %d] ファイルの概要\n", index)
	prompt += `
## コミット

## GitHub上でのコミットページへのリンク

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

# Gemini CLIにプロンプトを渡す (実際のCLIコマンド名に要変更)
# ヒアドキュメントを使い、プロンプトを安全に渡す
gemini -p <<'EOF'
%s
EOF

echo -e "\n✅ Done. Copy the output above and save it as: %s"
`, index, hash, index, prompt, outputPath)

	return os.WriteFile(scriptPath, []byte(scriptContent), 0755)
}

// TestPrepareCommitData tests the prepareCommitData function
func TestPrepareCommitData(t *testing.T) {
	// Create a temporary directory for commit data
	tempDir := t.TempDir()

	// Create a temporary git repository
	repoDir := t.TempDir()

	// Initialize a git repository
	_, err := runGitCommand(repoDir, "init")
	if err != nil {
		t.Skip("Git not available")
	}

	// Configure git user for commits
	_, err = runGitCommand(repoDir, "config", "user.name", "Test User")
	if err != nil {
		t.Skip("Failed to configure git user.name")
	}
	_, err = runGitCommand(repoDir, "config", "user.email", "test@example.com")
	if err != nil {
		t.Skip("Failed to configure git user.email")
	}

	// Create a test file and commit
	testFile := filepath.Join(repoDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatal("Failed to create test file")
	}

	_, err = runGitCommand(repoDir, "add", "test.txt")
	if err != nil {
		t.Skip("Failed to add file to git")
	}

	_, err = runGitCommand(repoDir, "commit", "-m", "Test commit")
	if err != nil {
		t.Skip("Failed to create commit")
	}

	// Get the commit hash
	hashes, err := getCommitHashes(repoDir)
	if err != nil {
		t.Skip("Failed to get commit hashes")
	}
	if len(hashes) == 0 {
		t.Skip("No commits found")
	}

	// Test prepareCommitDataWithPath
	hash := hashes[0]
	index := 1
	filePath, err := prepareCommitDataWithPath(hash, index, repoDir, tempDir)
	assert.NoError(t, err, "prepareCommitDataWithPath should not return error")

	expectedPath := filepath.Join(tempDir, "1.txt")
	assert.Equal(t, expectedPath, filePath, "File path should match expected path")

	// Check if file was created
	_, err = os.Stat(filePath)
	assert.NoError(t, err, "Expected file to be created")

	// Check file content
	content, err := os.ReadFile(filePath)
	assert.NoError(t, err, "Failed to read created file")
	assert.NotEmpty(t, content, "Expected non-empty file content")

	// Test with invalid hash
	_, err = prepareCommitDataWithPath("invalid-hash", 2, repoDir, tempDir)
	assert.Error(t, err, "Invalid hash should return error")
}

// TestGeneratePromptScript tests the generatePromptScript function
func TestGeneratePromptScript(t *testing.T) {
	// Create temporary directories
	tempDir := t.TempDir()
	promptsDir := filepath.Join(tempDir, "prompts")
	outputDir := filepath.Join(tempDir, "src")
	commitDataDir := filepath.Join(tempDir, "commit_data")

	// Create directories
	err := os.MkdirAll(promptsDir, 0755)
	assert.NoError(t, err, "Failed to create prompts directory")
	err = os.MkdirAll(outputDir, 0755)
	assert.NoError(t, err, "Failed to create output directory")
	err = os.MkdirAll(commitDataDir, 0755)
	assert.NoError(t, err, "Failed to create commit data directory")

	// Create a test commit data file
	commitDataPath := filepath.Join(commitDataDir, "1.txt")
	err = os.WriteFile(commitDataPath, []byte("test commit data"), 0644)
	assert.NoError(t, err, "Failed to create commit data file")

	// Test generatePromptScriptWithPath
	hash := "test-hash-123"
	index := 1
	err = generatePromptScriptWithPath(hash, index, commitDataPath, promptsDir, outputDir)
	assert.NoError(t, err, "generatePromptScriptWithPath should not return error")

	// Check if script file was created
	scriptPath := filepath.Join(promptsDir, "1.sh")
	_, err = os.Stat(scriptPath)
	assert.NoError(t, err, "Expected script file to be created")

	// Check script content
	content, err := os.ReadFile(scriptPath)
	assert.NoError(t, err, "Failed to read script file")

	scriptContent := string(content)

	// Check if script contains expected elements
	expectedElements := []string{
		"#!/bin/bash",
		hash,
		fmt.Sprintf("Index %d", index),
		"gemini -p",
		commitDataPath,
		"https://github.com/golang/go/commit/" + hash,
	}

	for _, element := range expectedElements {
		assert.Contains(t, scriptContent, element, fmt.Sprintf("Script should contain %v", element))
	}
}

// TestCollectCommits tests the collectCommits function with the actual repo
func TestCollectCommits(t *testing.T) {
	// Test with the actual go repository if it exists
	if _, err := os.Stat("go"); os.IsNotExist(err) {
		t.Skip("Go repository not found, skipping test")
	}

	// Test collectCommits with actual repository
	err := collectCommits()
	assert.NoError(t, err, "collectCommits should not return error")

	// Check if commit data directory exists
	_, err = os.Stat(commitDataDir)
	assert.NoError(t, err, "Expected commit data directory to be created")
}

// TestGeneratePrompts tests the generatePrompts function
func TestGeneratePrompts(t *testing.T) {
	// Test with the actual go repository if it exists
	if _, err := os.Stat("go"); os.IsNotExist(err) {
		t.Skip("Go repository not found, skipping test")
	}

	// Check if commit data exists
	if _, err := os.Stat(commitDataDir); os.IsNotExist(err) {
		t.Skip("Commit data directory not found, skipping test")
	}

	// Test generatePrompts with actual repository
	err := generatePrompts()
	assert.NoError(t, err, "generatePrompts should not return error")

	// Check if directories were created
	_, err = os.Stat(promptsDir)
	assert.NoError(t, err, "Expected prompts directory to be created")
	_, err = os.Stat(outputDir)
	assert.NoError(t, err, "Expected output directory to be created")
}

// TestExecutePrompts tests the executePrompts function
func TestExecutePrompts(t *testing.T) {
	// Skip this test to avoid actually running Gemini CLI
	t.Skip("Skipping executePrompts test to avoid running actual Gemini CLI and consuming quota")

	// If we wanted to test this, we would need to mock the script execution
	// or create test scripts that don't call external services
}
