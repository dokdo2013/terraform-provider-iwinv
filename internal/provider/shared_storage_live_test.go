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

type ownedNASRecord struct {
	ID                 string `json:"id"`
	DeleteAttempted    bool   `json:"delete_attempted"`
	DeleteAcknowledged bool   `json:"delete_acknowledged"`
	Absent             bool   `json:"absence_verified"`
}
type ownedNASAPI struct {
	*client.Client
	mu                             sync.Mutex
	journal                        string
	baseline                       map[string]bool
	allowedShares, attemptedShares map[string]bool
	records                        []*ownedNASRecord
	intents                        []map[string]any
	receipts                       []json.RawMessage
}

func (a *ownedNASAPI) saveLocked() error {
	raw, err := json.Marshal(struct {
		Records  []*ownedNASRecord `json:"created"`
		Intents  []map[string]any  `json:"intents"`
		Receipts []json.RawMessage `json:"create_receipts"`
	}{a.records, a.intents, a.receipts})
	if err != nil {
		return errors.New("cannot encode NAS ownership journal")
	}
	f, err := os.CreateTemp(filepath.Dir(a.journal), ".NAS-stage-*")
	if err != nil {
		return errors.New("cannot stage ownership journal")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	if err != nil {
		return errors.New("cannot persist NAS journal")
	}
	if err = f.Close(); err != nil {
		return errors.New("cannot close NAS journal")
	}
	if err = os.Rename(f.Name(), a.journal); err != nil {
		return errors.New("cannot replace NAS journal")
	}
	return nil
}
func (a *ownedNASAPI) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	if p != "/v1/apinas" && p != "/v1/apinas/products" {
		return client.Envelope{}, errors.New("read outside NAS test scope")
	}
	return a.Client.Get(ctx, p, q)
}
func (a *ownedNASAPI) PostJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := body.(map[string]any)
	if !ok || p != "/v1/apinas" {
		return client.Envelope{}, errors.New("create outside NAS test scope")
	}
	share, ok := b["sharename"].(string)
	if !ok || !a.allowedShares[share] || a.attemptedShares[share] {
		return client.Envelope{}, errors.New("unowned or repeated NAS share creation refused")
	}
	a.attemptedShares[share] = true
	intent := map[string]any{}
	for k, v := range b {
		intent[k] = v
	}
	a.intents = append(a.intents, intent)
	if err := a.saveLocked(); err != nil {
		return client.Envelope{}, err
	}
	e, err := a.Client.PostJSON(ctx, p, body)
	a.receipts = append(a.receipts, e.Result)
	id, _ := hosted.CreateID(e)
	if id != "" && !a.baseline[id] {
		duplicate := false
		for _, r := range a.records {
			if r.ID == id {
				duplicate = true
			}
		}
		if !duplicate {
			a.records = append(a.records, &ownedNASRecord{ID: id})
		}
	}
	if saveErr := a.saveLocked(); saveErr != nil {
		return client.Envelope{}, saveErr
	}
	return e, err
}
func (a *ownedNASAPI) Delete(ctx context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := strings.TrimPrefix(p, "/v1/apinas/")
	var record *ownedNASRecord
	for _, r := range a.records {
		if r.ID == id {
			record = r
		}
	}
	if record == nil || a.baseline[id] || p != "/v1/apinas/"+id || record.DeleteAttempted {
		return client.Envelope{}, errors.New("unowned or repeated NAS delete refused")
	}
	record.DeleteAttempted = true
	if err := a.saveLocked(); err != nil {
		return client.Envelope{}, err
	}
	e, err := a.Client.Delete(ctx, p)
	var ack string
	record.DeleteAcknowledged = err == nil && e.Status == 200 && json.Unmarshal(e.Result, &ack) == nil && ack != "" && len(e.Count) == 0 && len(e.Page) == 0 && len(e.PageNo) == 0 && len(e.PageSize) == 0 && len(e.Total) == 0
	if saveErr := a.saveLocked(); saveErr != nil {
		return client.Envelope{}, saveErr
	}
	return e, err
}
func (a *ownedNASAPI) markAbsent(ctx context.Context, id string) error {
	s := hosted.NASService{API: a}
	row, err := s.Read(ctx, id)
	if err != nil {
		return err
	}
	if row != nil {
		return errors.New("owned NAS service still present")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.records {
		if r.ID == id {
			if !r.DeleteAcknowledged {
				return errors.New("NAS deletion never acknowledged")
			}
			r.Absent = true
			return a.saveLocked()
		}
	}
	return errors.New("unknown NAS cleanup identity")
}
func (a *ownedNASAPI) cleanup(ctx context.Context) error {
	a.mu.Lock()
	ids := []string{}
	for _, r := range a.records {
		if !r.Absent {
			ids = append(ids, r.ID)
		}
	}
	a.mu.Unlock()
	s := hosted.NASService{API: a}
	var failures []error
	for _, id := range ids {
		a.mu.Lock()
		attempted, ack := false, false
		for _, r := range a.records {
			if r.ID == id {
				attempted = r.DeleteAttempted
				ack = r.DeleteAcknowledged
			}
		}
		a.mu.Unlock()
		if !attempted {
			if err := s.Delete(ctx, id); err != nil {
				failures = append(failures, errors.New("owned NAS delete failed"))
				continue
			}
			ack = true
		}
		if !ack {
			failures = append(failures, errors.New("unacknowledged NAS delete; no replay"))
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
			failures = append(failures, errors.New("NAS absence unverified"))
		}
	}
	return errors.Join(failures...)
}

func (a *ownedNASAPI) PutJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/apinas/"), "/allowip")
	owned := false
	for _, r := range a.records {
		if r.ID == id && !r.DeleteAttempted {
			owned = true
		}
	}
	if !owned || a.baseline[id] || p != "/v1/apinas/"+id+"/allowip" {
		return client.Envelope{}, errors.New("unowned NAS update refused")
	}
	b, ok := body.(map[string]any)
	if !ok {
		return client.Envelope{}, errors.New("invalid NAS update body")
	}
	a.intents = append(a.intents, map[string]any{"operation": "PUT", "service_id": id, "allowip": b["allowip"]})
	if err := a.saveLocked(); err != nil {
		return client.Envelope{}, err
	}
	return a.Client.PutJSON(ctx, p, body)
}

