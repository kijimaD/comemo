#!/bin/bash
set -eux

###########
# comment
###########

cd `dirname $0`
cd ..

echo "# Summary" > src/SUMMARY.md
for file in $(find src/ -name '*.md' | sort); do
    title=$(head -1 "$file" | sed 's/^#* //')
    relpath=${file#src/}
    echo "- [$title]($relpath)" >> src/SUMMARY.md
done
