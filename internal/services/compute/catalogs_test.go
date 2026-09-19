package compute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type apiFunc func(context.Context, string, url.Values) (client.Envelope, error)

func (f apiFunc) Get(c context.Context, p string, q url.Values) (client.Envelope, error) {
	return f(c, p, q)
}
func pageEnvelope(kind CatalogKind, page, count, total int) client.Envelope {
	rows := make([]CatalogItem, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("synthetic_%04d.1", (page-1)*10+i)
		rows = append(rows, CatalogItem{ImageID: id, FlavorID: id, Name: "Synthetic", Visibility: "public", ImageType: "os_linux"})
	}
	b, _ := json.Marshal(rows)
	e := client.Envelope{Status: 200, Result: b, Count: json.RawMessage(strconv.Itoa(count)), PageNo: json.RawMessage(strconv.Itoa(page)), PageSize: json.RawMessage(`10`)}
	if kind == InstanceTypes {
		e.Total = json.RawMessage(strconv.Itoa(total))
	}
	return e
}
func TestCatalogPagination(t *testing.T) {
	for _, kind := range []CatalogKind{Images, InstanceTypes} {
		t.Run(string(kind), func(t *testing.T) {
			calls := 0
			s := Service{API: apiFunc(func(_ context.Context, path string, q url.Values) (client.Envelope, error) {
				calls++
				if path != "/v1/"+string(kind) || q.Get("page_no") != strconv.Itoa(calls) || q.Get("page_size") != "10" {
					t.Fatal("wrong page request")
				}
				count := 10
				if calls == 3 {
					count = 0
				}
				return pageEnvelope(kind, calls, count, 20), nil
			})}
			ids, err := s.CatalogIDs(context.Background(), kind)
			wantCalls := 3
			if kind == InstanceTypes {
				wantCalls = 2
			}
			if err != nil || len(ids) != 20 || calls != wantCalls {
				t.Fatalf("pagination failed: count=%d calls=%d err=%v", len(ids), calls, err)
			}
		})
	}
}
func TestCatalogRejectsPartialResults(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*client.Envelope)
	}{
		{"wrong page", func(e *client.Envelope) { e.PageNo = json.RawMessage(`1`) }},
		{"missing size", func(e *client.Envelope) { e.PageSize = nil }},
		{"null size", func(e *client.Envelope) { e.PageSize = json.RawMessage(`null`) }},
		{"wrong count", func(e *client.Envelope) { e.Count = json.RawMessage(`1`) }},
		{"changed total", func(e *client.Envelope) { e.Total = json.RawMessage(`21`) }},
		{"missing total", func(e *client.Envelope) { e.Total = nil }},
		{"null rows", func(e *client.Envelope) { e.Result = json.RawMessage(`null`) }},
		{"duplicate page", func(e *client.Envelope) { e.Result = pageEnvelope(InstanceTypes, 1, 10, 20).Result }},
		{"missing IDs", func(e *client.Envelope) { e.Result = json.RawMessage(`[{}]`); e.Count = json.RawMessage(`1`) }},
		{"wrong status", func(e *client.Envelope) { e.Status = 202 }},
		{"short early page", func(e *client.Envelope) { *e = pageEnvelope(InstanceTypes, 2, 0, 20) }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
				calls++
				e := pageEnvelope(InstanceTypes, calls, 10, 20)
				if calls == 2 {
					tt.mutate(&e)
				}
				return e, nil
			})}
			ids, err := s.CatalogIDs(context.Background(), InstanceTypes)
			if err == nil || ids != nil {
				t.Fatal("invalid contract returned partial success")
			}
		})
	}
	want := errors.New("synthetic failure")
	calls := 0
	s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
		calls++
		if calls == 2 {
			return client.Envelope{}, want
		}
		return pageEnvelope(Images, 1, 10, 0), nil
	})}
	if ids, err := s.CatalogIDs(context.Background(), Images); ids != nil || !errors.Is(err, want) {
		t.Fatal("late API failure was hidden")
	}
}
func TestCatalogBoundsAndCancellation(t *testing.T) {
	calls := 0
	s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
		calls++
		return pageEnvelope(Images, calls, 10, 0), nil
	})}
	if ids, err := s.CatalogIDs(context.Background(), Images); ids != nil || err == nil || calls != catalogMaxPages {
		t.Fatal("unbounded pagination")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	if _, err := s.CatalogIDs(ctx, Images); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("canceled request executed")
	}
	if _, err := s.CatalogIDs(context.Background(), CatalogKind("other")); err == nil || calls != 0 {
		t.Fatal("invalid catalog executed")
	}
}
func TestCatalogExactLookup(t *testing.T) {
	for _, kind := range []CatalogKind{Images, InstanceTypes} {
		t.Run(string(kind), func(t *testing.T) {
			e := pageEnvelope(kind, 1, 1, 1)
			s := Service{API: apiFunc(func(_ context.Context, path string, q url.Values) (client.Envelope, error) {
				if path != "/v1/"+string(kind)+"/synthetic_0000.1" || len(q) != 0 {
					t.Fatal("wrong detail request")
				}
				return e, nil
			})}
			if _, err := s.CatalogItem(context.Background(), kind, "synthetic_0000.1"); err != nil {
				t.Fatal(err)
			}
			for _, count := range []int{0, 2} {
				e = pageEnvelope(kind, 1, count, count)
				if _, err := s.CatalogItem(context.Background(), kind, "synthetic_0000.1"); err == nil {
					t.Fatal("non-singleton accepted")
				}
			}
			e = pageEnvelope(kind, 2, 1, 1)
			if _, err := s.CatalogItem(context.Background(), kind, "synthetic_0000.1"); err == nil {
				t.Fatal("wrong ID accepted")
			}
		})
	}
	calls := 0
	s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
		calls++
		return client.Envelope{}, nil
	})}
	for _, id := range []string{"", ".", "..", "foo/bar", "foo?bar", "foo%2fbar", "한글"} {
		if _, err := s.CatalogItem(context.Background(), Images, id); err == nil {
			t.Fatal("invalid ID accepted")
		}
	}
	if calls != 0 {
		t.Fatal("invalid ID sent to API")
	}
}
