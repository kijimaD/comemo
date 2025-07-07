#!/bin/bash
# Index %!d(string=b08a3164c0a10b608db0e9fafcdc3c9168e3cffe): %!s(int=16195)

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/16195.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を**標準出力のみ**に出力してください。ファイル保存は行わないでください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 16195
- **コミットハッシュ**: %!s(int=16195)
- **GitHub URL**: b08a3164c0a10b608db0e9fafcdc3c9168e3cffe

### 章構成

# [インデックス %!d(string=https://github.com/golang/go/commit/b08a3164c0a10b608db0e9fafcdc3c9168e3cffe)] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[%!s(int=16195)](https://github.com/golang/go/commit/b08a3164c0a10b608db0e9fafcdc3c9168e3cffe)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク
%!(EXTRA string=https://github.com/golang/go/commit/b08a3164c0a10b608db0e9fafcdc3c9168e3cffe)
EOF
