package walrus

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"time"
)

// FetchResult holds timing and size metrics from a blob fetch.
type FetchResult struct {
	Size       int64
	TTFB       time.Duration // time to first byte: request sent to response headers received
	TTLB       time.Duration // time to last byte: request sent to body fully read
	Throughput float64       // bytes per second
}

// AggregatorClient fetches blobs from a Walrus aggregator via the HTTP API.
type AggregatorClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewAggregatorClient returns a client for the given aggregator base URL.
func NewAggregatorClient(baseURL string) *AggregatorClient {
	return &AggregatorClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// FetchBlob fetches a blob by ID, writes the content to w, and returns metrics.
// blobID may be a decimal u256 string (as returned by Walrus on-chain events) or
// a base64url string (as used by the HTTP API). Decimal IDs are converted automatically.
func (c *AggregatorClient) FetchBlob(ctx context.Context, blobID string, w io.Writer) (*FetchResult, error) {
	urlID, err := normaliseBlobID(blobID)
	if err != nil {
		return nil, fmt.Errorf("invalid blob id: %w", err)
	}

	url := fmt.Sprintf("%s/v1/blobs/%s", c.BaseURL, urlID)
	slog.Debug("fetching blob", "url", url, "blob_id_base64", urlID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	start := time.Now()
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	ttfb := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}

	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	ttlb := time.Since(start)

	return &FetchResult{
		Size:       n,
		TTFB:       ttfb,
		TTLB:       ttlb,
		Throughput: float64(n) / ttlb.Seconds(),
	}, nil
}

// BlobIDBase64 returns the base64url representation of a blob ID.
// Decimal u256 strings (as returned by Walrus on-chain events) are converted
// to the 32-byte little-endian base64url form used by the Walrus HTTP API.
// Strings that are not valid decimals are returned unchanged.
func BlobIDBase64(id string) (string, error) {
	return normaliseBlobID(id)
}

// normaliseBlobID converts a decimal blob ID to base64url if necessary.
// Walrus stores blob IDs as 32-byte little-endian values; decimal u256 strings
// (as returned by Sui on-chain events) are converted accordingly.
// Any non-decimal string is returned unchanged (assumed to be base64url already).
func normaliseBlobID(id string) (string, error) {
	n := new(big.Int)
	if _, ok := n.SetString(id, 10); !ok {
		return id, nil
	}
	be := n.Bytes() // big-endian from math/big
	if len(be) > 32 {
		return "", fmt.Errorf("value too large (%d bytes, max 32)", len(be))
	}
	// Convert to 32-byte little-endian (LSB first, zero-padded at the high end).
	var le [32]byte
	for i, b := range be {
		le[len(be)-1-i] = b
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(le[:]), nil
}
