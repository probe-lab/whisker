package walrus

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/probe-lab/whisker/pkg/sui"
)

func TestParseEvent(t *testing.T) {
	testCases := []struct {
		name       string
		eventType  string
		parsedJSON string
		wantType   string
		wantEvent  any
	}{
		{
			name:      "blob_registered",
			eventType: "0xfdc88f7d7cf30afab2f82e8380d11ee8f70efb90e863d1de8616fae1bb09ea77::events::BlobRegistered",
			parsedJSON: `{
				"epoch": 10,
				"blob_id": "99375887944986645936715392795498921156816310016218858990289773399590762835131",
				"size": "1024",
				"erasure_code_type": 0,
				"end_epoch": 11,
				"deletable": true,
				"object_id": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
			}`,
			wantType: EventTypeBlobRegistered,
			wantEvent: &BlobRegistered{
				Epoch:           10,
				BlobID:          "99375887944986645936715392795498921156816310016218858990289773399590762835131",
				Size:            "1024",
				ErasureCodeType: 0,
				EndEpoch:        11,
				Deletable:       true,
				ObjectID:        "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			},
		},
		{
			name:      "blob_certified",
			eventType: "0xfdc88f7d7cf30afab2f82e8380d11ee8f70efb90e863d1de8616fae1bb09ea77::events::BlobCertified",
			parsedJSON: `{
				"epoch": 10,
				"blob_id": "99375887944986645936715392795498921156816310016218858990289773399590762835131",
				"end_epoch": 11,
				"is_certified": true
			}`,
			wantType: EventTypeBlobCertified,
			wantEvent: &BlobCertified{
				Epoch:       10,
				BlobID:      "99375887944986645936715392795498921156816310016218858990289773399590762835131",
				EndEpoch:    11,
				IsCertified: true,
			},
		},
		{
			name:      "blob_deleted",
			eventType: "0xfdc88f7d7cf30afab2f82e8380d11ee8f70efb90e863d1de8616fae1bb09ea77::events::BlobDeleted",
			parsedJSON: `{
				"epoch": 10,
				"blob_id": "99375887944986645936715392795498921156816310016218858990289773399590762835131",
				"end_epoch": 11,
				"was_already_certified": true
			}`,
			wantType: EventTypeBlobDeleted,
			wantEvent: &BlobDeleted{
				Epoch:               10,
				BlobID:              "99375887944986645936715392795498921156816310016218858990289773399590762835131",
				EndEpoch:            11,
				WasAlreadyCertified: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ev := sui.Event{
				ID:          sui.EventCursor{TxDigest: "Bxq7v2GwQXNnQJ4KvWbGFzMoYhDcAsRpLkTe1UiNsOd", EventSeq: "0"},
				Type:        tc.eventType,
				ParsedJSON:  json.RawMessage(tc.parsedJSON),
				TimestampMs: "1744848000000",
			}

			envelope, err := ParseEvent(ev)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if envelope.EventType != tc.wantType {
				t.Errorf("event type: got %q, want %q", envelope.EventType, tc.wantType)
			}
			if envelope.Cursor != ev.ID {
				t.Errorf("cursor: got %+v, want %+v", envelope.Cursor, ev.ID)
			}
			if envelope.TimestampMs != ev.TimestampMs {
				t.Errorf("timestamp: got %q, want %q", envelope.TimestampMs, ev.TimestampMs)
			}
			if diff := cmp.Diff(tc.wantEvent, envelope.Event); diff != "" {
				t.Errorf("event mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseEvent_UnknownType(t *testing.T) {
	ev := sui.Event{
		Type:       "0xabc::events::UnknownEvent",
		ParsedJSON: json.RawMessage(`{}`),
	}
	_, err := ParseEvent(ev)
	if err == nil {
		t.Fatal("expected error for unknown event type, got nil")
	}
}

func TestBareTypeName(t *testing.T) {
	testCases := []struct {
		input string
		want  string
	}{
		{"0xabc::events::BlobRegistered", "BlobRegistered"},
		{"0xabc::events::BlobCertified", "BlobCertified"},
		{"BlobRegistered", "BlobRegistered"},
		{"", ""},
	}

	for _, tc := range testCases {
		got := bareTypeName(tc.input)
		if got != tc.want {
			t.Errorf("bareTypeName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
