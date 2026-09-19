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

const nasRow = `{"service_idx":9007199254740993,"product_id":"synthetic_nas","name":"NAS 한글 &amp; + %","description":"설명 &amp; + %","status":"active","spec":{"disk":100},"domain":"nas.example.invalid","mount_info":"nas.example.invalid:/opaque/path","allowip":{"192.0.2.1":"RO","192.0.2.2":"RW"},"password":"excluded-secret","sharename":"unverified-history"}`
const nasProduct = `{"product_id":"synthetic_nas","product_name":"Synthetic NAS","status":"available","spec":{"version":"NAS v2","disk":{"min":100,"max":2000}},"price":{"excluded":"unverified"}}`

type nasFake struct{ hostingFake }

func (f *nasFake) PutJSON(_ context.Context, p string, b any) (client.Envelope, error) {
	f.calls++
	f.method = "PUT"
	f.path = p
	f.body = b
	return f.reply, f.err
}
func nasInput() NASInput {
	d := "설명 &amp; + %"
	return NASInput{ProductID: "synthetic_nas", Name: "NAS 한글 &amp; + %", ShareName: "exampleshare", Description: &d, DiskGB: 100, AllowIPs: map[string]string{"192.0.2.1": "RO", "192.0.2.2": "RW"}}
}
func TestNASCreateIdentityAndInput(t *testing.T) {
	receipt := strings.Replace(nasRow, `"mount_info":"nas.example.invalid:/opaque/path",`, "", 1)
	f := &nasFake{hostingFake{reply: envelope(receipt)}}
	s := NASService{API: f}
	got, err := s.Create(context.Background(), nasInput())
	if err != nil || got.ID != "9007199254740993" || got.Service == nil || got.Service.DiskGB != 100 {
		t.Fatal("NAS receipt changed", err)
	}
	body := f.body.(map[string]any)
	if f.calls != 1 || f.path != "/v1/apinas" || body["sharename"] != "exampleshare" || body["hdd"] != int64(100) || !reflect.DeepEqual(body["allowip"], nasInput().AllowIPs) {
		t.Fatal("NAS request encoding changed")
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "excluded-secret") || strings.Contains(string(raw), "unverified-history") {
		t.Fatal("NAS receipt copied unknown fields")
	}
	in := nasInput()
	in.Description = nil
	if _, err = s.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if _, exists := f.body.(map[string]any)["description"]; exists {
		t.Fatal("omitted description was sent")
	}
	for _, e := range []client.Envelope{envelope(`{"service_idx":9007199254740993}`), {Status: 201, Result: json.RawMessage(receipt)}, {Status: 202, Result: json.RawMessage(receipt)}, {Status: 200, Result: json.RawMessage(receipt), Count: json.RawMessage(`1`)}, envelope(strings.Replace(receipt, `"disk":100`, `"disk":1.5`, 1))} {
		f.reply = e
		f.calls = 0
		got, err = s.Create(context.Background(), nasInput())
		if err == nil || got.ID != "9007199254740993" || f.calls != 1 {
			t.Fatal("failed NAS create lost ID or repeated")
		}
	}
	f.err = &client.Error{Kind: "transport"}
	f.calls = 0
	if _, err = s.Create(context.Background(), nasInput()); err == nil || f.calls != 1 {
		t.Fatal("uncertain NAS create retried")
	}
}
func TestNASReadCompleteList(t *testing.T) {
	f := &nasFake{hostingFake{reply: envelope("[" + nasRow + "]")}}
	s := NASService{API: f}
	r, err := s.Read(context.Background(), "9007199254740993")
	if err != nil || r == nil || r.DiskGB != 100 || r.MountInfo != "nas.example.invalid:/opaque/path" || !reflect.DeepEqual(r.AllowIPs, nasInput().AllowIPs) {
		t.Fatal("NAS read contract changed", err)
	}
	if r, err = s.Read(context.Background(), "1"); err != nil || r != nil {
		t.Fatal("exact miss invalid")
	}
	for _, e := range []client.Envelope{envelope("[" + nasRow + "," + nasRow + "]"), envelope("[" + nasRow + ",{}]"), envelope(`null`), envelope(`{}`), {Status: 404, Result: json.RawMessage(`[]`)}, {Status: 200, Result: json.RawMessage(`[]`), Page: json.RawMessage(`1`)}, envelope("[" + strings.Replace(nasRow, `"mount_info":"nas.example.invalid:/opaque/path",`, "", 1) + "]"), envelope("[" + strings.Replace(nasRow, `"disk":100`, `"disk":null`, 1) + "]"), envelope("[" + strings.Replace(nasRow, `"RO"`, `"ADMIN"`, 1) + "]")} {
		f.reply = e
		if r, err = s.Read(context.Background(), "1"); err == nil || r != nil {
			t.Fatal("invalid NAS list became absence")
		}
	}
	f.reply = envelope("[" + strings.Replace(strings.Replace(nasRow, `"description":"설명 &amp; + %"`, `"description":null`, 1), `{"192.0.2.1":"RO","192.0.2.2":"RW"}`, `{}`, 1) + "]")
	r, err = s.Read(context.Background(), "9007199254740993")
	if err != nil || r.Description != nil || r.AllowIPs == nil || len(r.AllowIPs) != 0 {
		t.Fatal("observed empty map or null description changed")
	}
	f.err = errors.New("synthetic read error")
	if r, err = s.Read(context.Background(), "1"); err == nil || r != nil {
		t.Fatal("read error became absence")
	}
}
func TestNASAllowlistAcknowledgements(t *testing.T) {
	f := &nasFake{hostingFake{reply: envelope(`[{"ip":"192.0.2.2","acl":"RW"},{"ip":"192.0.2.1","acl":"RO"}]`)}}
	s := NASService{API: f}
	want := nasInput().AllowIPs
	if err := s.ReplaceAllowIPs(context.Background(), "9007199254740993", want); err != nil || f.path != "/v1/apinas/9007199254740993/allowip" || !reflect.DeepEqual(f.body, map[string]any{"allowip": want}) {
		t.Fatal("NAS permission map encoding changed", err)
	}
	for _, raw := range []string{`null`, `{}`, `[]`, `[{"ip":"192.0.2.1","acl":"RO"},{"ip":"192.0.2.1","acl":"RO"}]`, `[{"ip":"192.0.2.1","acl":"RW"},{"ip":"192.0.2.2","acl":"RW"}]`, `[{"ip":"192.0.2.1"},{"ip":"192.0.2.2","acl":"RW"}]`} {
		f.reply = envelope(raw)
		f.calls = 0
		if err := s.ReplaceAllowIPs(context.Background(), "1", want); err == nil || f.calls != 1 {
			t.Fatal("NAS bad acknowledgement accepted or repeated")
		}
	}
	f.reply = envelope(`"deleted"`)
	if err := s.Delete(context.Background(), "1"); err != nil || f.path != "/v1/apinas/1" {
		t.Fatal("NAS delete changed", err)
	}
	for _, e := range []client.Envelope{envelope(`null`), envelope(`{}`), envelope(`""`), {Status: 202, Result: json.RawMessage(`"queued"`)}, {Status: 200, Result: json.RawMessage(`"deleted"`), Count: json.RawMessage(`1`)}} {
		f.reply = e
		f.calls = 0
		if err := s.Delete(context.Background(), "1"); err == nil || f.calls != 1 {
			t.Fatal("NAS delete acknowledgement invalid")
		}
	}
	f.err = &client.Error{Kind: "http_status", Status: 404, Code: "NOT_FOUND"}
	f.calls = 0
	if s.Delete(context.Background(), "1") == nil || f.calls != 1 {
		t.Fatal("NAS 404 swallowed or repeated")
	}
}
func TestNASInvalidInputsDoNotWrite(t *testing.T) {
	for _, ips := range []map[string]string{nil, {}, {"192.0.2.1": ""}, {"192.0.2.1": "rw"}, {"192.0.2.1/32": "RO"}, {"2001:db8::1": "RW"}, {"192.000.2.1": "RO"}} {
		f := &nasFake{}
		s := NASService{API: f}
		in := nasInput()
		in.AllowIPs = ips
		if _, err := s.Create(context.Background(), in); err == nil {
			t.Fatal("invalid NAS create accepted")
		}
		if s.ReplaceAllowIPs(context.Background(), "1", ips) == nil || f.calls != 0 {
			t.Fatal("invalid NAS map reached API")
		}
	}
	for _, change := range []func(*NASInput){func(v *NASInput) { v.ProductID = "" }, func(v *NASInput) { v.Name = "abc" }, func(v *NASInput) { v.ShareName = "bad_name" }, func(v *NASInput) { v.ShareName = "abc" }, func(v *NASInput) { v.DiskGB = 99 }, func(v *NASInput) { v.DiskGB = 2001 }, func(v *NASInput) { v.Description = new(string) }} {
		f := &nasFake{}
		s := NASService{API: f}
		in := nasInput()
		change(&in)
		if _, err := s.Create(context.Background(), in); err == nil || f.calls != 0 {
			t.Fatal("invalid NAS input reached API")
		}
	}
	for _, id := range []string{"0", "01", "-1", "1/allowip", "9223372036854775808"} {
		f := &nasFake{}
		s := NASService{API: f}
		if s.Delete(context.Background(), id) == nil || s.ReplaceAllowIPs(context.Background(), id, nasInput().AllowIPs) == nil {
			t.Fatal("invalid NAS path accepted")
		}
		if _, err := s.Read(context.Background(), id); err == nil || f.calls != 0 {
			t.Fatal("invalid NAS ID reached API")
		}
	}
}
func TestNASProductCatalog(t *testing.T) {
	coming := `{"product_id":"","product_name":"Coming soon","status":"coming_soon","spec":{"version":null,"disk":{"min":0,"max":0}}}`
	f := &nasFake{hostingFake{reply: envelope("[" + nasProduct + "," + coming + "]")}}
	s := NASCatalogService{API: f}
	rows, err := s.Products(context.Background())
	if err != nil || len(rows) != 2 || rows[1].ID != "" || rows[1].Version != nil || rows[0].MinimumDiskGB != 100 {
		t.Fatal("NAS catalog contract changed", err)
	}
	for _, raw := range []string{"[" + nasProduct + "," + nasProduct + "]", "[" + coming + "," + coming + "]", "[" + strings.Replace(nasProduct, `"min":100`, `"min":2001`, 1) + "]", "[" + strings.Replace(nasProduct, `"product_id":"synthetic_nas"`, `"product_id":null`, 1) + "]", "[{}]", "null"} {
		f.reply = envelope(raw)
		if _, err = s.Products(context.Background()); err == nil {
			t.Fatal("NAS malformed catalog accepted")
		}
	}
	f.reply = envelope(`[]`)
	if rows, err = s.Products(context.Background()); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("NAS empty catalog invalid")
	}
}
