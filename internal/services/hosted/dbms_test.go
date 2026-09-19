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

const dbmsRow = `{"service_idx":9007199254740993,"product_id":"synthetic_redis","name":"DB 한글 &amp; + %","description":"설명 &amp; + %","status":"active","spec":{"type":"STD","ver":"7","vcpu":1000,"memory":2,"disk":100},"domain":{"default":"db.example.invalid"},"allowip":["192.0.2.2","192.0.2.1"],"password":"excluded-secret","id":"unverified"}`
const dbmsReceipt = `{"service_idx":9007199254740993,"name":"DB 한글 &amp; + %","description":"설명 &amp; + %","status":"WAIT","domain":"db.example.invalid","allowip":["192.0.2.1"],"password":"excluded-secret"}`
const dbmsProduct = `{"product_id":"synthetic_redis","product_name":"Synthetic Redis","status":"available","spec":{"type":"STD","ver":"7","vcpu":1000},"price":{"unverified":"excluded"}}`

type dbmsFake struct{ hostingFake }

func (f *dbmsFake) PutJSON(_ context.Context, p string, b any) (client.Envelope, error) {
	f.calls++
	f.method = "PUT"
	f.path = p
	f.body = b
	return f.reply, f.err
}
func dbmsInput() DBMSInput {
	d := "설명 &amp; + %"
	return DBMSInput{ProductID: "synthetic_redis", Name: "DB 한글 &amp; + %", Description: &d, Account: "exampleuser", AllowIPs: []string{"192.0.2.1"}}
}
func TestDBMSCreateReceiptAndRequest(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope(dbmsReceipt)}}
	s := DBMSService{API: f}
	got, err := s.Create(context.Background(), dbmsInput())
	if err != nil || got.ID != "9007199254740993" || got.Receipt == nil || got.Receipt.Status != "WAIT" {
		t.Fatal("create receipt or exact identity changed", err)
	}
	b := f.body.(map[string]any)
	if f.method != "POST" || f.path != "/v1/dbms" || f.calls != 1 || b["id"] != "exampleuser" || !reflect.DeepEqual(b["allowip"], []string{"192.0.2.1"}) {
		t.Fatal("DBMS create encoding changed")
	}
	if _, ok := b["password"]; ok {
		t.Fatal("invented password input")
	}
	in := dbmsInput()
	in.Description = nil
	if _, err = s.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.body.(map[string]any)["description"]; ok {
		t.Fatal("omitted description was sent")
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "excluded-secret") {
		t.Fatal("credential copied to receipt model")
	}
	for _, e := range []client.Envelope{envelope(`{"service_idx":9007199254740993}`), {Status: 201, Result: json.RawMessage(dbmsReceipt)}, {Status: 202, Result: json.RawMessage(dbmsReceipt)}, {Status: 200, Result: json.RawMessage(dbmsReceipt), Count: json.RawMessage(`1`)}, envelope(strings.Replace(dbmsReceipt, `"db.example.invalid"`, `{"default":"db.example.invalid"}`, 1))} {
		f.reply = e
		f.calls = 0
		got, err = s.Create(context.Background(), dbmsInput())
		if err == nil || got.ID != "9007199254740993" || f.calls != 1 {
			t.Fatal("failed create lost ID or replayed POST")
		}
	}
	f.reply = envelope(strings.Replace(dbmsReceipt, `9007199254740993`, `"9007199254740993"`, 1))
	got, err = s.Create(context.Background(), dbmsInput())
	if err == nil || got.ID != "" {
		t.Fatal("unverified string identity accepted")
	}
}
func TestDBMSReadCompleteValidation(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope("[" + dbmsRow + "]")}}
	s := DBMSService{API: f}
	r, err := s.Read(context.Background(), "9007199254740993")
	if err != nil || r == nil || r.Tier != "STD" || r.Version != "7" || r.Domains["default"] != "db.example.invalid" || r.Name != "DB 한글 &amp; + %" || !reflect.DeepEqual(r.AllowIPs, []string{"192.0.2.1", "192.0.2.2"}) {
		t.Fatal("read contract changed", err)
	}
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "excluded-secret") || strings.Contains(string(raw), "unverified") {
		t.Fatal("unreadable creation history or credential copied")
	}
	if r, err = s.Read(context.Background(), "12"); err != nil || r != nil {
		t.Fatal("exact-ID miss invalid")
	}
	for _, e := range []client.Envelope{envelope("[" + dbmsRow + "," + dbmsRow + "]"), envelope("[" + dbmsRow + ",{}]"), envelope(`null`), envelope(`{}`), {Status: 404, Result: json.RawMessage(`[]`)}, {Status: 200, Result: json.RawMessage(`[]`), PageNo: json.RawMessage(`1`)}, {Status: 200, Result: json.RawMessage("[" + dbmsRow + "]"), Count: json.RawMessage(`0`)}, envelope("[" + strings.Replace(dbmsRow, `{"default":"db.example.invalid"}`, `"db.example.invalid"`, 1) + "]"), envelope("[" + strings.Replace(dbmsRow, `["192.0.2.2","192.0.2.1"]`, `["192.0.2.1","192.0.2.1"]`, 1) + "]")} {
		f.reply = e
		if r, err = s.Read(context.Background(), "12"); err == nil || r != nil {
			t.Fatal("partial/malformed/error response became absence")
		}
	}
	f.reply = envelope("[" + strings.Replace(strings.Replace(dbmsRow, `"description":"설명 &amp; + %"`, `"description":null`, 1), `["192.0.2.2","192.0.2.1"]`, `[]`, 1) + "]")
	r, err = s.Read(context.Background(), "9007199254740993")
	if err != nil || r.Description != nil || r.AllowIPs == nil || len(r.AllowIPs) != 0 {
		t.Fatal("nullable description/observed empty set not preserved", err)
	}
	f.reply = envelope(`[]`)
	r, err = s.Read(context.Background(), "12")
	if err != nil || r != nil {
		t.Fatal("validated empty list failed")
	}
}
func TestDBMSAuthoritativeAllowIPs(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope(`["192.0.2.2","192.0.2.1"]`)}}
	s := DBMSService{API: f}
	ips := []string{"192.0.2.1", "192.0.2.2"}
	if err := s.ReplaceAllowIPs(context.Background(), "9007199254740993", ips); err != nil {
		t.Fatal(err)
	}
	if f.method != "PUT" || f.path != "/v1/dbms/9007199254740993/allowip" || !reflect.DeepEqual(f.body, map[string]any{"allowip": ips}) {
		t.Fatal("allowlist replacement encoding changed")
	}
	for _, e := range []client.Envelope{envelope(`[]`), envelope(`["192.0.2.1"]`), envelope(`["192.0.2.1","192.0.2.3"]`), envelope(`["192.0.2.1","192.0.2.1"]`), envelope(`"ok"`), {Status: 202, Result: json.RawMessage(`["192.0.2.1","192.0.2.2"]`)}, {Status: 200, Result: json.RawMessage(`["192.0.2.1","192.0.2.2"]`), Count: json.RawMessage(`2`)}} {
		f.reply = e
		f.calls = 0
		if err := s.ReplaceAllowIPs(context.Background(), "12", ips); err == nil || f.calls != 1 {
			t.Fatal("unexpected allowlist acknowledgement or replay")
		}
	}
	f.err = errors.New("synthetic write failure")
	f.calls = 0
	if err := s.ReplaceAllowIPs(context.Background(), "12", ips); err == nil || f.calls != 1 {
		t.Fatal("write error hidden or retried")
	}
}
func TestDBMSValidationBeforeRequests(t *testing.T) {
	f := &dbmsFake{}
	s := DBMSService{API: f}
	ctx := context.Background()
	for _, ips := range [][]string{nil, {}, {""}, {"192.0.2.1", "192.0.2.1"}, {"192.0.2.1/32"}, {"::1"}, {"::ffff:192.0.2.1"}, {"192.0.02.1"}, {"secret-not-ip"}} {
		in := dbmsInput()
		in.AllowIPs = ips
		if _, err := s.Create(ctx, in); err == nil {
			t.Fatal("invalid creation allowlist accepted")
		}
		if err := s.ReplaceAllowIPs(ctx, "12", ips); err == nil || strings.Contains(err.Error(), "secret-not-ip") {
			t.Fatal("invalid update accepted or leaked value")
		}
	}
	for _, id := range []string{"", "0", "-1", "01", "1/allowip", "9223372036854775808"} {
		if _, err := s.Read(ctx, id); err == nil {
			t.Fatal("invalid read identity accepted")
		}
		if err := s.Delete(ctx, id); err == nil {
			t.Fatal("invalid deletion identity accepted")
		}
		if err := s.ReplaceAllowIPs(ctx, id, []string{"192.0.2.1"}); err == nil {
			t.Fatal("invalid update identity accepted")
		}
	}
	for _, mutate := range []func(*DBMSInput){func(in *DBMSInput) { in.ProductID = "" }, func(in *DBMSInput) { in.Account = "bad123" }, func(in *DBMSInput) { in.Name = "bad" }, func(in *DBMSInput) { v := ""; in.Description = &v }, func(in *DBMSInput) { v := strings.Repeat("한", 51); in.Description = &v }} {
		in := dbmsInput()
		mutate(&in)
		if _, err := s.Create(ctx, in); err == nil {
			t.Fatal("invalid creation input accepted")
		}
	}
	if f.calls != 0 {
		t.Fatal("invalid input reached the API")
	}
}
func TestDBMSCatalogPreservesProductVersions(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope("[" + dbmsProduct + "," + strings.Replace(dbmsProduct, `"ver":"7"`, `"ver":"8"`, 1) + "]")}}
	s := DBMSCatalogService{API: f}
	rows, err := s.Products(context.Background(), "STD", "redis")
	if err != nil || len(rows) != 2 || rows[0].ID != rows[1].ID || rows[0].Version == rows[1].Version {
		t.Fatal("product versions were collapsed", err)
	}
	if f.query.Get("type") != "STD" || f.query.Get("db") != "redis" || len(f.query) != 2 || f.path != "/v1/dbms/products" {
		t.Fatal("catalog filters changed")
	}
	f.reply = envelope("[" + dbmsProduct + "," + dbmsProduct + "]")
	if _, err = s.Products(context.Background(), "", ""); err == nil {
		t.Fatal("exact duplicate accepted")
	}
	f.reply = envelope("[" + dbmsProduct + "]")
	if _, err = s.Products(context.Background(), "HM", ""); err == nil {
		t.Fatal("mismatched tier filter accepted")
	}
	f.calls = 0
	for _, q := range [][2]string{{"bad", "redis"}, {"STD", "Redis"}, {"", "secret"}} {
		if _, err = s.Products(context.Background(), q[0], q[1]); err == nil {
			t.Fatal("invalid catalog query accepted")
		}
	}
	if f.calls != 0 {
		t.Fatal("invalid catalog query reached API")
	}
}
func TestDBMSDeleteAcknowledgement(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope(`"accepted"`)}}
	s := DBMSService{API: f}
	if err := s.Delete(context.Background(), "12"); err != nil || f.path != "/v1/dbms/12" || f.method != "DELETE" {
		t.Fatal("delete contract changed", err)
	}
	for _, e := range []client.Envelope{envelope(`""`), envelope(`null`), envelope(`[]`), {Status: 202, Result: json.RawMessage(`"accepted"`)}, {Status: 200, Result: json.RawMessage(`"accepted"`), Count: json.RawMessage(`1`)}} {
		f.reply = e
		f.calls = 0
		if err := s.Delete(context.Background(), "12"); err == nil || f.calls != 1 {
			t.Fatal("unverified delete accepted or replayed")
		}
	}
}

func TestDBMSCatalogEmptyCreationIdentifiers(t *testing.T) {
	f := &dbmsFake{hostingFake{reply: envelope("[" + strings.Replace(dbmsProduct, `"synthetic_redis"`, `""`, 1) + "]")}}
	s := DBMSCatalogService{API: f}
	rows, err := s.Products(context.Background(), "", "")
	if err != nil || len(rows) != 1 || rows[0].ID != "" || rows[0].Status != "available" {
		t.Fatal("unselectable catalog row was dropped or fabricated", err)
	}
	for _, raw := range []string{strings.Replace(dbmsProduct, `"synthetic_redis"`, `null`, 1), strings.Replace(dbmsProduct, `"product_id":"synthetic_redis",`, ``, 1)} {
		f.reply = envelope("[" + raw + "]")
		if _, err = s.Products(context.Background(), "", ""); err == nil {
			t.Fatal("null/missing product ID accepted as observed empty string")
		}
	}
}
