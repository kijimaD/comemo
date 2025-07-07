#!/bin/bash
# Index 46451: f25fd9bf57c9a082b1dba2a75e840f0caef0bac8

echo "🚀 Generating explanation for commit 46451..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/46451.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/46451.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 46451
- **コミットハッシュ**: f25fd9bf57c9a082b1dba2a75e840f0caef0bac8
- **GitHub URL**: https://github.com/golang/go/commit/f25fd9bf57c9a082b1dba2a75e840f0caef0bac8

### 章構成

# [インデックス 46451] ファイルの概要

## コミット

[https://github.com/golang/go/commit/f25fd9bf57c9a082b1dba2a75e840f0caef0bac8](https://github.com/golang/go/commit/f25fd9bf57c9a082b1dba2a75e840f0caef0bac8)

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
