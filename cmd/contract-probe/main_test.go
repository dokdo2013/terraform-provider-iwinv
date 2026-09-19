package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

func TestReportDoesNotDiscloseRemoteValuesOrFieldNames(t *testing.T) {
	var out bytes.Buffer
	err := report(&out, client.Envelope{Status: 200, Result: json.RawMessage(`[{"zone_id":"private-id","name":"private-name","private-dynamic-field":"private-secret"}]`), Count: json.RawMessage(`1`), Page: json.RawMessage(`null`)})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "private-") {
		t.Fatal("private values or dynamic keys disclosed")
	}
	if !strings.Contains(out.String(), `"zone_id":"string"`) || !strings.Contains(out.String(), `"page_type":"null"`) {
		t.Fatal("wrong shape")
	}
}

func TestReportRejectsInvalidResults(t *testing.T) {
	for _, body := range []string{"", "null", "{}", `[null]`, `["text"]`, "invalid"} {
		var out bytes.Buffer
		if err := report(&out, client.Envelope{Status: 200, Result: json.RawMessage(body)}); err == nil {
			t.Errorf("accepted %q", body)
		}
		if out.Len() != 0 {
			t.Fatal("partial output")
		}
	}
	if err := report(&bytes.Buffer{}, client.Envelope{Status: 202, Result: json.RawMessage(`[]`)}); err == nil {
		t.Fatal("accepted unexplained 202")
	}
}

func TestTypesDistinguishMissingNullAndEmpty(t *testing.T) {
	for _, tt := range []struct{ raw, expected string }{{"", "missing"}, {"null", "null"}, {`""`, "string"}, {"[]", "array"}, {"{}", "object"}, {"0", "number"}, {"true", "boolean"}, {"{", "invalid"}} {
		if got := valueType(json.RawMessage(tt.raw)); got != tt.expected {
			t.Errorf("%q: %s != %s", tt.raw, got, tt.expected)
		}
	}
}

func TestMissingKeysFailBeforeRequest(t *testing.T) {
	if err := run(context.Background(), func(string) string { return "" }, &bytes.Buffer{}); err == nil {
		t.Fatal("missing keys accepted")
	}
}
