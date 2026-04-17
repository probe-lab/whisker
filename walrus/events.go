package walrus

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/probe-lab/whisker/sui"
)

// Walrus event type names as they appear at the end of the fully-qualified
// Move type, e.g. "0xabc::events::BlobRegistered".
const (
	EventTypeBlobRegistered = "BlobRegistered"
	EventTypeBlobCertified  = "BlobCertified"
	EventTypeBlobDeleted    = "BlobDeleted"
)

// BlobRegistered is emitted when a blob is registered on-chain.
// Storage nodes should expect slivers after this event.
// blob_id and size are u256/u64 respectively, serialised by Sui as decimal strings.
type BlobRegistered struct {
	Epoch           uint32 `json:"epoch"`
	BlobID          string `json:"blob_id"`
	Size            string `json:"size"`
	ErasureCodeType uint8  `json:"erasure_code_type"`
	EndEpoch        uint32 `json:"end_epoch"`
	Deletable       bool   `json:"deletable"`
	ObjectID        string `json:"object_id,omitempty"`
}

// BlobCertified is emitted once 2/3 of shards have signed receipts.
// The blob is guaranteed available until EndEpoch.
type BlobCertified struct {
	Epoch       uint32 `json:"epoch"`
	BlobID      string `json:"blob_id"`
	EndEpoch    uint32 `json:"end_epoch"`
	IsCertified bool   `json:"is_certified"`
}

// BlobDeleted is emitted when the owner of a deletable blob deletes it.
type BlobDeleted struct {
	Epoch               uint32 `json:"epoch"`
	BlobID              string `json:"blob_id"`
	EndEpoch            uint32 `json:"end_epoch"`
	WasAlreadyCertified bool   `json:"was_already_certified"`
}

// EventEnvelope wraps a parsed Walrus event with its Sui metadata.
type EventEnvelope struct {
	EventType   string          `json:"event_type"`
	Cursor      sui.EventCursor `json:"cursor"`
	TimestampMs string          `json:"timestamp_ms,omitempty"`
	Event       any             `json:"event"`
}

// ParseEvent parses a raw Sui event into a typed Walrus EventEnvelope.
// Returns an error for unrecognised event types.
func ParseEvent(ev sui.Event) (*EventEnvelope, error) {
	typeName := bareTypeName(ev.Type)

	var data any
	switch typeName {
	case EventTypeBlobRegistered:
		var e BlobRegistered
		if err := json.Unmarshal(ev.ParsedJSON, &e); err != nil {
			return nil, fmt.Errorf("unmarshal BlobRegistered: %w", err)
		}
		data = &e
	case EventTypeBlobCertified:
		var e BlobCertified
		if err := json.Unmarshal(ev.ParsedJSON, &e); err != nil {
			return nil, fmt.Errorf("unmarshal BlobCertified: %w", err)
		}
		data = &e
	case EventTypeBlobDeleted:
		var e BlobDeleted
		if err := json.Unmarshal(ev.ParsedJSON, &e); err != nil {
			return nil, fmt.Errorf("unmarshal BlobDeleted: %w", err)
		}
		data = &e
	default:
		return nil, fmt.Errorf("unrecognised event type: %s", ev.Type)
	}

	return &EventEnvelope{
		EventType:   typeName,
		Cursor:      ev.ID,
		TimestampMs: ev.TimestampMs,
		Event:       data,
	}, nil
}

// bareTypeName extracts the identifier after the last "::" in a Move type string.
// "0xabc::events::BlobRegistered" -> "BlobRegistered"
func bareTypeName(fullType string) string {
	if i := strings.LastIndex(fullType, "::"); i >= 0 {
		return fullType[i+2:]
	}
	return fullType
}
