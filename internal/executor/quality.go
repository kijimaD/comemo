package executor

import (
	"fmt"
	"os"
	"strings"
)

// QualityCheckResult represents the result of a quality check
type QualityCheckResult struct {
	Passed        bool
	FailureReason string
	FileContent   string
}

// ValidateGeneratedContent performs quality validation on AI-generated content
func ValidateGeneratedContent(outputPath string) (*QualityCheckResult, error) {
	// Read the generated file
	fileContent, err := os.ReadFile(outputPath)
	if err != nil {
		return &QualityCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("ファイル読み込みエラー: %v", err),
		}, err
	}

	fileContentStr := string(fileContent)

	// Check file size - should be substantial
	if len(fileContentStr) < 500 {
		return &QualityCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("ファイルサイズが小さすぎます (実際: %d文字, 最小: 500文字)", len(fileContentStr)),
			FileContent:   fileContentStr,
		}, nil
	}

	// Check for required content patterns
	requiredPatterns := []string{
		"## コアとなるコードの解説",
		"## 技術的詳細",
		"# [インデックス",
	}

	foundValidContent := false
	for _, pattern := range requiredPatterns {
		if strings.Contains(fileContentStr, pattern) {
			foundValidContent = true
			break
		}
	}

	if !foundValidContent {
		return &QualityCheckResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("必要なコンテンツパターンが見つかりません。必要: %v", requiredPatterns),
			FileContent:   fileContentStr,
		}, nil
	}

	// Additional quality checks can be added here

	// Check for obvious error messages in content
	errorIndicators := []string{
		"error:",
		"エラー:",
		"失敗しました",
		"cannot",
		"unable to",
		"not found",
		"404",
	}

	for _, indicator := range errorIndicators {
		if strings.Contains(strings.ToLower(fileContentStr), strings.ToLower(indicator)) {
			return &QualityCheckResult{
				Passed:        false,
				FailureReason: fmt.Sprintf("コンテンツにエラー表現が含まれています: %s", indicator),
				FileContent:   fileContentStr,
			}, nil
		}
	}

	return &QualityCheckResult{
		Passed:      true,
		FileContent: fileContentStr,
	}, nil
}

// ValidateGeneratedContentForCLI performs CLI-specific quality validation
func ValidateGeneratedContentForCLI(outputPath, cliName, outputStr string) (*QualityCheckResult, error) {
	// First try to validate from file
	result, err := ValidateGeneratedContent(outputPath)

	// If file doesn't exist and we have output from CLI that doesn't create files (like Gemini)
	if err != nil && cliName == "gemini" && len(outputStr) > 100 {
		// Save the output to file for validation
		if writeErr := os.WriteFile(outputPath, []byte(outputStr), 0644); writeErr != nil {
			return &QualityCheckResult{
				Passed:        false,
				FailureReason: fmt.Sprintf("ファイル作成エラー: %v", writeErr),
			}, writeErr
		}
		// Retry validation
		return ValidateGeneratedContent(outputPath)
	}

	return result, err
}
