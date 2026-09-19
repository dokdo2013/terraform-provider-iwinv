package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type ownedCacheRecord struct {
	ID                 string `json:"id"`
	RetryDelete        bool   `json:"retry_delete"`
	DeleteAttempted    bool   `json:"delete_attempted"`
	DeleteAcknowledged bool   `json:"delete_acknowledged"`
	Absent             bool   `json:"absence_verified"`
}
type ownedCacheAPI struct {
	*client.Client
	mu                                 sync.Mutex
	journal                            string
	baseline                           map[string]bool
	allowedAccounts, attemptedAccounts map[string]bool
	records                            []*ownedCacheRecord
	intents                            []map[string]any
	receipts                           []json.RawMessage
}

func (a *ownedCacheAPI) saveLocked() error {
	raw, err := json.Marshal(struct {
		Records  []*ownedCacheRecord `json:"created"`
		Intents  []map[string]any    `json:"intents"`
		Receipts []json.RawMessage   `json:"create_receipts"`
	}{a.records, a.intents, a.receipts})
	if err != nil {
		return errors.New("cannot encode Cache ownership journal")
	}
	f, err := os.CreateTemp(filepath.Dir(a.journal), ".Cache-stage-*")
	if err != nil {
		return errors.New("cannot stage ownership journal")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	if err != nil {
		return errors.New("cannot persist Cache journal")
	}
	if err = f.Close(); err != nil {
		return errors.New("cannot close Cache journal")
	}
	if err = os.Rename(f.Name(), a.journal); err != nil {
		return errors.New("cannot replace Cache journal")
	}
	return nil
}
func (a *ownedCacheAPI) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	if p != "/v1/cache" && p != "/v1/cache/products" {
		return client.Envelope{}, errors.New("read outside Cache test scope")
	}
	return a.Client.Get(ctx, p, q)
}
func (a *ownedCacheAPI) PostJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := body.(map[string]any)
	if !ok || p != "/v1/cache" {
		return client.Envelope{}, errors.New("create outside Cache test scope")
	}
	account, ok := b["id"].(string)
	if !ok || !a.allowedAccounts[account] || a.attemptedAccounts[account] {
		return client.Envelope{}, errors.New("unowned or repeated Cache account creation refused")
	}
	a.attemptedAccounts[account] = true
	intent := map[string]any{}
	for k, v := range b {
		if k != "pw" {
			intent[k] = v
		}
	}
	a.intents = append(a.intents, intent)
	if err := a.saveLocked(); err != nil {
		return client.Envelope{}, err
	}
	e, err := a.Client.PostJSON(ctx, p, body)
	id, _ := hosted.CreateID(e)
	receipt, _ := json.Marshal(map[string]string{"service_idx": id})
	a.receipts = append(a.receipts, receipt)
	if id != "" && !a.baseline[id] {
		duplicate := false
		for _, r := range a.records {
			if r.ID == id {
				duplicate = true
			}
		}
		if !duplicate {
			a.records = append(a.records, &ownedCacheRecord{ID: id})
		}
	}
	if saveErr := a.saveLocked(); saveErr != nil {
		return client.Envelope{}, saveErr
	}
	return e, err
}
func (a *ownedCacheAPI) Delete(ctx context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := strings.TrimPrefix(p, "/v1/cache/")
	var record *ownedCacheRecord
	for _, r := range a.records {
		if r.ID == id {
			record = r
		}
	}
	if record == nil || a.baseline[id] || p != "/v1/cache/"+id || (record.DeleteAttempted && !record.RetryDelete) {
		return client.Envelope{}, errors.New("unowned or repeated Cache delete refused")
	}
	record.DeleteAttempted = true
	record.RetryDelete = false
	a.intents = append(a.intents, map[string]any{"operation": "DELETE", "service_id": id})
	if err := a.saveLocked(); err != nil {
		return client.Envelope{}, err
	}
	e, err := a.Client.Delete(ctx, p)
	record.RetryDelete = cacheBusy(err, "cache_delete_busy")
	a.intents[len(a.intents)-1]["busy_rejected"] = record.RetryDelete
	var ack string
	record.DeleteAcknowledged = err == nil && e.Status == 200 && json.Unmarshal(e.Result, &ack) == nil && ack != "" && len(e.Count) == 0 && len(e.Page) == 0 && len(e.PageNo) == 0 && len(e.PageSize) == 0 && len(e.Total) == 0
	if saveErr := a.saveLocked(); saveErr != nil {
		return client.Envelope{}, saveErr
	}
	return e, err
}
func (a *ownedCacheAPI) markAbsent(ctx context.Context, id string) error {
	s := hosted.CacheService{API: a}
	row, err := s.Read(ctx, id)
	if err != nil {
		return err
	}
	if row != nil {
		return errors.New("owned Cache service still present")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.records {
		if r.ID == id {
			if !r.DeleteAcknowledged {
				return errors.New("Cache deletion never acknowledged")
			}
			r.Absent = true
			return a.saveLocked()
		}
	}
	return errors.New("unknown Cache cleanup identity")
}
func (a *ownedCacheAPI) cleanup(ctx context.Context) error {
	a.mu.Lock()
	ids := []string{}
	for _, r := range a.records {
		if !r.Absent {
			ids = append(ids, r.ID)
		}
	}
	a.mu.Unlock()
	s := hosted.CacheService{API: a}
	var failures []error
	for _, id := range ids {
		a.mu.Lock()
		attempted, ack := false, false
		for _, r := range a.records {
			if r.ID == id {
				attempted = r.DeleteAttempted && !r.RetryDelete
				ack = r.DeleteAcknowledged
			}
		}
		a.mu.Unlock()
		if !attempted {
			row, readErr := s.Read(ctx, id)
			if readErr != nil {
				failures = append(failures, errors.New("cleanup read failed"))
				continue
			}
			r := &contentCacheResource{service: &s, pollInterval: 2 * time.Second}
			if err := r.write(ctx, row, id, nil, true); err != nil {
				failures = append(failures, errors.New("owned Cache delete failed"))
				continue
			}
			ack = true
		}
		if !ack {
			failures = append(failures, errors.New("unacknowledged Cache delete; no replay"))
			continue
		}
		var err error
		for attempt := 0; attempt < 20; attempt++ {
			err = a.markAbsent(ctx, id)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				break
			}
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
			case <-timer.C:
			}
		}
		if err != nil {
			failures = append(failures, errors.New("Cache absence unverified"))
		}
	}
	return errors.Join(failures...)
}

