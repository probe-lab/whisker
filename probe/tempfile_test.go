package probe

import (
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

			// file must exist on disk
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if info.Size() != tc.size {
				t.Errorf("size: got %d, want %d", info.Size(), tc.size)
			}

			// file must be readable from the start
			n, err := io.Copy(io.Discard, f)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if n != tc.size {
				t.Errorf("read bytes: got %d, want %d", n, tc.size)
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
