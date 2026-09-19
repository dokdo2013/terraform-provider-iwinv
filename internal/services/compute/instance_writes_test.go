package compute

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

type instanceWriteStub struct {
	post func(context.Context, string, map[string]string) (client.Envelope, error)
	put  func(context.Context, string, url.Values, map[string]string) (client.Envelope, error)
	del  func(context.Context, string) (client.Envelope, error)
}

func (s instanceWriteStub) PostForm(c context.Context, p string, b map[string]string) (client.Envelope, error) {
	return s.post(c, p, b)
}
func (s instanceWriteStub) PutFormWithQuery(c context.Context, p string, q url.Values, b map[string]string) (client.Envelope, error) {
	return s.put(c, p, q, b)
}
func (s instanceWriteStub) Delete(c context.Context, p string) (client.Envelope, error) {
	return s.del(c, p)
}
func instanceText(s string) *string { return &s }
func createInput() InstanceCreateInput {
	return InstanceCreateInput{ZoneID: "synthetic-zone", ImageID: "synthetic-image", FlavorID: "synthetic-flavor"}
}
func createEnvelope() client.Envelope {
	return client.Envelope{Status: 202, Count: json.RawMessage(`1`), Result: json.RawMessage(`[{"instance_id":"synthetic-instance","status":"building","default_account":{"password":"synthetic-password-canary"}}]`)}
}

func TestInstanceCreateOneMultipartRequest(t *testing.T) {
	in := createInput()
	in.Name = instanceText("한글 +%")
	in.Description = instanceText("")
	in.SSHKeyIDs = []string{"key-b", "key-a"}
	in.UserScriptID = instanceText("script-a")
	calls := 0
	s := InstanceWriter{API: instanceWriteStub{post: func(_ context.Context, p string, b map[string]string) (client.Envelope, error) {
		calls++
		want := map[string]string{"zone_id": "synthetic-zone", "image_id": "synthetic-image", "flavor_id": "synthetic-flavor", "count": "1", "name": "%ED%95%9C%EA%B8%80+%2B%25", "description": "", "ssh_key_id": "key-a,key-b", "user_script_id": "script-a"}
		if p != "/v1/instances" || !reflect.DeepEqual(b, want) {
			t.Fatal("unexpected fields, encoding, order or bulk create")
		}
		return createEnvelope(), nil
	}}}
	r, err := s.CreateInstance(context.Background(), in)
	if err != nil || calls != 1 || r.ID != "synthetic-instance" || !reflect.DeepEqual(r.RecoveryIDs, []string{"synthetic-instance"}) {
		t.Fatal("create receipt lost")
	}
	if in.SSHKeyIDs[0] != "key-b" {
		t.Fatal("caller input mutated")
	}
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "canary") {
		t.Fatal("response secret retained")
	}
	omitted, err := createInstanceFields(createInput())
	if err != nil || len(omitted) != 4 {
		t.Fatal("optional inputs not omitted")
	}
}

