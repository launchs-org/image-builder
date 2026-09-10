// 検証用の JWT 発行サーバー。トークン発行ロジックは tokenissuer パッケージを使う。
//
// 署名鍵は事前に openssl で用意しておくこと (README 参照)。
// レジストリ側は auth.token.rootcertbundle にこの鍵の証明書を指定することで、
// 発行された JWT の署名を検証できる。
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"auth-server/tokenissuer"
)

func main() {
	addr := flag.String("addr", ":8000", "listen address")
	issuerName := flag.String("issuer", "image-builder-auth-server", "JWT の iss クレーム")
	service := flag.String("service", "registry", "JWT の aud クレーム (デフォルトのサービス名)")
	ttl := flag.Duration("ttl", 10*time.Minute, "発行するトークンの有効期間")
	keyPath := flag.String("key", "./certs/token-signing.key", "署名用の秘密鍵 (PEM, PKCS1/PKCS8)")
	certPath := flag.String("cert", "./certs/token-signing.crt", "署名鍵に対応する証明書 (PEM)。レジストリの rootcertbundle と同じもの")
	flag.Parse()

	issuer, err := tokenissuer.New(*issuerName, *service, *ttl, *keyPath, *certPath)
	if err != nil {
		log.Fatalf("トークン発行器の初期化に失敗しました (openssl で鍵・証明書を事前に用意してください): %v", err)
	}

	http.HandleFunc("/token", tokenHandler(issuer))

	log.Printf("auth-server listening on %s (issuer=%s service=%s ttl=%s)", *addr, *issuerName, *service, *ttl)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

// GET /token?service=<service>&scope=repository:<name>:<actions>[&scope=...]
//
// 検証用サーバーのため認証情報のチェックは一切行わず、
// リクエストされた scope をそのまま許可した JWT を返す。
func tokenHandler(issuer *tokenissuer.Issuer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		access := tokenissuer.ParseScopes(r.URL.Query()["scope"])

		token, expiresIn, err := issuer.Issue(r.URL.Query().Get("service"), access)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			log.Printf("トークンの発行に失敗しました: %v", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"token":      token,
			"expires_in": int(expiresIn.Seconds()),
			"issued_at":  time.Now(),
		})
	}
}
