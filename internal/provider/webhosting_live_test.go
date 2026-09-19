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

type ownedHostingRecord struct {
	ID                 string `json:"id"`
	DeleteAttempted    bool   `json:"delete_attempted"`
	DeleteAcknowledged bool   `json:"delete_acknowledged"`
	Absent             bool   `json:"absence_verified"`
}
type ownedHostingAPI struct {
	*client.Client
	mu                                 sync.Mutex
	journal                            string
	baseline                           map[string]bool
	allowedAccounts, attemptedAccounts map[string]bool
	records                            []*ownedHostingRecord
	intents                            []map[string]any
	receipts                           []json.RawMessage
}

func (a *ownedHostingAPI) saveLocked() error {
	raw, err := json.Marshal(struct {
		Records  []*ownedHostingRecord `json:"created"`
		Intents  []map[string]any      `json:"intents"`
		Receipts []json.RawMessage     `json:"create_receipts"`
	}{a.records, a.intents, a.receipts})
	if err != nil {
		return errors.New("cannot encode hosting ownership journal")
	}
	f, err := os.CreateTemp(filepath.Dir(a.journal), ".hosting-stage-*")
	if err != nil {
		return errors.New("cannot stage ownership journal")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	if err != nil {
		return errors.New("cannot persist hosting journal")
	}
	if err = f.Close(); err != nil {
		return errors.New("cannot close hosting journal")
	}
	if err = os.Rename(f.Name(), a.journal); err != nil {
		return errors.New("cannot replace hosting journal")
	}
	return nil
}
func (a *ownedHostingAPI) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	if p != "/v1/webhosting" && p != "/v1/webhosting/products" && p != "/v1/webhosting/servers" {
		return client.Envelope{}, errors.New("read outside hosting test scope")
	}
	return a.Client.Get(ctx, p, q)
}
func (a *ownedHostingAPI) PostJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := body.(map[string]any)
	if !ok || p != "/v1/webhosting" {
		return client.Envelope{}, errors.New("create outside hosting test scope")
	}
	account, ok := b["id"].(string)
	if !ok || !a.allowedAccounts[account] || a.attemptedAccounts[account] {
		return client.Envelope{}, errors.New("unowned or repeated hosting account creation refused")
	}
	a.attemptedAccounts[account] = true
	intent := map[string]any{}
	for k, v := range b {
		if k != "ftppw" && k != "dbpw" {
			intent[k] = v
		}
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
			a.records = append(a.records, &ownedHostingRecord{ID: id})
		}
	}
	if saveErr := a.saveLocked(); saveErr != nil {
		return client.Envelope{}, saveErr
	}
	return e, err
}
func (a *ownedHostingAPI) Delete(ctx context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := strings.TrimPrefix(p, "/v1/webhosting/")
	var record *ownedHostingRecord
	for _, r := range a.records {
		if r.ID == id {
			record = r
		}
	}
	if record == nil || a.baseline[id] || p != "/v1/webhosting/"+id || record.DeleteAttempted {
		return client.Envelope{}, errors.New("unowned or repeated hosting delete refused")
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
func (a *ownedHostingAPI) markAbsent(ctx context.Context, id string) error {
	s := hosted.WebhostingService{API: a}
	row, err := s.Read(ctx, id)
	if err != nil {
		return err
	}
	if row != nil {
		return errors.New("owned hosting service still present")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.records {
		if r.ID == id {
			if !r.DeleteAcknowledged {
				return errors.New("hosting deletion never acknowledged")
			}
			r.Absent = true
			return a.saveLocked()
		}
	}
	return errors.New("unknown hosting cleanup identity")
}
func (a *ownedHostingAPI) cleanup(ctx context.Context) error {
	a.mu.Lock()
	ids := []string{}
	for _, r := range a.records {
		if !r.Absent {
			ids = append(ids, r.ID)
		}
	}
	a.mu.Unlock()
	s := hosted.WebhostingService{API: a}
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
				failures = append(failures, errors.New("owned hosting delete failed"))
				continue
			}
			ack = true
		}
		if !ack {
			failures = append(failures, errors.New("unacknowledged hosting delete; no replay"))
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
			failures = append(failures, errors.New("hosting absence unverified"))
		}
	}
	return errors.Join(failures...)
}

