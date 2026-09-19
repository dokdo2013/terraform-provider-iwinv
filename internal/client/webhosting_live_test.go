package client_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
)

type hostingCaptureClient struct {
	*client.Client
	capture func(client.Envelope, error) error
}

func (c *hostingCaptureClient) PostJSON(ctx context.Context, path string, body any) (client.Envelope, error) {
	e, err := c.Client.PostJSON(ctx, path, body)
	if captureErr := c.capture(e, err); captureErr != nil {
		return client.Envelope{}, captureErr
	}
	return e, err
}

// TestAccWebhostingControlPlaneWrites uses two newly created services only.
// It is an opt-in adapter contract test, NOT Terraform resource acceptance.
// No data is uploaded and no DNS, HTTP/FTP/database or billing claim is made.
func TestAccWebhostingControlPlaneWrites(t *testing.T) {
	if os.Getenv("IWINV_LIVE_WEBHOSTING_WRITE") != "1" {
		t.Skip("requires IWINV_LIVE_WEBHOSTING_WRITE=1 and private IWINV_TEST_JOURNAL_DIR")
	}
	dir := os.Getenv("IWINV_TEST_JOURNAL_DIR")
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute private journal directory required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal("cannot create journal directory")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatal("real mode-0700 journal directory required")
	}
	f, err := os.CreateTemp(dir, "webhosting-write-*.json")
	if err != nil {
		t.Fatal("cannot create private journal")
	}
	journal := f.Name()
	if err = f.Close(); err != nil {
		t.Fatal("cannot close private journal")
	}
	type entry struct {
		ID                 string `json:"created_id"`
		DeleteAttempted    bool   `json:"delete_attempted"`
		DeleteAcknowledged bool   `json:"delete_acknowledged"`
		Absent             bool   `json:"absence_verified"`
	}
	record := struct {
		Intents  []hosted.WebhostingInput `json:"intents"`
		Receipts []json.RawMessage        `json:"create_receipts"`
		Created  []*entry                 `json:"created"`
	}{Intents: []hosted.WebhostingInput{}, Receipts: []json.RawMessage{}, Created: []*entry{}}
	save := func() error {
		data, err := json.Marshal(record)
		if err != nil {
			return errors.New("cannot encode hosting journal")
		}
		f, err := os.CreateTemp(dir, ".webhosting-stage-*")
		if err != nil {
			return errors.New("cannot stage hosting journal")
		}
		defer os.Remove(f.Name())
		defer f.Close()
		if _, err = f.Write(data); err == nil {
			err = f.Sync()
		}
		if err != nil {
			return errors.New("cannot persist hosting journal")
		}
		if err = f.Close(); err != nil {
			return errors.New("cannot close hosting journal")
		}
		if err = os.Rename(f.Name(), journal); err != nil {
			return errors.New("cannot replace hosting journal")
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
	plain := hosted.WebhostingService{API: c}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	baseline, err := plain.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	existing := map[string]bool{}
	for _, r := range baseline {
		existing[r.ID] = true
	}
	captured := &hostingCaptureClient{Client: c, capture: func(e client.Envelope, callErr error) error {
		record.Receipts = append(record.Receipts, e.Result)
		if callErr != nil {
			t.Log("sanitized create error:", callErr)
		}
		// Extract identity before full adapter decoding, also for unexpected 201/202.
		id, _ := hosted.CreateID(e)
		if id != "" && !existing[id] {
			duplicate := false
			for _, r := range record.Created {
				if r.ID == id {
					duplicate = true
				}
			}
			if !duplicate {
				record.Created = append(record.Created, &entry{ID: id})
			}
		}
		return save()
	}}
	s := hosted.WebhostingService{API: captured}
	wait := func(ctx context.Context, id string, absence bool) (*hosted.Webhosting, error) {
		for {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row, err := s.Read(ctx, id)
			if err != nil {
				return nil, err
			}
			if absence && row == nil || !absence && row != nil && row.Status == "active" {
				return row, nil
			}
			if row != nil && row.Status != "active" && row.Status != "pending" {
				return nil, errors.New("unverified hosting status; inspect private receipt")
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
			return errors.New("refusing to delete a baseline service")
		}
		if !r.DeleteAttempted {
			r.DeleteAttempted = true
			if err := save(); err != nil {
				return err
			}
			deleteErr := s.Delete(ctx, r.ID)
			r.DeleteAcknowledged = deleteErr == nil
			if err := save(); err != nil {
				return err
			}
			if deleteErr != nil {
				return deleteErr
			}
		}
		// Never turn an earlier failed/uncertain DELETE into an acknowledged result.
		if !r.DeleteAcknowledged {
			return errors.New("hosting delete remains unacknowledged; no automatic retry")
		}
		if _, err := wait(ctx, r.ID, true); err != nil {
			return err
		}
		r.Absent = true
		return save()
	}
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()
		for _, r := range record.Created {
			if !r.Absent {
				if err := cleanupOne(cleanupCtx, r); err != nil {
					t.Error("hosting cleanup incomplete; inspect private journal")
				}
			}
		}
	}()
	products, err := s.Products(ctx, "SHARE")
	if err != nil {
		t.Fatal(err)
	}
	allProducts, err := s.Products(ctx, "")
	if err != nil || len(allProducts) < len(products) {
		t.Fatal("product catalog filter contract failed")
	}
	singleProducts, err := s.Products(ctx, "SINGLE")
	if err != nil || len(allProducts) != len(products)+len(singleProducts) {
		t.Fatal("full product catalog does not match disjoint type filters")
	}
	var product string
	for _, p := range products {
		if p.Status == "available" && p.AllowCustomDomain && p.MaxDomainCount >= 1 {
			product = p.ID
			break
		}
	}
	if product == "" {
		t.Fatal("no available shared product with custom domains")
	}
	servers, err := s.Servers(ctx, product)
	if err != nil {
		t.Fatal(err)
	}
	var server string
	for _, v := range servers {
		if v.PHPVersion == "PHP 8.4" {
			server = v.ID
			break
		}
	}
	if server == "" {
		t.Fatal("PHP 8.4 server choice unavailable; review catalog before writes")
	}
	randomHex := func() string {
		b := make([]byte, 7)
		if _, err := rand.Read(b); err != nil {
			t.Fatal("random generation failed")
		}
		return hex.EncodeToString(b)
	}
	newAccount := func() string {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			t.Fatal("random generation failed")
		}
		for i := range b {
			b[i] = 'a' + b[i]%26
		}
		return "tf" + string(b)
	}
	for i := 0; i < 2; i++ {
		in := hosted.WebhostingInput{ProductID: product, ServerID: server, Name: "tf-hosting-" + randomHex(), Account: newAccount(), FTPPassword: "Aa1!" + randomHex(), DatabasePassword: "Bb2!" + randomHex(), WebFirewall: i == 0}
		if i == 1 {
			description := "한글 설명 &amp; + %"
			in.Description = &description
			in.Domains = map[string]string{"tf-" + randomHex() + ".invalid": "/"}
		}
		record.Intents = append(record.Intents, in)
		if err = save(); err != nil {
			t.Fatal(err)
		}
		created, err := s.Create(ctx, in)
		if err != nil {
			t.Fatal("hosting create failed; inspect private receipt and cleanup journal:", err)
		}
		if created.ID == "" || existing[created.ID] || len(record.Created) != i+1 || record.Created[i].ID != created.ID {
			t.Fatal("creation identity ownership check failed")
		}
		row, err := wait(ctx, created.ID, false)
		if err != nil {
			t.Fatal("hosting readiness not verified; cleanup will run")
		}
		if row.Name != in.Name || row.Account != in.Account || row.ProductID != in.ProductID || row.WebFirewall != in.WebFirewall {
			t.Fatal("hosting settings did not roundtrip")
		}
		if in.Description == nil {
			if row.Description == nil || *row.Description != "" {
				t.Fatal("omitted description did not read as empty string")
			}
		} else if row.Description == nil || *row.Description != *in.Description {
			t.Fatal("literal description did not roundtrip")
		}
		for domain, folder := range in.Domains {
			if row.Domains[domain] != folder {
				t.Fatal("custom domain mapping did not roundtrip")
			}
		}
		t.Logf("service %d active; description_is_null=%t; custom_domain_count=%d", i, row.Description == nil, len(in.Domains))
	}
	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.ID] = true
	}
	for id := range existing {
		if !seen[id] {
			t.Fatal("baseline service disappeared from complete list")
		}
	}
	for _, r := range record.Created {
		if !seen[r.ID] {
			t.Fatal("new service absent from shared list")
		}
	}
	if err = cleanupOne(ctx, record.Created[0]); err != nil {
		t.Fatal("first hosting deletion unverified; inspect private journal")
	}
	peer, err := s.Read(ctx, record.Created[1].ID)
	if err != nil || peer == nil || peer.Status != "active" {
		t.Fatal("individual deletion did not preserve the other service")
	}
	if err = cleanupOne(ctx, record.Created[1]); err != nil {
		t.Fatal("second hosting deletion unverified; inspect private journal")
	}
	rows, err = s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen = map[string]bool{}
	for _, r := range rows {
		seen[r.ID] = true
	}
	for id := range existing {
		if !seen[id] {
			t.Fatal("baseline service missing after test cleanup")
		}
	}
	t.Log("two work-created hosting services acknowledged deleted and absent; no billing or data-plane claim")
}
