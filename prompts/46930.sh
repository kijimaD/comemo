#!/bin/bash
# Index 46930: 813b6bbaf9e24e6e11accca0cb4e7b7e8fb8db8d

echo "🚀 Generating explanation for commit 46930..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 commit_data/46930.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
  - 形式は ./src/{コミットインデックス}.md でお願いします
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### 章構成

# [インデックス 46930] ファイルの概要

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
%!(EXTRA string=813b6bbaf9e24e6e11accca0cb4e7b7e8fb8db8d, string=https://github.com/golang/go/commit/813b6bbaf9e24e6e11accca0cb4e7b7e8fb8db8d, int=46930)
EOF
