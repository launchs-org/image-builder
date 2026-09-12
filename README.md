# image-builder

git/zip から取得したソースを railpack または Dockerfile でビルドし、
OCI イメージとしてレジストリに push する CLI ツール。

## 構成

- `builder/`: ソース取得 → ビルド → push を行う CLI 本体 (ghcr.io に公開されるのはこれのみ、使い方は [builder/README.md](builder/README.md) 参照)
- `buildkit`: railpack/dockerfile ビルドを実行する BuildKit デーモン
- `auth-server/`: distribution レジストリの token auth (JWT push認証) のサンプル実装 (詳細は [auth-server/README.md](auth-server/README.md))。builder の実行には不要で、認証方式を検討する際の参考実装として置いている

## 起動手順

```bash
docker compose up --build builder
```

ビルド対象・ビルド方式・push 先は `docker-compose.yaml` の `builder.command` (CLI 引数) で指定する。

成果物は **push する** か **tar.gz として出力する (`-output-tar`)** のどちらか一方のみで、
ディレクトリに展開した状態で残すことはできない。

push する場合は `-push` と `-image-ref` に加えて、以下のいずれかで認証情報を指定する。

- `-registry-username` / `-registry-password` (Basic認証)
- `-registry-token` (Bearer トークン。事前に取得した JWT など。指定時は username/password より優先)

自己署名証明書のレジストリや HTTP のみのレジストリを使う場合は `-registry-insecure` を付ける。

詳しい CLI 引数は [builder/README.md](builder/README.md) を参照。

## 後片付け

```bash
docker compose down
```

## ビルド済みイメージ (ghcr.io)

`master` への push をトリガーに GitHub Actions ([.github/workflows/publish-images.yml](.github/workflows/publish-images.yml)) が
`builder` イメージをビルドし ghcr.io に公開する。

```
ghcr.io/launchs-org/image-builder/builder:latest
```
