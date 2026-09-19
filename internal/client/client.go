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
	"net"
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
	Code   string
}

func (e *Error) Error() string {
	if e.Code != "" {
		if _, ok := documentedErrors[e.Code]; ok {
			return fmt.Sprintf("iwinv API %s: %s (HTTP %d)", e.Kind, e.Code, e.Status)
		}
	}
	return fmt.Sprintf("iwinv API %s (HTTP %d)", e.Kind, e.Status)
}

// Source: https://api-kr.iwinv.kr/error. Only fixed, reviewed identifiers may
// enter diagnostics. Unknown remote code/message strings are never reflected.
var documentedErrors = map[string]string{
	"NOT_FOUND": "0x1", "CIDR_NOT_VALID": "0x2", "CIDR_NOT_REGISTERED": "0x3",
	"IPV6_NOT_SUPPORTED": "0x4", "CHECK_REQUEST_IP": "0x5", "CHECK_CREDENTIAL": "0x6",
	"CHECK_SIGNATURE": "0x7", "INVALID_SIGNATURE": "0x8", "CHECK_IP": "0x9",
	"REQUIRED_GET_PARAM_MISSING": "0xa", "REQUIRED_POST_PARAM_MISSING": "0xb",
	"DEV_CHECK_RETURN": "0xc", "UNAVAILABLE_FLAVOR": "0xd", "CHECK_PARAM": "0xe",
	"CHECK_PARAM_ENUM": "0xf", "UNAVAILABLE_COMBINATION": "0x10", "CHECK_LENGTH": "0x11",
	"UNAVAILABLE_SSH_KEY": "0x12", "DELETED_OR_WORKING_INSTANCE": "0x13",
	"EMPTY_SET": "0x14", "IN_USE": "0x15", "ID_INVALID": "0x16",
	"LIMIT_EXCEEDED": "0x17", "CHECK_CONSOLE": "0x18",
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
	return newClient(accessKey, secretKey, endpoint, singleAttemptTransport(), time.Now, time.Second)
}

// Disable transparent transport replay: Go's HTTP/2 transport can reconstruct
// request bodies after stream errors, and HTTP/1 can retry on reused connections.
// Until endpoint-specific idempotency is proven, use fresh HTTP/1 connections.
func singleAttemptTransport() *http.Transport {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		Protocols:             protocols,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
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
	return c.request(ctx, http.MethodGet, path, query, "", nil)
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, contentType string, bodyReader io.Reader) (Envelope, error) {
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
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return empty, &Error{Kind: "invalid_request"}
	}
	if method != http.MethodGet {
		req.GetBody = nil
	}
	ts := strconv.FormatInt(c.now().Unix(), 10)
	req.Header.Set("X-iwinv-Timestamp", ts)
	req.Header.Set("X-iwinv-Credential", c.accessKey)
	req.Header.Set("X-iwinv-Signature", signature(c.secretKey, ts, path))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "terraform-provider-iwinv/dev")
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return empty, ctx.Err()
		}
		// net/http errors may include a URL, transport text or echoed credentials.
		return empty, &Error{Kind: "transport"}
	}
	defer resp.Body.Close()
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
		Message   json.RawMessage `json:"message"`
		Result    json.RawMessage `json:"result"`
		Count     json.RawMessage `json:"count"`
		Page      json.RawMessage `json:"page"`
		PageNo    json.RawMessage `json:"page_no"`
		PageSize  json.RawMessage `json:"page_size"`
		Total     json.RawMessage `json:"total"`
	}
	decodeErr := json.Unmarshal(body, &wire)
	knownCode := ""
	if expected, ok := documentedErrors[wire.ErrorCode]; decodeErr == nil && ok && expected == wire.Code {
		knownCode = wire.ErrorCode
	}
	if resp.StatusCode < 200 || resp.StatusCode > 202 {
		// Cache endpoints overload NOT_FOUND for a rejected busy
		// operation while the parent still exists. Classify only this exact
		// observed response; no raw text escapes and this layer never retries.
		busyKind := ""
		if method == http.MethodPut && cacheReferrerPath(path) {
			busyKind = "cache_referrers_busy"
		} else if method == http.MethodDelete && cacheDeletePath(path) {
			busyKind = "cache_delete_busy"
		}
		if resp.StatusCode == http.StatusNotFound && knownCode == "NOT_FOUND" && busyKind != "" {
			const busy = "서비스가 다른 작업을 진행중입니다."
			var message, result string
			if json.Unmarshal(wire.Message, &message) == nil && message == busy && json.Unmarshal(wire.Result, &result) == nil && result == busy {
				return empty, &Error{Kind: busyKind, Status: resp.StatusCode, Code: knownCode}
			}
		}
		return empty, &Error{Kind: "http_status", Status: resp.StatusCode, Code: knownCode}
	}
	if decodeErr != nil {
		return empty, &Error{Kind: "invalid_json", Status: resp.StatusCode}
	}
	if wire.Code != "0x00" || wire.ErrorCode != "SUCCESS" {
		return empty, &Error{Kind: "business_error", Status: resp.StatusCode, Code: knownCode}
	}
	if len(wire.Result) == 0 {
		return empty, &Error{Kind: "missing_result", Status: resp.StatusCode}
	}
	return Envelope{Status: resp.StatusCode, Result: wire.Result, Count: wire.Count, Page: wire.Page, PageNo: wire.PageNo, PageSize: wire.PageSize, Total: wire.Total}, nil
}

func cacheReferrerPath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) != 5 || parts[0] != "" || parts[1] != "v1" || parts[2] != "cache" || parts[4] != "allow_referer" {
		return false
	}
	id, err := strconv.ParseInt(parts[3], 10, 64)
	return err == nil && id > 0 && strconv.FormatInt(id, 10) == parts[3]
}

func cacheDeletePath(path string) bool {
	parts := strings.Split(path, "/")
	if len(parts) != 4 || parts[0] != "" || parts[1] != "v1" || parts[2] != "cache" {
		return false
	}
	id, err := strconv.ParseInt(parts[3], 10, 64)
	return err == nil && id > 0 && strconv.FormatInt(id, 10) == parts[3]
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
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("/.-_", c)) {
			return false
		}
	}
	return true
}
