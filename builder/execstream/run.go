// Package execstream は外部コマンドを実行し、標準出力/標準エラーを
// 1行ずつコールバックで受け取るための共通ヘルパーです。
//
// railpack (railpack CLI) と dockerfile (buildctl CLI) など、
// 「BuildKit 関連の CLI を os/exec で呼び出し、進捗ログを流し込む」という
// 同じ形のビルド処理を複数のパッケージで使うため、ここに共通化しています。
package execstream

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
)

// コマンドの標準出力/標準エラーを1行ずつ通知するコールバック
type LogFunc func(line string)

/*
cmd を実行し、標準出力・標準エラーを1行ずつ onLog に通知しながら
完了まで待つ関数

呼び出し前に cmd.Stdout / cmd.Stderr を設定しないこと
(この関数が内部でパイプとして接続するため上書きされる)
*/
func Run(cmd *exec.Cmd, onLog LogFunc) error {
	// 進捗ログを行単位で読み取るためにパイプとして取得する
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("標準出力の取得に失敗しました: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("標準エラー出力の取得に失敗しました: %w", err)
	}

	// コマンドを起動する (完了は待たない)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s の起動に失敗しました: %w", cmd.Path, err)
	}

	// 標準出力・標準エラーをそれぞれ別ゴルーチンで並行して読み切ってから
	// コマンドの終了を待つ (パイプを読み切らないと、出力量次第で
	// プロセスがパイプのバッファ詰まりでブロックし続けるため)
	done := make(chan struct{}, 2)
	streamLines(stdout, onLog, done)
	streamLines(stderr, onLog, done)
	<-done
	<-done

	// 出力の読み取り完了後にプロセスの終了を待ち、終了コードを確認する
	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

// r から1行ずつ読み取り、onLog に通知するゴルーチンを起動するヘルパー
// 読み取り終了 (EOFまたはエラー) 時に done へ通知する
func streamLines(r io.Reader, onLog LogFunc, done chan<- struct{}) {
	go func() {
		defer func() { done <- struct{}{} }()

		scanner := bufio.NewScanner(r)
		// BuildKit の進捗行などは長くなることがあるため、
		// bufio.Scanner のデフォルトバッファ (64KB) では途中で打ち切られる
		// 可能性がある。最大1MBまで許容するようバッファを拡張する。
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			if onLog != nil {
				onLog(scanner.Text())
			}
		}
	}()
}
