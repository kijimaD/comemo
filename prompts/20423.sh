#!/bin/bash
# Index 20423: 2034dae94fe8d627c3f746b8a14b4b4c83aecaee

echo "🚀 Generating explanation for commit 20423..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/20423.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/20423.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 20423
- **コミットハッシュ**: 2034dae94fe8d627c3f746b8a14b4b4c83aecaee
- **GitHub URL**: https://github.com/golang/go/commit/2034dae94fe8d627c3f746b8a14b4b4c83aecaee

### 章構成

# [インデックス 20423] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[https://github.com/golang/go/commit/2034dae94fe8d627c3f746b8a14b4b4c83aecaee](https://github.com/golang/go/commit/2034dae94fe8d627c3f746b8a14b4b4c83aecaee)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク

EOF
