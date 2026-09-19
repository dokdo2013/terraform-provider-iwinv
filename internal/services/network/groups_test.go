package network

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type fakeAPI struct {
	get      func(string, url.Values) (client.Envelope, error)
	response client.Envelope
	err      error
	calls    int
	method   string
	path     string
	body     any
}

func (f *fakeAPI) call(method, path string, body any) (client.Envelope, error) {
	f.calls++
	f.method, f.path, f.body = method, path, body
	return f.response, f.err
}
func (f *fakeAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	if f.get != nil {
		f.calls++
		return f.get(p, q)
	}
	return f.call("GET", p, q)
}
func (f *fakeAPI) PostJSON(_ context.Context, p string, b any) (client.Envelope, error) {
	return f.call("POST", p, b)
}
func (f *fakeAPI) PutJSON(_ context.Context, p string, b any) (client.Envelope, error) {
	return f.call("PUT", p, b)
}
func (f *fakeAPI) Delete(_ context.Context, p string) (client.Envelope, error) {
	return f.call("DELETE", p, nil)
}

func groupEnvelope(body, count string) client.Envelope {
	return client.Envelope{Status: 200, Result: json.RawMessage(body), Count: json.RawMessage(count)}
}

const oneGroup = `[{"firewall_id":"FIREWALL-test","title":"한글 & + %","content":"description","icmp":"Y","rules":[{"secret":"synthetic-secret"}]}]`

func TestGroupReadsAndAbsence(t *testing.T) {
	f := &fakeAPI{response: groupEnvelope(oneGroup, "1")}
	s := Service{API: f}
	g, err := s.Group(context.Background(), "FIREWALL-test")
	if err != nil || g == nil || g.Name != "한글 & + %" || !g.AllowICMP || g.Description == nil || *g.Description != "description" || f.path != "/v1/security-groups/FIREWALL-test" {
		t.Fatalf("detail mapping failed: %v", err)
	}
	b, _ := json.Marshal(g)
	if strings.Contains(string(b), "synthetic-secret") {
		t.Fatal("unowned nested fields leaked")
	}
	f.response = groupEnvelope(`[]`, "0")
	if g, err := s.Group(context.Background(), "FIREWALL-test"); err != nil || g != nil {
		t.Fatal("verified endpoint absence not recognized")
	}
	f.response.PageNo = json.RawMessage(`1`)
	f.response.PageSize = json.RawMessage(`50`)
	if rows, err := s.Groups(context.Background()); err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("empty inventory became null or failed")
	}
	f.response = groupEnvelope(`[{"firewall_id":"FIREWALL-z","title":"same","content":null,"icmp":"N"},{"firewall_id":"FIREWALL-a","title":"same","content":"","icmp":"Y"}]`, "2")
	f.response.PageNo = json.RawMessage(`1`)
	f.response.PageSize = json.RawMessage(`50`)
	rows, err := s.Groups(context.Background())
	if err != nil || len(rows) != 2 || rows[0].ID != "FIREWALL-a" || rows[0].Description == nil || *rows[0].Description != "" || rows[1].Description != nil {
		t.Fatal("ordering or null/empty distinction failed")
	}
	if _, err := s.Group(context.Background(), "FIREWALL-z"); err == nil {
		t.Fatal("multiple detail rows accepted")
	}
}

func TestInvalidGroupResponsesNeverBecomeAbsence(t *testing.T) {
	for _, e := range []client.Envelope{
		{Status: 404, Result: json.RawMessage(`[]`), Count: json.RawMessage(`0`)},
		{Status: 202, Result: json.RawMessage(`[]`), Count: json.RawMessage(`0`)},
		{Status: 200, Result: json.RawMessage(`[]`)},
		{Status: 200, Result: json.RawMessage(`[]`), Count: json.RawMessage(`null`)},
		{Status: 200, Result: json.RawMessage(`[]`), Count: json.RawMessage(`0`), PageNo: json.RawMessage(`1`)},
		groupEnvelope(`null`, "0"), groupEnvelope(`{}`, "0"), groupEnvelope(`[]`, "1"),
		groupEnvelope(oneGroup, "0"),
		groupEnvelope(strings.ReplaceAll(oneGroup, "FIREWALL-test", "FIREWALL-other"), "1"),
		groupEnvelope(strings.ReplaceAll(oneGroup, `"icmp":"Y"`, `"icmp":"unknown"`), "1"),
		groupEnvelope(strings.ReplaceAll(oneGroup, `"content":"description",`, ""), "1"),
		groupEnvelope(strings.ReplaceAll(oneGroup, `"content":"description"`, `"content":{}`), "1"),
	} {
		s := Service{API: &fakeAPI{response: e}}
		if g, err := s.Group(context.Background(), "FIREWALL-test"); err == nil || g != nil {
			t.Fatal("malformed/error/unexpected detail became valid absence or a group")
		}
	}
	duplicate := strings.TrimSuffix(oneGroup, "]") + "," + strings.TrimPrefix(oneGroup, "[")
	e := groupEnvelope(duplicate, "2")
	e.PageNo = json.RawMessage(`1`)
	e.PageSize = json.RawMessage(`50`)
	s := Service{API: &fakeAPI{response: e}}
	if rows, err := s.Groups(context.Background()); err == nil || rows != nil {
		t.Fatal("duplicate inventory accepted")
	}
}

