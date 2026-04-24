package walrus

import (
	"testing"
)

func TestNormaliseBlobID(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			// verified against: walrus convert-blob-id <decimal>
			name:  "decimal_to_base64url",
			input: "18900247491672312846861066835399466208039540205050687619458183161462372245997",
			want:  "7SEFkvE57_MS0pl7FdJp9XE0xH85pFWRcxjNdDYpySk",
		},
		{
			name:  "already_base64url_passthrough",
			input: "7SEFkvE57_MS0pl7FdJp9XE0xH85pFWRcxjNdDYpySk",
			want:  "7SEFkvE57_MS0pl7FdJp9XE0xH85pFWRcxjNdDYpySk",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normaliseBlobID(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
