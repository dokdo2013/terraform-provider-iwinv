package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const success = `{"code":"0x00","error_code":"SUCCESS","result":[],"count":0}`

func testClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c, err := newClient("synthetic-access", "synthetic-secret", server.URL, server.Client().Transport,
		func() time.Time { return time.Unix(1710000000, 0) }, 0)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSigningAndEncoding(t *testing.T) {
	// Independently generated with Python hmac/hashlib; query is not signed.
	const expected = "dc173edadd25dbdd85193174137e90f73a82970b519cb74c09efe51b76715fb2"
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/zones" || r.URL.Query().Get("name") != "서울 & test+한글" {
			t.Errorf("request encoding mismatch")
		}
		if r.Header.Get("X-iwinv-Signature") != expected || r.Header.Get("X-iwinv-Timestamp") != "1710000000" || r.Header.Get("X-iwinv-Credential") != "synthetic-access" {
			t.Errorf("signature mismatch")
		}
		fmt.Fprint(w, success)
	}))
	defer s.Close()
	if _, err := testClient(t, s).Get(context.Background(), "/v1/zones", url.Values{"name": {"서울 & test+한글"}}); err != nil {
		t.Fatal(err)
	}
}

func TestPathsRejectedBeforeNetwork(t *testing.T) {
	c, _ := New("synthetic-access", "synthetic-secret")
	for _, path := range []string{"/v1/zones/", "/v1/zones?x=1", "//evil.example/v1/zones", "https://evil.example", "/v1/../zones", "/v1/%2f", "/v1/한글", "/v1//zones", "/v1/zones#fragment", "/v1/./zones"} {
		t.Run(path, func(t *testing.T) {
			_, err := c.Get(context.Background(), path, nil)
			var apiErr *Error
			if !errors.As(err, &apiErr) || apiErr.Kind != "invalid_path" {
				t.Fatalf("expected invalid_path, got %v", err)
			}
		})
	}
}

func TestEnvelopeAndErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body, kind string
	}{
		{"success", 200, success, ""},
		{"accepted preserved", 202, success, ""},
		{"created preserved", 201, success, ""},
		{"no content", 204, "", "http_status"},
		{"not found", 404, success, "http_status"},
		{"unauthenticated", 401, success, "http_status"},
		{"forbidden", 403, success, "http_status"},
		{"rate limited", 429, success, "http_status"},
		{"server failure", 500, success, "http_status"},
		{"business error", 200, `{"code":"0x1","error_code":"NOT_FOUND","message":"synthetic-secret","result":"error"}`, "business_error"},
		{"unknown code", 200, `{"code":"0x99","error_code":"SUCCESS","result":[]}`, "business_error"},
		{"conflicting success", 200, `{"code":"0x00","error_code":"FAILURE","result":[]}`, "business_error"},
		{"missing code", 200, `{"result":[]}`, "business_error"},
		{"numeric code unverified", 200, `{"code":0,"error_code":"SUCCESS","result":[]}`, "invalid_json"},
		{"invalid documented hex", 200, `{"code":0x00}`, "invalid_json"},
		{"HTML", 200, `<html>synthetic-secret</html>`, "invalid_json"},
		{"trailing JSON", 200, success + `{}`, "invalid_json"},
		{"missing result", 200, `{"code":"0x00","error_code":"SUCCESS"}`, "missing_result"},
		{"null result preserved for service", 200, `{"code":"0x00","error_code":"SUCCESS","result":null}`, ""},
		{"large body", 200, strings.Repeat("x", maxResponseBytes+1), "response_too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.body)
			}))
			defer s.Close()
			result, err := testClient(t, s).Get(context.Background(), "/v1/zones", nil)
			if requests.Load() != 1 {
				t.Fatal("request was retried")
			}
			if tt.kind == "" {
				if err != nil || result.Status != tt.status {
					t.Fatalf("unexpected result: %v", err)
				}
				return
			}
			var apiErr *Error
			if !errors.As(err, &apiErr) || apiErr.Kind != tt.kind || apiErr.Status != tt.status {
				t.Fatalf("expected %s/%d, got %v", tt.kind, tt.status, err)
			}
			if strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("secret leaked")
			}
		})
	}
}

func TestRedirectNeverFollowed(t *testing.T) {
	for _, code := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			var destinationCalls atomic.Int32
			destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destinationCalls.Add(1); fmt.Fprint(w, success) }))
			defer destination.Close()
			origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, code) }))
			defer origin.Close()
			_, err := testClient(t, origin).Get(context.Background(), "/v1/zones", nil)
			if err == nil || destinationCalls.Load() != 0 {
				t.Fatal("redirect followed")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTransportErrorRedacted(t *testing.T) {
	c, _ := newClient("synthetic-access", "synthetic-secret", endpoint, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("synthetic-secret in URL or proxy failure")
	}), time.Now, 0)
	_, err := c.Get(context.Background(), "/v1/zones", nil)
	if err == nil || strings.Contains(err.Error(), "synthetic-secret") || strings.Contains(err.Error(), endpoint) {
		t.Fatalf("unsafe error %v", err)
	}
}

func TestCancellationAndLimiter(t *testing.T) {
	var calls atomic.Int32
	c, _ := newClient("synthetic-access", "synthetic-secret", endpoint, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(success)), Header: make(http.Header)}, nil
	}), time.Now, time.Hour)
	if _, err := c.Get(context.Background(), "/v1/zones", nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := c.Get(ctx, "/v1/zones", nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	if calls.Load() != 1 {
		t.Fatal("limiter allowed a second call")
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if _, err := c.Get(ctx2, "/v1/zones", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestConcurrentClientIsolationAndFreshSignature(t *testing.T) {
	var tick atomic.Int64
	var seen sync.Map
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		access := r.Header.Get("X-iwinv-Credential")
		ts := r.Header.Get("X-iwinv-Timestamp")
		if r.Header.Get("X-iwinv-Signature") != signature(access+"-secret", ts, "/v1/zones") {
			t.Error("credential cross-contamination")
		}
		if _, duplicate := seen.LoadOrStore(ts, true); duplicate {
			t.Error("timestamp was cached")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(success)), Header: make(http.Header)}, nil
	})
	clock := func() time.Time { return time.Unix(tick.Add(1), 0) }
	a, _ := newClient("a", "a-secret", endpoint, transport, clock, 0)
	b, _ := newClient("b", "b-secret", endpoint, transport, clock, 0)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		for _, c := range []*Client{a, b} {
			wg.Add(1)
			go func(c *Client) {
				defer wg.Done()
				if _, err := c.Get(context.Background(), "/v1/zones", nil); err != nil {
					t.Error(err)
				}
			}(c)
		}
	}
	wg.Wait()
}

func TestCredentialsRejected(t *testing.T) {
	for _, keys := range [][2]string{{"", "x"}, {"x", " "}, {"x\r\n", "y"}, {"x", "y\n"}} {
		if _, err := New(keys[0], keys[1]); err == nil {
			t.Fatal("invalid credentials accepted")
		}
	}
}
