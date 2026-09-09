package downloader

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// ダウンロードするzipファイルの最大サイズ (500MB) - MaxSizeBytes未指定時のデフォルト
	defaultMaxDownloadSize int64 = 500 * 1024 * 1024

	// 展開後の合計サイズの上限 (2GB) - zip bomb 対策
	maxExtractedSize int64 = 2 * 1024 * 1024 * 1024

	// 展開するファイル数の上限 - zip bomb 対策
	maxFileCount = 10000

	// HTTP接続のタイムアウト
	httpTimeout = 60 * time.Second

	// 進捗コールバックを呼び出す間隔 (バイト数)
	progressReportInterval int64 = 1024 * 1024
)

type DirectoryDownloadArgs struct {
	// リモートのzipのダウンロードリンク (必須、https のみ許可)
	DownloadUrl string

	// ファイルのsha256ハッシュ (空文字の場合は検証無)
	Sha256FileHash string

	// 展開先ディレクトリ (必須)
	Directory string

	// ダウンロードするzipファイルの最大サイズ (バイト数、0以下の場合はデフォルト値 defaultMaxDownloadSize を使用)
	MaxSizeBytes int64
}

/*
リモートからzipを取得して対象ディレクトリに展開する関数
*/
func DownloadDirectory(args DirectoryDownloadArgs, onProgress ProgressFunc) error {
	// サイズ上限の決定 (未指定または不正値の場合はデフォルトを使用)
	maxDownloadSize := args.MaxSizeBytes
	if maxDownloadSize <= 0 {
		maxDownloadSize = defaultMaxDownloadSize
	}
	// 各種項目を検証
	if args.DownloadUrl == "" || args.Directory == "" {
		// どちらかが設定されていない場合エラー
		return errors.New("invalid option: DownloadUrl and Directory are required")
	}

	// URL の検証 (SSRF対策: httpsスキームのみ許可)
	parsedUrl, err := url.Parse(args.DownloadUrl)
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if parsedUrl.Scheme != "https" {
		return errors.New("invalid option: only https URLs are allowed")
	}
	if parsedUrl.Hostname() == "" {
		return errors.New("invalid option: URL must have a host")
	}

	// 展開先ディレクトリを絶対パスに正規化しておく (Zip Slip対策で後ほど使用)
	targetDir, err := filepath.Abs(args.Directory)
	if err != nil {
		return fmt.Errorf("failed to resolve directory: %w", err)
	}

	// http 接続を開始する (タイムアウト付きクライアントを使用)
	client := &http.Client{
		Timeout: httpTimeout,
	}
	resp, err := client.Get(args.DownloadUrl)

	// エラー処理
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	// 閉じる
	defer resp.Body.Close()

	// ステータスコードを確認する
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	// 進捗計算用の総サイズ (不明な場合は -1)
	totalSize := resp.ContentLength
	if totalSize < 0 {
		totalSize = -1
	}

	// 一時ファイルを作成して保存する
	tmpFile, err := os.CreateTemp(os.TempDir(), "zipDl-")

	// エラー処理
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpFilePath := tmpFile.Name()
	// 関数終了時に一時ファイルを必ず削除する
	defer os.Remove(tmpFilePath)
	// tempfile を閉じる
	defer tmpFile.Close()

	// ファイルの内容をコピーしつつ、同時に sha256 を計算する
	// LimitReader でダウンロードサイズに上限を設ける (DoS対策)
	hasher := sha256.New()
	limitedReader := io.LimitReader(resp.Body, maxDownloadSize+1)
	progressReader := newProgressReader(limitedReader, totalSize, onProgress)
	writer := io.MultiWriter(tmpFile, hasher)

	written, err := io.Copy(writer, progressReader)

	// エラー処理
	if err != nil {
		return fmt.Errorf("failed to save downloaded file: %w", err)
	}

	// サイズ上限を超えていないか確認する
	if written > maxDownloadSize {
		return fmt.Errorf("downloaded file exceeds maximum allowed size (%d bytes)", maxDownloadSize)
	}

	// sha256ハッシュの検証 (指定されている場合のみ)
	if args.Sha256FileHash != "" {
		actualHash := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualHash, args.Sha256FileHash) {
			return fmt.Errorf("sha256 hash mismatch: expected %s, got %s", args.Sha256FileHash, actualHash)
		}
	}

	// 書き込み内容を読み込めるようにフラッシュしておく
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// 展開先ディレクトリが存在しない場合は作成する
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// zip を展開する
	if err := extractZip(tmpFilePath, targetDir); err != nil {
		return fmt.Errorf("failed to extract zip: %w", err)
	}

	return nil
}

