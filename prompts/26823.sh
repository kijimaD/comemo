#!/bin/bash
# Index 26823: 7da4ceddd020a0f1de0b9a0af56a931f70a8bb08

echo "🚀 Generating explanation for commit 26823..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/26823.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/26823.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 26823
- **コミットハッシュ**: 7da4ceddd020a0f1de0b9a0af56a931f70a8bb08
- **GitHub URL**: https://github.com/golang/go/commit/7da4ceddd020a0f1de0b9a0af56a931f70a8bb08

### 章構成

# [インデックス 26823] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[https://github.com/golang/go/commit/7da4ceddd020a0f1de0b9a0af56a931f70a8bb08](https://github.com/golang/go/commit/7da4ceddd020a0f1de0b9a0af56a931f70a8bb08)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク

EOF
