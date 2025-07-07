#!/bin/bash
# Index 47635: 2ebe77a2fda1ee9ff6fd9a3e08933ad1ebaea039

echo "🚀 Generating explanation for commit 47635..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/47635.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/47635.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 47635
- **コミットハッシュ**: 2ebe77a2fda1ee9ff6fd9a3e08933ad1ebaea039
- **GitHub URL**: https://github.com/golang/go/commit/2ebe77a2fda1ee9ff6fd9a3e08933ad1ebaea039

### 章構成

# [インデックス 47635] ファイルの概要

## コミット

[https://github.com/golang/go/commit/2ebe77a2fda1ee9ff6fd9a3e08933ad1ebaea039](https://github.com/golang/go/commit/2ebe77a2fda1ee9ff6fd9a3e08933ad1ebaea039)

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
