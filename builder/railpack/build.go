// Package railpack は railpack CLI でビルドプランを生成し、
// buildctl (BuildKit のクライアント) + railpack の BuildKit フロントエンド
// (ghcr.io/railwayapp/railpack-frontend) を使ってイメージをビルドするための
// 薄いラッパーです。
//
// railpack CLI の `railpack build` コマンドは OCI イメージ形式での出力に
// 対応しておらず (Docker daemon への読み込み、またはファイルシステムの
// ディレクトリ展開の二択のみ)、レジストリへの push に使える形式で
// 出力できない。そのため、ここでは `railpack prepare` でビルドプラン
// (railpack-plan.json) だけを生成し、実際のビルドは dockerfile パッケージと
// 同様に buildctl 経由で行うことで、OCI image layout ディレクトリとして
// 出力できるようにしている。
//
// ビルド自体は BuildkitHost (docker-compose の buildkit サービスなど) に対して
// 行われるため、このパッケージ・このプロセス自体には BuildKit の実行環境
// (rootless 権限や privileged コンテナなど) は不要です。
package railpack

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"builder/execstream"
)

// railpack の BuildKit フロントエンドイメージ
// (buildctl --opt source=... に渡す)
const frontendImage = "ghcr.io/railwayapp/railpack-frontend"

// railpack の BuildKit フロントエンドがビルドプランを読み取る際の既定のファイル名
// (buildkit/frontend.go の defaultRailpackPlan と一致させる必要がある)
const planFileName = "railpack-plan.json"

// railpack ビルドに渡す設定の構造体
type BuildArgs struct {
	// ビルド対象のソースディレクトリ (必須)
	SourceDirectory string

	// ビルドしたイメージの OCI image layout の出力先ディレクトリ (必須)
	// buildctl の --output type=oci,dest=...,tar=false にそのまま渡される
	OutputDirectory string

	// 接続先 buildkitd のアドレス (例: "tcp://buildkit:1234", 必須)
	BuildkitHost string
}

/*
railpack prepare でビルドプランを生成し、続けて buildctl + railpack-frontend で
イメージをビルドして、OCI image layout を OutputDirectory に書き出す関数

注意: railpack prepare に --verbose オプションは付けない。railpack はこの
オプションが付いていると内部でビルド用の環境変数 MISE_VERBOSE を BuildKit の
シークレットとして要求するようになるが、その受け渡し経路を用意していないため
"secret MISE_VERBOSE: not found" で失敗する。
*/
func Build(ctx context.Context, args BuildArgs, onLog execstream.LogFunc) error {
	// 各種必須項目を検証
	if args.SourceDirectory == "" || args.OutputDirectory == "" || args.BuildkitHost == "" {
		return fmt.Errorf("invalid option: SourceDirectory, OutputDirectory and BuildkitHost are required")
	}

	// ビルドプランの置き場所として一時ディレクトリを使う
	// (buildctl --local dockerfile=<このディレクトリ> としてマウントする)
	planDir, err := os.MkdirTemp("", "railpack-plan-")
	if err != nil {
		return fmt.Errorf("プラン用の一時ディレクトリの作成に失敗しました: %w", err)
	}
	defer os.RemoveAll(planDir)

	planPath := filepath.Join(planDir, planFileName)

	prepareCmd := exec.CommandContext(ctx, "railpack", "prepare", args.SourceDirectory, "--plan-out", planPath)
	if err := execstream.Run(prepareCmd, onLog); err != nil {
		return fmt.Errorf("railpack prepare に失敗しました: %w", err)
	}

	buildCmd := exec.CommandContext(ctx, "buildctl",
		"--addr", args.BuildkitHost,
		"build",
		"--frontend", "gateway.v0",
		"--opt", "source="+frontendImage,
		"--local", "context="+args.SourceDirectory,
		"--local", "dockerfile="+planDir,
		"--output", "type=oci,dest="+args.OutputDirectory+",tar=false",
	)

	if err := execstream.Run(buildCmd, onLog); err != nil {
		return fmt.Errorf("railpack build (buildctl) に失敗しました: %w", err)
	}

	return nil
}
