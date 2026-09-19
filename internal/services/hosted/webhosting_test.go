package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

const hostingRow = `{"service_idx":9007199254740993,"product_id":"synthetic_hosting","name":"한글 &amp; + %","description":"설명 &amp; + %","id":"exampleuser","status":"active","ip":"192.0.2.10","domain":{"example.invalid":"/"},"security":"Y","ftppw":"should-never-copy"}`
const productRow = `{"product_id":"synthetic_hosting","product_name":"Synthetic","status":"available","spec":{"type":"SHARE","allow_php_version":["PHP 8.4"],"domain":{"allow_custom_domain":true,"max_domain_count":2,"enable_domain_folder":false,"edit_interval_days":1}},"price":{"unverified":"excluded"}}`
const serverRow = `{"idx":9007199254740993,"charset":"UTF-8","php_version":"PHP 8.4","db":"MariaDB","program":""}`

type hostingFake struct {
	reply        client.Envelope
	err          error
	calls        int
	method, path string
	query        url.Values
	body         any
}

func (f *hostingFake) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	f.calls++
	f.method = "GET"
	f.path = p
	f.query = q
	return f.reply, f.err
}
func (f *hostingFake) PostJSON(_ context.Context, p string, b any) (client.Envelope, error) {
	f.calls++
	f.method = "POST"
	f.path = p
	f.body = b
	return f.reply, f.err
}
func (f *hostingFake) Delete(_ context.Context, p string) (client.Envelope, error) {
	f.calls++
	f.method = "DELETE"
	f.path = p
	return f.reply, f.err
}
func hostingInput() WebhostingInput {
	desc := "설명 &amp; + %"
	return WebhostingInput{ProductID: "synthetic_hosting", ServerID: "9007199254740993", Name: "한글 &amp; + %", Description: &desc, Account: "exampleuser", FTPPassword: "Synthetic1!", DatabasePassword: "Synthetic2!", WebFirewall: true}
}

