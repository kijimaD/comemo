#!/bin/bash
# Index %!d(string=7e68f81dd8759ce7cc8a1ff596503f66d6a0eeae): %!s(int=41820)

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/41820.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を**標準出力のみ**に出力してください。ファイル保存は行わないでください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 41820
- **コミットハッシュ**: %!s(int=41820)
- **GitHub URL**: 7e68f81dd8759ce7cc8a1ff596503f66d6a0eeae

### 章構成

# [インデックス %!d(string=https://github.com/golang/go/commit/7e68f81dd8759ce7cc8a1ff596503f66d6a0eeae)] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[%!s(int=41820)](https://github.com/golang/go/commit/7e68f81dd8759ce7cc8a1ff596503f66d6a0eeae)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク
%!(EXTRA string=https://github.com/golang/go/commit/7e68f81dd8759ce7cc8a1ff596503f66d6a0eeae)
EOF
