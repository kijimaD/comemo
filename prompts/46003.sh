#!/bin/bash
# Index 46003: f26f227f66ff4113cad0cd51c0780e5849e9accc

echo "🚀 Generating explanation for commit 46003..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/46003.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を**標準出力のみ**に出力してください。ファイル保存は行わないでください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 46003
- **コミットハッシュ**: %!s(int=46003)
- **GitHub URL**: f26f227f66ff4113cad0cd51c0780e5849e9accc

### 章構成

# [インデックス %!d(string=https://github.com/golang/go/commit/f26f227f66ff4113cad0cd51c0780e5849e9accc)] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[%!s(int=46003)](https://github.com/golang/go/commit/f26f227f66ff4113cad0cd51c0780e5849e9accc)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク
%!(EXTRA string=https://github.com/golang/go/commit/f26f227f66ff4113cad0cd51c0780e5849e9accc)
EOF
