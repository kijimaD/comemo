#!/bin/bash
# Index 22486: db96e6821ad7dab72b459165a66c6dbb08160eda

echo "🚀 Generating explanation for commit 22486..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/22486.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/22486.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 22486
- **コミットハッシュ**: db96e6821ad7dab72b459165a66c6dbb08160eda
- **GitHub URL**: https://github.com/golang/go/commit/db96e6821ad7dab72b459165a66c6dbb08160eda

### 章構成

# [インデックス 22486] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[https://github.com/golang/go/commit/db96e6821ad7dab72b459165a66c6dbb08160eda](https://github.com/golang/go/commit/db96e6821ad7dab72b459165a66c6dbb08160eda)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク

EOF
