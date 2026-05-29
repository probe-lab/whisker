package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// ResultWriter persists or otherwise handles a completed StorageCheckResult.
type ResultWriter interface {
	WriteStorageCheckResult(ctx context.Context, r *StorageCheckResult) error
}

// LogWriter is a ResultWriter that logs each result using slog.
// It is the default writer and is also used when --dry-run is set.
type LogWriter struct{}

func (w *LogWriter) WriteStorageCheckResult(_ context.Context, r *StorageCheckResult) error {
	if r.Failure != "" {
		slog.Info("storage check failed",
			"run_id", r.RunID,
			"file_size", r.FileSize,
			"status", r.Status,
			"failure", r.Failure,
		)
		return nil
	}
	slog.Info("storage check complete",
		"run_id", r.RunID,
		"blob_id", r.BlobID,
		"file_size", r.FileSize,
		"upload_ms", r.UploadFinished.Sub(r.UploadStarted).Milliseconds(),
		"registration_ms", r.BlobRegisteredAt.Sub(r.UploadStarted).Milliseconds(),
		"certification_latency_ms", r.BlobCertifiedAt.Sub(r.BlobRegisteredAt).Milliseconds(),
		"ttfb_ms", r.FirstByteAt.Sub(r.DownloadStarted).Milliseconds(),
		"ttlb_ms", r.DownloadFinished.Sub(r.DownloadStarted).Milliseconds(),
		"download_size", r.DownloadSize,
		"length_ok", r.ContentLengthMatch,
		"hash_ok", r.ContentHashMatch,
	)
	return nil
}

// JSONFileWriter is a ResultWriter that appends each result as a JSON object
// (newline-delimited) to a file in a configured directory.
// Call Close when done to flush and release the file.
type JSONFileWriter struct {
	mu  sync.Mutex
	f   *os.File
	enc *json.Encoder
}

// NewJSONFileWriter creates a new output file in dir named by runID and returns
// a writer ready to accept results.
func NewJSONFileWriter(dir, runID string) (*JSONFileWriter, error) {
	name := filepath.Join(dir, fmt.Sprintf("whisker-%s.ndjson", runID))
	f, err := os.Create(name)
	if err != nil {
		return nil, fmt.Errorf("create output file: %w", err)
	}
	slog.Info("writing results to file", "path", name)
	return &JSONFileWriter{f: f, enc: json.NewEncoder(f)}, nil
}

func (w *JSONFileWriter) WriteStorageCheckResult(_ context.Context, r *StorageCheckResult) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.enc.Encode(r)
}

// Close flushes and closes the output file.
func (w *JSONFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}
