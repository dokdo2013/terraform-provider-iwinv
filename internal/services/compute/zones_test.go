package compute

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type fakeAPI struct {
	envelope client.Envelope
	err      error
}

func (f fakeAPI) Get(context.Context, string, url.Values) (client.Envelope, error) {
	return f.envelope, f.err
}

func TestZoneContract(t *testing.T) {
	for _, tt := range []struct {
		name, body, count string
		status            int
		valid             bool
	}{
		{"observed fields", `[{"zone_id":"synthetic-a","zone_name":"서울 테스트","status":"on"}]`, `1`, 200, true},
		{"empty", `[]`, `0`, 200, true},
		{"null list", `null`, `0`, 200, false},
		{"missing name", `[{"zone_id":"a","status":"on"}]`, `1`, 200, false},
		{"wrong name field", `[{"zone_id":"a","name":"Wrong","status":"on"}]`, `1`, 200, false},
		{"null field", `[{"zone_id":"a","zone_name":null,"status":"on"}]`, `1`, 200, false},
		{"duplicate", `[{"zone_id":"a","zone_name":"A","status":"on"},{"zone_id":"a","zone_name":"B","status":"on"}]`, `2`, 200, false},
		{"wrong count", `[]`, `1`, 200, false},
		{"missing count", `[]`, ``, 200, false},
		{"null count", `[]`, `null`, 200, false},
		{"string count", `[]`, `"0"`, 200, false},
		{"unexpected accepted", `[]`, `0`, 202, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := Service{API: fakeAPI{envelope: client.Envelope{Status: tt.status, Result: json.RawMessage(tt.body), Count: json.RawMessage(tt.count)}}}
			_, err := s.Zones(context.Background())
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%t, error=%v", tt.valid, err)
			}
		})
	}
}

func TestZoneErrorIsNotEmptyInventory(t *testing.T) {
	want := &client.Error{Kind: "http_status", Status: 403}
	s := Service{API: fakeAPI{err: want}}
	rows, err := s.Zones(context.Background())
	if rows != nil || !errors.Is(err, want) {
		t.Fatal("API failure became empty inventory")
	}
}
