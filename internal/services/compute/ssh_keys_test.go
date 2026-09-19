package compute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

func sshPage(page, count int) client.Envelope {
	rows := make([]map[string]string, 0, count)
	for i := count - 1; i >= 0; i-- {
		rows = append(rows, map[string]string{"ssh_key_id": fmt.Sprintf("key-%05d", (page-1)*10+i), "name": "같은 이름", "private_key": "synthetic-secret"})
	}
	b, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: b, Count: json.RawMessage(strconv.Itoa(count)), PageNo: json.RawMessage(strconv.Itoa(page)), PageSize: json.RawMessage(`10`)}
}

func TestSSHKeysPaginationAndExactLookup(t *testing.T) {
	calls := 0
	s := Service{API: apiFunc(func(_ context.Context, path string, q url.Values) (client.Envelope, error) {
		calls++
		page, _ := strconv.Atoi(q.Get("page_no"))
		if path != "/v1/auth/ssh_key" || len(q) != 2 || q.Get("page_size") != "10" {
			t.Fatal("unexpected endpoint or selection query")
		}
		if page == 3 {
			return sshPage(page, 0), nil
		}
		return sshPage(page, 10), nil
	})}
	keys, err := s.SSHKeys(context.Background())
	if err != nil || calls != 3 || len(keys) != 20 || keys[0].ID != "key-00000" || keys[19].ID != "key-00019" {
		t.Fatalf("pagination or order failed: %v", err)
	}
	b, _ := json.Marshal(keys)
	if strings.Contains(string(b), "synthetic-secret") {
		t.Fatal("key material escaped the wire response")
	}
	calls = 0
	key, err := s.SSHKey(context.Background(), "key-00000")
	if err != nil || key.ID != "key-00000" || calls != 3 {
		t.Fatal("exact lookup must validate all pages even when the match occurs early")
	}
	if _, err := s.SSHKey(context.Background(), "missing"); err == nil {
		t.Fatal("missing ID adopted another key")
	}
	calls = 0
	if _, err := s.SSHKey(context.Background(), ""); err == nil || calls != 0 {
		t.Fatal("empty ID reached API")
	}
}

func TestSSHKeysNeverReturnPartialLists(t *testing.T) {
	for _, mutate := range []func(*client.Envelope){
		func(e *client.Envelope) { e.Status = 202 },
		func(e *client.Envelope) { e.Result = json.RawMessage(`null`) },
		func(e *client.Envelope) { e.Result = json.RawMessage(`{}`) },
		func(e *client.Envelope) { e.PageNo = json.RawMessage(`1`) },
		func(e *client.Envelope) { e.PageSize = json.RawMessage(`"10"`) },
		func(e *client.Envelope) { e.Count = nil },
		func(e *client.Envelope) { e.Count = json.RawMessage(`null`) },
		func(e *client.Envelope) { e.Count = json.RawMessage(`0`) },
		func(e *client.Envelope) { e.Total = json.RawMessage(`20`) },
		func(e *client.Envelope) { e.Page = json.RawMessage(`2`) },
		func(e *client.Envelope) { e.Result = sshPage(1, 10).Result },
		func(e *client.Envelope) {
			e.Result = json.RawMessage(`[{"ssh_key_id":"new"}]`)
			e.Count = json.RawMessage(`1`)
		},
		func(e *client.Envelope) {
			e.Result = json.RawMessage(`[{"name":"new"}]`)
			e.Count = json.RawMessage(`1`)
		},
	} {
		calls := 0
		s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
			calls++
			e := sshPage(calls, 10)
			if calls == 2 {
				mutate(&e)
			}
			return e, nil
		})}
		if keys, err := s.SSHKeys(context.Background()); err == nil || keys != nil {
			t.Fatal("bad later page returned partial success")
		}
	}
	want := errors.New("synthetic late failure")
	calls := 0
	s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
		calls++
		if calls == 2 {
			return client.Envelope{}, want
		}
		return sshPage(1, 10), nil
	})}
	if key, err := s.SSHKey(context.Background(), "key-00000"); !errors.Is(err, want) || key.ID != "" {
		t.Fatal("early match hid a later failure")
	}
}

func TestSSHKeysEmptyBoundsAndCancellation(t *testing.T) {
	s := Service{API: apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) { return sshPage(1, 0), nil })}
	if keys, err := s.SSHKeys(context.Background()); err != nil || keys == nil || len(keys) != 0 {
		t.Fatal("valid empty list lost")
	}
	calls := 0
	s.API = apiFunc(func(context.Context, string, url.Values) (client.Envelope, error) {
		calls++
		return sshPage(calls, 10), nil
	})
	if keys, err := s.SSHKeys(context.Background()); err == nil || keys != nil || calls != sshKeyMaxPages {
		t.Fatal("pagination bound not enforced")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls = 0
	if _, err := s.SSHKeys(ctx); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("canceled request reached API")
	}
}
