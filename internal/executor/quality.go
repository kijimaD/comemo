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
		"# [インデックス",
		"## コミット",
		"## GitHub上でのコミットページへのリンク",
		"## 元コミット内容",
		"## 技術的詳細",
		"## コアとなるコードの解説",
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
	// 通常の文章に含まれないように注意する
	errorIndicators := []string{
		"API Error",
		"Resource exhausted. Please try again later",
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
