package walrus

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/blobs/%s", c.BaseURL, urlID), nil)
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

// normaliseBlobID converts a decimal blob ID to base64url if necessary.
// A decimal u256 string is converted to a 32-byte big-endian base64url value.
// Any other string is returned unchanged (assumed to be base64url already).
func normaliseBlobID(id string) (string, error) {
	n := new(big.Int)
	if _, ok := n.SetString(id, 10); !ok {
		return id, nil
	}
	b := n.Bytes()
	if len(b) > 32 {
		return "", fmt.Errorf("value too large (%d bytes, max 32)", len(b))
	}
	padded := make([]byte, 32)
	copy(padded[32-len(b):], b)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(padded), nil
}
