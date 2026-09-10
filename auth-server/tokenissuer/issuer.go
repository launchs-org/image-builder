// Package tokenissuer は distribution (レジストリ) の Token Authentication 仕様
// (https://distribution.github.io/distribution/spec/auth/token/) に沿った
// push/pull 用の JWT を発行するライブラリ。
package tokenissuer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Issuer は署名鍵・証明書を保持し、JWT を発行する
type Issuer struct {
	IssuerName string        // JWT の iss クレーム
	Service    string        // JWT の aud クレーム (デフォルトのサービス名)
	TTL        time.Duration // 発行するトークンの有効期間

	key     *rsa.PrivateKey
	certDER []byte
}

// New は鍵・証明書ファイル (PEM) を読み込み、Issuer を作る
func New(issuerName, service string, ttl time.Duration, keyPath, certPath string) (*Issuer, error) {
	key, err := loadPrivateKey(keyPath)
	if err != nil {
		return nil, err
	}
	certDER, err := loadCertDER(certPath)
	if err != nil {
		return nil, err
	}
	return &Issuer{IssuerName: issuerName, Service: service, TTL: ttl, key: key, certDER: certDER}, nil
}

// Access は JWT の access クレームの1要素
type Access struct {
	Type    string
	Name    string
	Actions []string
}

// Issue は指定された service/access を持つ JWT を発行する。
// service が空の場合は Issuer.Service を使う。
func (i *Issuer) Issue(service string, access []Access) (token string, expiresIn time.Duration, err error) {
	if service == "" {
		service = i.Service
	}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":    i.IssuerName,
		"sub":    "anonymous",
		"aud":    service,
		"exp":    now.Add(i.TTL).Unix(),
		"nbf":    now.Unix(),
		"iat":    now.Unix(),
		"access": accessClaims(access),
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	// レジストリ (distribution) は JWT ヘッダーの x5c (証明書チェーン) を
	// rootcertbundle と突き合わせて署名鍵を検証するため必須。
	jwtToken.Header["x5c"] = []string{base64.StdEncoding.EncodeToString(i.certDER)}

	signed, err := jwtToken.SignedString(i.key)
	if err != nil {
		return "", 0, err
	}
	return signed, i.TTL, nil
}

// ParseScopes は scope クエリパラメータ ("repository:name:pull,push" 形式) を
// Access のスライスに変換する
func ParseScopes(scopes []string) []Access {
	result := make([]Access, 0, len(scopes))
	for _, scope := range scopes {
		parts := strings.SplitN(scope, ":", 3)
		if len(parts) != 3 {
			continue
		}
		result = append(result, Access{
			Type:    parts[0],
			Name:    parts[1],
			Actions: strings.Split(parts[2], ","),
		})
	}
	return result
}

func accessClaims(access []Access) []map[string]any {
	claims := make([]map[string]any, 0, len(access))
	for _, a := range access {
		claims = append(claims, map[string]any{
			"type":    a.Type,
			"name":    a.Name,
			"actions": a.Actions,
		})
	}
	return claims
}

// レジストリの rootcertbundle と同じ証明書を DER 形式で読み込む (x5c ヘッダー用)
func loadCertDER(certPath string) ([]byte, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, err
	}
	return block.Bytes, nil
}

func loadPrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, err
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return parsed.(*rsa.PrivateKey), nil
}
