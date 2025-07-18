#!/bin/bash
# Index 66122: 35c0ea22a94aa5ad447bf640c4f7388d3f1d75eb

echo "🚀 Generating explanation for commit 66122..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/66122.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/66122.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 66122
- **コミットハッシュ**: 35c0ea22a94aa5ad447bf640c4f7388d3f1d75eb
- **GitHub URL**: https://github.com/golang/go/commit/35c0ea22a94aa5ad447bf640c4f7388d3f1d75eb

### 章構成

# [インデックス 66122] ファイルの概要

## コミット

[https://github.com/golang/go/commit/35c0ea22a94aa5ad447bf640c4f7388d3f1d75eb](https://github.com/golang/go/commit/35c0ea22a94aa5ad447bf640c4f7388d3f1d75eb)

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
