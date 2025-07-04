#!/bin/bash
# Index 14936: 2bddbf5e8f864890f5a8cda1a5e00dbf04b4f7e9

echo "🚀 Generating explanation for commit 14936..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、"commit_data/14936.txt" を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 14936
- **コミットハッシュ**: 2bddbf5e8f864890f5a8cda1a5e00dbf04b4f7e9
- **GitHub URL**: https://github.com/golang/go/commit/2bddbf5e8f864890f5a8cda1a5e00dbf04b4f7e9

### 章構成

# [インデックス 14936] ファイルの概要

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

echo -e "\n✅ Done. Output will be saved automatically to: src/14936.md"
