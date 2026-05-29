package db

import (
	"time"

	"github.com/google/uuid"
)

type StorageCheck struct {
	Timestamp            time.Time  `ch:"timestamp"`
	Network              string     `ch:"network"`
	ProbeLocation        string     `ch:"probe_location"`
	RunID                uuid.UUID  `ch:"run_id"`
	PublisherURL         string     `ch:"publisher_url"`
	AggregatorURL        string     `ch:"aggregator_url"`
	BlobID               string     `ch:"blob_id"`
	SuiObjectID          string     `ch:"sui_object_id"`
	FileSizeBytes        uint64     `ch:"file_size_bytes"`
	UploadStartedAt      *time.Time `ch:"upload_started_at"`
	UploadEndedAt        *time.Time `ch:"upload_ended_at"`
	RegisteredAt         *time.Time `ch:"registered_at"`
	CertifiedAt          *time.Time `ch:"certified_at"`
	RetrievalStartedAt   *time.Time `ch:"retrieval_started_at"`
	RetrievalFirstByteAt *time.Time `ch:"retrieval_first_byte_at"`
	RetrievalLastByteAt  *time.Time `ch:"retrieval_last_byte_at"`
	BytesRetrieved       *uint64    `ch:"bytes_retrieved"`
	Status               string     `ch:"status"`
	ContentLengthOK      bool       `ch:"content_length_ok"`
	ContentHashOK        bool       `ch:"content_hash_ok"`
	Failure              string     `ch:"failure"`
}
