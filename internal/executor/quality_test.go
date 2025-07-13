package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateGeneratedContent(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "quality_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name          string
		content       string
		expectedPass  bool
		expectedError string
	}{
		{
			name: "valid content",
			content: RequiredTitlePattern + ` 123] ファイルの概要

## 技術的詳細
詳細な説明がここに続きます。この文章は500文字を超えるために十分な長さを持っています。
Goの技術的詳細について説明します。コミットの変更内容を詳しく解説し、
背景や理由についても詳しく述べます。実装の詳細やパフォーマンスへの影響、
互換性についても言及します。これにより、読者は変更の意図と効果を
理解できるようになります。さらに、関連する他の変更や将来の展望についても
説明し、包括的な理解を提供します。

## コアとなるコードの解説
コードの詳細な解説がここに入ります。`,
			expectedPass: true,
		},
		{
			name:          "too short content",
			content:       "短すぎる内容",
			expectedPass:  false,
			expectedError: "ファイルサイズが小さすぎます",
		},
		{
			name: "missing required sections",
			content: `長い内容ですが必要なセクションが含まれていません。
この文章は500文字を超えるために十分な長さを持っています。
しかし、必要なマークダウンセクションが不足しています。
技術的詳細やコードの解説などの重要な部分がありません。
これは品質チェックで失敗するはずです。
さらに文字数を増やすために追加のテキストを含めます。
品質チェックは特定のパターンを探しているため、
それらが見つからない場合は失敗となります。`,
			expectedPass:  false,
			expectedError: "必要なコンテンツパターンが見つかりません",
		},
		{
			name: "contains error message",
			content: RequiredTitlePattern + ` 123] ファイルの概要

## 技術的詳細
API Error: 何かがうまくいきませんでした。この文章は500文字を超えるために十分な長さを持っています。
Goの技術的詳細について説明します。コミットの変更内容を詳しく解説し、
背景や理由についても詳しく述べます。実装の詳細やパフォーマンスへの影響、
互換性についても言及します。これにより、読者は変更の意図と効果を
理解できるようになります。

## コアとなるコードの解説
コードの詳細な解説がここに入ります。`,
			expectedPass:  false,
			expectedError: "コンテンツにエラー表現が含まれています",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, tt.name+".md")
			err := os.WriteFile(testFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatal(err)
			}

			// Test validation
			result, err := ValidateGeneratedContent(testFile)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result.Passed != tt.expectedPass {
				t.Errorf("Expected pass=%v, got pass=%v", tt.expectedPass, result.Passed)
			}

			if !tt.expectedPass && tt.expectedError != "" {
				if result.FailureReason == "" {
					t.Errorf("Expected failure reason to contain '%s', got empty", tt.expectedError)
				}
			}

			if result.FileContent != tt.content {
				t.Errorf("FileContent mismatch")
			}
		})
	}
}
