package walrus

import (
	"testing"
)

func TestParseUploadResponse(t *testing.T) {
	testCases := []struct {
		name        string
		body        string
		wantNewly   *NewlyCreatedResult
		wantAlready *AlreadyCertifiedResult
		wantErr     bool
	}{
		{
			name: "newly_created",
			body: `{
				"newlyCreated": {
					"blobObject": {
						"id": "0xabc123",
						"blobId": "7SEFkvE57_MS0pl7FdJp9XE0xH85pFWRcxjNdDYpySk",
						"certifiedEpoch": 34,
						"storage": {"startEpoch": 34, "endEpoch": 35, "storageSize": 66034000},
						"deletable": false
					},
					"cost": 132300
				}
			}`,
			wantNewly: &NewlyCreatedResult{
				BlobID:         "7SEFkvE57_MS0pl7FdJp9XE0xH85pFWRcxjNdDYpySk",
				SuiObjectID:    "0xabc123",
				CertifiedEpoch: 34,
				EndEpoch:       35,
				Cost:           132300,
				Deletable:      false,
			},
		},
		{
			name: "already_certified",
			body: `{
				"alreadyCertified": {
					"blobId": "7SEFkvE57_MS0pl7FdJp9XE0xH85pFWRcxjNdDYpySk",
					"event": {"txDigest": "deadbeef", "eventSeq": "0"},
					"endEpoch": 35
				}
			}`,
			wantAlready: &AlreadyCertifiedResult{
				BlobID:   "7SEFkvE57_MS0pl7FdJp9XE0xH85pFWRcxjNdDYpySk",
				TxDigest: "deadbeef",
				EventSeq: "0",
				EndEpoch: 35,
			},
		},
		{
			name:    "unrecognised_shape",
			body:    `{"unknown": {}}`,
			wantErr: true,
		},
		{
			name:    "invalid_json",
			body:    `not json`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseUploadResponse([]byte(tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr {
				return
			}

			if tc.wantNewly != nil {
				if got.NewlyCreated == nil {
					t.Fatal("expected NewlyCreated, got nil")
				}
				if *got.NewlyCreated != *tc.wantNewly {
					t.Errorf("NewlyCreated:\n got  %+v\n want %+v", *got.NewlyCreated, *tc.wantNewly)
				}
			}

			if tc.wantAlready != nil {
				if got.AlreadyCertified == nil {
					t.Fatal("expected AlreadyCertified, got nil")
				}
				if *got.AlreadyCertified != *tc.wantAlready {
					t.Errorf("AlreadyCertified:\n got  %+v\n want %+v", *got.AlreadyCertified, *tc.wantAlready)
				}
			}
		})
	}
}
