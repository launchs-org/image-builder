# auth-server (検証用)

distribution レジストリの push/pull を、10分だけ有効な JWT で認証させるための検証用トークン発行サーバー。
[Docker Registry Token Authentication](https://distribution.github.io/distribution/spec/auth/token/) 仕様に沿った `/token` エンドポイントのみを持つ。認証情報のチェックは行わず、リクエストされた scope をそのまま許可する。

## 事前準備: 署名鍵と証明書の作成

registry と auth-server で共有する `./certs` 以下に、openssl で自己署名証明書を作成する。

```bash
mkdir -p certs
openssl req -x509 -newkey rsa:4096 -nodes \
  -keyout certs/token-signing.key \
  -out certs/token-signing.crt \
  -days 36500 \
  -subj "/CN=image-builder-auth-server"
```

(`-days` は省略できず必須の引数だが、`36500` (100年) 指定で検証用途としては実質恒久的に扱える)

- `certs/token-signing.key`: auth-server がトークンの署名に使う秘密鍵
- `certs/token-signing.crt`: registry の `auth.token.rootcertbundle` に指定する証明書 (この証明書の公開鍵で JWT の署名を検証する)

## トークンの取得例

```bash
curl "http://localhost:8000/token?service=registry&scope=repository:project/app:pull,push"
```

## ライブラリとして使う

トークン発行ロジックは `tokenissuer` パッケージに切り出してあり、他の Go プログラムからも呼び出せる。

```go
import "auth-server/tokenissuer"

issuer, err := tokenissuer.New(
    "image-builder-auth-server", // iss
    "registry",                  // aud (デフォルトのサービス名)
    10*time.Minute,               // TTL
    "./certs/token-signing.key",
    "./certs/token-signing.crt",
)

token, expiresIn, err := issuer.Issue("registry", []tokenissuer.Access{
    {Type: "repository", Name: "project/app", Actions: []string{"pull", "push"}},
})
```
