package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type webmailCatalogAPI struct {
	envelope client.Envelope
	err      error
	calls    int
}

func (a *webmailCatalogAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.calls++
	if p != "/v1/webmail/products" || len(q) != 0 {
		return client.Envelope{}, errors.New("unexpected webmail catalog path or filters")
	}
	return a.envelope, a.err
}

const webmailCatalogRows = `[
 {"product_id":"wm-b","product_name":"same name","status":"available","spec":{"type":"SHARE"}},
 {"product_id":"","product_name":"soon B","status":"coming_soon","spec":{"type":"SHARE"}},
 {"product_id":"wm-a","product_name":"same name","status":"available","spec":{"type":"SHARE"}},
 {"product_id":"","product_name":"soon A","status":"coming_soon","spec":{"type":"SHARE"}}
]`

func TestWebmailProducts(t *testing.T) {
	a := &webmailCatalogAPI{envelope: client.Envelope{Status: 200, Result: json.RawMessage(webmailCatalogRows)}}
	s := WebmailCatalogService{API: a}
	got, err := s.Products(context.Background())
	want := []WebmailProduct{{"", "soon A", "coming_soon", "SHARE"}, {"", "soon B", "coming_soon", "SHARE"}, {"wm-a", "same name", "available", "SHARE"}, {"wm-b", "same name", "available", "SHARE"}}
	if err != nil || !reflect.DeepEqual(got, want) || a.calls != 1 {
		t.Fatalf("catalog changed: %v %v", got, err)
	}
	a.envelope.Result = json.RawMessage(`[]`)
	if got, err := s.Products(context.Background()); err != nil || got == nil || len(got) != 0 {
		t.Fatal("valid empty catalog changed")
	}
	a.envelope.Result = json.RawMessage(`[{"product_id":"","product_name":"same","status":"future-status","spec":{"type":"A"}},{"product_id":"","product_name":"same","status":"future-status","spec":{"type":"B"}}]`)
	if got, err := s.Products(context.Background()); err != nil || len(got) != 2 {
		t.Fatal("distinct typed placeholder products lost")
	}
}
func TestWebmailCatalogFailures(t *testing.T) {
	valid := `{"product_id":"wm-a","product_name":"한글 &amp; 이름","status":"available","spec":{"type":"SHARE"}}`
	cases := map[string]string{"null": "null", "object": "{}", "null-row": "[null]", "duplicate-id": "[" + valid + "," + valid + "]", "duplicate-placeholder": `[{"product_id":"","product_name":"soon","status":"coming_soon","spec":{"type":"SHARE"}},{"product_id":"","product_name":"soon","status":"coming_soon","spec":{"type":"SHARE"}}]`}
	for _, key := range []string{"product_id", "product_name", "status", "spec"} {
		for _, kind := range []string{"missing", "null", "wrong-type"} {
			var row map[string]any
			_ = json.Unmarshal([]byte(valid), &row)
			switch kind {
			case "missing":
				delete(row, key)
			case "null":
				row[key] = nil
			case "wrong-type":
				row[key] = false
			}
			b, _ := json.Marshal([]any{row})
			cases[key+"-"+kind] = string(b)
		}
	}
	for _, spec := range []string{`{}`, `{"type":null}`, `{"type":""}`, `{"type":1}`} {
		cases["spec-"+spec] = `[{"product_id":"wm-a","product_name":"name","status":"available","spec":` + spec + `}]`
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s := WebmailCatalogService{API: &webmailCatalogAPI{envelope: client.Envelope{Status: 200, Result: json.RawMessage(body)}}}
			if got, err := s.Products(context.Background()); err == nil || got != nil {
				t.Fatal("invalid or partial catalog accepted")
			}
		})
	}
	for _, change := range []func(*client.Envelope){
		func(e *client.Envelope) { e.Status = 202 }, func(e *client.Envelope) { e.Count = json.RawMessage(`2`) }, func(e *client.Envelope) { e.Count = json.RawMessage(`null`) },
		func(e *client.Envelope) { e.Page = json.RawMessage(`1`) }, func(e *client.Envelope) { e.PageNo = json.RawMessage(`1`) }, func(e *client.Envelope) { e.PageSize = json.RawMessage(`10`) }, func(e *client.Envelope) { e.Total = json.RawMessage(`1`) },
	} {
		e := client.Envelope{Status: 200, Result: json.RawMessage("[" + valid + "]")}
		change(&e)
		s := WebmailCatalogService{API: &webmailCatalogAPI{envelope: e}}
		if got, err := s.Products(context.Background()); err == nil || got != nil {
			t.Fatal("changed envelope accepted")
		}
	}
	a := &webmailCatalogAPI{err: &client.Error{Kind: "http_status", Status: 404, Code: "NOT_FOUND"}}
	s := WebmailCatalogService{API: a}
	if got, err := s.Products(context.Background()); !errors.Is(err, a.err) || got != nil {
		t.Fatal("API error became empty catalog")
	}
	a.calls = 0
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Products(ctx); !errors.Is(err, context.Canceled) || a.calls != 0 {
		t.Fatal("cancelled context reached API")
	}
}
