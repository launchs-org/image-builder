package downloader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

// Github のクローン元の設定の構造体
type GithubRepositoryArgs struct {
	// リモートのURL (必須)
	RemoteUrl string

	// ブランチ名 (ない場合 Master)
	Branch string

	// コミットハッシュ (最新の場合 latest)
	CommitHash string `default:"latest"`

	// クローン先ディレクトリ (必須)
	Directory string
}

// コミットハッシュが未指定 (latest扱い) かどうかを判定
func isLatestCommit(hash string) bool {
	return hash == "" || hash == "latest"
}

// go-git の Progress (io.Writer) を ProgressFunc にブリッジする Writer
// go-git は進捗をテキスト行 (例: "Receiving objects: 45% (450/1000)") として書き込むため、
// バイト数としての意味は持たず、書き込まれた累積バイト数を current として通知する。
// total は不明なため常に -1 を渡す。
type gitProgressWriter struct {
	current    int64
	onProgress ProgressFunc
}

func (w *gitProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.current += int64(n)

	if w.onProgress != nil {
		w.onProgress(w.current, -1)
	}

	return n, nil
}

/*
Github からクローンしてくる関数
注意: 指定先フォルダは削除されます
*/
func CloneFromGitHub(args GithubRepositoryArgs, onProgress ProgressFunc) error {
	// パスを 絶対パスにする
	absPath, err := filepath.Abs(args.Directory)

	// エラーを返す
	if err != nil {
		return err
	}

	// 元のパスを置き換える
	args.Directory = absPath

	// クローン先ディレクトリの削除を試みる ディレクトリが存在するとクローンできないため
	err = os.RemoveAll(args.Directory)

	// エラー処理
	if err != nil {
		return err
	}

	// ブランチ名の検証 (デフォルトMaster)
	targetBranch := plumbing.Master

	if args.Branch != "" {
		// 設定されてるときは 文字列から設定する
		targetBranch = plumbing.ReferenceName(args.Branch)
	}

	// 特定コミットが指定されている場合、Depth:1 だと目的のコミットが
	// 取得範囲外になり得るため、その場合はフルクローン相当にする。
	// (go-git は任意コミットの shallow fetch を直接サポートしないため)
	// 進捗の出力先を決定する (コールバック未指定時は従来通り標準出力に流す)
	var progressWriter io.Writer = os.Stdout
	if onProgress != nil {
		progressWriter = &gitProgressWriter{onProgress: onProgress}
	}

	cloneOptions := &git.CloneOptions{
		URL:            args.RemoteUrl,
		AllowEmptyRepo: false,
		ReferenceName:  targetBranch,
		Progress:       progressWriter,
		SingleBranch:   true,
	}

	if isLatestCommit(args.CommitHash) {
		// 最新コミットだけでよいので浅くクローンして高速化する
		cloneOptions.Depth = 1
	}

	// クローン操作実行
	repo, err := git.PlainClone(args.Directory, cloneOptions)

	// エラー処理
	if err != nil {
		return fmt.Errorf("クローンに失敗しました: %w", err)
	}

	// latest の場合は Clone 時点で既に対象ブランチの HEAD が
	// チェックアウトされているので、追加のチェックアウトは不要
	if isLatestCommit(args.CommitHash) {
		return nil
	}

	// 操作するためにワークツリーオブジェクトを取得
	wtree, err := repo.Worktree()

	// エラー処理
	if err != nil {
		return fmt.Errorf("ワークツリーの取得に失敗しました: %w", err)
	}

	// 特定コミットへチェックアウト
	// 注意: Hash と Branch は同時指定しない (排他)
	err = wtree.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(args.CommitHash),
	})

	if err != nil {
		return fmt.Errorf("コミット %s へのチェックアウトに失敗しました: %w", args.CommitHash, err)
	}

	return nil
}
