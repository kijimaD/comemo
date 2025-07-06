package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// RunCommand executes a git command in the specified directory
func RunCommand(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git command failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// GetCommitHashes retrieves all commit hashes from the repository
func GetCommitHashes(repoPath string) ([]string, error) {
	output, err := RunCommand(repoPath, "rev-list", "--all", "--reverse")
	if err != nil {
		return nil, fmt.Errorf("error running git rev-list: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	hashes := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			hashes = append(hashes, trimmed)
		}
	}

	return hashes, nil
}

// GetCommitData retrieves detailed commit information
func GetCommitData(repoPath string, hash string) (string, error) {
	return RunCommand(repoPath, "show", "--patch-with-stat", hash)
}

// GetCommitIndex finds the index of a commit hash in the list
func GetCommitIndex(allHashes []string, targetHash string) int {
	for i, hash := range allHashes {
		if hash == targetHash {
			return i + 1
		}
	}
	return 0
}
