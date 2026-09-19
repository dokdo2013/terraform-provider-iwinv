package storage

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type fakeAPI struct {
	calls    int
	query    url.Values
	envelope client.Envelope
	err      error
}

func (a *fakeAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.calls++
	a.query = q
	if p != "/v1/block-storages/types" {
		return client.Envelope{}, errors.New("wrong path")
	}
	return a.envelope, a.err
}
func envelope(body string, count string) client.Envelope {
	return client.Envelope{Status: 200, Result: json.RawMessage(body), Count: json.RawMessage(count)}
}

const rows = `[{"type":"ssd","min":10,"max":9007199254740993,"zones":null},{"type":"sata","min":0,"max":20000,"zones":["zone_z","zone-a"]},{"type":"empty-zones","min":0,"max":0,"zones":[]}]`

func TestTypes(t *testing.T) {
	a := &fakeAPI{envelope: envelope(rows, "3")}
	s := Service{API: a}
	got, err := s.Types(context.Background(), nil)
	want := []BlockStorageType{{"empty-zones", 0, 0, []string{}}, {"sata", 0, 20000, []string{"zone-a", "zone_z"}}, {"ssd", 10, 9007199254740993, nil}}
	if err != nil || !reflect.DeepEqual(got, want) || len(a.query) != 0 || a.calls != 1 {
		t.Fatalf("unexpected types or request: %v %v", got, err)
	}
	filter := "type + & special"
	a.envelope = envelope(`[{"type":"type + & special","min":0,"max":0,"zones":null}]`, "1")
	got, err = s.Types(context.Background(), &filter)
	if err != nil || len(got) != 1 || a.query.Get("type") != filter || len(a.query) != 1 {
		t.Fatal("exact filter not preserved")
	}
	a.envelope = envelope(`[]`, "0")
	got, err = s.Types(context.Background(), nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatal("valid empty result changed")
	}
}
func TestTypesRejectChangedContracts(t *testing.T) {
	valid := `[{"type":"ssd","min":10,"max":2000,"zones":null}]`
	for name, body := range map[string]string{
		"null": "null", "object": "{}", "null-row": "[null]", "type-missing": `[{"min":10,"max":20,"zones":null}]`,
		"type-null": `[{"type":null,"min":10,"max":20,"zones":null}]`, "type-empty": `[{"type":"","min":10,"max":20,"zones":null}]`,
		"min-missing": `[{"type":"ssd","max":20,"zones":null}]`, "min-null": `[{"type":"ssd","min":null,"max":20,"zones":null}]`,
		"fraction": `[{"type":"ssd","min":0.5,"max":20,"zones":null}]`, "overflow": `[{"type":"ssd","min":0,"max":9223372036854775808,"zones":null}]`,
		"negative": `[{"type":"ssd","min":-1,"max":20,"zones":null}]`, "reversed": `[{"type":"ssd","min":21,"max":20,"zones":null}]`,
		"string-size": `[{"type":"ssd","min":"10","max":20,"zones":null}]`, "zones-missing": `[{"type":"ssd","min":10,"max":20}]`,
		"zones-object": `[{"type":"ssd","min":10,"max":20,"zones":{}}]`, "zone-null": `[{"type":"ssd","min":10,"max":20,"zones":[null]}]`,
		"zone-empty": `[{"type":"ssd","min":10,"max":20,"zones":[""]}]`, "zone-duplicate": `[{"type":"ssd","min":10,"max":20,"zones":["x","x"]}]`,
		"duplicate-type": `[{"type":"ssd","min":10,"max":20,"zones":null},{"type":"ssd","min":10,"max":20,"zones":[]}]`,
	} {
		t.Run(name, func(t *testing.T) {
			count := "1"
			if name == "duplicate-type" {
				count = "2"
			}
			s := Service{API: &fakeAPI{envelope: envelope(body, count)}}
			if got, err := s.Types(context.Background(), nil); err == nil || got != nil {
				t.Fatal("malformed result accepted")
			}
		})
	}
	for _, mutate := range []func(*client.Envelope){
		func(e *client.Envelope) { e.Status = 202 }, func(e *client.Envelope) { e.Count = nil }, func(e *client.Envelope) { e.Count = json.RawMessage(`null`) }, func(e *client.Envelope) { e.Count = json.RawMessage(`"1"`) }, func(e *client.Envelope) { e.Count = json.RawMessage(`0`) },
		func(e *client.Envelope) { e.Page = json.RawMessage(`null`) }, func(e *client.Envelope) { e.PageNo = json.RawMessage(`1`) }, func(e *client.Envelope) { e.PageSize = json.RawMessage(`50`) }, func(e *client.Envelope) { e.Total = json.RawMessage(`1`) },
	} {
		e := envelope(valid, "1")
		mutate(&e)
		s := Service{API: &fakeAPI{envelope: e}}
		if got, err := s.Types(context.Background(), nil); err == nil || got != nil {
			t.Fatal("changed metadata accepted")
		}
	}
	a := &fakeAPI{envelope: envelope(valid, "1")}
	s := Service{API: a}
	filter := "sata"
	if got, err := s.Types(context.Background(), &filter); err == nil || got != nil {
		t.Fatal("filter mismatch accepted")
	}
	for _, remote := range []error{&client.Error{Kind: "http_status", Status: 400, Code: "CHECK_PARAM"}, &client.Error{Kind: "http_status", Status: 404, Code: "NOT_FOUND"}, errors.New("transport error")} {
		a.err = remote
		if got, err := s.Types(context.Background(), nil); !errors.Is(err, remote) || got != nil {
			t.Fatal("API error became empty result")
		}
	}
	a.calls = 0
	filter = ""
	if _, err := s.Types(context.Background(), &filter); err == nil || a.calls != 0 {
		t.Fatal("empty filter reached API")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Types(ctx, nil); !errors.Is(err, context.Canceled) || a.calls != 0 {
		t.Fatal("cancelled context reached API")
	}
}
