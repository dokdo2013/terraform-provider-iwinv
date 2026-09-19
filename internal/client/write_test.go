package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestWriteSigningAndEncoding(t *testing.T) {
	for _, format := range []string{"json", "multipart", "delete"} {
		methods := []string{http.MethodPost, http.MethodPut}
		if format == "delete" {
			methods = []string{http.MethodDelete}
		}
		for _, method := range methods {
			t.Run(format+"/"+method, func(t *testing.T) {
				calls := 0
				s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != method || r.URL.Path != "/v1/zones" || r.URL.RawQuery != "" {
						t.Error("unexpected method or path")
					}
					// Same independent signature vector as GET: the documented signature
					// includes timestamp and path, not method or body.
					if r.Header.Get("X-iwinv-Signature") != "dc173edadd25dbdd85193174137e90f73a82970b519cb74c09efe51b76715fb2" {
						t.Error("signature mismatch")
					}
					switch format {
					case "json":
						if r.Header.Get("Content-Type") != "application/json" {
							t.Error("wrong JSON content type")
						}
						var body map[string]any
						if json.NewDecoder(r.Body).Decode(&body) != nil || body["name"] != "한글 & + %" || body["description"] != "" || body["count"] != float64(1) {
							t.Error("JSON values changed")
						}
						if _, ok := body["omitted"]; ok {
							t.Error("omitted value sent")
						}
					case "multipart":
						if err := r.ParseMultipartForm(maxRequestBytes); err != nil {
							t.Error("invalid multipart")
							return
						}
						defer r.MultipartForm.RemoveAll()
						for key, want := range map[string]string{"name": "한글 & + %", "description": "", "count": "1", "block_storage.0.size": "100"} {
							got, ok := r.MultipartForm.Value[key]
							if !ok || len(got) != 1 || got[0] != want {
								t.Error("multipart value changed or omitted")
							}
						}
						if _, ok := r.MultipartForm.Value["omitted"]; ok {
							t.Error("omitted value sent")
						}
					case "delete":
						b, _ := io.ReadAll(r.Body)
						if len(b) != 0 || r.Header.Get("Content-Type") != "" {
							t.Error("DELETE sent a body")
						}
					}
					w.WriteHeader(202)
					fmt.Fprint(w, success)
				}))
				defer s.Close()
				c := testClient(t, s)
				var result Envelope
				var err error
				switch format {
				case "json":
					body := map[string]any{"name": "한글 & + %", "description": "", "count": 1}
					if method == http.MethodPost {
						result, err = c.PostJSON(context.Background(), "/v1/zones", body)
					} else {
						result, err = c.PutJSON(context.Background(), "/v1/zones", body)
					}
				case "multipart":
					body := map[string]string{"name": "한글 & + %", "description": "", "count": "1", "block_storage.0.size": "100"}
					if method == http.MethodPost {
						result, err = c.PostForm(context.Background(), "/v1/zones", body)
					} else {
						result, err = c.PutForm(context.Background(), "/v1/zones", body)
					}
				case "delete":
					result, err = c.Delete(context.Background(), "/v1/zones")
				}
				if err != nil || result.Status != 202 || calls != 1 {
					t.Fatalf("write contract failed: %v, calls=%d", err, calls)
				}
			})
		}
	}
}