func TestInstanceCreatePreservesIdentityOnResponseFailure(t *testing.T) {
	cases := []struct {
		name         string
		mutate       func(*client.Envelope)
		wantID       string
		wantRecovery []string
	}{
		{"missing count", func(e *client.Envelope) { e.Count = nil }, "synthetic-instance", []string{"synthetic-instance"}},
		{"wrong count", func(e *client.Envelope) { e.Count = json.RawMessage(`2`) }, "synthetic-instance", []string{"synthetic-instance"}},
		{"unexpected success", func(e *client.Envelope) { e.Status = 200 }, "synthetic-instance", []string{"synthetic-instance"}},
		{"missing status", func(e *client.Envelope) { e.Result = json.RawMessage(`[{"instance_id":"synthetic-instance"}]`) }, "synthetic-instance", []string{"synthetic-instance"}},
		{"bad status type", func(e *client.Envelope) {
			e.Result = json.RawMessage(`[{"instance_id":"synthetic-instance","status":123}]`)
		}, "synthetic-instance", []string{"synthetic-instance"}},
		{"two rows", func(e *client.Envelope) {
			e.Result = json.RawMessage(`[{"instance_id":"synthetic-b"},{"instance_id":"synthetic-a"}]`)
		}, "", []string{"synthetic-a", "synthetic-b"}},
		{"mixed malformed row", func(e *client.Envelope) { e.Result = json.RawMessage(`[{"instance_id":"synthetic-a"},42]`) }, "", []string{"synthetic-a"}},
		{"duplicate rows", func(e *client.Envelope) {
			e.Result = json.RawMessage(`[{"instance_id":"synthetic-a"},{"instance_id":"synthetic-a"}]`)
		}, "", []string{"synthetic-a"}},
		{"wrong id type", func(e *client.Envelope) { e.Result = json.RawMessage(`[{"instance_id":123}]`) }, "", nil},
		{"invalid id", func(e *client.Envelope) { e.Result = json.RawMessage(`[{"instance_id":"../bad"}]`) }, "", nil},
		{"empty result", func(e *client.Envelope) { e.Result = json.RawMessage(`[]`) }, "", nil},
		{"null result", func(e *client.Envelope) { e.Result = json.RawMessage(`null`) }, "", nil},
		{"malformed result", func(e *client.Envelope) { e.Result = json.RawMessage(`[{`) }, "", nil},
		{"object result", func(e *client.Envelope) { e.Result = json.RawMessage(`{"instance_id":"synthetic-a"}`) }, "", nil},
		{"unsuccessful status", func(e *client.Envelope) { e.Status = 500 }, "", nil},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			s := InstanceWriter{API: instanceWriteStub{post: func(context.Context, string, map[string]string) (client.Envelope, error) {
				calls++
				e := createEnvelope()
				tt.mutate(&e)
				return e, nil
			}}}
			r, err := s.CreateInstance(context.Background(), createInput())
			if err == nil || calls != 1 || r.ID != tt.wantID || !reflect.DeepEqual(r.RecoveryIDs, tt.wantRecovery) || strings.Contains(err.Error(), "canary") {
				t.Fatal("create recovery identity lost, guessed or replayed")
			}
		})
	}
}

