#!/bin/bash
# Index 60693: 660e7d60f28ed4b2c14f6e1fcf6d0e5a7f4aa1e0

echo "🚀 Generating explanation for commit 60693..."

# AI CLIにプロンプトを渡す
# ヒアドキュメントを使い、プロンプトを安全に渡す
{{AI_CLI_COMMAND}} <<'EOF'
これからコミット解説を生成します。以下の指示に厳密に従ってください。

1.  まず、`read_file("commit_data/60693.txt")` を実行して、コミット情報を取得してください。
2.  取得した情報と、以下のメタデータを基に、Web検索も活用して包括的な技術解説をMarkdown形式で生成してください。
3.  下記の「章構成」の全項目を、その順番通りに必ず含めてください。
4.  解説は日本語で、最大限詳細にお願いします。特に背景、前提知識、技術的詳細は深く掘り下げてください。

### メタデータ
- **コミットインデックス**: 60693
- **コミットハッシュ**: 660e7d60f28ed4b2c14f6e1fcf6d0e5a7f4aa1e0
- **GitHub URL**: https://github.com/golang/go/commit/660e7d60f28ed4b2c14f6e1fcf6d0e5a7f4aa1e0

### 章構成

# [インデックス 60693] ファイルの概要

## コミット

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

echo -e "\n✅ Done. Output will be saved automatically to: src/60693.md"
