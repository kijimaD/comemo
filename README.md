golang/goのコミットを生成AIに解説させた。正確性は検証してない。

## TODO

- [x] git submoduleを設定してリポジトリを持ってくる
- [x] より正確になるようなフロー設計する
- [ ] 定期実行する
- [ ] リンク化する
- [ ] ページタイトルをコミット名にする
- [ ] Claude Codeだとリミットが来ても止まらない

## run

```shell
docker build . -t comemo
docker run -d -v "$PWD/":/work -w /work -p 3003:3003 --name comemo-server --restart always comemo bash -c "mdbook serve -p 3003 -n 0.0.0.0"
```

## submodule

```shell
git submodule update --init
git submodule update --remote
```
