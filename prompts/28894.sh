#!/bin/bash
# Index 28894: 297d1d736e3e6db6fa390dd54b1e3de9ea8f1fba

echo "🚀 Generating explanation for commit 28894..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、`read_file("commit_data/28894.txt")` を実行して、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 28894
- **コミットハッシュ**: 297d1d736e3e6db6fa390dd54b1e3de9ea8f1fba
- **GitHub URL**: https://github.com/golang/go/commit/297d1d736e3e6db6fa390dd54b1e3de9ea8f1fba

### 章構成

# [インデックス 28894] ファイルの概要

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

EOF

echo -e "\n✅ Done. Output will be saved automatically to: src/28894.md"
