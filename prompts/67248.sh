#!/bin/bash
# Index 67248: ed08d2ad0928c0fc77cc2053863616ffb58c5aac

echo "🚀 Generating explanation for commit 67248..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/67248.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/67248.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 67248
- **コミットハッシュ**: ed08d2ad0928c0fc77cc2053863616ffb58c5aac
- **GitHub URL**: https://github.com/golang/go/commit/ed08d2ad0928c0fc77cc2053863616ffb58c5aac

### 章構成

# [インデックス 67248] ファイルの概要

## コミット

[https://github.com/golang/go/commit/ed08d2ad0928c0fc77cc2053863616ffb58c5aac](https://github.com/golang/go/commit/ed08d2ad0928c0fc77cc2053863616ffb58c5aac)

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
