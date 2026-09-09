package downloader

/*
進捗通知用コールバック
current: 現在までに処理されたバイト数
total: 総バイト数 (不明な場合は -1)
*/
type ProgressFunc func(current int64, total int64)

/*
ダウンロード元を表す抽象化インターフェース
Git リポジトリのクローンや zip ファイルのダウンロードなど、
異なる取得手段を同じ方法で扱えるようにする
*/
type Source interface {
	// 指定ディレクトリにソースの内容を取得する
	// onProgress は nil でもよい
	// 注意: 指定先ディレクトリは削除・上書きされる場合がある
	Fetch(directory string, onProgress ProgressFunc) error
}

// GithubRepositoryArgs を Source として扱えるようにする
func (args GithubRepositoryArgs) Fetch(directory string, onProgress ProgressFunc) error {
	args.Directory = directory
	return CloneFromGitHub(args, onProgress)
}

// DirectoryDownloadArgs を Source として扱えるようにする
func (args DirectoryDownloadArgs) Fetch(directory string, onProgress ProgressFunc) error {
	args.Directory = directory
	return DownloadDirectory(args, onProgress)
}
