#!/bin/bash
# Index 14067: 9844e4cd7cc52ecbe7d49307b6546a118de5bea7

echo "🚀 Generating explanation for commit 14067..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/14067.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/14067.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 14067
- **コミットハッシュ**: 9844e4cd7cc52ecbe7d49307b6546a118de5bea7
- **GitHub URL**: https://github.com/golang/go/commit/9844e4cd7cc52ecbe7d49307b6546a118de5bea7

### 章構成

# [インデックス 14067] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[https://github.com/golang/go/commit/9844e4cd7cc52ecbe7d49307b6546a118de5bea7](https://github.com/golang/go/commit/9844e4cd7cc52ecbe7d49307b6546a118de5bea7)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク

EOF
