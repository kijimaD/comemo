#!/bin/bash
# Index 9745: 514c9243f21fb8ec1df73ac63717d10e3136afa2

echo "🚀 Generating explanation for commit 9745..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/9745.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/9745.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 9745
- **コミットハッシュ**: 514c9243f21fb8ec1df73ac63717d10e3136afa2
- **GitHub URL**: https://github.com/golang/go/commit/514c9243f21fb8ec1df73ac63717d10e3136afa2

### 章構成

# [インデックス 9745] ファイルの概要

## コミット

[https://github.com/golang/go/commit/514c9243f21fb8ec1df73ac63717d10e3136afa2](https://github.com/golang/go/commit/514c9243f21fb8ec1df73ac63717d10e3136afa2)

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
