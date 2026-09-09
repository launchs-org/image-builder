package downloader

import "io"

/*
io.Reader をラップし、読み取ったバイト数に応じて
onProgress コールバックを呼び出す Reader
呼び出し頻度は progressReportInterval バイトごとに間引く
*/
type progressReader struct {
	reader     io.Reader
	total      int64
	current    int64
	lastReport int64
	onProgress ProgressFunc
}

func newProgressReader(reader io.Reader, total int64, onProgress ProgressFunc) *progressReader {
	return &progressReader{
		reader:     reader,
		total:      total,
		onProgress: onProgress,
	}
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.reader.Read(buf)

	if n > 0 {
		p.current += int64(n)

		if p.onProgress != nil {
			// 間引きつつ通知する (最後の読み取りはEOF直後にerr!=nilで検知されるため、
			// ここでは単純に一定間隔ごとに呼び出す)
			if p.current-p.lastReport >= progressReportInterval || err != nil {
				p.lastReport = p.current
				p.onProgress(p.current, p.total)
			}
		}
	}

	return n, err
}
