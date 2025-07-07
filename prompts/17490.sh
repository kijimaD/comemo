#!/bin/bash
# Index 17490: 5f1af1608f11f58edad85445bde2c96f5a3157fe

echo "🚀 Generating explanation for commit 17490..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、 ./commit_data/17490.txt を開いて、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  生成した解説を ./src/17490.md というファイル名で保存してください。
4.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
5.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 17490
- **コミットハッシュ**: 5f1af1608f11f58edad85445bde2c96f5a3157fe
- **GitHub URL**: https://github.com/golang/go/commit/5f1af1608f11f58edad85445bde2c96f5a3157fe

### 章構成

# [インデックス 17490] ファイルの概要

## コミット

[https://github.com/golang/go/commit/5f1af1608f11f58edad85445bde2c96f5a3157fe](https://github.com/golang/go/commit/5f1af1608f11f58edad85445bde2c96f5a3157fe)

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
