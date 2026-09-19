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

type ownedDBMSRecord struct {
	ID                 string `json:"id"`
	DeleteAttempted    bool   `json:"delete_attempted"`
	DeleteAcknowledged bool   `json:"delete_acknowledged"`
	Absent             bool   `json:"absence_verified"`
}
type ownedDBMSAPI struct {
	*client.Client
	mu                                 sync.Mutex
	journal                            string
	baseline                           map[string]bool
	allowedAccounts, attemptedAccounts map[string]bool
	records                            []*ownedDBMSRecord
	intents                            []map[string]any
	receipts                           []json.RawMessage
}

func (a *ownedDBMSAPI) saveLocked() error {
	raw, err := json.Marshal(struct {
		Records  []*ownedDBMSRecord `json:"created"`
		Intents  []map[string]any   `json:"intents"`
		Receipts []json.RawMessage  `json:"create_receipts"`
	}{a.records, a.intents, a.receipts})
	if err != nil {
		return errors.New("cannot encode DBMS ownership journal")
	}
	f, err := os.CreateTemp(filepath.Dir(a.journal), ".DBMS-stage-*")
	if err != nil {
		return errors.New("cannot stage ownership journal")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	if err != nil {
		return errors.New("cannot persist DBMS journal")
	}
	if err = f.Close(); err != nil {
		return errors.New("cannot close DBMS journal")
	}
	if err = os.Rename(f.Name(), a.journal); err != nil {
		return errors.New("cannot replace DBMS journal")
	}
	return nil
}
func (a *ownedDBMSAPI) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	if p != "/v1/dbms" && p != "/v1/dbms/products" {
		return client.Envelope{}, errors.New("read outside DBMS test scope")
	}
	return a.Client.Get(ctx, p, q)
}
func (a *ownedDBMSAPI) PostJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := body.(map[string]any)
	if !ok || p != "/v1/dbms" {
		return client.Envelope{}, errors.New("create outside DBMS test scope")
	}
	account, ok := b["id"].(string)
	if !ok || !a.allowedAccounts[account] || a.attemptedAccounts[account] {
		return client.Envelope{}, errors.New("unowned or repeated DBMS account creation refused")
	}
	a.attemptedAccounts[account] = true
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
			a.records = append(a.records, &ownedDBMSRecord{ID: id})
		}
	}
	if saveErr := a.saveLocked(); saveErr != nil {
		return client.Envelope{}, saveErr
	}
	return e, err
}
func (a *ownedDBMSAPI) Delete(ctx context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := strings.TrimPrefix(p, "/v1/dbms/")
	var record *ownedDBMSRecord
	for _, r := range a.records {
		if r.ID == id {
			record = r
		}
	}
	if record == nil || a.baseline[id] || p != "/v1/dbms/"+id || record.DeleteAttempted {
		return client.Envelope{}, errors.New("unowned or repeated DBMS delete refused")
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
func (a *ownedDBMSAPI) markAbsent(ctx context.Context, id string) error {
	s := hosted.DBMSService{API: a}
	row, err := s.Read(ctx, id)
	if err != nil {
		return err
	}
	if row != nil {
		return errors.New("owned DBMS service still present")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.records {
		if r.ID == id {
			if !r.DeleteAcknowledged {
				return errors.New("DBMS deletion never acknowledged")
			}
			r.Absent = true
			return a.saveLocked()
		}
	}
	return errors.New("unknown DBMS cleanup identity")
}
func (a *ownedDBMSAPI) cleanup(ctx context.Context) error {
	a.mu.Lock()
	ids := []string{}
	for _, r := range a.records {
		if !r.Absent {
			ids = append(ids, r.ID)
		}
	}
	a.mu.Unlock()
	s := hosted.DBMSService{API: a}
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
				failures = append(failures, errors.New("owned DBMS delete failed"))
				continue
			}
			ack = true
		}
		if !ack {
			failures = append(failures, errors.New("unacknowledged DBMS delete; no replay"))
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
			failures = append(failures, errors.New("DBMS absence unverified"))
		}
	}
	return errors.Join(failures...)
}

func (a *ownedDBMSAPI) PutJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/dbms/"), "/allowip")
	owned := false
	for _, r := range a.records {
		if r.ID == id && !r.DeleteAttempted {
			owned = true
		}
	}
	if !owned || a.baseline[id] || p != "/v1/dbms/"+id+"/allowip" {
		return client.Envelope{}, errors.New("unowned DBMS update refused")
	}
	b, ok := body.(map[string]any)
	if !ok {
		return client.Envelope{}, errors.New("invalid DBMS update body")
	}
	a.intents = append(a.intents, map[string]any{"operation": "PUT", "service_id": id, "allowip": b["allowip"]})
	if err := a.saveLocked(); err != nil {
		return client.Envelope{}, err
	}
	return a.Client.PutJSON(ctx, p, body)
}