/*
zip ファイルを targetDir に安全に展開する関数
Zip Slip (パストラバーサル) と zip bomb (展開サイズ/ファイル数の爆発) を防止する
*/
func extractZip(zipPath string, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	// zip bomb 対策: ファイル数の上限チェック
	if len(reader.File) > maxFileCount {
		return fmt.Errorf("zip contains too many files (%d > %d)", len(reader.File), maxFileCount)
	}

	// 展開先ディレクトリの正規化されたパス (末尾にセパレータを付けてprefix比較を安全にする)
	cleanTargetDir := filepath.Clean(targetDir)

	var totalExtractedSize int64

	for _, file := range reader.File {
		// --- Zip Slip 対策 ---
		// ファイル名に含まれる ".." や絶対パスを使った
		// ディレクトリトラバーサルを防ぐため、
		// 展開後の実パスが targetDir 配下にあることを必ず検証する。
		entryPath, err := sanitizeExtractPath(cleanTargetDir, file.Name)
		if err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(entryPath, 0o755); err != nil {
				return fmt.Errorf("failed to create directory %q: %w", file.Name, err)
			}
			continue
		}

		// 親ディレクトリを作成する
		if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
			return fmt.Errorf("failed to create parent directory for %q: %w", file.Name, err)
		}

		// --- zip bomb 対策: 展開後サイズの累積チェック ---
		// file.UncompressedSize64 は zip ヘッダの申告値なので過信せず、
		// 実際のコピー時にも LimitReader で二重に制限する。
		totalExtractedSize += int64(file.UncompressedSize64)
		if totalExtractedSize > maxExtractedSize {
			return fmt.Errorf("extracted content exceeds maximum allowed size (%d bytes)", maxExtractedSize)
		}

		if err := extractFile(file, entryPath); err != nil {
			return fmt.Errorf("failed to extract file %q: %w", file.Name, err)
		}
	}

	return nil
}

/*
zip 内の1エントリを実際にディスクへ書き出す。
シンボリックリンクは展開しない (悪用防止のため通常ファイルのみ扱う)。
*/
func extractFile(file *zip.File, destPath string) error {
	// シンボリックリンクなど特殊ファイルは無視する
	if file.Mode()&os.ModeSymlink != 0 {
		return errors.New("symlinks in zip archives are not allowed")
	}

	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// パーミッションは実行可能ビットなどを引き継ぎつつ、過度に緩い設定は避ける
	mode := file.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}

	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// 展開サイズにも上限を設けて二重に守る (ヘッダ申告値との不一致対策)
	limitedSrc := io.LimitReader(srcFile, maxExtractedSize+1)
	written, err := io.Copy(destFile, limitedSrc)
	if err != nil {
		return err
	}
	if written > maxExtractedSize {
		return errors.New("file exceeds maximum allowed extracted size")
	}

	return nil
}

/*
zip エントリ名から実際の展開先パスを計算し、
targetDir の外側を指していないことを検証する (Zip Slip 対策)。
*/
func sanitizeExtractPath(targetDir string, entryName string) (string, error) {
	// バックスラッシュ区切りのzip (Windows製) にも対応させるため統一する
	normalized := strings.ReplaceAll(entryName, `\`, `/`)

	// 展開先パスを結合して正規化する
	joined := filepath.Join(targetDir, filepath.FromSlash(normalized))
	cleanPath := filepath.Clean(joined)

	// targetDir 自身、または targetDir + セパレータ で始まっているかを確認する
	// (前方一致だけだと "/target-evil" が "/target" にマッチしてしまうので注意)
	if cleanPath != targetDir && !strings.HasPrefix(cleanPath, targetDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal file path in zip (path traversal attempt): %q", entryName)
	}

	return cleanPath, nil
}