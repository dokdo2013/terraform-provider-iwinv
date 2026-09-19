package client_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
)

type dbmsCaptureClient struct {
	*client.Client
	capture func(client.Envelope, error) error
	owned   func(string) bool
}

func (c *dbmsCaptureClient) PostJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	if p != "/v1/dbms" {
		return client.Envelope{}, errors.New("unexpected DBMS create path")
	}
	e, err := c.Client.PostJSON(ctx, p, body)
	if captureErr := c.capture(e, err); captureErr != nil {
		return client.Envelope{}, captureErr
	}
	return e, err
}
func (c *dbmsCaptureClient) PutJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/dbms/"), "/allowip")
	if p != "/v1/dbms/"+id+"/allowip" || !c.owned(id) {
		return client.Envelope{}, errors.New("refusing non-owned DBMS update")
	}
	return c.Client.PutJSON(ctx, p, body)
}
func (c *dbmsCaptureClient) Delete(ctx context.Context, p string) (client.Envelope, error) {
	id := strings.TrimPrefix(p, "/v1/dbms/")
	if p != "/v1/dbms/"+id || !c.owned(id) {
		return client.Envelope{}, errors.New("refusing non-owned DBMS deletion")
	}
	return c.Client.Delete(ctx, p)
}
func (c *dbmsCaptureClient) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	if p != "/v1/dbms" && p != "/v1/dbms/products" {
		return client.Envelope{}, errors.New("unexpected DBMS read path")
	}
	return c.Client.Get(ctx, p, q)
}