func (a *ownedCacheAPI) PutJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/cache/"), "/allow_referer")
	owned := false
	for _, r := range a.records {
		if r.ID == id && !r.DeleteAttempted {
			owned = true
		}
	}
	if !owned || a.baseline[id] || p != "/v1/cache/"+id+"/allow_referer" {
		return client.Envelope{}, errors.New("unowned Cache update refused")
	}
	b, ok := body.(map[string]any)
	if !ok {
		return client.Envelope{}, errors.New("invalid Cache update body")
	}
	a.intents = append(a.intents, map[string]any{"operation": "PUT", "service_id": id, "allow_referer": b["allow_referer"]})
	if err := a.saveLocked(); err != nil {
		return client.Envelope{}, err
	}
	e, err := a.Client.PutJSON(ctx, p, body)
	a.intents[len(a.intents)-1]["busy_rejected"] = cacheBusy(err, "cache_referrers_busy")
	if saveErr := a.saveLocked(); saveErr != nil {
		return client.Envelope{}, saveErr
	}
	return e, err
}

// TestAccContentCache manages five fresh identities; it never uses a pre-existing
// service or accesses FTP/content. Journals intentionally exclude passwords.
func TestAccContentCache(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_TERRAFORM_CACHE_WRITE") != "1" {
		t.Skip("requires explicit live cache gates and private journal directory")
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
	f, err := os.CreateTemp(dir, "terraform-cache-*.json")
	if err != nil {
		t.Fatal("cannot create journal")
	}
	journal := f.Name()
	if f.Close() != nil {
		t.Fatal("cannot close journal")
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("valid environment credentials required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	plain := &hosted.CacheService{API: c}
	rows, err := plain.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	baseline := map[string]bool{}
	for _, r := range rows {
		baseline[r.ID] = true
	}
	random := func() string {
		b := make([]byte, 5)
		if _, e := rand.Read(b); e != nil {
			t.Fatal("random generation failed")
		}
		return hex.EncodeToString(b)
	}
	accounts := []string{"tf" + random(), "tf" + random(), "tf" + random(), "tf" + random(), "tf" + random()}
	allowed := map[string]bool{}
	for _, v := range accounts {
		if allowed[v] {
			t.Fatal("random collision")
		}
		allowed[v] = true
	}
	a := &ownedCacheAPI{Client: c, journal: journal, baseline: baseline, allowedAccounts: allowed, attemptedAccounts: map[string]bool{}, records: []*ownedCacheRecord{}, intents: []map[string]any{}, receipts: []json.RawMessage{}}
	if err = a.saveLocked(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 3*time.Minute)
		defer done()
		if e := a.cleanup(cleanupCtx); e != nil {
			t.Error("cache cleanup incomplete; inspect private journal")
		}
	}()
	products, err := (&hosted.CacheCatalogService{API: c}).Products(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, p := range products {
		if p.ID != nil && *p.ID == "cache_lite" && p.Status == "available" {
			matches++
		}
	}
	if matches != 1 {
		t.Fatal("verified cache_lite product unavailable or ambiguous")
	}
	ftp := "Aa1!" + random()
	t.Setenv("TF_VAR_cache_ftp", ftp)
	work := t.TempDir()
	block := func(address, account, refs, extra string, credentials bool) string {
		secret := ""
		if credentials {
			secret = "ftp_password_wo = var.cache_ftp\npassword_wo_version = 1\n"
		}
		return fmt.Sprintf("resource \"iwinv_content_cache\" %q {\nproduct_id = \"cache_lite\"\naccount_name = %q\nname = \"tf-cache-live-%s\"\nallowed_referrers = %s\n%s\n%s\n}\n", address, account, address, refs, secret, extra)
	}
	peer := block("peer", accounts[1], `[]`, "", true)
	cfg := func(i int, refs, extra string, credentials bool) string {
		return cacheVariables + block("test", accounts[i], refs, extra, credentials) + peer
	}
	initial := cfg(0, `["a.example.invalid"]`, "", true)
	updated := cfg(0, `["b.example.invalid","c.example.invalid"]`, "", true)
	imported := cfg(0, `["c.example.invalid","b.example.invalid"]`, "", false)
	cleared := cfg(2, `[]`, `lifecycle { create_before_destroy = true }`, true)
	replaced := strings.Replace(cfg(3, `["d.example.invalid"]`, `description = "한글 설명 &amp; + %"`, true), "password_wo_version = 1", "password_wo_version = 2", 1)
	const address = "iwinv_content_cache.test"
	currentID := ""
	capture := func(state *terraform.State) error {
		currentID = state.RootModule().Resources[address].Primary.ID
		a.mu.Lock()
		owned := false
		for _, r := range a.records {
			if r.ID == currentID {
				owned = true
			}
		}
		a.mu.Unlock()
		if !owned {
			return errors.New("unowned cache identity entered Terraform state")
		}
		b, _ := json.Marshal(state)
		if strings.Contains(string(b), ftp) {
			return errors.New("cache credential entered state")
		}
		return scanHostingArtifacts(work, ftp)
	}
	check := resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr("iwinv_content_cache.peer", "status", "active"), resource.TestCheckResourceAttr("iwinv_content_cache.peer", "allowed_referrers.#", "0"))
	secretCheck := hostingSecretPlanCheck{secrets: []string{ftp}, directory: work}
	service := &hosted.CacheService{API: a}
	writer := &contentCacheResource{service: service, pollInterval: 2 * time.Second}
	resource.Test(t, resource.TestCase{WorkingDir: work, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		verifyCtx, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()
		a.mu.Lock()
		ids := []string{}
		for _, r := range a.records {
			ids = append(ids, r.ID)
		}
		a.mu.Unlock()
		for _, id := range ids {
			if e := a.markAbsent(verifyCtx, id); e != nil {
				return e
			}
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: initial, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{secretCheck}}, Check: resource.ComposeAggregateTestCheckFunc(check, resource.TestCheckResourceAttr(address, "allowed_referrers.#", "1"))},
		{Config: initial, PlanOnly: true},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"password_wo_version"}},
		{Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeAggregateTestCheckFunc(func(state *terraform.State) error {
			if state.RootModule().Resources[address].Primary.ID != currentID {
				return errors.New("referrer update replaced cache")
			}
			return nil
		}, check, resource.TestCheckResourceAttr(address, "allowed_referrers.#", "2"))},
		{Config: updated, PlanOnly: true},
		{Config: updated, PreConfig: func() {
			op, done := context.WithTimeout(context.Background(), time.Minute)
			defer done()
			row, e := service.Read(op, currentID)
			if e != nil || row == nil {
				t.Fatal("owned drift read failed")
			}
			if writer.write(op, row, currentID, []string{"external.example.invalid"}, false) != nil {
				t.Fatal("owned drift write failed")
			}
		}, Check: resource.ComposeAggregateTestCheckFunc(check, resource.TestCheckResourceAttr(address, "allowed_referrers.#", "2"))},
		{Config: cacheVariables + peer + `removed {
 from = iwinv_content_cache.test
 lifecycle { destroy = false }
}`},
		{Config: imported, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return currentID, nil }},
		{Config: imported, PlanOnly: true},
		{Config: cleared, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{secretCheck, plancheck.ExpectResourceAction(address, plancheck.ResourceActionCreateBeforeDestroy)}}, Check: resource.ComposeAggregateTestCheckFunc(check, resource.TestCheckResourceAttr(address, "allowed_referrers.#", "0"))},
		{Config: cleared, PlanOnly: true},
		{Config: replaced, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{secretCheck}}, Check: resource.ComposeAggregateTestCheckFunc(check, resource.TestCheckResourceAttr(address, "description", "한글 설명 &amp; + %"))},
		{Config: replaced, PlanOnly: true},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"password_wo_version"}},
		{Config: cfg(4, `["a.example.invalid"]`, "", true), PreConfig: func() {
			op, done := context.WithTimeout(context.Background(), time.Minute)
			defer done()
			row, e := service.Read(op, currentID)
			if e != nil || row == nil {
				t.Fatal("owned deletion read failed")
			}
			if writer.write(op, row, currentID, nil, true) != nil {
				t.Fatal("owned external deletion failed")
			}
			if a.markAbsent(op, currentID) != nil {
				t.Fatal("owned external absence unverified")
			}
		}, Check: check},
	}})
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.records) != 5 {
		t.Fatal("unexpected cache fixture count")
	}
	for _, r := range a.records {
		if !r.DeleteAcknowledged || !r.Absent {
			t.Fatal("cache cleanup unverified")
		}
	}
}
