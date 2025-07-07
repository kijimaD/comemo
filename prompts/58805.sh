#!/bin/bash
# Index 58805: 6396a42b99daab77f6207f6c3011f7ffd7bbe173

echo "🚀 Generating explanation for commit 58805..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/58805.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/58805.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 58805
- **コミットハッシュ**: 6396a42b99daab77f6207f6c3011f7ffd7bbe173
- **GitHub URL**: https://github.com/golang/go/commit/6396a42b99daab77f6207f6c3011f7ffd7bbe173

### 章構成

# [インデックス 58805] ファイルの概要

## コミット

[https://github.com/golang/go/commit/6396a42b99daab77f6207f6c3011f7ffd7bbe173](https://github.com/golang/go/commit/6396a42b99daab77f6207f6c3011f7ffd7bbe173)

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