func TestWebhostingCreateEncodingAndRecovery(t *testing.T) {
	f := &hostingFake{reply: envelope(hostingRow)}
	s := WebhostingService{API: f}
	created, err := s.Create(context.Background(), hostingInput())
	if err != nil || created.ID != "9007199254740993" || created.Service == nil || created.Service.ID != created.ID {
		t.Fatal("create identity was lost or rounded", err)
	}
	body := f.body.(map[string]any)
	if f.calls != 1 || f.method != "POST" || f.path != "/v1/webhosting" || body["server_idx"] != "9007199254740993" || body["security"] != "Y" || body["ftppw"] != "Synthetic1!" || body["dbpw"] != "Synthetic2!" {
		t.Fatal("create request encoding changed")
	}
	if _, ok := body["domain"]; ok {
		t.Fatal("omitted domain was sent")
	}
	serialized, _ := json.Marshal(created)
	if strings.Contains(string(serialized), "should-never-copy") {
		t.Fatal("password entered readable model")
	}
	in := hostingInput()
	in.Description = nil
	in.WebFirewall = false
	in.Domains = map[string]string{"new.example.invalid": "/"}
	if _, err = s.Create(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	body = f.body.(map[string]any)
	if body["security"] != "N" || !reflect.DeepEqual(body["domain"], in.Domains) {
		t.Fatal("custom domains or explicit false changed")
	}
	if _, ok := body["description"]; ok {
		t.Fatal("description omission changed")
	}
	for _, e := range []client.Envelope{
		envelope(`{"service_idx":9007199254740993}`),
		{Status: 201, Result: json.RawMessage(hostingRow)},
		{Status: 202, Result: json.RawMessage(hostingRow)},
		{Status: 200, Result: json.RawMessage(hostingRow), Count: json.RawMessage(`1`)},
	} {
		f.reply = e
		f.calls = 0
		created, err = s.Create(context.Background(), hostingInput())
		if err == nil || created.ID != "9007199254740993" || f.calls != 1 {
			t.Fatal("known create identity lost, unverified receipt accepted, or create retried")
		}
	}
	f.reply = envelope(`[{"service_idx":1},{"service_idx":2}]`)
	f.calls = 0
	if created, err = s.Create(context.Background(), hostingInput()); err == nil || created.ID != "" || f.calls != 1 {
		t.Fatal("ambiguous create was adopted")
	}
	f.err = errors.New("synthetic transport failure")
	f.calls = 0
	if created, err = s.Create(context.Background(), hostingInput()); err == nil || created.ID != "" || f.calls != 1 {
		t.Fatal("uncertain create retried")
	}
}

func TestWebhostingCreateValidationDoesNotExposePasswords(t *testing.T) {
	mutations := []func(*WebhostingInput){
		func(x *WebhostingInput) { x.ProductID = "" }, func(x *WebhostingInput) { x.ServerID = "01" },
		func(x *WebhostingInput) { x.ServerID = "1/other" }, func(x *WebhostingInput) { x.Name = "短" },
		func(x *WebhostingInput) { x.Name = strings.Repeat("한", 33) }, func(x *WebhostingInput) { v := strings.Repeat("한", 51); x.Description = &v },
		func(x *WebhostingInput) { v := ""; x.Description = &v },
		func(x *WebhostingInput) { x.Account = "root" }, func(x *WebhostingInput) { x.Account = "account123" },
		func(x *WebhostingInput) { x.FTPPassword = "sensitive-password-value-too-long" }, func(x *WebhostingInput) { x.DatabasePassword = x.FTPPassword },
		func(x *WebhostingInput) { x.FTPPassword = "short1" }, func(x *WebhostingInput) { x.FTPPassword = "onlyletters" },
		func(x *WebhostingInput) { x.FTPPassword = "Has space1!" }, func(x *WebhostingInput) { x.Domains = map[string]string{} },
		func(x *WebhostingInput) { x.Domains = map[string]string{"": "/"} }, func(x *WebhostingInput) { x.Domains = map[string]string{"example.invalid": ""} },
	}
	for i, mutate := range mutations {
		in := hostingInput()
		mutate(&in)
		f := &hostingFake{}
		s := WebhostingService{API: f}
		out, err := s.Create(context.Background(), in)
		if err == nil || out.ID != "" || f.calls != 0 {
			t.Fatalf("case %d wrote invalid input", i)
		}
		if strings.Contains(err.Error(), in.FTPPassword) || strings.Contains(err.Error(), in.DatabasePassword) {
			t.Fatalf("case %d leaked a password", i)
		}
	}
}

func TestWebhostingListAndExactRead(t *testing.T) {
	second := strings.Replace(hostingRow, "9007199254740993", "9223372036854775807", 1)
	f := &hostingFake{reply: envelope("[" + hostingRow + "," + second + "]")}
	s := WebhostingService{API: f}
	row, err := s.Read(context.Background(), "9223372036854775807")
	if err != nil || row == nil || row.ID != "9223372036854775807" || row.Name != "한글 &amp; + %" || *row.Description != "설명 &amp; + %" || !row.WebFirewall {
		t.Fatal("exact ID or literal text changed", err)
	}
	if f.path != "/v1/webhosting" || len(f.query) != 0 {
		t.Fatal("invented detail endpoint or pagination")
	}
	if row, err = s.Read(context.Background(), "42"); err != nil || row != nil {
		t.Fatal("missing ID selected another service")
	}
	for _, description := range []string{`null`, `""`} {
		f.reply = envelope("[" + strings.Replace(hostingRow, `"설명 &amp; + %"`, description, 1) + "]")
		row, err = s.Read(context.Background(), "9007199254740993")
		if err != nil || (description == `null` && row.Description != nil) || (description == `""` && (row.Description == nil || *row.Description != "")) {
			t.Fatal("null and empty description collapsed")
		}
	}
	f.reply = envelope(`[]`)
	rows, err := s.List(context.Background())
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("empty complete list rejected")
	}
	for _, id := range []string{"", "0", "01", "+1", "1/2", "1?x=2", "9223372036854775808"} {
		f.calls = 0
		if _, err = s.Read(context.Background(), id); err == nil || f.calls != 0 {
			t.Fatal("invalid identity made a request")
		}
	}
}

