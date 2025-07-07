#!/bin/bash
# Index 49535: de83ef67acaaf5c2ce12dd831e8d3d04f02a6fc9

echo "🚀 Generating explanation for commit 49535..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/49535.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  **必須**: 生成した解説を ./src/49535.md というファイル名で保存してください。この手順は省略できません。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。
6.  **確認**: ファイル作成が完了したら「ファイル ./src/49535.md を作成しました」と出力してください。

**重要**: 必ず最後に ./src/%!d(string=de83ef67acaaf5c2ce12dd831e8d3d04f02a6fc9).md ファイルを作成してください。ファイル作成は必須です。

### メタデータ
- **コミットインデックス**: %!d(string=https://github.com/golang/go/commit/de83ef67acaaf5c2ce12dd831e8d3d04f02a6fc9)
- **コミットハッシュ**: %!s(int=49535)
- **GitHub URL**: https://github.com/golang/go/commit/de83ef67acaaf5c2ce12dd831e8d3d04f02a6fc9

### 章構成

# [インデックス %!d(string=https://github.com/golang/go/commit/de83ef67acaaf5c2ce12dd831e8d3d04f02a6fc9)] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[%!s(MISSING)](%!s(MISSING))

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク

EOF
