#!/bin/bash
# Index 47753: 4d7d7a4c50ea0d16879458a032f1a05db2c7ae4f

echo "🚀 Generating explanation for commit 47753..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、"/home/violet/Project/comemo/commit_data/47753.txt" を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 47753
- **コミットハッシュ**: 4d7d7a4c50ea0d16879458a032f1a05db2c7ae4f
- **GitHub URL**: https://github.com/golang/go/commit/4d7d7a4c50ea0d16879458a032f1a05db2c7ae4f

### 章構成

# [インデックス 47753] ファイルの概要

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

echo -e "\n✅ Done. Output will be saved automatically to: src/47753.md"
