// Package dockerfile は buildctl CLI (BuildKit のクライアント) を使って、
// ユーザーが用意した Dockerfile からイメージファイルシステムをビルドするための
// 薄いラッパーです。
//
// railpack パッケージと同様に、ビルド自体は BuildkitHost で指定した
// buildkitd (docker-compose の buildkit サービスなど) に対して行われるため、
// このパッケージ・このプロセス自体には BuildKit の実行環境は不要です。
package dockerfile

import (
	"context"
	"fmt"
	"os/exec"

	"builder/execstream"
)

// buildctl build (dockerfile.v0 フロントエンド) に渡す設定の構造体
type BuildArgs struct {
	// ビルドコンテキストとなるディレクトリ (必須)
	// Dockerfile 内の COPY/ADD の相対パスの基点になる
	ContextDirectory string

	// Dockerfile が置かれているディレクトリ (省略時は ContextDirectory と同じ)
	DockerfileDirectory string

	// ビルドしたイメージの OCI image layout の出力先ディレクトリ (必須)
	OutputDirectory string

	// 接続先 buildkitd のアドレス (例: "tcp://buildkit:1234", 必須)
	// buildctl は railpack と異なり環境変数からの自動解決を行わないため、
	// 呼び出し側で明示的に指定する必要がある
	BuildkitHost string
}

/*
buildctl build --frontend dockerfile.v0 を実行し、Dockerfile の内容に沿って
イメージをビルドし、OCI image layout を OutputDirectory に書き出す関数
*/
func Build(ctx context.Context, args BuildArgs, onLog execstream.LogFunc) error {
	// 各種必須項目を検証
	if args.ContextDirectory == "" || args.OutputDirectory == "" || args.BuildkitHost == "" {
		return fmt.Errorf("invalid option: ContextDirectory, OutputDirectory and BuildkitHost are required")
	}

	dockerfileDir := args.DockerfileDirectory
	if dockerfileDir == "" {
		dockerfileDir = args.ContextDirectory
	}

	cmd := exec.CommandContext(ctx, "buildctl",
		"--addr", args.BuildkitHost,
		"build",
		"--frontend", "dockerfile.v0",
		"--local", "context="+args.ContextDirectory,
		"--local", "dockerfile="+dockerfileDir,
		"--output", "type=oci,dest="+args.OutputDirectory+",tar=false",
	)

	if err := execstream.Run(cmd, onLog); err != nil {
		return fmt.Errorf("buildctl build に失敗しました: %w", err)
	}

	return nil
}
