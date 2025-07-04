#!/bin/bash
# Index 35215: 16d1a8e6e3701a8ed2e9b16f7c9b8e70a7b1fc8d

echo "🚀 Generating explanation for commit 35215..."

# Gemini CLIにプロンプトを渡す (実際のCLIコマンド名に要変更)
# ヒアドキュメントを使い、プロンプトを安全に渡す
gemini -p <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、`read_file("commit_data/35215.txt")` を実行して、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 35215
- **コミットハッシュ**: 16d1a8e6e3701a8ed2e9b16f7c9b8e70a7b1fc8d
- **GitHub URL**: https://github.com/golang/go/commit/16d1a8e6e3701a8ed2e9b16f7c9b8e70a7b1fc8d

### 章構成

# [インデックス 35215] ファイルの概要

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

echo -e "\n✅ Done. Copy the output above and save it as: src/35215.md"