func TestCreatePreservesIdentityBeforeOtherFields(t *testing.T) {
	for _, e := range []client.Envelope{
		groupEnvelope(strings.ReplaceAll(oneGroup, `"title":"한글 & + %"`, `"title":{}`), "1"),
		groupEnvelope(oneGroup, "0"),
		{Status: 202, Result: json.RawMessage(oneGroup), Count: json.RawMessage(`1`)},
		{Status: 201, Result: json.RawMessage(oneGroup), Count: json.RawMessage(`1`)},
	} {
		f := &fakeAPI{response: e}
		s := Service{API: f}
		created, err := s.CreateGroup(context.Background(), GroupInput{Name: "test"})
		if err == nil || created.ID != "FIREWALL-test" || created.Group != nil || f.calls != 1 {
			t.Fatal("partial create lost ID or retried")
		}
	}
	for _, body := range []string{`null`, `[]`, `{}`, `[{}]`, `[{"firewall_id":"FIREWALL-a"},{"firewall_id":"FIREWALL-b"}]`} {
		f := &fakeAPI{response: groupEnvelope(body, "1")}
		s := Service{API: f}
		created, err := s.CreateGroup(context.Background(), GroupInput{Name: "test"})
		if err == nil || created.ID != "" || f.calls != 1 {
			t.Fatal("ambiguous create selected identity or retried")
		}
	}
	for _, method := range []string{"create", "update", "delete", "read"} {
		want := &client.Error{Kind: "api", Status: 404, Code: "NOT_FOUND"}
		f := &fakeAPI{err: want}
		s := Service{API: f}
		var err error
		switch method {
		case "create":
			_, err = s.CreateGroup(context.Background(), GroupInput{Name: "test"})
		case "update":
			_, err = s.UpdateGroup(context.Background(), "FIREWALL-test", GroupInput{Name: "test"})
		case "delete":
			err = s.DeleteGroup(context.Background(), "FIREWALL-test")
		case "read":
			_, err = s.Group(context.Background(), "FIREWALL-test")
		}
		if !errors.Is(err, want) || f.calls != 1 {
			t.Fatal("API error was hidden or retried")
		}
	}
}

func TestGroupWriteEncodingAndValidation(t *testing.T) {
	f := &fakeAPI{response: groupEnvelope(oneGroup, "1")}
	s := Service{API: f}
	desc := "한글 & + %"
	created, err := s.CreateGroup(context.Background(), GroupInput{Name: desc, Description: &desc, AllowICMP: true})
	if err != nil || created.ID != "FIREWALL-test" || created.Group == nil {
		t.Fatal("valid create failed")
	}
	body := f.body.(map[string]string)
	if f.method != "POST" || f.path != "/v1/security-groups" || body["title"] != desc || body["content"] != desc || body["icmp"] != "Y" || len(body) != 3 {
		t.Fatal("write normalized or added fields")
	}
	if _, err := s.UpdateGroup(context.Background(), "FIREWALL-test", GroupInput{Name: "test"}); err != nil {
		t.Fatal(err)
	}
	body = f.body.(map[string]string)
	if _, ok := body["content"]; ok || body["icmp"] != "N" || f.method != "PUT" {
		t.Fatal("omitted description or explicit false changed")
	}
	calls := f.calls
	empty := ""
	if _, err := s.UpdateGroup(context.Background(), "FIREWALL-test", GroupInput{Name: "test", Description: &empty}); err == nil || f.calls != calls {
		t.Fatal("unsupported clear reached API")
	}
	if _, err := s.CreateGroup(context.Background(), GroupInput{}); err == nil || f.calls != calls {
		t.Fatal("invalid name reached API")
	}
	for _, id := range []string{"", "../other", "FIREWALL-x/y", "FIREWALL-x?x=y", "FIREWALL-%2F", "FIREWALL-"} {
		if _, err := s.Group(context.Background(), id); err == nil {
			t.Fatal("invalid read ID")
		}
		if _, err := s.UpdateGroup(context.Background(), id, GroupInput{Name: "test"}); err == nil {
			t.Fatal("invalid update ID")
		}
		if err := s.DeleteGroup(context.Background(), id); err == nil {
			t.Fatal("invalid delete ID")
		}
	}
	if f.calls != calls {
		t.Fatal("invalid ID reached API")
	}
	f.response = client.Envelope{Status: 200, Result: json.RawMessage(`"acknowledged"`)}
	if err := s.DeleteGroup(context.Background(), "FIREWALL-test"); err != nil || f.calls != calls+1 || f.method != "DELETE" {
		t.Fatal("delete must be one acknowledgement, not an implicit waiter")
	}
	f.response.Result = json.RawMessage(`null`)
	if err := s.DeleteGroup(context.Background(), "FIREWALL-test"); err == nil {
		t.Fatal("invalid delete ack accepted")
	}
}