// TestAccDBInstance manages four fresh Redis identities across replacements.
// It never connects to database contents or changes existing infrastructure.
func TestAccDBInstance(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_TERRAFORM_DBMS_WRITE") != "1" {
		t.Skip("requires TF_ACC=1, IWINV_LIVE_TERRAFORM_DBMS_WRITE=1 and private journal directory")
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
	f, err := os.CreateTemp(dir, "terraform-dbms-*.json")
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
	rows, err := (&hosted.DBMSService{API: c}).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	baseline := map[string]bool{}
	for _, r := range rows {
		baseline[r.ID] = true
	}
	account := func() string {
		b := make([]byte, 10)
		if _, err := rand.Read(b); err != nil {
			t.Fatal("account generation failed")
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
	name := "tf-dbms-" + hex.EncodeToString(suffix)
	accounts := []string{account(), account(), account(), account()}
	allowed := map[string]bool{}
	for _, v := range accounts {
		if allowed[v] {
			t.Fatal("random collision")
		}
		allowed[v] = true
	}
	a := &ownedDBMSAPI{Client: c, journal: journal, baseline: baseline, allowedAccounts: allowed, attemptedAccounts: map[string]bool{}, records: []*ownedDBMSRecord{}, intents: []map[string]any{}, receipts: []json.RawMessage{}}
	if err = a.saveLocked(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 3*time.Minute)
		defer done()
		if err := a.cleanup(cleanupCtx); err != nil {
			t.Error("DBMS cleanup incomplete; inspect private journal")
		}
	}()
	catalog := hosted.DBMSCatalogService{API: a}
	all, err := catalog.Products(ctx, "", "")
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := catalog.Products(ctx, "STD", "redis")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, p := range all {
		counts[p.ID]++
	}
	product := ""
	for _, p := range filtered {
		if p.ID != "" && p.Status == "available" && counts[p.ID] == 1 {
			product = p.ID
			break
		}
	}
	if product == "" {
		t.Fatal("no available unambiguous STD Redis product")
	}
	block := func(address, account, ips, extra string) string {
		history := ""
		if account != "" {
			history = fmt.Sprintf("account_name = %q\n", account)
		}
		return fmt.Sprintf("resource \"iwinv_db_instance\" %q {\nproduct_id = %q\nname = %q\n%sallowed_ips = %s\n%s\n}\n", address, product, name+"-"+address, history, ips, extra)
	}
	peer := block("peer", accounts[1], `["192.0.2.1"]`, "")
	initial := `provider "iwinv" {}` + "\n" + block("test", accounts[0], `["192.0.2.1"]`, "") + peer
	updated := `provider "iwinv" {}` + "\n" + block("test", accounts[0], `["192.0.2.3","192.0.2.2"]`, "") + peer
	imported := `provider "iwinv" {}` + "\n" + block("test", "", `["192.0.2.2","192.0.2.3"]`, "") + peer
	extra := `description = "한글 설명 &amp; + %"
lifecycle { create_before_destroy = true }`
	replaced := `provider "iwinv" {}` + "\n" + block("test", accounts[2], `["192.0.2.2","192.0.2.3"]`, extra) + peer
	address := "iwinv_db_instance.test"
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
		return errors.New("unowned DBMS identity entered Terraform state")
	}
	check := resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr("iwinv_db_instance.peer", "status", "active"))
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
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(check, resource.TestCheckResourceAttr(address, "description", ""), resource.TestCheckResourceAttrSet(address, "address"))},
		{Config: initial, PlanOnly: true},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"account_name"}},
		{Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeAggregateTestCheckFunc(func(s *terraform.State) error {
			if s.RootModule().Resources[address].Primary.ID != currentID {
				return errors.New("allowlist update replaced DBMS")
			}
			return nil
		}, resource.TestCheckResourceAttr(address, "allowed_ips.#", "2"))},
		{Config: updated, PlanOnly: true},
		{Config: updated, PreConfig: func() {
			writeCtx, done := context.WithTimeout(context.Background(), time.Minute)
			defer done()
			if err := (&hosted.DBMSService{API: a}).ReplaceAllowIPs(writeCtx, currentID, []string{"192.0.2.4"}); err != nil {
				t.Fatal("owned external allowlist change failed")
			}
		}, Check: resource.TestCheckResourceAttr(address, "allowed_ips.#", "2")},
		{Config: `provider "iwinv" {}` + "\n" + peer + `removed {
 from = iwinv_db_instance.test
 lifecycle { destroy = false }
}`},
		{Config: imported, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return currentID, nil }},
		{Config: imported, PlanOnly: true},
		{Config: replaced, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionCreateBeforeDestroy)}}, Check: check},
		{Config: replaced, PlanOnly: true},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"account_name"}},
		{Config: `provider "iwinv" {}` + "\n" + block("test", accounts[3], `["192.0.2.1"]`, "") + peer, PreConfig: func() {
			deleteCtx, done := context.WithTimeout(context.Background(), time.Minute)
			defer done()
			if err := (&hosted.DBMSService{API: a}).Delete(deleteCtx, currentID); err != nil {
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
		t.Fatal("unexpected DBMS fixture count")
	}
	for _, r := range a.records {
		if !r.DeleteAcknowledged || !r.Absent {
			t.Fatal("DBMS fixture cleanup unverified")
		}
	}
}
