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

type cacheCaptureClient struct {
	*client.Client
	capture func(client.Envelope, error) error
	owned   func(string) bool
}

func (c *cacheCaptureClient) PostJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	if p != "/v1/cache" {
		return client.Envelope{}, errors.New("unexpected Cache create path")
	}
	e, err := c.Client.PostJSON(ctx, p, body)
	if captureErr := c.capture(e, err); captureErr != nil {
		return client.Envelope{}, captureErr
	}
	return e, err
}
func (c *cacheCaptureClient) PutJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/cache/"), "/allow_referer")
	if p != "/v1/cache/"+id+"/allow_referer" || !c.owned(id) {
		return client.Envelope{}, errors.New("refusing non-owned Cache update")
	}
	return c.Client.PutJSON(ctx, p, body)
}
func (c *cacheCaptureClient) Delete(ctx context.Context, p string) (client.Envelope, error) {
	id := strings.TrimPrefix(p, "/v1/cache/")
	if p != "/v1/cache/"+id || !c.owned(id) {
		return client.Envelope{}, errors.New("refusing non-owned Cache deletion")
	}
	return c.Client.Delete(ctx, p)
}
func (c *cacheCaptureClient) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	if p != "/v1/cache" && p != "/v1/cache/products" {
		return client.Envelope{}, errors.New("unexpected Cache read path")
	}
	return c.Client.Get(ctx, p, q)
}

