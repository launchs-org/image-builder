# builder イメージ

ソースを git/zip で取得し、railpack または Dockerfile でビルドして、
レジストリに push まで行う CLI。ghcr.io に公開されているイメージをそのまま利用できる。

```
ghcr.io/launchs-org/image-builder/builder:latest
```

## 前提

ビルドの実行には別途 BuildKit デーモンが必要。`BUILDKIT_HOST` 環境変数でエンドポイントを指定する。

```bash
docker run -d --name buildkit --privileged \
  moby/buildkit:v0.27.0 --addr tcp://0.0.0.0:1234
```

## 使い方

ビルド対象・ビルド方式・push 先はすべて CLI 引数で指定する。

```bash
docker run --rm \
  --link buildkit \
  -e BUILDKIT_HOST=tcp://buildkit:1234 \
  -v "$(pwd)/output:/app/output" \
  ghcr.io/launchs-org/image-builder/builder:latest \
  -source-type=git \
  -git-url=https://github.com/launchs-org/sample-go-app \
  -git-branch=main \
  -build-method=railpack
```

ビルド結果は OCI image layout として `/app/output/image` (コンテナ内) に出力される。
ホストから参照する場合は上記のように `-v` でボリュームをマウントする。

### push する場合

```bash
docker run --rm \
  --link buildkit \
  -e BUILDKIT_HOST=tcp://buildkit:1234 \
  ghcr.io/launchs-org/image-builder/builder:latest \
  -source-type=git \
  -git-url=https://github.com/launchs-org/sample-go-app \
  -git-branch=main \
  -build-method=railpack \
  -push \
  -image-ref=registry.example.com/project/app:latest \
  -registry-username=... \
  -registry-password=...
```

## CLI 引数一覧

### ソース取得

| 引数 | 説明 |
|---|---|
| `-source-type` | `git` または `zip` (デフォルト: `git`) |
| `-git-url` | クローンする Git リポジトリの URL (`source-type=git` のとき必須) |
| `-git-branch` | チェックアウトするブランチ名 (省略時: 既定ブランチ) |
| `-git-commit` | チェックアウトするコミットハッシュ (省略時: 最新) |
| `-zip-url` | ダウンロードする zip の URL (`source-type=zip` のとき必須, https のみ) |
| `-zip-sha256` | zip の sha256 ハッシュ (省略時は検証しない) |

### ビルド方式

| 引数 | 説明 |
|---|---|
| `-build-method` | `railpack` または `dockerfile` (デフォルト: `railpack`) |

### push

| 引数 | 説明 |
|---|---|
| `-push` | ビルドしたイメージをレジストリに push する |
| `-image-ref` | push 先イメージ参照 (例: `registry.example.com/project/app:latest`)。`-push` 指定時は必須 |
| `-registry-username` / `-registry-password` | Basic 認証情報 (省略可) |
| `-registry-token` | Bearer トークン認証 (事前に取得した JWT など)。指定時は username/password より優先 |
| `-registry-insecure` | TLS 証明書検証を行わず、HTTP でのアクセスも許可する (自己署名証明書やローカル検証用) |

### 環境変数

| 変数 | 説明 |
|---|---|
| `BUILDKIT_HOST` | BuildKit デーモンのエンドポイント (例: `tcp://buildkit:1234`) |
