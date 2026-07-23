package ssm

import "io"

// ProgressFunc receives the completed and total byte counts for a transfer.
// A negative total means the transfer size is unknown.
type ProgressFunc func(done, total int64)

// TransferOptions contains optional transfer behavior.
type TransferOptions struct {
	Progress ProgressFunc
}

func reportProgress(progress ProgressFunc, done, total int64) {
	if progress != nil {
		progress(done, total)
	}
}

type progressReader struct {
	reader   io.Reader
	total    int64
	done     int64
	progress ProgressFunc
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.done += int64(n)
		reportProgress(r.progress, r.done, r.total)
	}
	return n, err //nolint:wrapcheck // io.Reader implementations must preserve sentinel errors such as io.EOF.
}