// TestAccSharedStorage manages four fresh NAS identities across replacements.
// It never connects to file contents or changes existing infrastructure.
func TestAccSharedStorage(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_TERRAFORM_NAS_WRITE") != "1" {
		t.Skip("requires TF_ACC=1, IWINV_LIVE_TERRAFORM_NAS_WRITE=1 and private journal directory")
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
	f, err := os.CreateTemp(dir, "terraform-nas-*.json")
	if err != nil {
		t.Fatal("cannot create private journal")
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
	rows, err := (&hosted.NASService{API: c}).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	baseline := map[string]bool{}
	for _, r := range rows {
		baseline[r.ID] = true
	}
	share := func() string {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			t.Fatal("share generation failed")
		}
		for i := range b {
			b[i] = 'a' + b[i]%26
		}
		return "tf" + string(b)
	}
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal("name generation failed")
	}
	name := "tf-nas-" + hex.EncodeToString(suffix)
	shares := []string{share(), share(), share(), share()}
	allowed := map[string]bool{}
	for _, v := range shares {
		if allowed[v] {
			t.Fatal("random collision")
		}
		allowed[v] = true
	}
	a := &ownedNASAPI{Client: c, journal: journal, baseline: baseline, allowedShares: allowed, attemptedShares: map[string]bool{}, records: []*ownedNASRecord{}, intents: []map[string]any{}, receipts: []json.RawMessage{}}
	if err = a.saveLocked(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 3*time.Minute)
		defer done()
		if err := a.cleanup(cleanupCtx); err != nil {
			t.Error("NAS cleanup incomplete; inspect private journal")
		}
	}()
	catalog := hosted.NASCatalogService{API: a}
	products, err := catalog.Products(ctx)
	if err != nil {
		t.Fatal(err)
	}
	product := ""
	matches := 0
	for _, p := range products {
		if p.ID == "api_nas" && p.Status == "available" && p.MinimumDiskGB <= 100 && p.MaximumDiskGB >= 200 {
			product = p.ID
			matches++
		}
	}
	if matches != 1 {
		t.Fatal("no available NAS product supporting tested capacities")
	}
	block := func(address, share, ips, extra string) string {
		history := ""
		if share != "" {
			history = fmt.Sprintf("share_name = %q\n", share)
		}
		return fmt.Sprintf("resource \"iwinv_shared_storage\" %q {\nproduct_id = %q\nname = %q\nsize_gb = 100\n%sallowed_ips = %s\n%s\n}\n", address, product, name+"-"+address, history, ips, extra)
	}
	peer := block("peer", shares[1], `{"192.0.2.1" = "RO"}`, "")
	initial := `provider "iwinv" {}` + "\n" + block("test", shares[0], `{"192.0.2.1" = "RO"}`, "") + peer
	updated := `provider "iwinv" {}` + "\n" + block("test", shares[0], `{"192.0.2.3" = "RO", "192.0.2.2" = "RW"}`, "") + peer
	imported := `provider "iwinv" {}` + "\n" + block("test", "", `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, "") + peer
	extra := `description = "한글 설명 &amp; + %"
lifecycle { create_before_destroy = true }`
	replaced := `provider "iwinv" {}` + "\n" + strings.Replace(block("test", shares[2], `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, extra), "size_gb = 100", "size_gb = 200", 1) + peer
	address := "iwinv_shared_storage.test"
	currentID := ""
	capture := func(state *terraform.State) error {
		currentID = state.RootModule().Resources[address].Primary.ID
		a.mu.Lock()
		defer a.mu.Unlock()
		for _, r := range a.records {
			if r.ID == currentID {
				return nil
			}
		}
		return errors.New("unowned NAS identity entered Terraform state")
	}
	check := resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr("iwinv_shared_storage.peer", "status", "active"))
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		verifyCtx, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()
		a.mu.Lock()
		ids := []string{}
		for _, r := range a.records {
			ids = append(ids, r.ID)
		}
		a.mu.Unlock()
		for _, id := range ids {
			if err := a.markAbsent(verifyCtx, id); err != nil {
				return err
			}
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(check, resource.TestCheckResourceAttr(address, "description", ""), resource.TestCheckResourceAttrSet(address, "address"), resource.TestCheckResourceAttrSet(address, "mount_info"), resource.TestCheckResourceAttr(address, "size_gb", "100"))},
		{Config: initial, PlanOnly: true},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"share_name"}},
		{Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeAggregateTestCheckFunc(func(s *terraform.State) error {
			if s.RootModule().Resources[address].Primary.ID != currentID {
				return errors.New("allowlist update replaced NAS")
			}
			return nil
		}, resource.TestCheckResourceAttr(address, "allowed_ips.%", "2"))},
		{Config: updated, PlanOnly: true},
		{Config: updated, PreConfig: func() {
			writeCtx, done := context.WithTimeout(context.Background(), time.Minute)
			defer done()
			if err := (&hosted.NASService{API: a}).ReplaceAllowIPs(writeCtx, currentID, map[string]string{"192.0.2.4": "RW"}); err != nil {
				t.Fatal("owned external allowlist change failed")
			}
		}, Check: resource.TestCheckResourceAttr(address, "allowed_ips.%", "2")},
		{Config: `provider "iwinv" {}` + "\n" + peer + `removed {
 from = iwinv_shared_storage.test
 lifecycle { destroy = false }
}`},
		{Config: imported, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return currentID, nil }},
		{Config: imported, PlanOnly: true},
		{Config: `provider "iwinv" {}` + "\n" + block("test", "", `{"192.0.2.2" = "RO"}`, "") + peer, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "allowed_ips.%", "1"), resource.TestCheckResourceAttr(address, "allowed_ips.192.0.2.2", "RO"))},
		{Config: replaced, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionCreateBeforeDestroy)}}, Check: resource.ComposeAggregateTestCheckFunc(check, resource.TestCheckResourceAttr(address, "size_gb", "200"))},
		{Config: replaced, PlanOnly: true},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"share_name"}},
		{Config: `provider "iwinv" {}` + "\n" + block("test", shares[3], `{"192.0.2.1" = "RO"}`, "") + peer, PreConfig: func() {
			deleteCtx, done := context.WithTimeout(context.Background(), time.Minute)
			defer done()
			if err := (&hosted.NASService{API: a}).Delete(deleteCtx, currentID); err != nil {
				t.Fatal("owned external deletion failed")
			}
			if err := a.markAbsent(deleteCtx, currentID); err != nil {
				t.Fatal("owned external absence not verified")
			}
		}, Check: check},
	}})
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.records) != 4 {
		t.Fatal("unexpected NAS fixture count")
	}
	for _, r := range a.records {
		if !r.DeleteAcknowledged || !r.Absent {
			t.Fatal("NAS fixture cleanup unverified")
		}
	}
}
