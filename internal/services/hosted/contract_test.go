package hosted

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

func envelope(body string) client.Envelope {
	return client.Envelope{Status: 200, Result: json.RawMessage(body)}
}
func TestCreateIdentityBeforeRemainingFields(t *testing.T) {
	for _, body := range []string{
		`{"service_idx":9007199254740993,"status":"pending"}`,
		`{"service_idx":9007199254740993,"product_id":null,"description":{},"unknown_secret":"synthetic-secret"}`,
	} {
		id, err := CreateID(envelope(body))
		if err != nil || id != "9007199254740993" {
			t.Fatalf("identity lost or rounded: %v", err)
		}
	}
	for _, body := range []string{`{}`, `null`, `[]`, `[{"service_idx":12}]`, `{"service_idx":0}`, `{"service_idx":-1}`, `{"service_idx":"12"}`, `{"service_idx":1.5}`, `{"service_idx":1e2}`, `{"service_idx":9223372036854775808}`, `{"service_idx":12} trailing`} {
		if _, err := CreateID(envelope(body)); err == nil {
			t.Fatalf("invalid receipt accepted: %s", body)
		}
	}
	for _, status := range []int{201, 202} {
		e := envelope(`{"service_idx":12}`)
		e.Status = status
		if id, err := CreateID(e); id != "12" || err == nil {
			t.Fatal("unverified success lost recovery identity")
		}
	}
	e := envelope(`{"service_idx":12}`)
	e.Status = 500
	if _, err := CreateID(e); err == nil {
		t.Fatal("error response treated as a created resource")
	}
}
func TestListIdentityAndSelection(t *testing.T) {
	e := envelope(`[
 {"service_idx":42,"product_id":"synthetic_product","name":"한글 테스트","description":"","status":"pending","password":"synthetic-secret"},
 {"service_idx":9007199254740993,"product_id":"synthetic_product","name":"Other","description":null,"status":"active"}
 ]`)
	rows, err := Records(e)
	if err != nil || len(rows) != 2 || rows[0].Description == nil || *rows[0].Description != "" || rows[1].Description != nil {
		t.Fatalf("nullable fields changed: %v", err)
	}
	r, err := Find(e, "9007199254740993")
	if err != nil || r == nil || r.ID != "9007199254740993" || r.Name == nil || *r.Name != "Other" {
		t.Fatal("exact lookup failed")
	}
	r, err = Find(e, "99")
	if err != nil || r != nil {
		t.Fatal("missing exact ID selected another object")
	}
	encoded, _ := json.Marshal(rows)
	if strings.Contains(string(encoded), "synthetic-secret") {
		t.Fatal("unknown credential copied into decoded record")
	}
	for _, id := range []string{"", "0", "-1", "042", "+42", "42/other", "9223372036854775808"} {
		if _, err := Find(e, id); err == nil {
			t.Fatal("noncanonical ID accepted")
		}
	}
	if rows, err := Records(envelope(`[]`)); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("empty list lost")
	}
}
func TestListErrorsCannotBecomeAbsence(t *testing.T) {
	for _, body := range []string{
		`null`, `{}`, `[null]`, `[{"service_idx":1}]`,
		`[{"service_idx":1,"product_id":"p","name":"n","status":"active"},{"service_idx":1,"product_id":"p","name":"n","status":"active"}]`,
		`[{"service_idx":1,"product_id":"p","name":"n","status":"active","description":42}]`,
	} {
		if row, err := Find(envelope(body), "99"); err == nil || row != nil {
			t.Fatalf("invalid response became absence: %s", body)
		}
	}
	for _, e := range []client.Envelope{
		{Status: 404, Result: json.RawMessage(`[]`)},
		{Status: 202, Result: json.RawMessage(`[]`)},
		{Status: 200, Result: json.RawMessage(`[]`), Count: json.RawMessage(`1`)},
		{Status: 200, Result: json.RawMessage(`[]`), Count: json.RawMessage(`null`)},
		{Status: 200, Result: json.RawMessage(`[]`), PageNo: json.RawMessage(`1`)},
		{Status: 200, Result: json.RawMessage(`[]`), Total: json.RawMessage(`0`)},
	} {
		if row, err := Find(e, "99"); err == nil || row != nil {
			t.Fatal("changed response contract became absence")
		}
	}
}

func TestWebmailReadMayOmitCreateName(t *testing.T) {
	e := envelope(`[{"service_idx":42,"product_id":"synthetic_mail","description":"test","status":"active"}]`)
	rows, err := Records(e)
	if err != nil || len(rows) != 1 || rows[0].Name != nil {
		t.Fatal("webmail absent name was synthesized or rejected")
	}
}
