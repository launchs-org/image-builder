package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"builder/archive"
	"builder/dockerfile"
	"builder/downloader"
	"builder/railpack"
	"builder/registry"
)

func main() {
	// --- ソース関連 ---
	sourceType := flag.String("source-type", "git", `ビルド対象の取得方法 ("git" または "zip")`)
	gitURL := flag.String("git-url", "", "クローンする Git リポジトリの URL (source-type=git のとき必須)")
	gitBranch := flag.String("git-branch", "", "チェックアウトするブランチ名 (省略時: リポジトリの既定ブランチ)")
	gitCommit := flag.String("git-commit", "", `チェックアウトするコミットハッシュ (省略時: "latest")`)
	zipURL := flag.String("zip-url", "", "ダウンロードする zip の URL (source-type=zip のとき必須, https のみ)")
	zipSha256 := flag.String("zip-sha256", "", "zip の sha256 ハッシュ (省略時は検証しない)")

	// --- ビルド方式関連 ---
	buildMethod := flag.String("build-method", "railpack", `ビルド方式 ("railpack" または "dockerfile")`)

	// --- push 関連 ---
	push := flag.Bool("push", false, "ビルドしたイメージをレジストリに push するかどうか")
	imageRef := flag.String("image-ref", "", "push 先イメージ参照 (例: registry.example.com/project/app:latest) (push=true のとき必須)")
	registryUsername := flag.String("registry-username", "", "レジストリ認証のユーザー名 (省略可)")
	registryPassword := flag.String("registry-password", "", "レジストリ認証のパスワード (省略可)")
	registryToken := flag.String("registry-token", "", "レジストリ認証用の Bearer トークン (事前に取得したJWTなど、指定時は registry-username/password より優先)")
	registryInsecure := flag.Bool("registry-insecure", false, "TLS証明書検証を行わず、HTTPでのアクセスも許可する (自己署名証明書やローカル検証用)")

	// --- 出力関連 (push しない場合) ---
	outputTar := flag.String("output-tar", "", "push しない場合の出力先 tar.gz ファイルパス (push=true の場合は無視される)")

	flag.Parse()

	if !*push && *outputTar == "" {
		log.Fatal("引数が不正です: push しない場合は output-tar が必須です")
	}

	source, err := buildSource(*sourceType, *gitURL, *gitBranch, *gitCommit, *zipURL, *zipSha256)
	if err != nil {
		log.Fatalf("引数が不正です: %v", err)
	}

	workDir, err := os.MkdirTemp("", "image-builder-")
	if err != nil {
		log.Fatalf("作業ディレクトリの作成に失敗しました: %v", err)
	}
	defer os.RemoveAll(workDir)

	sourceDir := filepath.Join(workDir, "source")
	imageDir := filepath.Join(workDir, "image")

	if err := source.Fetch(sourceDir, func(current, total int64) {
		log.Printf("download progress: %d/%d", current, total)
	}); err != nil {
		log.Fatalf("ソースの取得に失敗しました: %v", err)
	}

	buildkitHost := os.Getenv("BUILDKIT_HOST")
	onLog := func(line string) { log.Println(line) }

	if err := runBuild(*buildMethod, sourceDir, imageDir, buildkitHost, onLog); err != nil {
		log.Fatalf("イメージのビルドに失敗しました: %v", err)
	}

	log.Println("ビルドが完了しました:", imageDir)

	if *push {
		if *imageRef == "" {
			log.Fatal("引数が不正です: push=true の場合 image-ref は必須です")
		}

		err := registry.Push(context.Background(), registry.PushArgs{
			ImageDirectory: imageDir,
			ImageReference: *imageRef,
			Username:       *registryUsername,
			Password:       *registryPassword,
			BearerToken:    *registryToken,
			Insecure:       *registryInsecure,
		})
		if err != nil {
			log.Fatalf("イメージの push に失敗しました: %v", err)
		}

		log.Println("push が完了しました:", *imageRef)
		return
	}

	if err := archive.WriteTarGz(imageDir, *outputTar); err != nil {
		log.Fatalf("tar.gz への出力に失敗しました: %v", err)
	}

	log.Println("tar.gz を出力しました:", *outputTar)
}

// CLI 引数からビルド対象の取得元 (Source) を組み立てる
func buildSource(sourceType, gitURL, gitBranch, gitCommit, zipURL, zipSha256 string) (downloader.Source, error) {
	switch sourceType {
	case "git":
		if gitURL == "" {
			return nil, errRequired("source-type=git", "git-url")
		}
		return downloader.GithubRepositoryArgs{
			RemoteUrl:  gitURL,
			Branch:     gitBranch,
			CommitHash: gitCommit,
		}, nil
	case "zip":
		if zipURL == "" {
			return nil, errRequired("source-type=zip", "zip-url")
		}
		return downloader.DirectoryDownloadArgs{
			DownloadUrl:    zipURL,
			Sha256FileHash: zipSha256,
		}, nil
	default:
		return nil, errInvalidChoice("source-type", sourceType, "git", "zip")
	}
}

// CLI 引数で指定されたビルド方式でイメージをビルドする
func runBuild(buildMethod, sourceDir, imageDir, buildkitHost string, onLog func(string)) error {
	switch buildMethod {
	case "dockerfile":
		return dockerfile.Build(context.Background(), dockerfile.BuildArgs{
			ContextDirectory: sourceDir,
			OutputDirectory:  imageDir,
			BuildkitHost:     buildkitHost,
		}, onLog)
	case "railpack":
		return railpack.Build(context.Background(), railpack.BuildArgs{
			SourceDirectory: sourceDir,
			OutputDirectory: imageDir,
			BuildkitHost:    buildkitHost,
		}, onLog)
	default:
		return errInvalidChoice("build-method", buildMethod, "railpack", "dockerfile")
	}
}
