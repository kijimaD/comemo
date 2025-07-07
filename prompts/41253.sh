#!/bin/bash
# Index 41253: 7987238d9cfcef5f79ccd9458e59e22d8a8b3cf2

echo "🚀 Generating explanation for commit 41253..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/41253.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  **必須**: 生成した解説を ./src/41253.md というファイル名で保存してください。この手順は省略できません。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。
6.  **確認**: ファイル作成が完了したら「ファイル ./src/41253.md を作成しました」と出力してください。

**重要**: 必ず最後に ./src/%!d(string=7987238d9cfcef5f79ccd9458e59e22d8a8b3cf2).md ファイルを作成してください。ファイル作成は必須です。

### メタデータ
- **コミットインデックス**: %!d(string=https://github.com/golang/go/commit/7987238d9cfcef5f79ccd9458e59e22d8a8b3cf2)
- **コミットハッシュ**: %!s(int=41253)
- **GitHub URL**: https://github.com/golang/go/commit/7987238d9cfcef5f79ccd9458e59e22d8a8b3cf2

### 章構成

# [インデックス %!d(string=https://github.com/golang/go/commit/7987238d9cfcef5f79ccd9458e59e22d8a8b3cf2)] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[%!s(MISSING)](%!s(MISSING))

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク

EOF
