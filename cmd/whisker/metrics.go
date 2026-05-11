package main

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/probe-lab/whisker/pkg/probe"
)

type probeMetrics struct {
	probeTotal     metric.Int64Counter
	uploadSeconds  metric.Float64Histogram
	certifySeconds metric.Float64Histogram
	ttfbSeconds    metric.Float64Histogram
	ttlbSeconds    metric.Float64Histogram
}

func newProbeMetrics(mp metric.MeterProvider) (*probeMetrics, error) {
	m := mp.Meter("whisker")

	probeTotal, err := m.Int64Counter("probe_total",
		metric.WithDescription("Total number of probe cycles by status"),
	)
	if err != nil {
		return nil, fmt.Errorf("probe_total: %w", err)
	}

	uploadSeconds, err := m.Float64Histogram("upload_seconds",
		metric.WithDescription("Blob upload duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("upload_seconds: %w", err)
	}

	certifySeconds, err := m.Float64Histogram("certify_seconds",
		metric.WithDescription("Time from upload start to blob certified in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("certify_seconds: %w", err)
	}

	ttfbSeconds, err := m.Float64Histogram("download_ttfb_seconds",
		metric.WithDescription("Time to first byte during blob download in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("download_ttfb_seconds: %w", err)
	}

	ttlbSeconds, err := m.Float64Histogram("download_ttlb_seconds",
		metric.WithDescription("Time to last byte during blob download in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("download_ttlb_seconds: %w", err)
	}

	return &probeMetrics{
		probeTotal:     probeTotal,
		uploadSeconds:  uploadSeconds,
		certifySeconds: certifySeconds,
		ttfbSeconds:    ttfbSeconds,
		ttlbSeconds:    ttlbSeconds,
	}, nil
}

func (m *probeMetrics) record(ctx context.Context, r *probe.StorageCheckResult) {
	sizeAttr := attribute.String("size", strconv.FormatInt(r.FileSize, 10))

	m.probeTotal.Add(ctx, 1, metric.WithAttributes(
		sizeAttr,
		attribute.String("status", r.Status),
	))

	if !r.UploadFinished.IsZero() {
		m.uploadSeconds.Record(ctx,
			r.UploadFinished.Sub(r.UploadStarted).Seconds(),
			metric.WithAttributes(sizeAttr),
		)
	}

	if !r.BlobCertifiedAt.IsZero() {
		m.certifySeconds.Record(ctx,
			r.BlobCertifiedAt.Sub(r.UploadStarted).Seconds(),
			metric.WithAttributes(sizeAttr),
		)
	}

	if !r.FirstByteAt.IsZero() {
		m.ttfbSeconds.Record(ctx,
			r.FirstByteAt.Sub(r.DownloadStarted).Seconds(),
			metric.WithAttributes(sizeAttr),
		)
	}

	if !r.DownloadFinished.IsZero() {
		m.ttlbSeconds.Record(ctx,
			r.DownloadFinished.Sub(r.DownloadStarted).Seconds(),
			metric.WithAttributes(sizeAttr),
		)
	}
}

func (m *probeMetrics) recordError(ctx context.Context, size int64) {
	m.probeTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("size", strconv.FormatInt(size, 10)),
		attribute.String("status", "error"),
	))
}
