package sui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// Client is a thin wrapper around the Sui JSON-RPC API.
// Configure fields directly before calling Start, or use the functional options.
type Client struct {
	RPCURL     string
	HTTPClient *http.Client
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration

	nextID atomic.Int64
}

// NewClient creates a Client with sensible defaults for the given RPC endpoint.
func NewClient(rpcURL string) *Client {
	return &Client{
		RPCURL:     rpcURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		MaxRetries: 3,
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
	}
}

// EventFilter specifies which events to query.
// Use the constructor functions to create a filter.
type EventFilter map[string]any

// MoveEventTypeFilter returns a filter that matches events of the given
// fully-qualified Move type, e.g. "0xabc::walrus::BlobRegistered".
func MoveEventTypeFilter(moveEventType string) EventFilter {
	return EventFilter{"MoveEventType": moveEventType}
}

// MoveEventModuleFilter returns a filter that matches all events emitted by
// the given module within a package, e.g. package "0xabc", module "events".
func MoveEventModuleFilter(packageID, module string) EventFilter {
	return EventFilter{
		"MoveEventModule": map[string]string{
			"package": packageID,
			"module":  module,
		},
	}
}

// EventCursor identifies a position in the event stream.
type EventCursor struct {
	TxDigest string `json:"txDigest"`
	EventSeq string `json:"eventSeq"`
}

// Event is a raw Sui event as returned by suix_queryEvents.
// ParsedJSON contains the event payload; unmarshal it into a concrete type.
type Event struct {
	ID          EventCursor     `json:"id"`
	PackageID   string          `json:"packageId"`
	Module      string          `json:"transactionModule"`
	Sender      string          `json:"sender"`
	Type        string          `json:"type"`
	ParsedJSON  json.RawMessage `json:"parsedJson"`
	TimestampMs string          `json:"timestampMs,omitempty"`
}

// EventPage is a page of events returned by suix_queryEvents.
type EventPage struct {
	Data        []Event      `json:"data"`
	NextCursor  *EventCursor `json:"nextCursor"`
	HasNextPage bool         `json:"hasNextPage"`
}

// ObjectDataOptions controls which fields are included in a sui_getObject response.
type ObjectDataOptions struct {
	ShowType    bool `json:"showType,omitempty"`
	ShowContent bool `json:"showContent,omitempty"`
	ShowOwner   bool `json:"showOwner,omitempty"`
}

// Object is a Sui object as returned by sui_getObject.
// Content holds the raw Move object fields; unmarshal it into a concrete type.
type Object struct {
	ObjectID string          `json:"objectId"`
	Version  string          `json:"version"`
	Digest   string          `json:"digest"`
	Type     string          `json:"type,omitempty"`
	Content  json.RawMessage `json:"content,omitempty"`
}

// QueryEvents queries events matching filter, starting from cursor (nil = from genesis).
// limit controls the maximum number of events per page.
func (c *Client) QueryEvents(ctx context.Context, filter EventFilter, cursor *EventCursor, limit int) (*EventPage, error) {
	return c.queryEvents(ctx, filter, cursor, limit, false)
}

// LatestEventCursor returns the cursor of the most recent event matching filter,
// or nil if no matching events exist yet.
func (c *Client) LatestEventCursor(ctx context.Context, filter EventFilter) (*EventCursor, error) {
	page, err := c.queryEvents(ctx, filter, nil, 1, true)
	if err != nil {
		return nil, err
	}
	if len(page.Data) == 0 {
		return nil, nil
	}
	cursor := page.Data[0].ID
	return &cursor, nil
}

func (c *Client) queryEvents(ctx context.Context, filter EventFilter, cursor *EventCursor, limit int, descending bool) (*EventPage, error) {
	var page EventPage
	if err := c.call(ctx, "suix_queryEvents", []any{filter, cursor, limit, descending}, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetObject retrieves a Sui object by ID.
func (c *Client) GetObject(ctx context.Context, objectID string, opts ObjectDataOptions) (*Object, error) {
	var result struct {
		Data  *Object         `json:"data"`
		Error json.RawMessage `json:"error,omitempty"`
	}
	if err := c.call(ctx, "sui_getObject", []any{objectID, opts}, &result); err != nil {
		return nil, err
	}
	if result.Data == nil {
		return nil, fmt.Errorf("object %s not found", objectID)
	}
	return result.Data, nil
}

// --- internal JSON-RPC plumbing ---

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// call makes a JSON-RPC call, retrying on transient errors with exponential backoff.
func (c *Client) call(ctx context.Context, method string, params []any, result any) error {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      c.nextID.Add(1),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	delay := c.BaseDelay
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
			if delay > c.MaxDelay {
				delay = c.MaxDelay
			}
		}

		raw, transient, err := c.doOnce(ctx, body)
		if err != nil {
			if !transient {
				return err
			}
			lastErr = err
			continue
		}

		var resp rpcResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
		if resp.Error != nil {
			return resp.Error
		}
		if result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// doOnce performs a single HTTP POST to the RPC endpoint.
// Returns the response body, whether the error is transient, and any error.
func (c *Client) doOnce(ctx context.Context, body []byte) (_ []byte, transient bool, _ error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RPCURL, bytes.NewReader(body))
	if err != nil {
		return nil, false, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode >= http.StatusInternalServerError:
		return nil, true, fmt.Errorf("http status %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, false, fmt.Errorf("http status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("read response: %w", err)
	}
	return data, false, nil
}
