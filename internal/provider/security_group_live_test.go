package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type groupLiveRecord struct {
	ID             string          `json:"created_id,omitempty"`
	Request        any             `json:"request"`
	Response       json.RawMessage `json:"response,omitempty"`
	Error          bool            `json:"error"`
	DeleteAttempts int             `json:"delete_attempts"`
	Deleted        bool            `json:"deleted_verified"`
}

// The wrapper journals intent before create and the raw receipt immediately
// afterward. Only exact new IDs returned by this run may be changed or deleted.
// It is shared by all reattached provider instances in the acceptance test.
type ownedGroupAPI struct {
	*client.Client
	mu       sync.Mutex
	baseline map[string]bool
	records  []*groupLiveRecord
	journal  string
}

func (a *ownedGroupAPI) save() error {
	b, err := json.Marshal(a.records)
	if err != nil {
		return errors.New("cannot encode private journal")
	}
	f, err := os.CreateTemp(filepath.Dir(a.journal), ".group-stage-*")
	if err != nil {
		return errors.New("cannot stage private journal")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	if err != nil {
		return errors.New("cannot persist private journal")
	}
	if err = f.Close(); err != nil {
		return errors.New("cannot close private journal")
	}
	if err = os.Rename(f.Name(), a.journal); err != nil {
		return errors.New("cannot replace private journal")
	}
	return nil
}
func (a *ownedGroupAPI) owned(p string) *groupLiveRecord {
	if !strings.HasPrefix(p, "/v1/security-groups/") {
		return nil
	}
	id := strings.TrimPrefix(p, "/v1/security-groups/")
	for _, r := range a.records {
		if r.ID != "" && r.ID == id && !a.baseline[id] {
			return r
		}
	}
	return nil
}
func (a *ownedGroupAPI) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.owned(p) == nil {
		return client.Envelope{}, errors.New("read outside run-owned group scope")
	}
	return a.Client.Get(ctx, p, q)
}
func (a *ownedGroupAPI) PostJSON(ctx context.Context, p string, b any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p != "/v1/security-groups" {
		return client.Envelope{}, errors.New("create outside group scope")
	}
	r := &groupLiveRecord{Request: b}
	a.records = append(a.records, r)
	if err := a.save(); err != nil {
		return client.Envelope{}, err
	}
	e, callErr := a.Client.PostJSON(ctx, p, b)
	r.Response = e.Result
	r.Error = callErr != nil
	var ids []struct {
		ID string `json:"firewall_id"`
	}
	if callErr == nil && e.Status >= 200 && e.Status <= 202 && json.Unmarshal(e.Result, &ids) == nil && len(ids) == 1 && network.ValidateGroupID(ids[0].ID) == nil && !a.baseline[ids[0].ID] {
		for _, prior := range a.records[:len(a.records)-1] {
			if prior.ID == ids[0].ID {
				r.Error = true
				_ = a.save()
				return client.Envelope{}, errors.New("create returned an already-owned ID")
			}
		}
		r.ID = ids[0].ID
	}
	if err := a.save(); err != nil {
		return client.Envelope{}, err
	}
	if callErr != nil {
		return e, callErr
	}
	if r.ID == "" {
		return client.Envelope{}, errors.New("new create identity unresolved; inspect private journal")
	}
	return e, nil
}
func (a *ownedGroupAPI) PutJSON(ctx context.Context, p string, b any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.owned(p) == nil {
		return client.Envelope{}, errors.New("update outside run-owned group scope")
	}
	return a.Client.PutJSON(ctx, p, b)
}
func (a *ownedGroupAPI) Delete(ctx context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.owned(p)
	if r == nil {
		return client.Envelope{}, errors.New("delete outside run-owned group scope")
	}
	if r.DeleteAttempts > 0 {
		return client.Envelope{}, errors.New("delete already attempted; reconcile before another write")
	}
	r.DeleteAttempts++
	if err := a.save(); err != nil {
		return client.Envelope{}, err
	}
	return a.Client.Delete(ctx, p)
}
func (a *ownedGroupAPI) identities() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	ids := []string{}
	for _, r := range a.records {
		if r.ID != "" {
			ids = append(ids, r.ID)
		}
	}
	return ids
}
func (a *ownedGroupAPI) verifyDeleted(ctx context.Context, id string) error {
	s := network.Service{API: a}
	for attempts := 0; attempts < 10; attempts++ {
		g, err := s.Group(ctx, id)
		if err != nil {
			return err
		}
		if g == nil {
			a.mu.Lock()
			defer a.mu.Unlock()
			a.owned("/v1/security-groups/" + id).Deleted = true
			return a.save()
		}
	}
	return errors.New("created group absence not verified")
}
func TestAccSecurityGroup(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_TERRAFORM_WRITE") != "1" {
		t.Skip("requires TF_ACC=1, IWINV_LIVE_TERRAFORM_WRITE=1 and a private journal directory")
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
		t.Fatal("journal directory must be a real directory with mode 0700")
	}
	f, err := os.CreateTemp(dir, "terraform-group-*.json")
	if err != nil {
		t.Fatal("cannot create private journal")
	}
	journal := f.Name()
	if err = f.Close(); err != nil {
		t.Fatal("cannot close journal")
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("environment credentials required")
	}
	setupCtx, setupCancel := context.WithTimeout(context.Background(), time.Minute)
	defer setupCancel()
	baseService := network.Service{API: c}
	baseline, err := baseService.Groups(setupCtx)
	if err != nil {
		t.Fatal(err)
	}
	a := &ownedGroupAPI{Client: c, baseline: map[string]bool{}, journal: journal, records: []*groupLiveRecord{}}
	for _, g := range baseline {
		a.baseline[g.ID] = true
	}
	if err = a.save(); err != nil {
		t.Fatal(err)
	}
	s := network.Service{API: a}
	// Runs even if Terraform or its state checks fail. A previous delete is never
	// blindly replayed; unresolved outcomes stay visible in the private journal.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		for _, id := range a.identities() {
			g, err := s.Group(ctx, id)
			if err != nil {
				t.Error("cleanup read failed; inspect private journal")
				continue
			}
			if g != nil {
				if err = s.DeleteGroup(ctx, id); err != nil {
					t.Error("cleanup delete failed; inspect private journal")
				}
			}
			if err = a.verifyDeleted(ctx, id); err != nil {
				t.Error("cleanup absence unverified; inspect private journal")
			}
		}
	})
	name := "tf-res-" + time.Now().UTC().Format("150405.000000000")
	initial := groupConfig(name, "한글 &amp; <group> + %", false)
	updated := groupConfig(name+"-u", "수정 &#39; &lt; + %", true)
	const address = "iwinv_security_group.test"
	var firstID, currentID string
	capture := func(state *terraform.State) error {
		instance := state.RootModule().Resources[address]
		if instance == nil || instance.Primary == nil {
			return errors.New("group state missing")
		}
		currentID = instance.Primary.ID
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.owned("/v1/security-groups/"+currentID) == nil {
			return errors.New("state does not contain a run-owned ID")
		}
		if firstID == "" {
			firstID = currentID
		}
		return nil
	}
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for _, id := range a.identities() {
			if err := a.verifyDeleted(ctx, id); err != nil {
				return err
			}
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "description", "한글 &amp; <group> + %"))},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(capture, func(*terraform.State) error {
			if currentID != firstID {
				return errors.New("update replaced identity")
			}
			return nil
		}, resource.TestCheckResourceAttr(address, "allow_icmp", "true"), resource.TestCheckResourceAttr(address, "description", "수정 &#39; &lt; + %"))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: groupForgetConfig},
		{Config: updated, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return currentID, nil }},
		{Config: updated, PlanOnly: true},
		{PreConfig: func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if _, err := s.UpdateGroup(ctx, currentID, network.GroupInput{Name: name + "-drift", AllowICMP: true}); err != nil {
				t.Fatal(err)
			}
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: resource.TestCheckResourceAttr(address, "name", name+"-u")},
		{PreConfig: func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			if err := s.DeleteGroup(ctx, currentID); err != nil {
				t.Fatal(err)
			}
			if err := a.verifyDeleted(ctx, currentID); err != nil {
				t.Fatal(err)
			}
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(capture, func(*terraform.State) error {
			if currentID == firstID {
				return errors.New("external deletion did not produce a new identity")
			}
			return nil
		})},
		{Config: updated, PlanOnly: true},
	}})
	if len(a.identities()) != 2 {
		t.Fatal("expected two individually recorded create identities")
	}
}
