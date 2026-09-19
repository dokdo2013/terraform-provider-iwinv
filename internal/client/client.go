// Package client implements the documented iwinv control-plane authentication
// contract. Live compatibility is tracked separately from synthetic tests.
package client

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const endpoint = "https://api-kr.iwinv.kr"
const maxResponseBytes = 2 << 20

// Error deliberately excludes URLs, headers, bodies and server-controlled text.
// Callers must not turn an arbitrary HTTP 404 into resource absence: services
// need a separately verified not-found contract.
type Error struct {
	Kind   string
	Status int
}

func (e *Error) Error() string {
	return fmt.Sprintf("iwinv API %s (HTTP %d)", e.Kind, e.Status)
}

// Envelope preserves missing/null/value distinctions for service decoders.
// It must never be logged: Result can contain account or access information.
type Envelope struct {
	Status   int
	Result   json.RawMessage
	Count    json.RawMessage
	Page     json.RawMessage
	PageNo   json.RawMessage
	PageSize json.RawMessage
	Total    json.RawMessage
}

// Client owns its credentials, transport and rate limiter. No shared account
// cache or global credentials are used. Separate clients cannot coordinate an
// account quota across processes; callers must also control total concurrency.
type Client struct {
	accessKey string
	secretKey string
	baseURL   string
	http      *http.Client
	now       func() time.Time
	interval  time.Duration
	mu        sync.Mutex
	next      time.Time
}

// New creates a TLS-verifying, redirect-rejecting client for the official API.
// Keys are required and must come from provider configuration, not CLI profiles.
func New(accessKey, secretKey string) (*Client, error) {
	return newClient(accessKey, secretKey, endpoint, http.DefaultTransport, time.Now, time.Second)
}

func newClient(accessKey, secretKey, baseURL string, transport http.RoundTripper, now func() time.Time, interval time.Duration) (*Client, error) {
	if strings.TrimSpace(accessKey) == "" || strings.TrimSpace(secretKey) == "" || strings.ContainsAny(accessKey+secretKey, "\r\n") {
		return nil, errors.New("iwinv access key and secret key must be non-empty and contain no line breaks")
	}
	return &Client{
		accessKey: accessKey, secretKey: secretKey, baseURL: baseURL,
		now: now, interval: interval,
		http: &http.Client{Transport: transport, Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
	}, nil
}

func signature(secret, timestamp, path string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + path))
	return hex.EncodeToString(mac.Sum(nil))
}

// wait serializes request admission without holding a mutex during network I/O.
// Cancellation can leave a reserved slot unused; it never increases quota use.
func (c *Client) wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	now := time.Now()
	slot := c.next
	if slot.Before(now) {
		slot = now
	}
	c.next = slot.Add(c.interval)
	c.mu.Unlock()
	timer := time.NewTimer(time.Until(slot))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

// Get performs one attempt only. Retries, pagination and operation-specific 202
// semantics remain service contracts; this layer must not guess them.
// Paths must be canonical ASCII paths. Escaped/non-ASCII IDs need a verified
// signing contract before support is added. Query values are encoded once.
func (c *Client) Get(ctx context.Context, path string, query url.Values) (Envelope, error) {
	var empty Envelope
	if !validPath(path) {
		return empty, &Error{Kind: "invalid_path"}
	}
	if err := c.wait(ctx); err != nil {
		return empty, err
	}
	u := c.baseURL + path
	if q := query.Encode(); q != "" {
		u += "?" + q
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return empty, &Error{Kind: "invalid_request"}
	}
	ts := strconv.FormatInt(c.now().Unix(), 10)
	req.Header.Set("X-iwinv-Timestamp", ts)
	req.Header.Set("X-iwinv-Credential", c.accessKey)
	req.Header.Set("X-iwinv-Signature", signature(c.secretKey, ts, path))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "terraform-provider-iwinv/contract-probe")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		// net/http errors may include a URL, transport text or echoed credentials.
		return empty, &Error{Kind: "transport"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 202 {
		return empty, &Error{Kind: "http_status", Status: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		return empty, &Error{Kind: "response_read", Status: resp.StatusCode}
	}
	if len(body) > maxResponseBytes {
		return empty, &Error{Kind: "response_too_large", Status: resp.StatusCode}
	}
	var wire struct {
		Code      string          `json:"code"`
		ErrorCode string          `json:"error_code"`
		Result    json.RawMessage `json:"result"`
		Count     json.RawMessage `json:"count"`
		Page      json.RawMessage `json:"page"`
		PageNo    json.RawMessage `json:"page_no"`
		PageSize  json.RawMessage `json:"page_size"`
		Total     json.RawMessage `json:"total"`
	}
	if err := json.Unmarshal(body, &wire); err != nil {
		return empty, &Error{Kind: "invalid_json", Status: resp.StatusCode}
	}
	if wire.Code != "0x00" || wire.ErrorCode != "SUCCESS" {
		return empty, &Error{Kind: "business_error", Status: resp.StatusCode}
	}
	if len(wire.Result) == 0 {
		return empty, &Error{Kind: "missing_result", Status: resp.StatusCode}
	}
	return Envelope{Status: resp.StatusCode, Result: wire.Result, Count: wire.Count, Page: wire.Page, PageNo: wire.PageNo, PageSize: wire.PageSize, Total: wire.Total}, nil
}

func validPath(path string) bool {
	if !strings.HasPrefix(path, "/v1/") || strings.HasSuffix(path, "/") || strings.Contains(path, "//") {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	for _, c := range path {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("/-_", c)) {
			return false
		}
	}
	return true
}
