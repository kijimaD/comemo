#!/bin/bash
# Index 57226: 47b403ad2dba61e653ec24da7d84cb9decea4939

echo "🚀 Generating explanation for commit 57226..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/57226.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/57226.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 57226
- **コミットハッシュ**: 47b403ad2dba61e653ec24da7d84cb9decea4939
- **GitHub URL**: https://github.com/golang/go/commit/47b403ad2dba61e653ec24da7d84cb9decea4939

### 章構成

# [インデックス 57226] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[https://github.com/golang/go/commit/47b403ad2dba61e653ec24da7d84cb9decea4939](https://github.com/golang/go/commit/47b403ad2dba61e653ec24da7d84cb9decea4939)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク

EOF
