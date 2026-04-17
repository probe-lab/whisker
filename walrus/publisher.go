package walrus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// UploadResult holds the outcome of a blob upload.
// Exactly one of NewlyCreated and AlreadyCertified is non-nil.
type UploadResult struct {
	NewlyCreated     *NewlyCreatedResult
	AlreadyCertified *AlreadyCertifiedResult
}

// BlobID returns the base64url blob ID from whichever response shape was received.
func (r *UploadResult) BlobID() string {
	if r.NewlyCreated != nil {
		return r.NewlyCreated.BlobID
	}
	return r.AlreadyCertified.BlobID
}

// NewlyCreatedResult is the response when the publisher stored a new blob.
type NewlyCreatedResult struct {
	BlobID         string
	SuiObjectID    string
	CertifiedEpoch uint32
	EndEpoch       uint32
	Cost           uint64
	Deletable      bool
}

// AlreadyCertifiedResult is the response when the blob was already certified on-chain.
type AlreadyCertifiedResult struct {
	BlobID   string
	TxDigest string
	EventSeq string
	EndEpoch uint32
}

// PublisherClient uploads blobs to a Walrus publisher via the HTTP API.
type PublisherClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewPublisherClient returns a client for the given publisher base URL.
func NewPublisherClient(baseURL string) *PublisherClient {
	return &PublisherClient{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

// UploadOptions controls how a blob is stored.
type UploadOptions struct {
	Epochs    uint32 // number of epochs to store; 0 uses the publisher default (usually 1)
	Deletable bool   // whether the blob can be deleted by the owner
	SendTo    string // Sui address to send the blob object to; empty uses the publisher's address
}

// UploadBlob streams r to the publisher and returns the upload result.
// contentLength should be set to the exact byte count of r, or -1 if unknown (chunked transfer).
func (c *PublisherClient) UploadBlob(ctx context.Context, r io.Reader, contentLength int64, opts UploadOptions) (*UploadResult, error) {
	endpoint := fmt.Sprintf("%s/v1/blobs", c.BaseURL)

	params := url.Values{}
	if opts.Epochs > 0 {
		params.Set("epochs", strconv.FormatUint(uint64(opts.Epochs), 10))
	}
	if opts.Deletable {
		params.Set("deletable", "true")
	}
	if opts.SendTo != "" {
		params.Set("send_object_to", opts.SendTo)
	}
	if len(params) > 0 {
		endpoint = endpoint + "?" + params.Encode()
	}

	slog.Debug("uploading blob", "url", endpoint, "epochs", opts.Epochs, "deletable", opts.Deletable)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, r)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.ContentLength = contentLength

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	return parseUploadResponse(body)
}

// publisherResponse mirrors the publisher JSON response, handling both shapes.
type publisherResponse struct {
	NewlyCreated *struct {
		BlobObject struct {
			ID             string `json:"id"`
			BlobID         string `json:"blobId"`
			CertifiedEpoch uint32 `json:"certifiedEpoch"`
			Storage        struct {
				EndEpoch uint32 `json:"endEpoch"`
			} `json:"storage"`
			Deletable bool `json:"deletable"`
		} `json:"blobObject"`
		Cost uint64 `json:"cost"`
	} `json:"newlyCreated"`

	AlreadyCertified *struct {
		BlobID string `json:"blobId"`
		Event  struct {
			TxDigest string `json:"txDigest"`
			EventSeq string `json:"eventSeq"`
		} `json:"event"`
		EndEpoch uint32 `json:"endEpoch"`
	} `json:"alreadyCertified"`
}

func parseUploadResponse(body []byte) (*UploadResult, error) {
	var raw publisherResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	switch {
	case raw.NewlyCreated != nil:
		nc := raw.NewlyCreated
		return &UploadResult{
			NewlyCreated: &NewlyCreatedResult{
				BlobID:         nc.BlobObject.BlobID,
				SuiObjectID:    nc.BlobObject.ID,
				CertifiedEpoch: nc.BlobObject.CertifiedEpoch,
				EndEpoch:       nc.BlobObject.Storage.EndEpoch,
				Cost:           nc.Cost,
				Deletable:      nc.BlobObject.Deletable,
			},
		}, nil

	case raw.AlreadyCertified != nil:
		ac := raw.AlreadyCertified
		return &UploadResult{
			AlreadyCertified: &AlreadyCertifiedResult{
				BlobID:   ac.BlobID,
				TxDigest: ac.Event.TxDigest,
				EventSeq: ac.Event.EventSeq,
				EndEpoch: ac.EndEpoch,
			},
		}, nil

	default:
		return nil, fmt.Errorf("unrecognised response shape: %s", truncate(string(body), 200))
	}
}

// truncate shortens s to at most n bytes, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