// TestAccWebhosting exercises Terraform Core with new, journaled accounts only.
// It never uses a console endpoint or writes DNS/content. It does not establish
// billing termination, data-plane access, or the unresolved webmail contract.
func TestAccWebhosting(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_TERRAFORM_HOSTING_WRITE") != "1" {
		t.Skip("requires TF_ACC=1, IWINV_LIVE_TERRAFORM_HOSTING_WRITE=1 and private journal directory")
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
	f, err := os.CreateTemp(dir, "terraform-hosting-*.json")
	if err != nil {
		t.Fatal("cannot create private journal")
	}
	journal := f.Name()
	if err = f.Close(); err != nil {
		t.Fatal("cannot close private journal")
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("valid environment credentials required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	plain := hosted.WebhostingService{API: c}
	rows, err := plain.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	baseline := map[string]bool{}
	for _, row := range rows {
		baseline[row.ID] = true
	}
	random := func() string {
		b := make([]byte, 7)
		if _, err := rand.Read(b); err != nil {
			t.Fatal("random generation failed")
		}
		return hex.EncodeToString(b)
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
	accounts := []string{account(), account(), account(), account()}
	allowed := map[string]bool{}
	for _, v := range accounts {
		if allowed[v] {
			t.Fatal("random account collision")
		}
		allowed[v] = true
	}
	a := &ownedHostingAPI{Client: c, journal: journal, baseline: baseline, allowedAccounts: allowed, attemptedAccounts: map[string]bool{}, records: []*ownedHostingRecord{}, intents: []map[string]any{}, receipts: []json.RawMessage{}}
	if err = a.saveLocked(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 3*time.Minute)
		defer done()
		if err := a.cleanup(cleanupCtx); err != nil {
			t.Error("hosting cleanup incomplete; inspect private ownership journal")
		}
	}()
	products, err := plain.Products(ctx, "SHARE")
	if err != nil {
		t.Fatal(err)
	}
	product := ""
	for _, p := range products {
		if p.Status == "available" && p.AllowCustomDomain && p.MaxDomainCount >= 1 {
			product = p.ID
			break
		}
	}
	if product == "" {
		t.Fatal("no verified candidate hosting product")
	}
	servers, err := plain.Servers(ctx, product)
	if err != nil {
		t.Fatal(err)
	}
	server := ""
	for _, v := range servers {
		if v.PHPVersion == "PHP 8.4" {
			server = v.ID
			break
		}
	}
	if server == "" {
		t.Fatal("PHP 8.4 server unavailable")
	}
	ftp, db := "Aa1!"+random(), "Bb2!"+random()
	t.Setenv("TF_VAR_hosting_ftp", ftp)
	t.Setenv("TF_VAR_hosting_db", db)
	secrets := []string{ftp, db}
	work := t.TempDir()
	domain := "tf-" + random() + ".invalid"
	block := func(address, account, extra string, inputs bool) string {
		creation := ""
		if inputs {
			creation = fmt.Sprintf("server_id = %q\nftp_password_wo = var.hosting_ftp\ndatabase_password_wo = var.hosting_db\npassword_wo_version = 1\n", server)
		}
		return fmt.Sprintf("resource \"iwinv_webhosting\" %q {\nproduct_id = %q\naccount_name = %q\nname = \"tf-hosting-live\"\n%s\n%s\n}\n", address, product, account, creation, extra)
	}
	peer := block("peer", accounts[1], "", true)
	initial := hostingVariables + block("test", accounts[0], "", true) + peer
	extra := fmt.Sprintf("description = \"한글 설명 &amp; + %%\"\nweb_firewall_enabled = false\ncustom_domains = { %q = \"/\" }\nlifecycle { create_before_destroy = true }", domain)
	replaced := hostingVariables + block("test", accounts[2], extra, true) + peer
	imported := hostingVariables + block("test", accounts[2], extra, false) + peer
	var currentID string
	capture := func(state *terraform.State) error {
		currentID = state.RootModule().Resources["iwinv_webhosting.test"].Primary.ID
		a.mu.Lock()
		owned := false
		for _, record := range a.records {
			if record.ID == currentID {
				owned = true
			}
		}
		a.mu.Unlock()
		if !owned {
			return errors.New("Terraform returned an unowned hosting identity")
		}
		b, _ := json.Marshal(state)
		for _, secret := range secrets {
			if strings.Contains(string(b), secret) {
				return errors.New("credential entered live Terraform state")
			}
		}
		return scanHostingArtifacts(work, secrets...)
	}
	checkSecrets := hostingSecretPlanCheck{secrets: secrets, directory: work}
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
			if err := a.markAbsent(verifyCtx, id); err != nil {
				return err
			}
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: initial, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{checkSecrets}}, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr("iwinv_webhosting.test", "custom_domains.%", "0"), resource.TestCheckResourceAttr("iwinv_webhosting.peer", "status", "active"))},
		{Config: initial, PlanOnly: true},
		{ResourceName: "iwinv_webhosting.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"server_id", "password_wo_version"}},
		{Config: replaced, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{checkSecrets, plancheck.ExpectResourceAction("iwinv_webhosting.test", plancheck.ResourceActionCreateBeforeDestroy)}}, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr("iwinv_webhosting.test", "custom_domains.%", "1"), resource.TestCheckResourceAttr("iwinv_webhosting.test", "domains.%", "2"), resource.TestCheckResourceAttr("iwinv_webhosting.peer", "status", "active"))},
		{Config: replaced, PlanOnly: true},
		{ResourceName: "iwinv_webhosting.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"server_id", "password_wo_version"}},
		{Config: hostingVariables + peer + `removed {
 from = iwinv_webhosting.test
 lifecycle { destroy = false }
}`},
		{Config: imported, ResourceName: "iwinv_webhosting.test", ImportState: true, ImportStatePersist: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return currentID, nil }},
		{Config: imported, PlanOnly: true},
		{PreConfig: func() {
			deleteCtx, done := context.WithTimeout(context.Background(), time.Minute)
			defer done()
			s := hosted.WebhostingService{API: a}
			if err := s.Delete(deleteCtx, currentID); err != nil {
				t.Fatal("owned external deletion failed")
			}
			if err := a.markAbsent(deleteCtx, currentID); err != nil {
				t.Fatal("owned external deletion absence not verified")
			}
		}, Config: hostingVariables + block("test", accounts[3], "", true) + peer, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr("iwinv_webhosting.peer", "status", "active"))},
	}})
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.records) != 4 {
		t.Fatal("unexpected hosting fixture count")
	}
	for _, r := range a.records {
		if !r.DeleteAcknowledged || !r.Absent {
			t.Fatal("hosting fixture cleanup remains unverified")
		}
	}
}
