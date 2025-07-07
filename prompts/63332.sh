#!/bin/bash
# Index 63332: d0051be847163f1bedb5a5c68f7827f40b4ef4e4

echo "🚀 Generating explanation for commit 63332..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/63332.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/63332.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 63332
- **コミットハッシュ**: d0051be847163f1bedb5a5c68f7827f40b4ef4e4
- **GitHub URL**: https://github.com/golang/go/commit/d0051be847163f1bedb5a5c68f7827f40b4ef4e4

### 章構成

# [インデックス 63332] ファイルの概要

## コミット

[https://github.com/golang/go/commit/d0051be847163f1bedb5a5c68f7827f40b4ef4e4](https://github.com/golang/go/commit/d0051be847163f1bedb5a5c68f7827f40b4ef4e4)

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
