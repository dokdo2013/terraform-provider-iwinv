package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCacheBusyClassificationIsNarrowAndSingleAttempt(t *testing.T) {
	const busy = "서비스가 다른 작업을 진행중입니다."
	for _, tc := range []struct {
		name, method, path, code, reason, message, result string
		status                                            int
		want                                              bool
	}{
		{"exact", "PUT", "/v1/cache/9007199254740993/allow_referer", "0x1", "NOT_FOUND", busy, busy, 404, true},
		{"ordinary missing", "PUT", "/v1/cache/12/allow_referer", "0x1", "NOT_FOUND", "missing", "missing", 404, false},
		{"message only", "PUT", "/v1/cache/12/allow_referer", "0x1", "NOT_FOUND", busy, "missing", 404, false},
		{"result only", "PUT", "/v1/cache/12/allow_referer", "0x1", "NOT_FOUND", "missing", busy, 404, false},
		{"changed text", "PUT", "/v1/cache/12/allow_referer", "0x1", "NOT_FOUND", busy + " synthetic-secret", busy, 404, false},
		{"wrong code", "PUT", "/v1/cache/12/allow_referer", "0xc", "NOT_FOUND", busy, busy, 404, false},
		{"wrong status", "PUT", "/v1/cache/12/allow_referer", "0x1", "NOT_FOUND", busy, busy, 500, false},
		{"business error", "PUT", "/v1/cache/12/allow_referer", "0x1", "NOT_FOUND", busy, busy, 200, false},
		{"different service", "PUT", "/v1/dbms/12/allow_referer", "0x1", "NOT_FOUND", busy, busy, 404, false},
		{"different method", "POST", "/v1/cache/12/allow_referer", "0x1", "NOT_FOUND", busy, busy, 404, false},
		{"delete exact", "DELETE", "/v1/cache/12", "0x1", "NOT_FOUND", busy, busy, 404, true},
		{"other delete", "DELETE", "/v1/dbms/12", "0x1", "NOT_FOUND", busy, busy, 404, false},
		{"delete ordinary missing", "DELETE", "/v1/cache/12", "0x1", "NOT_FOUND", "missing", "missing", 404, false},
		{"read same path", "GET", "/v1/cache/12", "0x1", "NOT_FOUND", busy, busy, 404, false},
		{"noncanonical identity", "PUT", "/v1/cache/012/allow_referer", "0x1", "NOT_FOUND", busy, busy, 404, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": tc.code, "error_code": tc.reason, "message": tc.message, "result": tc.result})
			}))
			defer srv.Close()
			c := testClient(t, srv)
			_, err := c.request(context.Background(), tc.method, tc.path, nil, "application/json", strings.NewReader(`{}`))
			var apiErr *Error
			if !errors.As(err, &apiErr) || (apiErr.Kind == "cache_referrers_busy" || apiErr.Kind == "cache_delete_busy") != tc.want || calls.Load() != 1 {
				t.Fatal("busy classification or single-attempt contract changed")
			}
			if tc.want && tc.method == "DELETE" && apiErr.Kind != "cache_delete_busy" {
				t.Fatal("delete received PUT classification")
			}
			if strings.Contains(err.Error(), busy) || strings.Contains(err.Error(), "synthetic-secret") {
				t.Fatal("server text leaked")
			}
		})
	}
}
func TestIgnoredMessageShapesRemainCompatible(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":"0x00","error_code":"SUCCESS","message":{"unrelated":"synthetic-secret"},"result":[]}`)
	}))
	defer srv.Close()
	if _, err := testClient(t, srv).Get(context.Background(), "/v1/zones", nil); err != nil {
		t.Fatal("unrelated message shape became a new success-envelope requirement")
	}
}
