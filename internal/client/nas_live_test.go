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

type nasCaptureClient struct {
	*client.Client
	capture func(client.Envelope, error) error
	owned   func(string) bool
}

func (c *nasCaptureClient) PostJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	if p != "/v1/apinas" {
		return client.Envelope{}, errors.New("unexpected NAS create path")
	}
	e, err := c.Client.PostJSON(ctx, p, body)
	if captureErr := c.capture(e, err); captureErr != nil {
		return client.Envelope{}, captureErr
	}
	return e, err
}
func (c *nasCaptureClient) PutJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/apinas/"), "/allowip")
	if p != "/v1/apinas/"+id+"/allowip" || !c.owned(id) {
		return client.Envelope{}, errors.New("refusing non-owned NAS update")
	}
	return c.Client.PutJSON(ctx, p, body)
}
func (c *nasCaptureClient) Delete(ctx context.Context, p string) (client.Envelope, error) {
	id := strings.TrimPrefix(p, "/v1/apinas/")
	if p != "/v1/apinas/"+id || !c.owned(id) {
		return client.Envelope{}, errors.New("refusing non-owned NAS deletion")
	}
	return c.Client.Delete(ctx, p)
}
func (c *nasCaptureClient) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	if p != "/v1/apinas" && p != "/v1/apinas/products" {
		return client.Envelope{}, errors.New("unexpected NAS read path")
	}
	return c.Client.Get(ctx, p, q)
}

