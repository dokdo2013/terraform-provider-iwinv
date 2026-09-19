package network

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

const oneRule = `[{"rule_id":9007199254740993,"bound":"IN","protocol":"TCP","port":"8443","ip":"192.0.2.1/32","title":"한글 &amp; <name>","content":"literal &amp; &#39; <x>"}]`

func validRuleInput() RuleInput {
	description := "literal &amp; <x>"
	return RuleInput{Bound: "IN", Protocol: "TCP", Port: "8443", IP: "192.0.2.1/32", Name: "tf-rule", Description: &description}
}
func TestRuleReadsPreserveIdentityAndText(t *testing.T) {
	f := &fakeAPI{response: groupEnvelope(oneRule, "1")}
	s := Service{API: f}
	rows, err := s.Rules(context.Background(), "FIREWALL-test")
	if err != nil || len(rows) != 1 {
		t.Fatalf("read failed: %v", err)
	}
	r := rows[0]
	if r.ID != "9007199254740993" || r.Name != "한글 &amp; <name>" || r.Description == nil || *r.Description != "literal &amp; &#39; <x>" {
		t.Fatal("ID precision or verbatim text was lost")
	}
	if f.path != "/v1/security-groups/FIREWALL-test/rules" || len(f.body.(url.Values)) != 0 {
		t.Fatal("unexpected path or pagination request")
	}
	for _, value := range []string{`null`, `""`} {
		f.response = groupEnvelope(strings.Replace(oneRule, `"literal &amp; &#39; <x>"`, value, 1), "1")
		rows, err = s.Rules(context.Background(), "FIREWALL-test")
		if err != nil {
			t.Fatal(err)
		}
		if value == `null` && rows[0].Description != nil {
			t.Fatal("null was erased")
		}
		if value == `""` && (rows[0].Description == nil || *rows[0].Description != "") {
			t.Fatal("empty was erased")
		}
	}
}
func TestRuleReadContractFailures(t *testing.T) {
	fixtures := []client.Envelope{
		groupEnvelope(`null`, `0`), groupEnvelope(`{}`, `0`), groupEnvelope(oneRule, `0`), groupEnvelope(oneRule, `null`), groupEnvelope(oneRule, ``),
		groupEnvelope(strings.Replace(oneRule, `9007199254740993`, `"9007199254740993"`, 1), `1`),
		groupEnvelope(strings.Replace(oneRule, `9007199254740993`, `9223372036854775808`, 1), `1`),
		groupEnvelope(strings.Replace(oneRule, `9007199254740993`, `0`, 1), `1`),
		groupEnvelope(strings.Replace(oneRule, `9007199254740993`, `1.5`, 1), `1`),
		groupEnvelope(strings.Replace(oneRule, `"IN"`, `"inbound"`, 1), `1`),
		groupEnvelope(strings.Replace(oneRule, `"TCP"`, `"ICMP"`, 1), `1`),
		groupEnvelope(strings.Replace(oneRule, `"8443"`, `"65536"`, 1), `1`),
		groupEnvelope(strings.Replace(oneRule, `"192.0.2.1/32"`, `"2001:db8::/64"`, 1), `1`),
		groupEnvelope(strings.Replace(oneRule, `,"content":"literal &amp; &#39; <x>"`, ``, 1), `1`),
		groupEnvelope(oneRule[:len(oneRule)-1]+","+oneRule[1:], `2`),
	}
	for _, field := range []string{"Page", "PageNo", "PageSize", "Total"} {
		e := groupEnvelope(oneRule, `1`)
		switch field {
		case "Page":
			e.Page = json.RawMessage(`1`)
		case "PageNo":
			e.PageNo = json.RawMessage(`1`)
		case "PageSize":
			e.PageSize = json.RawMessage(`50`)
		case "Total":
			e.Total = json.RawMessage(`1`)
		}
		fixtures = append(fixtures, e)
	}
	e := groupEnvelope(oneRule, `1`)
	e.Status = 202
	fixtures = append(fixtures, e)
	for i, e := range fixtures {
		f := &fakeAPI{response: e}
		s := Service{API: f}
		rows, err := s.Rules(context.Background(), "FIREWALL-test")
		if err == nil || rows != nil {
			t.Fatalf("malformed fixture %d accepted", i)
		}
	}
}
func TestRuleAbsenceRequiresVerifiedParentOrCompleteList(t *testing.T) {
	scenarios := []struct {
		name               string
		parent, listing    client.Envelope
		parentErr, listErr error
		absent             bool
		calls              int
	}{
		{name: "parent-absent", parent: groupEnvelope(`[]`, `0`), absent: true, calls: 1},
		{name: "parent-error", parentErr: &client.Error{Status: 404, Kind: "response", Code: "NOT_FOUND"}, calls: 1},
		{name: "list-empty", parent: groupEnvelope(oneGroup, `1`), listing: groupEnvelope(`[]`, `0`), absent: true, calls: 2},
		{name: "rule-found", parent: groupEnvelope(oneGroup, `1`), listing: groupEnvelope(oneRule, `1`), calls: 2},
		{name: "list-check-param", parent: groupEnvelope(oneGroup, `1`), listErr: &client.Error{Status: 400, Kind: "response", Code: "CHECK_PARAM"}, calls: 2},
		{name: "list-404", parent: groupEnvelope(oneGroup, `1`), listErr: &client.Error{Status: 404, Kind: "response", Code: "NOT_FOUND"}, calls: 2},
		{name: "malformed-list", parent: groupEnvelope(oneGroup, `1`), listing: groupEnvelope(oneRule, `2`), calls: 2},
	}
	for _, tc := range scenarios {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAPI{get: func(p string, q url.Values) (client.Envelope, error) {
				if p == "/v1/security-groups/FIREWALL-test" {
					return tc.parent, tc.parentErr
				}
				return tc.listing, tc.listErr
			}}
			s := Service{API: f}
			r, err := s.Rule(context.Background(), "FIREWALL-test", "9007199254740993")
			if tc.absent {
				if err != nil || r != nil {
					t.Fatalf("absence failed: %v", err)
				}
			} else if tc.name == "rule-found" {
				if err != nil || r == nil || r.ID != "9007199254740993" {
					t.Fatal("exact-ID lookup failed")
				}
			} else if err == nil {
				t.Fatal("unverified absence accepted")
			}
			if f.calls != tc.calls {
				t.Fatal("unexpected parent/list requests")
			}
		})
	}
}
func TestRuleCreateRetainsIDBeforeContractErrors(t *testing.T) {
	for _, mutation := range []string{"count", "field", "status", "maximum-ID"} {
		t.Run(mutation, func(t *testing.T) {
			e := groupEnvelope(oneRule, `1`)
			wantID := "9007199254740993"
			switch mutation {
			case "count":
				e.Count = json.RawMessage(`99`)
			case "field":
				e.Result = json.RawMessage(strings.Replace(oneRule, `"TCP"`, `"unknown"`, 1))
			case "status":
				e.Status = 202
			case "maximum-ID":
				e.Result = json.RawMessage(strings.Replace(oneRule, wantID, "9223372036854775807", 1))
				wantID = "9223372036854775807"
			}
			f := &fakeAPI{response: e}
			s := Service{API: f}
			r, err := s.CreateRule(context.Background(), "FIREWALL-test", validRuleInput())
			if r.ID != wantID || f.calls != 1 {
				t.Fatal("identity was lost or create replayed")
			}
			if mutation == "maximum-ID" {
				if err != nil || r.Rule == nil {
					t.Fatal("exact max integer rejected")
				}
			} else if err == nil || r.Rule != nil {
				t.Fatal("remaining contract error hidden")
			}
		})
	}
}
func TestRuleAmbiguousCreateNeverInventsIdentity(t *testing.T) {
	for _, result := range []string{`[]`, `null`, `[{"rule_id":0}]`, `[{"rule_id":"12"}]`, `[{"rule_id":12},{"rule_id":13}]`} {
		f := &fakeAPI{response: groupEnvelope(result, `1`)}
		s := Service{API: f}
		r, err := s.CreateRule(context.Background(), "FIREWALL-test", validRuleInput())
		if err == nil || r.ID != "" || f.calls != 1 {
			t.Fatal("ambiguous create adopted or retried")
		}
	}
}
func TestRuleWritesAndOmission(t *testing.T) {
	f := &fakeAPI{response: groupEnvelope(oneRule, `1`)}
	s := Service{API: f}
	in := validRuleInput()
	_, err := s.CreateRule(context.Background(), "FIREWALL-test", in)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"bound": "IN", "protocol": "TCP", "port": "8443", "ip": "192.0.2.1/32", "title": "tf-rule", "content": *in.Description}
	if f.method != "POST" || !reflect.DeepEqual(f.body, want) {
		t.Fatal("write encoding changed")
	}
	in.Description = nil
	_, err = s.UpdateRule(context.Background(), "FIREWALL-test", "9007199254740993", in)
	if err != nil {
		t.Fatal(err)
	}
	delete(want, "content")
	if f.method != "PUT" || !reflect.DeepEqual(f.body, want) {
		t.Fatal("omitted description was not omitted")
	}
	empty := ""
	in.Description = &empty
	before := f.calls
	if _, err = s.UpdateRule(context.Background(), "FIREWALL-test", "9007199254740993", in); err == nil || f.calls != before {
		t.Fatal("unsupported clear reached API")
	}
	f.response = groupEnvelope(strings.Replace(oneRule, `"literal &amp; &#39; <x>"`, `null`, 1), `1`)
	r, err := s.CreateRule(context.Background(), "FIREWALL-test", in)
	if err != nil || r.Rule.Description != nil || f.body.(map[string]string)["content"] != "" {
		t.Fatal("empty create/null response contract failed")
	}
	f.response = client.Envelope{Status: 200, Result: json.RawMessage(`"deleted"`)}
	if err = s.DeleteRule(context.Background(), "FIREWALL-test", "9007199254740993"); err != nil {
		t.Fatal(err)
	}
	if f.method != "DELETE" || f.path != "/v1/security-groups/FIREWALL-test/rules/9007199254740993" {
		t.Fatal("wrong delete target")
	}
	f.response.Result = json.RawMessage(`null`)
	if s.DeleteRule(context.Background(), "FIREWALL-test", "9007199254740993") == nil {
		t.Fatal("null acknowledgement accepted")
	}
}
func TestRuleValidationBeforeIO(t *testing.T) {
	f := &fakeAPI{}
	s := Service{API: f}
	ctx := context.Background()
	for _, id := range []string{"", "0", "01", "-1", "1/other", "1?x", "+1", "9223372036854775808"} {
		if _, err := s.Rule(ctx, "FIREWALL-test", id); err == nil {
			t.Fatal("invalid rule ID accepted")
		}
		if err := s.DeleteRule(ctx, "FIREWALL-test", id); err == nil {
			t.Fatal("invalid delete ID accepted")
		}
	}
	for _, modify := range []func(*RuleInput){func(r *RuleInput) { r.Bound = "inbound" }, func(r *RuleInput) { r.Protocol = "tcp" }, func(r *RuleInput) { r.Port = "-1" }, func(r *RuleInput) { r.Port = "80-1" }, func(r *RuleInput) { r.IP = "2001:db8::/64" }, func(r *RuleInput) { r.Name = "" }, func(r *RuleInput) { r.Name = strings.Repeat("한", 26) }, func(r *RuleInput) { d := strings.Repeat("x", 26); r.Description = &d }} {
		in := validRuleInput()
		modify(&in)
		if _, err := s.CreateRule(ctx, "FIREWALL-test", in); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	if _, err := s.CreateRule(ctx, "FIREWALL-test/other", validRuleInput()); err == nil {
		t.Fatal("invalid parent accepted")
	}
	if f.calls != 0 {
		t.Fatal("invalid inputs reached API")
	}
}
func TestRuleErrorsAreSingleAttempt(t *testing.T) {
	sentinel := errors.New("synthetic API error")
	f := &fakeAPI{err: sentinel}
	s := Service{API: f}
	ctx := context.Background()
	_, e1 := s.CreateRule(ctx, "FIREWALL-test", validRuleInput())
	_, e2 := s.UpdateRule(ctx, "FIREWALL-test", "1", validRuleInput())
	e3 := s.DeleteRule(ctx, "FIREWALL-test", "1")
	_, e4 := s.Rules(ctx, "FIREWALL-test")
	for _, err := range []error{e1, e2, e3, e4} {
		if !errors.Is(err, sentinel) {
			t.Fatal("error classification changed")
		}
	}
	if f.calls != 4 {
		t.Fatal("operation was retried")
	}
}

func TestRulePortAndCIDRBoundaries(t *testing.T) {
	for _, port := range []string{"1", "65535", "1-65535", "8443-8443"} {
		if _, _, err := ParseRulePorts(port); err != nil {
			t.Fatal(err)
		}
	}
	for _, port := range []string{"0", "0-65535", "65536", "1-65536", "8444-8443", "-1", "1-2-3", "1.5", " 80", "+80"} {
		if _, _, err := ParseRulePorts(port); err == nil {
			t.Fatal("invalid port accepted")
		}
	}
	for _, ip := range []string{"192.0.2.1/32", "192.0.2.1/24", "0.0.0.0/0"} {
		if err := ValidateRuleIPv4(ip); err != nil {
			t.Fatal(err)
		}
	}
	for _, ip := range []string{"192.0.2.1", "2001:db8::/64", "::ffff:192.0.2.1/128", "192.0.2.1/33"} {
		if ValidateRuleIPv4(ip) == nil {
			t.Fatal("unverified or invalid IP accepted")
		}
	}
}