// TestAccDBMSControlPlaneWrites only owns newly returned IDs, never baseline
// resources. It proves control-plane contracts, not Terraform or SQL/Redis access.
func TestAccDBMSControlPlaneWrites(t *testing.T) {
	if os.Getenv("IWINV_LIVE_DBMS_WRITE") != "1" {
		t.Skip("requires IWINV_LIVE_DBMS_WRITE=1 and private IWINV_TEST_JOURNAL_DIR")
	}
	dir := os.Getenv("IWINV_TEST_JOURNAL_DIR")
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute private journal directory required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal("cannot create private journal directory")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatal("real mode-0700 journal directory required")
	}
	f, err := os.CreateTemp(dir, "dbms-write-*.json")
	if err != nil {
		t.Fatal("cannot create journal")
	}
	journal := f.Name()
	if f.Close() != nil {
		t.Fatal("cannot close journal")
	}
	type entry struct {
		ID                 string `json:"created_id"`
		DeleteAttempted    bool   `json:"delete_attempted"`
		DeleteAcknowledged bool   `json:"delete_acknowledged"`
		Absent             bool   `json:"absence_verified"`
	}
	type update struct {
		ID           string   `json:"id"`
		IPs          []string `json:"allowip"`
		Acknowledged bool     `json:"acknowledged"`
		ReadVerified bool     `json:"read_verified"`
	}
	record := struct {
		Intents  []hosted.DBMSInput `json:"intents"`
		Receipts []json.RawMessage  `json:"create_receipts"`
		Created  []*entry           `json:"created"`
		Updates  []*update          `json:"updates"`
	}{Intents: []hosted.DBMSInput{}, Receipts: []json.RawMessage{}, Created: []*entry{}, Updates: []*update{}}
	save := func() error {
		raw, err := json.Marshal(record)
		if err != nil {
			return errors.New("cannot encode DBMS journal")
		}
		stage, err := os.CreateTemp(dir, ".dbms-stage-*")
		if err != nil {
			return errors.New("cannot stage DBMS journal")
		}
		defer os.Remove(stage.Name())
		defer stage.Close()
		if _, err = stage.Write(raw); err == nil {
			err = stage.Sync()
		}
		if err != nil {
			return errors.New("cannot persist DBMS journal")
		}
		if stage.Close() != nil {
			return errors.New("cannot close DBMS stage")
		}
		if os.Rename(stage.Name(), journal) != nil {
			return errors.New("cannot replace DBMS journal")
		}
		return nil
	}
	if err = save(); err != nil {
		t.Fatal(err)
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("valid environment credentials required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	baseline, err := (&hosted.DBMSService{API: c}).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	existing := map[string]bool{}
	for _, r := range baseline {
		existing[r.ID] = true
	}
	captured := &dbmsCaptureClient{Client: c, owned: func(id string) bool {
		if existing[id] {
			return false
		}
		for _, r := range record.Created {
			if r.ID == id {
				return true
			}
		}
		return false
	}, capture: func(e client.Envelope, callErr error) error {
		record.Receipts = append(record.Receipts, e.Result)
		id, _ := hosted.CreateID(e)
		if id != "" && !existing[id] {
			found := false
			for _, r := range record.Created {
				if r.ID == id {
					found = true
				}
			}
			if !found {
				record.Created = append(record.Created, &entry{ID: id})
			}
		}
		return save()
	}}
	s := hosted.DBMSService{API: captured}
	catalog := hosted.DBMSCatalogService{API: captured}
	wait := func(ctx context.Context, id string, absent bool, ips []string) (*hosted.DBMS, error) {
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row, err := s.Read(ctx, id)
			if err != nil {
				return nil, err
			}
			if absent && row == nil {
				return nil, nil
			}
			if !absent && row != nil && row.Status == "active" && (ips == nil || reflect.DeepEqual(row.AllowIPs, ips)) {
				return row, nil
			}
			if row != nil && row.Status != "active" && row.Status != "pending" && row.Status != "waiting" {
				return nil, errors.New("unverified DBMS status; inspect private receipt")
			}
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	cleanupOne := func(ctx context.Context, r *entry) error {
		if existing[r.ID] {
			return errors.New("refusing baseline deletion")
		}
		if !r.DeleteAttempted {
			r.DeleteAttempted = true
			if err := save(); err != nil {
				return err
			}
			err := s.Delete(ctx, r.ID)
			r.DeleteAcknowledged = err == nil
			if saveErr := save(); saveErr != nil {
				return saveErr
			}
			if err != nil {
				return err
			}
		}
		if !r.DeleteAcknowledged {
			return errors.New("DBMS delete remains unacknowledged; no automatic retry")
		}
		if _, err := wait(ctx, r.ID, true, nil); err != nil {
			return err
		}
		r.Absent = true
		return save()
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		for _, r := range record.Created {
			if !r.Absent {
				if err := cleanupOne(cleanupCtx, r); err != nil {
					t.Error("DBMS cleanup incomplete; inspect private journal")
				}
			}
		}
	}()
	all, err := catalog.Products(ctx, "", "")
	if err != nil {
		t.Fatal("DBMS catalog contract failed:", err)
	}
	products, err := catalog.Products(ctx, "STD", "redis")
	if err != nil {
		t.Fatal("filtered DBMS catalog failed:", err)
	}
	counts := map[string]int{}
	for _, p := range all {
		counts[p.ID]++
	}
	product := ""
	for _, p := range products {
		if p.ID != "" && p.Status == "available" && counts[p.ID] == 1 {
			product = p.ID
			break
		}
	}
	if product == "" {
		t.Fatal("no available unambiguous STD Redis product; no writes performed")
	}
	account := func() string {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			t.Fatal("random account generation failed")
		}
		for i := range b {
			b[i] = 'a' + b[i]%26
		}
		return "tf" + string(b)
	}
	randomName := func() string {
		b := make([]byte, 6)
		if _, err := rand.Read(b); err != nil {
			t.Fatal("random name generation failed")
		}
		return "tf-dbms-" + hex.EncodeToString(b)
	}
	for i := 0; i < 2; i++ {
		in := hosted.DBMSInput{ProductID: product, Name: randomName(), Account: account(), AllowIPs: []string{"192.0.2.1"}}
		if i == 0 {
			desc := "한글 설명 &amp; + %"
			in.Description = &desc
		}
		record.Intents = append(record.Intents, in)
		if err = save(); err != nil {
			t.Fatal(err)
		}
		created, err := s.Create(ctx, in)
		if err != nil {
			t.Fatal("DBMS create failed; inspect private journal:", err)
		}
		if created.ID == "" || existing[created.ID] || len(record.Created) != i+1 || record.Created[i].ID != created.ID {
			t.Fatal("creation ownership mismatch")
		}
		row, err := wait(ctx, created.ID, false, in.AllowIPs)
		if err != nil {
			t.Fatal("DBMS readiness not verified; cleanup will run")
		}
		if row.ProductID != in.ProductID || row.Name != in.Name || row.Version == "" || row.Domains["default"] != created.Receipt.Domain {
			t.Fatal("DBMS create/read settings mismatch")
		}
		if in.Description != nil && (row.Description == nil || *row.Description != *in.Description) {
			t.Fatal("literal DBMS description mismatch")
		}
		if in.Description == nil && row.Description != nil && *row.Description != "" {
			t.Fatal("omitted DBMS description unexpectedly changed")
		}
		t.Logf("service %d active; description_is_null=%t", i, row.Description == nil)
	}
	rows, err := s.List(ctx)
	if err != nil || len(rows) != len(baseline)+2 {
		t.Fatal("two-service list does not match owned inventory")
	}
	u := &update{ID: record.Created[0].ID, IPs: []string{"192.0.2.2", "192.0.2.3"}}
	record.Updates = append(record.Updates, u)
	if err = save(); err != nil {
		t.Fatal(err)
	}
	err = s.ReplaceAllowIPs(ctx, u.ID, u.IPs)
	u.Acknowledged = err == nil
	if saveErr := save(); saveErr != nil {
		t.Fatal(saveErr)
	}
	if err != nil {
		t.Fatal("DBMS allowlist update failed; no automatic retry:", err)
	}
	row, err := wait(ctx, u.ID, false, u.IPs)
	if err != nil || row.ID != u.ID {
		t.Fatal("DBMS allowlist read-back failed")
	}
	u.ReadVerified = true
	if err = save(); err != nil {
		t.Fatal(err)
	}
	peer, err := s.Read(ctx, record.Created[1].ID)
	if err != nil || peer == nil || !reflect.DeepEqual(peer.AllowIPs, []string{"192.0.2.1"}) {
		t.Fatal("peer DBMS changed during allowlist replacement")
	}
	if err = cleanupOne(ctx, record.Created[0]); err != nil {
		t.Fatal("first DBMS cleanup failed:", err)
	}
	peer, err = s.Read(ctx, record.Created[1].ID)
	if err != nil || peer == nil || peer.Status != "active" {
		t.Fatal("deleting first DBMS changed the peer")
	}
	if err = cleanupOne(ctx, record.Created[1]); err != nil {
		t.Fatal("second DBMS cleanup failed:", err)
	}
	t.Log("two DBMS services deleted with acknowledgements and exact-ID absence")
}
