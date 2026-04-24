package probe

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// TempFile is a temporary file containing pseudo-random bytes.
// It embeds *os.File and adds a Remove method to delete the file from disk.
// SHA256 holds the hash of the file's contents, computed at creation time.
// The caller is responsible for calling Close and Remove when done.
type TempFile struct {
	*os.File
	SHA256 [32]byte
	size   int64
}

// NewTempFile creates a temporary file in dir filled with size pseudo-random bytes.
// The SHA256 of the content is computed during writing and stored in the returned struct.
// The file's read position is seeked to the start before returning, so it is
// ready to use as an io.Reader.
func NewTempFile(dir string, size int64) (*TempFile, error) {
	f, err := os.CreateTemp(dir, "whisker-probe-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}

	h := sha256.New()
	if _, err := io.CopyN(io.MultiWriter(f, h), rand.Reader, size); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("write random content: %w", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, fmt.Errorf("seek to start: %w", err)
	}

	tf := &TempFile{File: f, size: size}
	copy(tf.SHA256[:], h.Sum(nil))
	return tf, nil
}

// Size returns the number of bytes written to the file.
func (f *TempFile) Size() int64 {
	return f.size
}

// Remove deletes the file from disk.
func (f *TempFile) Remove() error {
	return os.Remove(f.Name())
}
