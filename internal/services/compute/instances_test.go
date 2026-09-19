package compute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// Entirely synthetic, including the deliberately unsolicited access fields.
func instanceFixture(id string) map[string]any {
	return map[string]any{
		"instance_id": id, "name": "테스트 & <name>", "description": nil, "status": "building",
		"zone":            map[string]any{"zone_id": "synthetic-zone"},
		"flavor":          map[string]any{"flavor_id": "synthetic-flavor"},
		"image":           map[string]any{"image_id": "synthetic-image"},
		"default_account": map[string]any{"password": "synthetic-password-canary"},
		"vnc":             map[string]any{"link": "https://example.invalid/synthetic-console-canary"},
	}
}

func instanceEnvelope(page, count int) client.Envelope {
	rows := make([]map[string]any, 0, count)
	for i := count - 1; i >= 0; i-- {
		rows = append(rows, instanceFixture(fmt.Sprintf("synthetic-%05d", (page-1)*instancePageSize+i)))
	}
	b, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: b, Count: json.RawMessage(strconv.Itoa(count)),
		PageNo: json.RawMessage(strconv.Itoa(page)), PageSize: json.RawMessage(`10`)}
}

func TestInstanceProjectionAndPagination(t *testing.T) {
	for _, total := range []int{0, 1, 10, 21} {
		t.Run(strconv.Itoa(total), func(t *testing.T) {
			calls := 0
			s := Service{API: apiFunc(func(_ context.Context, path string, q url.Values) (client.Envelope, error) {
				calls++
				// Hard-coded documented mask: changing the implementation constant
				// must not silently expand the requested sensitive fields.
				want := url.Values{"fields": {"3599"}, "page_no": {strconv.Itoa(calls)}, "page_size": {"10"}}
				if path != "/v1/instances" || !reflect.DeepEqual(q, want) {
					t.Fatal("incorrect instance list path, fields or pagination")
				}
				return instanceEnvelope(calls, min(10, total-(calls-1)*10)), nil
			})}
			rows, err := s.Instances(context.Background())
			if err != nil || rows == nil || len(rows) != total || calls != total/10+1 {
				t.Fatalf("list traversal failed: %v", err)
			}
			for i, row := range rows {
				if row.ID != fmt.Sprintf("synthetic-%05d", i) || row.Status != "building" || row.Name != "테스트 & <name>" || row.Description != nil || row.ZoneID != "synthetic-zone" || row.FlavorID != "synthetic-flavor" || row.ImageID != "synthetic-image" {
					t.Fatal("projection altered identifiers, Unicode, order or state")
				}
			}
			b, _ := json.Marshal(rows)
			if strings.Contains(string(b), "canary") || strings.Contains(string(b), "default_account") || strings.Contains(string(b), "vnc") {
				t.Fatal("unsolicited credentials escaped typed projection")
			}
		})
	}
}

func TestInstanceMalformedPagesNeverReturnPartialResults(t *testing.T) {
	mutations := map[string]func(*client.Envelope){
		"null result":          func(e *client.Envelope) { e.Result = json.RawMessage(`null`) },
		"object result":        func(e *client.Envelope) { e.Result = json.RawMessage(`{}`) },
		"missing count":        func(e *client.Envelope) { e.Count = nil },
		"null count":           func(e *client.Envelope) { e.Count = json.RawMessage(`null`) },
		"string count":         func(e *client.Envelope) { e.Count = json.RawMessage(`"1"`) },
		"wrong count":          func(e *client.Envelope) { e.Count = json.RawMessage(`0`) },
		"unexpected status":    func(e *client.Envelope) { e.Status = 202 },
		"missing page":         func(e *client.Envelope) { e.PageNo = nil },
		"repeated page":        func(e *client.Envelope) { e.PageNo = json.RawMessage(`1`) },
		"wrong page size":      func(e *client.Envelope) { e.PageSize = json.RawMessage(`100`) },
		"null page size":       func(e *client.Envelope) { e.PageSize = json.RawMessage(`null`) },
		"oversized page":       func(e *client.Envelope) { *e = instanceEnvelope(2, 11) },
		"duplicate identifier": func(e *client.Envelope) { e.Result = instanceEnvelope(1, 1).Result },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			calls := 0
			s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
				calls++
				if calls == 1 {
					return instanceEnvelope(1, 10), nil
				}
				e := instanceEnvelope(2, 1)
				mutate(&e)
				return e, nil
			})}
			if rows, err := s.Instances(context.Background()); err == nil || rows != nil || calls != 2 {
				t.Fatal("malformed late page returned partial success")
			}
		})
	}
}

