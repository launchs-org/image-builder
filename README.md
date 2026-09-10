# image-builder

git/zip から取得したソースを railpack または Dockerfile でビルドし、
OCI イメージとしてレジストリに push する検証用ツール一式。

## 構成

- `builder/`: ソース取得 → ビルド → push を行う CLI 本体
- `buildkit`: railpack/dockerfile ビルドを実行する BuildKit デーモン
- `registry/`: push 先の OCI レジストリ ([distribution/distribution](https://github.com/distribution/distribution)) の設定
- `auth-server/`: レジストリの push/pull を認証する短命 JWT 発行サーバー (検証用、詳細は [auth-server/README.md](auth-server/README.md))

## 起動手順

### 1. 認証用の鍵・証明書を用意する (初回のみ)

registry と auth-server で共有する証明書を openssl で作成する。

```bash
cd auth-server
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout certs/token-signing.key \
  -out certs/token-signing.crt \
  -days 36500 \
  -subj "/CN=image-builder-auth-server"
cd ..
```

### 2. registry と auth-server を起動する

```bash
docker compose up -d --build registry auth-server
```

### 3. push 用の JWT を取得する

push 先イメージ参照 (例: `project/app`) に対する scope で、10分間有効なトークンを取得する。

```bash
curl "http://localhost:8000/token?service=registry&scope=repository:project/app:pull,push"
```

レスポンスの `token` フィールドの値を、`docker-compose.yaml` の `builder.command` にある
`-registry-token=...` に差し替える。

### 4. builder を実行する

```bash
docker compose up --build builder
```

ビルド対象・ビルド方式・push 先は `docker-compose.yaml` の `builder.command` (CLI 引数) で指定する。

### 5. push されたイメージを確認する (任意)

```bash
TOKEN=$(curl -sS "http://localhost:8000/token?service=registry&scope=repository:project/app:pull" | python3 -c 'import json,sys;print(json.load(sys.stdin)["token"])')
curl -sS -H "Authorization: Bearer $TOKEN" http://localhost:5000/v2/project/app/tags/list
```

## 後片付け

```bash
docker compose down
```
