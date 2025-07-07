#!/bin/bash
# Index 46466: bfa97ba48fa2924d9c2da1dca01fdb65b44cdb5f

echo "🚀 Generating explanation for commit 46466..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/46466.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/46466.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 46466
- **コミットハッシュ**: bfa97ba48fa2924d9c2da1dca01fdb65b44cdb5f
- **GitHub URL**: https://github.com/golang/go/commit/bfa97ba48fa2924d9c2da1dca01fdb65b44cdb5f

### 章構成

# [インデックス 46466] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[https://github.com/golang/go/commit/bfa97ba48fa2924d9c2da1dca01fdb65b44cdb5f](https://github.com/golang/go/commit/bfa97ba48fa2924d9c2da1dca01fdb65b44cdb5f)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク

EOF