func TestInstanceSelectedFieldValidation(t *testing.T) {
	cases := map[string]func(map[string]any){
		"missing id":          func(r map[string]any) { delete(r, "instance_id") },
		"invalid id":          func(r map[string]any) { r["instance_id"] = "../bad" },
		"missing name":        func(r map[string]any) { delete(r, "name") },
		"null name":           func(r map[string]any) { r["name"] = nil },
		"null row":            nil,
		"missing description": func(r map[string]any) { delete(r, "description") },
		"wrong description":   func(r map[string]any) { r["description"] = 12 },
		"missing status":      func(r map[string]any) { delete(r, "status") },
		"null zone":           func(r map[string]any) { r["zone"] = nil },
		"missing flavor id":   func(r map[string]any) { r["flavor"] = map[string]any{} },
		"wrong image shape":   func(r map[string]any) { r["image"] = []string{} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := instanceFixture("synthetic-00000")
			if mutate == nil {
				r = nil
			} else {
				mutate(r)
			}
			b, _ := json.Marshal([]map[string]any{r})
			e := instanceEnvelope(1, 1)
			e.Result = b
			if rows, err := instanceRows(e); err == nil || rows != nil || strings.Contains(err.Error(), "canary") {
				t.Fatal("invalid selected field accepted or raw response exposed")
			}
		})
	}
	for _, text := range []string{"", "설명 &amp; <x>", "e\u0301"} {
		r := instanceFixture("synthetic-00000")
		r["description"] = text
		b, _ := json.Marshal([]map[string]any{r})
		e := instanceEnvelope(1, 1)
		e.Result = b
		rows, err := instanceRows(e)
		if err != nil || rows[0].Description == nil || *rows[0].Description != text {
			t.Fatal("description null/empty/Unicode distinction lost")
		}
	}
}

func TestInstanceDetailDoesNotInferAbsence(t *testing.T) {
	e := instanceEnvelope(1, 1)
	e.PageNo = nil
	e.PageSize = nil
	calls := 0
	s := Service{API: apiFunc(func(_ context.Context, path string, q url.Values) (client.Envelope, error) {
		calls++
		if path != "/v1/instances/synthetic-00000" || !reflect.DeepEqual(q, url.Values{"fields": {"3599"}}) {
			t.Fatal("detail request changed")
		}
		return e, nil
	})}
	if row, err := s.Instance(context.Background(), "synthetic-00000"); err != nil || row.ID != "synthetic-00000" {
		t.Fatalf("exact lookup failed: %v", err)
	}
	for _, bad := range []client.Envelope{instanceEnvelope(1, 0), instanceEnvelope(1, 2), instanceEnvelope(2, 1)} {
		e = bad
		if row, err := s.Instance(context.Background(), "synthetic-00000"); err == nil || row.ID != "" {
			t.Fatal("empty, multiple or mismatched detail accepted")
		}
	}
	before := calls
	for _, id := range []string{"", ".", "..", "bad/id", "bad?fields=128", "bad#x", "%2F", "bad\nvalue"} {
		if _, err := s.Instance(context.Background(), id); err == nil || calls != before {
			t.Fatal("invalid path segment reached API")
		}
	}
	for _, status := range []int{401, 403, 404, 429, 500} {
		want := &client.Error{Kind: "http", Status: status}
		s.API = apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) { return client.Envelope{}, want })
		if _, err := s.Instance(context.Background(), "synthetic-00000"); !errors.Is(err, want) {
			t.Fatal("API error was converted to absence or success")
		}
	}
}

func TestInstanceBoundsCancellationAndLateFailure(t *testing.T) {
	calls := 0
	s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
		calls++
		return instanceEnvelope(calls, 10), nil
	})}
	if rows, err := s.Instances(context.Background()); err == nil || rows != nil || calls != instanceMaxPages {
		t.Fatal("pagination did not stop at bounded limit")
	}
	calls = 0
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Instances(ctx); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("canceled list executed")
	}
	if _, err := s.Instance(ctx, "synthetic-00000"); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("canceled detail executed")
	}
	want := errors.New("synthetic transport failure")
	s.API = apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
		calls++
		if calls == 2 {
			return client.Envelope{}, want
		}
		return instanceEnvelope(1, 10), nil
	})
	if rows, err := s.Instances(context.Background()); rows != nil || !errors.Is(err, want) || calls != 2 {
		t.Fatal("late failure hidden or automatically replayed")
	}
}