func TestWebhostingMalformedListsCannotMeanAbsence(t *testing.T) {
	invalid := []string{`null`, `{}`, `[null]`, `[` + hostingRow + `,` + hostingRow + `]`,
		`[` + hostingRow + `,{}]`,
		`[` + strings.Replace(hostingRow, `"service_idx":9007199254740993`, `"service_idx":"9007199254740993"`, 1) + `]`,
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal([]byte(hostingRow), &fields)
	for _, key := range []string{"service_idx", "product_id", "name", "id", "status", "description", "security", "ip", "domain"} {
		original := fields[key]
		delete(fields, key)
		raw, _ := json.Marshal(fields)
		invalid = append(invalid, "["+string(raw)+"]")
		fields[key] = original
	}
	for _, body := range invalid {
		f := &hostingFake{reply: envelope(body)}
		s := WebhostingService{API: f}
		if row, err := s.Read(context.Background(), "42"); err == nil || row != nil {
			t.Fatal("malformed or partial list became absence")
		}
	}
	for _, e := range []client.Envelope{
		{Status: 404, Result: json.RawMessage(`[]`)}, {Status: 202, Result: json.RawMessage(`[]`)},
		{Status: 200, Result: json.RawMessage(`[]`), Count: json.RawMessage(`1`)},
		{Status: 200, Result: json.RawMessage(`[]`), Count: json.RawMessage(`null`)},
		{Status: 200, Result: json.RawMessage(`[]`), Page: json.RawMessage(`1`)},
		{Status: 200, Result: json.RawMessage(`[]`), PageNo: json.RawMessage(`1`)},
		{Status: 200, Result: json.RawMessage(`[]`), PageSize: json.RawMessage(`1`)},
		{Status: 200, Result: json.RawMessage(`[]`), Total: json.RawMessage(`0`)},
	} {
		s := WebhostingService{API: &hostingFake{reply: e}}
		if row, err := s.Read(context.Background(), "42"); err == nil || row != nil {
			t.Fatal("unverified list became absence")
		}
	}
	f := &hostingFake{err: errors.New("synthetic API failure")}
	s := WebhostingService{API: f}
	if row, err := s.Read(context.Background(), "42"); err == nil || row != nil || f.calls != 1 {
		t.Fatal("API failure became absence or retry")
	}
}

func TestWebhostingDeleteAcknowledgement(t *testing.T) {
	f := &hostingFake{reply: envelope(`"acknowledged"`)}
	s := WebhostingService{API: f}
	if err := s.Delete(context.Background(), "9007199254740993"); err != nil || f.path != "/v1/webhosting/9007199254740993" || f.calls != 1 {
		t.Fatal("delete contract changed")
	}
	for _, e := range []client.Envelope{envelope(`null`), envelope(`{}`), envelope(`""`), {Status: 202, Result: json.RawMessage(`"ack"`)}, {Status: 404, Result: json.RawMessage(`"ack"`)}, {Status: 200, Result: json.RawMessage(`"ack"`), Count: json.RawMessage(`1`)}} {
		f.reply = e
		f.calls = 0
		if err := s.Delete(context.Background(), "42"); err == nil || f.calls != 1 {
			t.Fatal("unexpected delete response accepted or retried")
		}
	}
	f.calls = 0
	if err := s.Delete(context.Background(), "42/other"); err == nil || f.calls != 0 {
		t.Fatal("invalid deletion ID used")
	}
}

func TestWebhostingCatalogs(t *testing.T) {
	f := &hostingFake{reply: envelope("[" + productRow + "]")}
	s := WebhostingService{API: f}
	products, err := s.Products(context.Background(), "SHARE")
	if err != nil || len(products) != 1 || products[0].ID != "synthetic_hosting" || !products[0].AllowCustomDomain || products[0].EnableDomainFolder || products[0].MaxDomainCount != 2 || f.query.Get("type") != "SHARE" || f.path != "/v1/webhosting/products" {
		t.Fatal("product choice metadata changed", err)
	}
	f.calls = 0
	if _, err = s.Products(context.Background(), "share"); err == nil || f.calls != 0 {
		t.Fatal("invalid product type made a request")
	}
	if _, err = s.Products(context.Background(), "SINGLE"); err == nil {
		t.Fatal("mismatched product filter accepted")
	}
	f.reply = envelope("[" + serverRow + "]")
	servers, err := s.Servers(context.Background(), "synthetic&product")
	if err != nil || len(servers) != 1 || servers[0].ID != "9007199254740993" || servers[0].Program != "" || f.query.Get("product_id") != "synthetic&product" || f.path != "/v1/webhosting/servers" {
		t.Fatal("server selection or query changed", err)
	}
	f.calls = 0
	if _, err = s.Servers(context.Background(), ""); err == nil || f.calls != 0 {
		t.Fatal("missing product made a request")
	}
	for _, body := range []string{`null`, `[null]`, `[` + serverRow + `,` + serverRow + `]`, `[{"idx":42}]`, `[` + strings.Replace(serverRow, `"idx":9007199254740993`, `"idx":1.5`, 1) + `]`} {
		f.reply = envelope(body)
		if _, err = s.Servers(context.Background(), "p"); err == nil {
			t.Fatal("invalid server catalog accepted")
		}
	}
	for _, body := range []string{`null`, `[null]`, `[` + productRow + `,` + productRow + `]`, `[` + strings.Replace(productRow, `"allow_custom_domain":true,`, "", 1) + `]`, `[` + strings.Replace(productRow, `"max_domain_count":2`, `"max_domain_count":-1`, 1) + `]`, `[` + strings.Replace(productRow, `["PHP 8.4"]`, `["PHP 8.4","PHP 8.4"]`, 1) + `]`} {
		f.reply = envelope(body)
		if _, err = s.Products(context.Background(), ""); err == nil {
			t.Fatal("invalid product catalog accepted")
		}
	}
}
