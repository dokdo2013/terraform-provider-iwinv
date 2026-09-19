package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

const cacheRow = `{"service_idx":9007199254740993,"product_id":"synthetic_cache","name":"캐시 &amp; + %","description":"설명 &amp; + %","status":"active","id":"example123","ip":"192.0.2.8","domain":"cache.example.invalid","spec":{"type":"SINGLE","disk":10},"allow_referer":["b.example.invalid","a.example.invalid"],"password":"excluded-secret"}`

func cacheReceipt() string {
	return strings.Replace(cacheRow, `"product_id":"synthetic_cache",`, "", 1)
}
func cacheInput() CacheInput {
	d := "설명 &amp; + %"
	return CacheInput{ProductID: "synthetic_cache", Name: "캐시 &amp; + %", Account: "example123", Description: &d, FTPPassword: "Synthetic1!"}
}
func TestCacheCreateEncodingAndIdentity(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope(cacheReceipt())}}
	s := CacheService{API: f}
	in := cacheInput()
	out, err := s.Create(context.Background(), in)
	if err != nil || out.ID != "9007199254740993" || out.Service == nil || out.Service.ProductID != "" || out.Service.Account != in.Account {
		t.Fatal("cache create receipt changed", err)
	}
	b := f.body.(map[string]any)
	if f.calls != 1 || f.path != "/v1/cache" || f.method != "POST" || !reflect.DeepEqual(b["pw"], map[string]string{"FTP": in.FTPPassword}) || b["id"] != in.Account {
		t.Fatal("cache nested password contract changed")
	}
	for _, key := range []string{"ftppw", "allow_referer", "version", "server_id"} {
		if _, ok := b[key]; ok {
			t.Fatal("unverified create input sent")
		}
	}
	raw, _ := json.Marshal(in)
	if strings.Contains(string(raw), in.FTPPassword) {
		t.Fatal("password serialized into journal model")
	}
	raw, _ = json.Marshal(out)
	if strings.Contains(string(raw), "excluded-secret") {
		t.Fatal("response credential copied")
	}
	in.Description = nil
	_, err = s.Create(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.body.(map[string]any)["description"]; ok {
		t.Fatal("empty description not omitted")
	}
	for _, e := range []client.Envelope{envelope(`{"service_idx":9007199254740993}`), {Status: 201, Result: json.RawMessage(cacheReceipt())}, {Status: 202, Result: json.RawMessage(cacheReceipt())}, {Status: 200, Result: json.RawMessage(cacheReceipt()), Page: json.RawMessage(`1`)}, envelope(strings.Replace(cacheReceipt(), `"cache.example.invalid"`, `{}`, 1))} {
		f.reply = e
		f.calls = 0
		out, err = s.Create(context.Background(), cacheInput())
		if err == nil || out.ID != "9007199254740993" || f.calls != 1 {
			t.Fatal("partial receipt lost identity or replayed POST")
		}
	}
}
func TestCacheReadAndFullListContracts(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope("[" + cacheRow + "]")}}
	s := CacheService{API: f}
	r, err := s.Read(context.Background(), "9007199254740993")
	if err != nil || r == nil || r.Account != "example123" || r.Name != "캐시 &amp; + %" || r.Type != "SINGLE" || !reflect.DeepEqual(r.Referrers, []string{"a.example.invalid", "b.example.invalid"}) {
		t.Fatal("cache read shape changed", err)
	}
	for _, e := range []client.Envelope{envelope("[" + cacheRow + "," + cacheRow + "]"), envelope("[" + cacheRow + ",{}]"), envelope(`null`), envelope(`{}`), {Status: 404, Result: json.RawMessage(`[]`)}, {Status: 200, Result: json.RawMessage(`[]`), Total: json.RawMessage(`0`)}, {Status: 200, Result: json.RawMessage("[" + cacheRow + "]"), Count: json.RawMessage(`0`)}, envelope("[" + strings.Replace(cacheRow, `["b.example.invalid","a.example.invalid"]`, `null`, 1) + "]"), envelope("[" + strings.Replace(cacheRow, `9007199254740993`, `"9007199254740993"`, 1) + "]")} {
		f.reply = e
		r, err = s.Read(context.Background(), "12")
		if err == nil || r != nil {
			t.Fatal("partial/error response became absence")
		}
	}
	f.reply = envelope("[" + strings.Replace(strings.Replace(cacheRow, `"description":"설명 &amp; + %"`, `"description":null`, 1), `["b.example.invalid","a.example.invalid"]`, `[]`, 1) + "]")
	r, err = s.Read(context.Background(), "9007199254740993")
	if err != nil || r.Description != nil || r.Referrers == nil || len(r.Referrers) != 0 {
		t.Fatal("observed empty/null fields not preserved")
	}
	f.reply = envelope(`[]`)
	r, err = s.Read(context.Background(), "12")
	if err != nil || r != nil {
		t.Fatal("successful empty read failed")
	}
}
func TestCacheReferrersAndDeleteSingleAttempt(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope(`["b.example.invalid","a.example.invalid"]`)}}
	s := CacheService{API: f}
	ctx := context.Background()
	refs := []string{"a.example.invalid", "b.example.invalid"}
	if err := s.ReplaceReferrers(ctx, "12", refs); err != nil {
		t.Fatal(err)
	}
	if f.method != "PUT" || f.path != "/v1/cache/12/allow_referer" || !reflect.DeepEqual(f.body, map[string]any{"allow_referer": refs}) {
		t.Fatal("whole-referrer-set encoding changed")
	}
	for _, e := range []client.Envelope{envelope(`[]`), envelope(`["a.example.invalid"]`), envelope(`["a.example.invalid","a.example.invalid"]`), envelope(`["a.example.invalid","c.example.invalid"]`), envelope(`"ok"`), {Status: 202, Result: json.RawMessage(`["a.example.invalid","b.example.invalid"]`)}, {Status: 200, Result: json.RawMessage(`["a.example.invalid","b.example.invalid"]`), Count: json.RawMessage(`2`)}} {
		f.reply = e
		f.calls = 0
		if err := s.ReplaceReferrers(ctx, "12", refs); err == nil || f.calls != 1 {
			t.Fatal("invalid acknowledgement or replay")
		}
	}
	for _, err := range []error{&client.Error{Kind: "cache_referrers_busy", Status: 404, Code: "NOT_FOUND"}, &client.Error{Kind: "transport"}, errors.New("synthetic failure")} {
		f.err = err
		f.calls = 0
		if got := s.ReplaceReferrers(ctx, "12", refs); got != err || f.calls != 1 {
			t.Fatal("write error hidden or retried")
		}
	}
	f.err = nil
	f.reply = envelope(`"accepted"`)
	if err := s.Delete(ctx, "12"); err != nil || f.method != "DELETE" || f.path != "/v1/cache/12" {
		t.Fatal("delete contract changed")
	}
	for _, raw := range []string{`null`, `[]`, `{}`, `""`} {
		f.reply = envelope(raw)
		if err := s.Delete(ctx, "12"); err == nil {
			t.Fatal("invalid deletion acknowledgement accepted")
		}
	}
}
func TestCacheValidationBeforeWrites(t *testing.T) {
	f := &dbmsFake{}
	s := CacheService{API: f}
	ctx := context.Background()
	for _, refs := range [][]string{nil, {}, {""}, {"a.example.invalid", "a.example.invalid"}, {"https://a.example.invalid"}, {"a.example.invalid/"}, {"*.example.invalid"}, {"a.example.invalid:80"}, {"UPPER.example.invalid"}, {"192.0.2.1"}, {"한글.example.invalid"}, {"-bad.example.invalid"}, {"example.invalid."}} {
		if err := s.ReplaceReferrers(ctx, "12", refs); err == nil {
			t.Fatal("unsupported referrer accepted")
		}
	}
	for _, mutate := range []func(*CacheInput){func(i *CacheInput) { i.Account = "bad" }, func(i *CacheInput) { i.Account = "bad_name" }, func(i *CacheInput) { i.ProductID = "" }, func(i *CacheInput) { i.Name = "x" }, func(i *CacheInput) { i.FTPPassword = "secret" }, func(i *CacheInput) { i.FTPPassword = "longlonglonglonglonglong" }, func(i *CacheInput) { d := ""; i.Description = &d }} {
		in := cacheInput()
		mutate(&in)
		if _, err := s.Create(ctx, in); err == nil || strings.Contains(err.Error(), in.FTPPassword) {
			t.Fatal("invalid creation accepted or password leaked")
		}
	}
	for _, id := range []string{"", "0", "01", "1/path", "9223372036854775808"} {
		if _, err := s.Read(ctx, id); err == nil {
			t.Fatal("invalid ID accepted")
		}
		if err := s.Delete(ctx, id); err == nil {
			t.Fatal("invalid delete accepted")
		}
	}
	if f.calls != 0 {
		t.Fatal("invalid configuration reached API")
	}
}
func TestCacheCatalogNullableIDs(t *testing.T) {
	const a = `{"product_id":null,"product_name":"Future","status":"coming_soon","spec":{"type":"SHARE"}}`
	const b = `{"product_id":"synthetic_cache","product_name":"Synthetic","status":"available","spec":{"type":"SHARE"}}`
	f := &dbmsFake{hostingFake{reply: envelope("[" + a + "," + b + "]")}}
	s := CacheCatalogService{API: f}
	rows, err := s.Products(context.Background(), "SHARE")
	if err != nil || len(rows) != 2 || rows[0].ID != nil || *rows[1].ID != "synthetic_cache" || f.path != "/v1/cache/products" || f.query.Get("type") != "SHARE" {
		t.Fatal("nullable catalog contract changed", err)
	}
	for _, raw := range []string{"[" + b + "," + b + "]", "[" + strings.Replace(a, `"product_id":null,`, "", 1) + "]", "[" + strings.Replace(b, `"SHARE"`, `"SINGLE"`, 1) + "]"} {
		f.reply = envelope(raw)
		if _, err = s.Products(context.Background(), "SHARE"); err == nil {
			t.Fatal("invalid catalog accepted")
		}
	}
	f.reply = envelope(`[]`)
	rows, err = s.Products(context.Background(), "")
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("empty catalog not preserved")
	}
	f.calls = 0
	if _, err = s.Products(context.Background(), "share"); err == nil || f.calls != 0 {
		t.Fatal("invalid filter reached API")
	}
}