func TestWriteValidationBeforeNetwork(t *testing.T) {
	calls := 0
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, success) }))
	defer s.Close()
	c := testClient(t, s)
	for _, body := range []any{nil, []string{"x"}, "scalar", make(chan int), map[string]string{"value": strings.Repeat("x", maxRequestBytes)}} {
		if _, err := c.PostJSON(context.Background(), "/v1/instances", body); err == nil {
			t.Fatal("invalid JSON payload accepted")
		}
	}
	for _, fields := range []map[string]string{{"bad\r\nfield": "value"}, {"name": strings.Repeat("x", maxRequestBytes)}, {"name\"": "x"}} {
		if _, err := c.PutForm(context.Background(), "/v1/instances", fields); err == nil {
			t.Fatal("invalid form accepted")
		}
	}
	for _, fn := range []func() (Envelope, error){
		func() (Envelope, error) {
			return c.PostJSON(context.Background(), "/v1/../instances", map[string]string{})
		},
		func() (Envelope, error) {
			return c.PostForm(context.Background(), "/v1/instances?bad", map[string]string{})
		},
		func() (Envelope, error) { return c.Delete(context.Background(), "/v1/../instances") },
	} {
		if _, err := fn(); err == nil {
			t.Fatal("invalid write path accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.PostJSON(ctx, "/v1/instances", map[string]string{}); !errors.Is(err, context.Canceled) {
		t.Fatal("JSON cancellation lost")
	}
	if _, err := c.PostForm(ctx, "/v1/instances", map[string]string{}); !errors.Is(err, context.Canceled) {
		t.Fatal("form cancellation lost")
	}
	if _, err := c.Delete(ctx, "/v1/instances"); !errors.Is(err, context.Canceled) {
		t.Fatal("delete cancellation lost")
	}
	if calls != 0 {
		t.Fatal("invalid/canceled request reached API")
	}
}

func TestWriteFailureNeverRetried(t *testing.T) {
	for _, status := range []int{400, 401, 403, 422, 429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
				fmt.Fprint(w, `{"code":"0xc","error_code":"DEV_CHECK_RETURN","result":"synthetic-secret"}`)
			}))
			defer s.Close()
			_, err := testClient(t, s).PostJSON(context.Background(), "/v1/security-groups", map[string]string{"title": "synthetic"})
			if err == nil || strings.Contains(err.Error(), "synthetic-secret") || calls.Load() != 1 {
				t.Fatalf("write failure retried or exposed: %v", err)
			}
		})
	}
	var calls atomic.Int32
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			conn.Close()
		}
	}))
	defer s.Close()
	_, err := testClient(t, s).PostForm(context.Background(), "/v1/instances", map[string]string{"name": "synthetic"})
	if err == nil || calls.Load() != 1 {
		t.Fatal("ambiguous dropped write was retried")
	}
}

func TestWriteRedirectNeverFollowed(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			destination := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, success) }))
			defer destination.Close()
			origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, status) }))
			defer origin.Close()
			_, err := testClient(t, origin).PostJSON(context.Background(), "/v1/security-groups", map[string]string{"title": "synthetic"})
			if err == nil || calls != 0 {
				t.Fatal("write credentials redirected")
			}
		})
	}
}

// Exercise the production transport with a server that offers HTTP/2 and then
// drops a write after receiving it. No hidden protocol or connection replay is
// allowed, even though net/http's default transport supports both mechanisms.
func TestProductionTransportDoesNotReplay(t *testing.T) {
	var calls atomic.Int32
	var mu sync.Mutex
	peers := map[string]bool{}
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.ProtoMajor != 1 {
			t.Error("HTTP/2 transport replay remains enabled")
		}
		mu.Lock()
		peers[r.RemoteAddr] = true
		mu.Unlock()
		if r.Method == http.MethodPost {
			_, _ = io.Copy(io.Discard, r.Body)
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				conn.Close()
			}
			return
		}
		fmt.Fprint(w, success)
	}))
	s.EnableHTTP2 = true
	s.StartTLS()
	defer s.Close()
	c, err := New("synthetic-access", "synthetic-secret")
	if err != nil {
		t.Fatal(err)
	}
	c.baseURL = s.URL
	c.interval = 0
	roots := x509.NewCertPool()
	roots.AddCert(s.Certificate())
	c.http.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots}
	for i := 0; i < 2; i++ {
		if _, err := c.Get(context.Background(), "/v1/zones", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.PostJSON(context.Background(), "/v1/security-groups", map[string]string{"title": "synthetic"}); err == nil {
		t.Fatal("dropped response reported success")
	}
	if calls.Load() != 3 {
		t.Fatal("transport replayed a request")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(peers) != 3 {
		t.Fatal("production transport reused a connection")
	}
}