// TestAccNASControlPlaneWrites only owns newly returned IDs, never baseline
// resources. It proves control-plane contracts, not Terraform, NFS mounts or tenant file access.
func TestAccNASControlPlaneWrites(t *testing.T) {
	if os.Getenv("IWINV_LIVE_NAS_WRITE") != "1" {
		t.Skip("requires IWINV_LIVE_NAS_WRITE=1 and private IWINV_TEST_JOURNAL_DIR")
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
	f, err := os.CreateTemp(dir, "nas-write-*.json")
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
		ID           string            `json:"id"`
		IPs          map[string]string `json:"allowip"`
		Acknowledged bool              `json:"acknowledged"`
		ReadVerified bool              `json:"read_verified"`
	}
	record := struct {
		Intents  []hosted.NASInput `json:"intents"`
		Receipts []json.RawMessage `json:"create_receipts"`
		Created  []*entry          `json:"created"`
		Updates  []*update         `json:"updates"`
	}{Intents: []hosted.NASInput{}, Receipts: []json.RawMessage{}, Created: []*entry{}, Updates: []*update{}}
	save := func() error {
		raw, err := json.Marshal(record)
		if err != nil {
			return errors.New("cannot encode NAS journal")
		}
		stage, err := os.CreateTemp(dir, ".nas-stage-*")
		if err != nil {
			return errors.New("cannot stage NAS journal")
		}
		defer os.Remove(stage.Name())
		defer stage.Close()
		if _, err = stage.Write(raw); err == nil {
			err = stage.Sync()
		}
		if err != nil {
			return errors.New("cannot persist NAS journal")
		}
		if stage.Close() != nil {
			return errors.New("cannot close NAS stage")
		}
		if os.Rename(stage.Name(), journal) != nil {
			return errors.New("cannot replace NAS journal")
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
	baseline, err := (&hosted.NASService{API: c}).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	existing := map[string]bool{}
	for _, r := range baseline {
		existing[r.ID] = true
	}
	captured := &nasCaptureClient{Client: c, owned: func(id string) bool {
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
	s := hosted.NASService{API: captured}
	catalog := hosted.NASCatalogService{API: captured}
	wait := func(ctx context.Context, id string, absent bool, ips map[string]string) (*hosted.NAS, error) {
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
				return nil, errors.New("unverified NAS status; inspect private receipt")
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
			return errors.New("NAS delete remains unacknowledged; no automatic retry")
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
					t.Error("NAS cleanup incomplete; inspect private journal")
				}
			}
		}
	}()
	products, err := catalog.Products(ctx)
	if err != nil {
		t.Fatal("NAS catalog contract failed", err)
	}
	product := ""
	var disk int64
	matches := 0
	for _, p := range products {
		if p.ID == "api_nas" && p.Status == "available" {
			matches++
			product = p.ID
			disk = p.MinimumDiskGB
		}
	}
	if matches != 1 || disk < 100 || disk > 2000 {
		t.Fatal("no verified NAS product/minimum capacity; no writes")
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
		return "tf-nas-" + hex.EncodeToString(b)
	}
	for i := 0; i < 2; i++ {
		in := hosted.NASInput{ProductID: product, Name: randomName(), ShareName: account(), DiskGB: disk, AllowIPs: map[string]string{"192.0.2.1": "RO"}}
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
			t.Fatal("NAS create failed; inspect private journal:", err)
		}
		if created.ID == "" || existing[created.ID] || len(record.Created) != i+1 || record.Created[i].ID != created.ID {
			t.Fatal("creation ownership mismatch")
		}
		row, err := wait(ctx, created.ID, false, in.AllowIPs)
		if err != nil {
			t.Fatal("NAS readiness not verified; cleanup will run")
		}
		if row.ProductID != in.ProductID || row.Name != in.Name || row.DiskGB != disk || row.Domain != created.Service.Domain || row.MountInfo == "" {
			t.Fatal("NAS create/read settings mismatch")
		}
		if in.Description != nil && (row.Description == nil || *row.Description != *in.Description) {
			t.Fatal("literal NAS description mismatch")
		}
		if in.Description == nil && row.Description != nil && *row.Description != "" {
			t.Fatal("omitted NAS description unexpectedly changed")
		}
		t.Logf("service %d active; description_is_null=%t", i, row.Description == nil)
	}
	rows, err := s.List(ctx)
	if err != nil || len(rows) != len(baseline)+2 {
		t.Fatal("two-service list does not match owned inventory")
	}
	u := &update{ID: record.Created[0].ID, IPs: map[string]string{"192.0.2.2": "RW", "192.0.2.3": "RO"}}
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
		t.Fatal("NAS allowlist update failed; no automatic retry:", err)
	}
	row, err := wait(ctx, u.ID, false, u.IPs)
	if err != nil || row.ID != u.ID {
		t.Fatal("NAS allowlist read-back failed")
	}
	u.ReadVerified = true
	if err = save(); err != nil {
		t.Fatal(err)
	}
	peer, err := s.Read(ctx, record.Created[1].ID)
	if err != nil || peer == nil || !reflect.DeepEqual(peer.AllowIPs, map[string]string{"192.0.2.1": "RO"}) {
		t.Fatal("peer NAS changed during allowlist replacement")
	}
	// Replace the same host's permission and remove the other host: this is
	// authoritative map ownership, not append-only membership.
	second := &update{ID: record.Created[0].ID, IPs: map[string]string{"192.0.2.2": "RO"}}
	record.Updates = append(record.Updates, second)
	if err = save(); err != nil {
		t.Fatal(err)
	}
	err = s.ReplaceAllowIPs(ctx, second.ID, second.IPs)
	second.Acknowledged = err == nil
	if saveErr := save(); saveErr != nil {
		t.Fatal(saveErr)
	}
	if err != nil {
		t.Fatal("NAS permission update failed; no retry", err)
	}
	if _, err = wait(ctx, second.ID, false, second.IPs); err != nil {
		t.Fatal("NAS permission update read-back failed")
	}
	second.ReadVerified = true
	if err = save(); err != nil {
		t.Fatal(err)
	}
	if err = cleanupOne(ctx, record.Created[0]); err != nil {
		t.Fatal("first NAS cleanup failed:", err)
	}
	peer, err = s.Read(ctx, record.Created[1].ID)
	if err != nil || peer == nil || peer.Status != "active" {
		t.Fatal("deleting first NAS changed the peer")
	}
	if err = cleanupOne(ctx, record.Created[1]); err != nil {
		t.Fatal("second NAS cleanup failed:", err)
	}
	t.Log("two NAS services deleted with acknowledgements and exact-ID absence")
}
