// Package registry は go-containerregistry (crane) を使って、
// OCI image layout ディレクトリの内容をコンテナレジストリに push するための
// 薄いラッパーです。
//
// railpack パッケージ・dockerfile パッケージのビルド結果 (buildctl の
// --output type=oci,dest=<dir>,tar=false で出力した OCI image layout
// ディレクトリ) をそのまま入力として受け取る。
package registry

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
)

// push 先レジストリの設定
type PushArgs struct {
	// push するイメージの OCI image layout ディレクトリ (必須)
	// (railpack.BuildArgs.OutputDirectory / dockerfile.BuildArgs.OutputDirectory の出力先と同じ)
	ImageDirectory string

	// push 先イメージ参照 (必須, 例: "registry.example.com/project/app:latest")
	ImageReference string

	// レジストリ認証のユーザー名 (省略時は匿名アクセス、または環境の認証情報を使用)
	Username string

	// レジストリ認証のパスワード (Username とセットで指定する)
	Password string

	// true の場合、TLS 証明書検証を行わず、HTTP でのアクセスも許可する
	// (自己署名証明書のレジストリやローカル検証用のプレーンHTTPレジストリ向け)
	Insecure bool
}

/*
ImageDirectory (OCI image layout) を読み込み、ImageReference へ push する関数
*/
func Push(ctx context.Context, args PushArgs) error {
	if args.ImageDirectory == "" || args.ImageReference == "" {
		return fmt.Errorf("invalid option: ImageDirectory and ImageReference are required")
	}

	img, err := loadImage(args.ImageDirectory)
	if err != nil {
		return fmt.Errorf("OCI image layout の読み込みに失敗しました: %w", err)
	}

	opts := []crane.Option{crane.WithContext(ctx)}
	if args.Username != "" {
		opts = append(opts, crane.WithAuth(&authn.Basic{
			Username: args.Username,
			Password: args.Password,
		}))
	}
	if args.Insecure {
		opts = append(opts, crane.Insecure)
	}

	if err := crane.Push(img, args.ImageReference, opts...); err != nil {
		return fmt.Errorf("イメージの push に失敗しました: %w", err)
	}

	return nil
}

// dir を OCI image layout として読み込み、中の単一イメージを返す
// (buildctl は index.json に単一マニフェストのみを書き出すため、
// 複数プラットフォーム/複数イメージの index には未対応)
func loadImage(dir string) (v1.Image, error) {
	path, err := layout.FromPath(dir)
	if err != nil {
		return nil, fmt.Errorf("layout の読み込みに失敗しました: %w", err)
	}

	index, err := path.ImageIndex()
	if err != nil {
		return nil, fmt.Errorf("image index の取得に失敗しました: %w", err)
	}

	manifest, err := index.IndexManifest()
	if err != nil {
		return nil, fmt.Errorf("manifest の取得に失敗しました: %w", err)
	}
	if len(manifest.Manifests) == 0 {
		return nil, fmt.Errorf("OCI image layout にイメージが含まれていません")
	}

	return index.Image(manifest.Manifests[0].Digest)
}
