#!/bin/bash
# Index 34941: a631daba5fe9d91ebdddd4148e8af82e07c1ae3e

echo "🚀 Generating explanation for commit 34941..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/34941.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を**標準出力のみ**に出力してください。ファイル保存は行わないでください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 34941
- **コミットハッシュ**: %!s(int=34941)
- **GitHub URL**: a631daba5fe9d91ebdddd4148e8af82e07c1ae3e

### 章構成

# [インデックス %!d(string=https://github.com/golang/go/commit/a631daba5fe9d91ebdddd4148e8af82e07c1ae3e)] ファイルの概要

## コミット

## GitHub上でのコミットページへのリンク

[%!s(int=34941)](https://github.com/golang/go/commit/a631daba5fe9d91ebdddd4148e8af82e07c1ae3e)

## 元コミット内容

## 変更の背景

## 前提知識の解説

## 技術的詳細

## コアとなるコードの変更箇所

## コアとなるコードの解説

## 関連リンク

## 参考にした情報源リンク
%!(EXTRA string=https://github.com/golang/go/commit/a631daba5fe9d91ebdddd4148e8af82e07c1ae3e)
EOF
