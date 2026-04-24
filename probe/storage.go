package probe

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/probe-lab/whisker/sui"
	"github.com/probe-lab/whisker/walrus"
)

// StorageCheckResult holds all timing and verification data from one end-to-end
// upload-certify-download cycle.
type StorageCheckResult struct {
	RunID       string // UUID v7 generated at the start of each check
	FileSize    int64
	BlobID      string // base64url blob ID
	SuiObjectID string // Sui object ID of the uploaded blob; empty if blob was already certified

	UploadStarted  time.Time
	UploadFinished time.Time

	BlobRegisteredAt time.Time // timestamp from the BlobRegistered Sui event
	BlobCertifiedAt  time.Time // timestamp from the BlobCertified Sui event

	DownloadStarted  time.Time
	FirstByteAt      time.Time // DownloadStarted + TTFB
	DownloadFinished time.Time // DownloadStarted + TTLB
	DownloadSize     int64

	ContentLengthMatch bool
	ContentHashMatch   bool
}

// StorageChecker runs end-to-end storage check cycles against a Walrus publisher
// and aggregator, watching Sui events for certification confirmation.
type StorageChecker struct {
	RunID        string // set once at process start; copied to every result
	Publisher    *walrus.PublisherClient
	Aggregator   *walrus.AggregatorClient
	Sui          *sui.Client
	PackageID    string        // Walrus package ID used to filter Sui events
	PollInterval time.Duration // how often to poll for new events
	EventTimeout time.Duration // how long to wait for BlobCertified before giving up
	UploadOpts   walrus.UploadOptions

	// Recycling fields: when Executor is non-nil, each successful cycle deletes the
	// uploaded blob and returns the Storage resource to the wallet for reuse.
	// Set SystemObjectID to the Walrus system object ID for the target network.
	// DryRun disables deletion so recycling is skipped without error.
	Executor       *sui.TransactionExecutor
	SystemObjectID string
	DryRun         bool
}

// Check executes one storage check: creates a temp file of the given size in dir,
// uploads it, waits for BlobRegistered and BlobCertified events on Sui, downloads
// the blob, then verifies length and SHA256 hash.
func (c *StorageChecker) Check(ctx context.Context, dir string, size int64) (*StorageCheckResult, error) {
	result := &StorageCheckResult{RunID: c.RunID, FileSize: size}

	tf, err := NewTempFile(dir, size)
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	originalHash := tf.SHA256

	filter := sui.MoveEventModuleFilter(c.PackageID, "events")
	cursor, err := c.Sui.LatestEventCursor(ctx, filter)
	if err != nil {
		tf.Close()
		tf.Remove()
		return nil, fmt.Errorf("get latest event cursor: %w", err)
	}

	uploadOpts := c.UploadOpts
	if c.Executor != nil {
		uploadOpts.ReuseResources = true
	}

	result.UploadStarted = time.Now()
	uploadResult, err := c.Publisher.UploadBlob(ctx, tf, size, uploadOpts)
	tf.Close()
	tf.Remove()
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}
	result.UploadFinished = time.Now()
	result.BlobID = uploadResult.BlobID()
	if uploadResult.NewlyCreated != nil {
		result.SuiObjectID = uploadResult.NewlyCreated.SuiObjectID
	}

	// Watch for BlobRegistered and BlobCertified events matching our blob ID.
	var registered, certified bool
	watchCtx, cancel := context.WithTimeout(ctx, c.EventTimeout)
	defer cancel()

	watchErr := c.Sui.WatchEvents(watchCtx, filter, cursor, c.PollInterval, func(ev sui.Event) error {
		envelope, err := walrus.ParseEvent(ev)
		if err != nil {
			return nil // skip unrecognised event types
		}
		evTime := parseTimestampMs(envelope.TimestampMs)
		switch e := envelope.Event.(type) {
		case *walrus.BlobRegistered:
			if blobIDMatches(e.BlobID, result.BlobID) {
				result.BlobRegisteredAt = evTime
				registered = true
			}
		case *walrus.BlobCertified:
			if blobIDMatches(e.BlobID, result.BlobID) {
				result.BlobCertifiedAt = evTime
				certified = true
			}
		}
		if registered && certified {
			return sui.ErrStopWatching
		}
		return nil
	})
	if watchErr != nil && !errors.Is(watchErr, context.DeadlineExceeded) {
		return nil, fmt.Errorf("watch events: %w", watchErr)
	}
	if !registered {
		return nil, fmt.Errorf("timed out waiting for BlobRegistered event for blob %s", result.BlobID)
	}
	if !certified {
		return nil, fmt.Errorf("timed out waiting for BlobCertified event for blob %s", result.BlobID)
	}

	// Download into a hash writer to verify content integrity.
	h := sha256.New()
	result.DownloadStarted = time.Now()
	fetchResult, err := c.Aggregator.FetchBlob(ctx, result.BlobID, h)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	result.FirstByteAt = result.DownloadStarted.Add(fetchResult.TTFB)
	result.DownloadFinished = result.DownloadStarted.Add(fetchResult.TTLB)
	result.DownloadSize = fetchResult.Size

	result.ContentLengthMatch = fetchResult.Size == size
	result.ContentHashMatch = bytes.Equal(h.Sum(nil), originalHash[:])

	// Recycle the storage resource after a complete successful cycle.
	if !c.DryRun && c.Executor != nil && result.SuiObjectID != "" {
		storageID, _, recycleErr := c.Executor.DeleteBlob(ctx, c.PackageID, c.SystemObjectID, result.SuiObjectID, 0)
		if recycleErr != nil {
			slog.Warn("storage recycle failed, will purchase new storage next cycle",
				"err", recycleErr,
				"blob_object_id", result.SuiObjectID,
			)
		} else {
			slog.Info("storage recycled", "storage_object_id", storageID, "blob_object_id", result.SuiObjectID)
		}
	}

	return result, nil
}

// blobIDMatches reports whether eventBlobID (decimal u256 from Sui events) matches
// targetBlobID (base64url from the Walrus HTTP API).
func blobIDMatches(eventBlobID, targetBlobID string) bool {
	normalized, err := walrus.BlobIDBase64(eventBlobID)
	if err != nil {
		return false
	}
	return normalized == targetBlobID
}

// parseTimestampMs converts a Sui event timestamp (milliseconds as decimal string)
// to time.Time. Falls back to time.Now() if the string is empty or unparseable.
func parseTimestampMs(ms string) time.Time {
	if ms == "" {
		return time.Now()
	}
	n, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.UnixMilli(n).UTC()
}
