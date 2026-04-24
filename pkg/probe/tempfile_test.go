package probe

import (
	"crypto/sha256"
	"io"
	"os"
	"testing"
)

func TestNewTempFile(t *testing.T) {
	testCases := []struct {
		name string
		size int64
	}{
		{name: "zero", size: 0},
		{name: "small", size: 1024},
		{name: "medium", size: 64 * 1024},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := NewTempFile(t.TempDir(), tc.size)
			if err != nil {
				t.Fatalf("NewTempFile: %v", err)
			}

			path := f.Name()

			if f.Size() != tc.size {
				t.Errorf("Size: got %d, want %d", f.Size(), tc.size)
			}

			// file must exist on disk with the correct size
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if info.Size() != tc.size {
				t.Errorf("disk size: got %d, want %d", info.Size(), tc.size)
			}

			// file must be readable from the start; hash must match stored SHA256
			h := sha256.New()
			n, err := io.Copy(h, f)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if n != tc.size {
				t.Errorf("read bytes: got %d, want %d", n, tc.size)
			}
			var got [32]byte
			copy(got[:], h.Sum(nil))
			if got != f.SHA256 {
				t.Errorf("SHA256 mismatch: computed hash differs from stored hash")
			}

			if err := f.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}

			if err := f.Remove(); err != nil {
				t.Fatalf("remove: %v", err)
			}

			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("file still exists after Remove")
			}
		})
	}
}
