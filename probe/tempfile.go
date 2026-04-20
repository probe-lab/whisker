package probe

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// TempFile is a temporary file containing pseudo-random bytes.
// It embeds *os.File and adds a Remove method to delete the file from disk.
// The caller is responsible for calling Close and Remove when done.
type TempFile struct {
	*os.File
}

// NewTempFile creates a temporary file in dir filled with size pseudo-random bytes.
// The file's read position is seeked to the start before returning, so it is
// ready to use as an io.Reader. The caller must call Close and Remove when done.
func NewTempFile(dir string, size int64) (*TempFile, error) {
	f, err := os.CreateTemp(dir, "whisker-probe-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.CopyN(f, rand.Reader, size); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("write random content: %w", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("seek to start: %w", err)
	}

	return &TempFile{File: f}, nil
}

// Remove deletes the file from disk.
func (f *TempFile) Remove() error {
	return os.Remove(f.Name())
}