func networkPage(page, count int) client.Envelope {
	rows := make([]map[string]string, 0, count)
	for i := 0; i < count; i++ {
		rows = append(rows, map[string]string{"firewall_id": "FIREWALL-" + strconv.Itoa((page-1)*groupPageSize+i), "title": "test", "content": "", "icmp": "N"})
	}
	b, _ := json.Marshal(rows)
	e := groupEnvelope(string(b), strconv.Itoa(count))
	e.PageNo = json.RawMessage(strconv.Itoa(page))
	e.PageSize = json.RawMessage(strconv.Itoa(groupPageSize))
	return e
}
func TestGroupListPagination(t *testing.T) {
	f := &fakeAPI{}
	f.get = func(path string, q url.Values) (client.Envelope, error) {
		page, _ := strconv.Atoi(q.Get("page_no"))
		if path != "/v1/security-groups" || page != f.calls || q.Get("page_size") != "50" {
			t.Fatal("incorrect pagination request")
		}
		count := groupPageSize
		if page == 3 {
			count = 0
		}
		return networkPage(page, count), nil
	}
	s := Service{API: f}
	if rows, err := s.Groups(context.Background()); err != nil || len(rows) != 100 || f.calls != 3 {
		t.Fatal("full final page did not read the next empty page")
	}
	for _, mutate := range []func(*client.Envelope){
		func(e *client.Envelope) { e.PageNo = json.RawMessage(`1`) },
		func(e *client.Envelope) { e.PageSize = nil },
		func(e *client.Envelope) { e.Result = networkPage(1, 50).Result },
		func(e *client.Envelope) { e.Count = json.RawMessage(`49`) },
		func(e *client.Envelope) { e.Total = json.RawMessage(`100`) },
	} {
		f.calls = 0
		f.get = func(_ string, q url.Values) (client.Envelope, error) {
			e := networkPage(f.calls, 50)
			if f.calls == 2 {
				mutate(&e)
			}
			return e, nil
		}
		if rows, err := s.Groups(context.Background()); err == nil || rows != nil {
			t.Fatal("invalid later page returned partial inventory")
		}
	}
	want := errors.New("late failure")
	f.calls = 0
	f.get = func(_ string, _ url.Values) (client.Envelope, error) {
		if f.calls == 2 {
			return client.Envelope{}, want
		}
		return networkPage(1, 50), nil
	}
	if rows, err := s.Groups(context.Background()); !errors.Is(err, want) || rows != nil {
		t.Fatal("late error hidden")
	}
	f.calls = 0
	f.get = func(_ string, _ url.Values) (client.Envelope, error) { return networkPage(f.calls, 50), nil }
	if rows, err := s.Groups(context.Background()); err == nil || rows != nil || f.calls != groupMaxPages {
		t.Fatal("unbounded pagination")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.calls = 0
	if _, err := s.Groups(ctx); !errors.Is(err, context.Canceled) || f.calls != 0 {
		t.Fatal("canceled inventory read reached API")
	}
}

func TestDescriptionHTMLDecodingIsFieldSpecificAndSinglePass(t *testing.T) {
	for _, tt := range []struct{ wire, want string }{
		{`계약 + &amp; %`, `계약 + & %`},
		{`literal &amp;amp; &amp;#39; &amp;lt;`, `literal &amp; &#39; &lt;`},
		{`quotes &quot; &#039; &lt; &gt; &amp;`, `quotes " ' < > &`},
		{`%EA%B3%84%EC%95%BD+%2B+%26+%25`, `%EA%B3%84%EC%95%BD+%2B+%26+%25`},
		{"한글 e\u0301", "한글 e\u0301"},
	} {
		b, _ := json.Marshal([]map[string]string{{"firewall_id": "FIREWALL-test", "title": "name &amp; < >", "content": tt.wire, "icmp": "N"}})
		s := Service{API: &fakeAPI{response: groupEnvelope(string(b), "1")}}
		g, err := s.Group(context.Background(), "FIREWALL-test")
		if err != nil || g.Description == nil || *g.Description != tt.want || g.Name != "name &amp; < >" {
			t.Fatal("field-specific text contract changed")
		}
	}
}