// TestAccCacheControlPlaneWrites only owns newly returned IDs, never baseline
// resources. It proves control-plane contracts, not Terraform, content delivery or FTP access.
func TestAccCacheControlPlaneWrites(t *testing.T) {
	if os.Getenv("IWINV_LIVE_CACHE_WRITE") != "1" {
		t.Skip("requires IWINV_LIVE_CACHE_WRITE=1 and private IWINV_TEST_JOURNAL_DIR")
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
	f, err := os.CreateTemp(dir, "cache-write-*.json")
	if err != nil {
		t.Fatal("cannot create journal")
	}
	journal := f.Name()
	if f.Close() != nil {
		t.Fatal("cannot close journal")
	}
	type entry struct {
		ID                       string `json:"created_id"`
		DeleteAttempted          bool   `json:"delete_attempted"`
		DeleteAcknowledged       bool   `json:"delete_acknowledged"`
		Absent                   bool   `json:"absence_verified"`
		DeleteAttempts           int    `json:"delete_attempts"`
		DeleteBusyReadsUnchanged int    `json:"delete_busy_reads_unchanged"`
	}
	type update struct {
		ID                 string   `json:"id"`
		Referrers          []string `json:"allow_referer"`
		Acknowledged       bool     `json:"acknowledged"`
		ReadVerified       bool     `json:"read_verified"`
		Attempts           int      `json:"attempts"`
		BusyReadsUnchanged int      `json:"busy_reads_unchanged"`
	}
	record := struct {
		Intents  []hosted.CacheInput `json:"intents"`
		Receipts []json.RawMessage   `json:"create_receipts"`
		Created  []*entry            `json:"created"`
		Updates  []*update           `json:"updates"`
	}{Intents: []hosted.CacheInput{}, Receipts: []json.RawMessage{}, Created: []*entry{}, Updates: []*update{}}
	save := func() error {
		raw, err := json.Marshal(record)
		if err != nil {
			return errors.New("cannot encode Cache journal")
		}
		stage, err := os.CreateTemp(dir, ".cache-stage-*")
		if err != nil {
			return errors.New("cannot stage Cache journal")
		}
		defer os.Remove(stage.Name())
		defer stage.Close()
		if _, err = stage.Write(raw); err == nil {
			err = stage.Sync()
		}
		if err != nil {
			return errors.New("cannot persist Cache journal")
		}
		if stage.Close() != nil {
			return errors.New("cannot close Cache stage")
		}
		if os.Rename(stage.Name(), journal) != nil {
			return errors.New("cannot replace Cache journal")
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
	baseline, err := (&hosted.CacheService{API: c}).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	existing := map[string]bool{}
	for _, r := range baseline {
		existing[r.ID] = true
	}
	captured := &cacheCaptureClient{Client: c, owned: func(id string) bool {
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
	s := hosted.CacheService{API: captured}
	catalog := hosted.CacheCatalogService{API: captured}
	wait := func(ctx context.Context, id string, absent bool, refs []string) (*hosted.Cache, error) {
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
			if !absent && row != nil && row.Status == "active" && (refs == nil || reflect.DeepEqual(row.Referrers, refs)) {
				return row, nil
			}
			if row != nil && row.Status != "active" && row.Status != "pending" && row.Status != "waiting" {
				return nil, errors.New("unverified Cache status; inspect private receipt")
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
			before, readErr := s.Read(ctx, r.ID)
			if readErr != nil || before == nil {
				return errors.New("cache missing before acknowledged cleanup")
			}
			r.DeleteAttempted = true
			for {
				r.DeleteAttempts++
				if err := save(); err != nil {
					return err
				}
				err := s.Delete(ctx, r.ID)
				if err == nil {
					r.DeleteAcknowledged = true
					if err = save(); err != nil {
						return err
					}
					break
				}
				var apiErr *client.Error
				if !errors.As(err, &apiErr) || apiErr.Kind != "cache_delete_busy" {
					return err
				}
				after, readErr := s.Read(ctx, r.ID)
				if readErr != nil || !reflect.DeepEqual(after, before) {
					return errors.New("busy deletion did not preserve the owned parent")
				}
				r.DeleteBusyReadsUnchanged++
				if err = save(); err != nil {
					return err
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(2 * time.Second):
				}
			}
		}

		if !r.DeleteAcknowledged {
			return errors.New("Cache delete remains unacknowledged; no automatic retry")
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
					t.Error("Cache cleanup incomplete; inspect private journal")
				}
			}
		}
	}()
	all, err := catalog.Products(ctx, "")
	if err != nil {
		t.Fatal("Cache catalog contract failed:", err)
	}
	products, err := catalog.Products(ctx, "SHARE")
	if err != nil {
		t.Fatal("filtered Cache catalog failed:", err)
	}
	counts := map[string]int{}
	for _, p := range all {
		if p.ID != nil {
			counts[*p.ID]++
		}
	}
	product := ""
	for _, p := range products {
		if p.ID != nil && *p.ID == "cache_lite" && p.Status == "available" && counts[*p.ID] == 1 {
			product = *p.ID
			break
		}
	}
	if product == "" {
		t.Fatal("no available unambiguous SHARE cache_lite product; no writes performed")
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
		return "tf-cache-" + hex.EncodeToString(b)
	}
	randomPassword := func() string {
		b := make([]byte, 6)
		if _, err := rand.Read(b); err != nil {
			t.Fatal("password generation failed")
		}
		return hex.EncodeToString(b)
	}
	for i := 0; i < 2; i++ {
		in := hosted.CacheInput{ProductID: product, Name: randomName(), Account: account(), FTPPassword: "Aa1!" + randomPassword()}
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
			t.Fatal("Cache create failed; inspect private journal:", err)
		}
		if created.ID == "" || existing[created.ID] || len(record.Created) != i+1 || record.Created[i].ID != created.ID {
			t.Fatal("creation ownership mismatch")
		}
		row, err := wait(ctx, created.ID, false, []string{})
		if err != nil {
			t.Fatal("Cache readiness not verified; cleanup will run")
		}
		if row.ProductID != in.ProductID || row.Name != in.Name || row.Account != in.Account || row.Domain != created.Service.Domain {
			t.Fatal("Cache create/read settings mismatch")
		}
		if in.Description != nil && (row.Description == nil || *row.Description != *in.Description) {
			t.Fatal("literal Cache description mismatch")
		}
		if in.Description == nil && row.Description != nil && *row.Description != "" {
			t.Fatal("omitted Cache description unexpectedly changed")
		}
		t.Logf("service %d active; description_is_null=%t", i, row.Description == nil)
	}
	rows, err := s.List(ctx)
	if err != nil || len(rows) != len(baseline)+2 {
		t.Fatal("two-service list does not match owned inventory")
	}

	peerBefore, err := s.Read(ctx, record.Created[1].ID)
	if err != nil || peerBefore == nil {
		t.Fatal("peer read failed")
	}
	for _, refs := range [][]string{{"first.example.invalid"}, {"second.example.invalid", "third.example.invalid"}} {
		u := &update{ID: record.Created[0].ID, Referrers: refs}
		record.Updates = append(record.Updates, u)
		before, err := s.Read(ctx, u.ID)
		if err != nil || before == nil {
			t.Fatal("owned cache disappeared before referrer update")
		}
		opCtx, stop := context.WithTimeout(ctx, 2*time.Minute)
		for {
			u.Attempts++
			if err = save(); err != nil {
				stop()
				t.Fatal(err)
			}
			err = s.ReplaceReferrers(opCtx, u.ID, u.Referrers)
			if err == nil {
				u.Acknowledged = true
				if err = save(); err != nil {
					stop()
					t.Fatal(err)
				}
				break
			}
			var apiErr *client.Error
			if !errors.As(err, &apiErr) || apiErr.Kind != "cache_referrers_busy" {
				stop()
				t.Fatal("cache write failed without a verified busy rejection; not retried:", err)
			}
			// This experiment retries only a known rejected request after proving the
			// target still exists and retains the complete previous referrer set.
			after, readErr := s.Read(opCtx, u.ID)
			if readErr != nil || after == nil || !reflect.DeepEqual(after.Referrers, before.Referrers) {
				stop()
				t.Fatal("busy rejection did not preserve the previous referrer set")
			}
			u.BusyReadsUnchanged++
			if err = save(); err != nil {
				stop()
				t.Fatal(err)
			}
			select {
			case <-opCtx.Done():
				stop()
				t.Fatal("cache busy window exceeded test bound")
			case <-time.After(2 * time.Second):
			}
		}
		row, err := wait(opCtx, u.ID, false, u.Referrers)
		stop()
		if err != nil || row == nil {
			t.Fatal("cache referrer read-back failed")
		}
		u.ReadVerified = true
		if err = save(); err != nil {
			t.Fatal(err)
		}
		peer, err := s.Read(ctx, record.Created[1].ID)
		if err != nil || !reflect.DeepEqual(peer, peerBefore) {
			t.Fatal("peer cache changed during referrer replacement")
		}
	}
	if err = cleanupOne(ctx, record.Created[0]); err != nil {
		t.Fatal("first Cache cleanup failed:", err)
	}
	peer, err := s.Read(ctx, record.Created[1].ID)
	if err != nil || peer == nil || peer.Status != "active" {
		t.Fatal("deleting first Cache changed the peer")
	}
	if err = cleanupOne(ctx, record.Created[1]); err != nil {
		t.Fatal("second Cache cleanup failed:", err)
	}
	t.Log("two cache services deleted with acknowledgements and exact-ID absence")
	for i, r := range record.Created {
		t.Logf("delete %d attempts=%d busy_unchanged=%d acknowledged=%t absent=%t", i, r.DeleteAttempts, r.DeleteBusyReadsUnchanged, r.DeleteAcknowledged, r.Absent)
	}
	for i, u := range record.Updates {
		t.Logf("update %d attempts=%d busy_unchanged=%d read_verified=%t", i, u.Attempts, u.BusyReadsUnchanged, u.ReadVerified)
	}
}