func TestInstanceWriteValidationAndCancellation(t *testing.T) {
	calls := 0
	s := InstanceWriter{API: instanceWriteStub{
		post: func(context.Context, string, map[string]string) (client.Envelope, error) {
			calls++
			return createEnvelope(), nil
		},
		put: func(context.Context, string, url.Values, map[string]string) (client.Envelope, error) {
			calls++
			return client.Envelope{}, nil
		},
		del: func(context.Context, string) (client.Envelope, error) { calls++; return client.Envelope{}, nil },
	}}
	for _, mutate := range []func(*InstanceCreateInput){
		func(i *InstanceCreateInput) { i.ZoneID = "" }, func(i *InstanceCreateInput) { i.ImageID = "../bad" }, func(i *InstanceCreateInput) { i.FlavorID = "bad?a" },
		func(i *InstanceCreateInput) { i.SSHKeyIDs = []string{"a", "a"} }, func(i *InstanceCreateInput) { i.SSHKeyIDs = []string{"a,b"} },
		func(i *InstanceCreateInput) { i.UserScriptID = instanceText("") }, func(i *InstanceCreateInput) { i.Name = instanceText("") },
		func(i *InstanceCreateInput) { i.Description = instanceText(string([]byte{0xff})) },
	} {
		i := createInput()
		mutate(&i)
		if _, err := s.CreateInstance(context.Background(), i); err == nil {
			t.Fatal("invalid create accepted")
		}
	}
	for _, id := range []string{"", "..", "bad/id", "bad?fields=128"} {
		if _, err := s.UpdateInstance(context.Background(), id, InstanceUpdateInput{Name: instanceText("test")}); err == nil {
			t.Fatal("invalid update path")
		}
		if _, err := s.DeleteInstance(context.Background(), id); err == nil {
			t.Fatal("invalid delete path")
		}
	}
	if _, err := s.UpdateInstance(context.Background(), "synthetic", InstanceUpdateInput{}); err == nil {
		t.Fatal("empty update accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.CreateInstance(ctx, createInput()); !errors.Is(err, context.Canceled) {
		t.Fatal("create cancellation lost")
	}
	if _, err := s.UpdateInstance(ctx, "synthetic", InstanceUpdateInput{Name: instanceText("test")}); !errors.Is(err, context.Canceled) {
		t.Fatal("update cancellation lost")
	}
	if _, err := s.DeleteInstance(ctx, "synthetic"); !errors.Is(err, context.Canceled) {
		t.Fatal("delete cancellation lost")
	}
	if calls != 0 {
		t.Fatal("invalid/canceled write reached API")
	}
}

func TestInstanceUpdateSafeProjection(t *testing.T) {
	calls := 0
	e := instanceEnvelope(1, 1)
	s := InstanceWriter{API: instanceWriteStub{put: func(_ context.Context, p string, q url.Values, b map[string]string) (client.Envelope, error) {
		calls++
		if p != "/v1/instances/synthetic-00000" || !reflect.DeepEqual(q, url.Values{"fields": {"3599"}}) || !reflect.DeepEqual(b, map[string]string{"description": ""}) {
			t.Fatal("update omitted/empty distinction or projection changed")
		}
		return e, nil
	}}}
	r, err := s.UpdateInstance(context.Background(), "synthetic-00000", InstanceUpdateInput{Description: instanceText("")})
	if err != nil || r.ID != "synthetic-00000" || calls != 1 {
		t.Fatal("update response invalid")
	}
	for _, bad := range []client.Envelope{instanceEnvelope(1, 0), instanceEnvelope(1, 2), instanceEnvelope(2, 1), {Status: 202}} {
		e = bad
		before := calls
		if _, err := s.UpdateInstance(context.Background(), "synthetic-00000", InstanceUpdateInput{Description: instanceText("")}); err == nil || calls != before+1 {
			t.Fatal("malformed update accepted or replayed")
		}
	}
}

func TestInstanceDeleteOnlyAcknowledgesAndRetainsVolumes(t *testing.T) {
	calls := 0
	e := client.Envelope{Status: 202, Count: json.RawMessage(`1`), Result: json.RawMessage(`[{"status":"deleting","block_storage":["synthetic-volume-b","synthetic-volume-a"]}]`)}
	s := InstanceWriter{API: instanceWriteStub{del: func(_ context.Context, p string) (client.Envelope, error) {
		calls++
		if p != "/v1/instances/synthetic" {
			t.Fatal("delete expanded beyond exact instance")
		}
		return e, nil
	}}}
	r, err := s.DeleteInstance(context.Background(), "synthetic")
	if err != nil || calls != 1 || !reflect.DeepEqual(r.RetainedBlockStorageIDs, []string{"synthetic-volume-a", "synthetic-volume-b"}) {
		t.Fatal("retained storage receipt lost")
	}
	for _, raw := range []string{`[]`, `null`, `[{"status":"active","block_storage":[]}]`, `[{"status":"deleting"}]`, `[{"status":"deleting","block_storage":null}]`, `[{"status":"deleting","block_storage":["a","a"]}]`, `[{"status":"deleting","block_storage":["../bad"]}]`} {
		e.Result = json.RawMessage(raw)
		before := calls
		if _, err := s.DeleteInstance(context.Background(), "synthetic"); err == nil || calls != before+1 {
			t.Fatal("unverified delete accepted or replayed")
		}
	}
}

func TestInstanceWriteErrorsNeverReplayOrAdoptResponseIDs(t *testing.T) {
	for _, want := range []error{errors.New("synthetic transport failure"), &client.Error{Kind: "http", Status: 500, Code: "DEV_CHECK_RETURN"}} {
		calls := 0
		s := InstanceWriter{API: instanceWriteStub{
			post: func(context.Context, string, map[string]string) (client.Envelope, error) {
				calls++
				return createEnvelope(), want
			},
			put: func(context.Context, string, url.Values, map[string]string) (client.Envelope, error) {
				calls++
				return client.Envelope{}, want
			},
			del: func(context.Context, string) (client.Envelope, error) { calls++; return client.Envelope{}, want },
		}}
		if r, err := s.CreateInstance(context.Background(), createInput()); !errors.Is(err, want) || r.ID != "" || len(r.RecoveryIDs) != 0 {
			t.Fatal("error response identity adopted")
		}
		if _, err := s.UpdateInstance(context.Background(), "synthetic", InstanceUpdateInput{Name: instanceText("test")}); !errors.Is(err, want) {
			t.Fatal("update error hidden")
		}
		if _, err := s.DeleteInstance(context.Background(), "synthetic"); !errors.Is(err, want) {
			t.Fatal("delete error hidden")
		}
		if calls != 3 {
			t.Fatal("uncertain writes retried")
		}
	}
}
