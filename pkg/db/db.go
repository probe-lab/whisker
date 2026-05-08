package db

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	pldb "github.com/probe-lab/go-commons/db"

	"github.com/probe-lab/whisker/pkg/probe"
)

//go:embed migrations
var migrations embed.FS

var _ probe.ResultWriter = (*ClickhouseClient)(nil)

// ClickhouseClient writes probe results to ClickHouse.
// Set Network, ProbeLocation, PublisherURL, and AggregatorURL before calling
// WriteStorageCheckResult.
type ClickhouseClient struct {
	conn driver.Conn

	Network       string
	ProbeLocation string
	PublisherURL  string
	AggregatorURL string
}

func NewClickhouseClient(
	ctx context.Context,
	chCfg *pldb.ClickHouseConfig,
	migCfg *pldb.ClickHouseMigrationsConfig,
) (*ClickhouseClient, error) {
	conn, err := chCfg.OpenAndPing(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect clickhouse: %w", err)
	}

	if err = migCfg.Apply(chCfg.Options(), migrations); err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return &ClickhouseClient{conn: conn}, nil
}

func (c *ClickhouseClient) WriteStorageCheckResult(ctx context.Context, r *probe.StorageCheckResult) error {
	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO storage_checks")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}
	row := c.toRow(r)
	if err := batch.AppendStruct(&row); err != nil {
		_ = batch.Abort()
		return fmt.Errorf("append row: %w", err)
	}
	return batch.Send()
}

func (c *ClickhouseClient) Close() error {
	return c.conn.Close()
}

func (c *ClickhouseClient) toRow(r *probe.StorageCheckResult) StorageCheck {
	runID, _ := uuid.Parse(r.RunID)

	row := StorageCheck{
		Timestamp:       time.Now(),
		Network:         c.Network,
		ProbeLocation:   c.ProbeLocation,
		RunID:           runID,
		PublisherURL:    c.PublisherURL,
		AggregatorURL:   c.AggregatorURL,
		BlobID:          r.BlobID,
		SuiObjectID:     r.SuiObjectID,
		FileSizeBytes:   uint64(r.FileSize),
		ContentLengthOK: r.ContentLengthMatch,
		ContentHashOK:   r.ContentHashMatch,
	}

	row.UploadStartedAt = nonZeroTime(r.UploadStarted)
	row.UploadEndedAt = nonZeroTime(r.UploadFinished)
	row.RegisteredAt = nonZeroTime(r.BlobRegisteredAt)
	row.CertifiedAt = nonZeroTime(r.BlobCertifiedAt)
	row.RetrievalStartedAt = nonZeroTime(r.DownloadStarted)

	row.RetrievalFirstByteAt = nonZeroTime(r.FirstByteAt)
	row.RetrievalLastByteAt = nonZeroTime(r.DownloadFinished)
	if r.DownloadSize > 0 {
		v := uint64(r.DownloadSize)
		row.BytesRetrieved = &v
	}
	row.Status = r.Status

	return row
}

func nonZeroTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
