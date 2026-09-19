package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type liveRuleRecord struct {
	GroupID        string          `json:"parent_id"`
	RuleID         string          `json:"rule_id,omitempty"`
	Request        any             `json:"request"`
	Receipt        json.RawMessage `json:"receipt,omitempty"`
	DeleteAttempts int             `json:"delete_attempts"`
	Deleted        bool            `json:"deleted_verified"`
}
type ownedRuleAPI struct {
	*ownedGroupAPI
	childMu      sync.Mutex
	children     []*liveRuleRecord
	childJournal string
}
type ruleSnapshotAPI struct {
	network.API
	response client.Envelope
}

func (a ruleSnapshotAPI) Get(context.Context, string, url.Values) (client.Envelope, error) {
	return a.response, nil
}
func (a *ownedRuleAPI) saveChildren() error {
	b, err := json.Marshal(a.children)
	if err != nil {
		return errors.New("cannot encode private rule journal")
	}
	f, err := os.CreateTemp(filepath.Dir(a.childJournal), ".rule-stage-*")
	if err != nil {
		return errors.New("cannot stage private rule journal")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	if err != nil {
		return errors.New("cannot persist private rule journal")
	}
	if err = f.Close(); err != nil {
		return errors.New("cannot close private rule journal")
	}
	if err = os.Rename(f.Name(), a.childJournal); err != nil {
		return errors.New("cannot replace private rule journal")
	}
	return nil
}
func (a *ownedRuleAPI) parentOwned(group string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.owned("/v1/security-groups/"+group) != nil
}
func (a *ownedRuleAPI) child(group, id string) *liveRuleRecord {
	for _, r := range a.children {
		if r.RuleID != "" && r.GroupID == group && r.RuleID == id {
			return r
		}
	}
	return nil
}
func (a *ownedRuleAPI) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	group, id, isRule := ruleAPIPath(p)
	if !isRule {
		return a.ownedGroupAPI.Get(ctx, p, q)
	}
	if id != "" || !a.parentOwned(group) {
		return client.Envelope{}, errors.New("rule read outside owned parent scope")
	}
	a.childMu.Lock()
	defer a.childMu.Unlock()
	e, err := a.Client.Get(ctx, p, q)
	if err != nil {
		return e, err
	}
	// Reuse the production decoder without making another request. A complete
	// successful rules list, while its parent exists, proves individual absence.
	snapshot := network.Service{API: ruleSnapshotAPI{API: a.Client, response: e}}
	rows, decodeErr := snapshot.Rules(ctx, group)
	if decodeErr == nil {
		seen := map[string]bool{}
		for _, r := range rows {
			seen[r.ID] = true
		}
		for _, r := range a.children {
			if r.GroupID == group && r.RuleID != "" && r.DeleteAttempts > 0 && !seen[r.RuleID] {
				r.Deleted = true
			}
		}
		if err = a.saveChildren(); err != nil {
			return client.Envelope{}, err
		}
	}
	return e, nil
}
func (a *ownedRuleAPI) PostJSON(ctx context.Context, p string, b any) (client.Envelope, error) {
	group, id, isRule := ruleAPIPath(p)
	if !isRule {
		return a.ownedGroupAPI.PostJSON(ctx, p, b)
	}
	if id != "" || !a.parentOwned(group) {
		return client.Envelope{}, errors.New("rule create outside owned parent scope")
	}
	a.childMu.Lock()
	defer a.childMu.Unlock()
	entry := &liveRuleRecord{GroupID: group, Request: b}
	a.children = append(a.children, entry)
	if err := a.saveChildren(); err != nil {
		return client.Envelope{}, err
	}
	e, err := a.Client.PostJSON(ctx, p, b)
	entry.Receipt = e.Result
	if err == nil && e.Status >= 200 && e.Status <= 202 {
		var ids []struct {
			ID *int64 `json:"rule_id"`
		}
		if json.Unmarshal(e.Result, &ids) == nil && len(ids) == 1 && ids[0].ID != nil && *ids[0].ID > 0 {
			candidate := strconv.FormatInt(*ids[0].ID, 10)
			if a.child(group, candidate) == nil {
				entry.RuleID = candidate
			}
		}
	}
	if saveErr := a.saveChildren(); saveErr != nil {
		return client.Envelope{}, saveErr
	}
	if err != nil {
		return e, err
	}
	if entry.RuleID == "" {
		return client.Envelope{}, errors.New("new rule identity unresolved; inspect private journal")
	}
	return e, nil
}
func (a *ownedRuleAPI) PutJSON(ctx context.Context, p string, b any) (client.Envelope, error) {
	group, id, isRule := ruleAPIPath(p)
	if !isRule {
		return a.ownedGroupAPI.PutJSON(ctx, p, b)
	}
	a.childMu.Lock()
	defer a.childMu.Unlock()
	if a.child(group, id) == nil {
		return client.Envelope{}, errors.New("rule update outside owned ID scope")
	}
	return a.Client.PutJSON(ctx, p, b)
}
func (a *ownedRuleAPI) Delete(ctx context.Context, p string) (client.Envelope, error) {
	group, id, isRule := ruleAPIPath(p)
	if !isRule {
		group = strings.TrimPrefix(p, "/v1/security-groups/")
		a.childMu.Lock()
		pending := false
		for _, r := range a.children {
			if r.GroupID == group && r.RuleID != "" && !r.Deleted {
				pending = true
			}
		}
		a.childMu.Unlock()
		if pending {
			return client.Envelope{}, errors.New("refusing parent delete before owned rule absence is verified")
		}
		return a.ownedGroupAPI.Delete(ctx, p)
	}
	a.childMu.Lock()
	defer a.childMu.Unlock()
	entry := a.child(group, id)
	if entry == nil {
		return client.Envelope{}, errors.New("rule delete outside owned ID scope")
	}
	if entry.DeleteAttempts > 0 {
		return client.Envelope{}, errors.New("rule delete already attempted; reconcile before another write")
	}
	entry.DeleteAttempts++
	if err := a.saveChildren(); err != nil {
		return client.Envelope{}, err
	}
	return a.Client.Delete(ctx, p)
}
func newLiveRuleAPI(t *testing.T) *ownedRuleAPI {
	t.Helper()
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
	makeFile := func(pattern string) string {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			t.Fatal("cannot create private journal")
		}
		name := f.Name()
		if err = f.Close(); err != nil {
			t.Fatal("cannot close private journal")
		}
		return name
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("environment credentials required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s := network.Service{API: c}
	existing, err := s.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	base := &ownedGroupAPI{Client: c, baseline: map[string]bool{}, records: []*groupLiveRecord{}, journal: makeFile("rule-parents-*.json")}
	for _, g := range existing {
		base.baseline[g.ID] = true
	}
	if err = base.save(); err != nil {
		t.Fatal(err)
	}
	a := &ownedRuleAPI{ownedGroupAPI: base, children: []*liveRuleRecord{}, childJournal: makeFile("terraform-rules-*.json")}
	if err = a.saveChildren(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		s := network.Service{API: a}
		for _, entry := range a.children {
			if entry.RuleID == "" {
				t.Error("unknown rule creation outcome; inspect private journal")
				continue
			}
			if entry.Deleted {
				continue
			}
			if entry.DeleteAttempts == 0 {
				if err := s.DeleteRule(ctx, entry.GroupID, entry.RuleID); err != nil {
					t.Error("rule fallback delete failed")
				}
			}
			for attempt := 0; attempt < 5; attempt++ {
				r, err := s.Rule(ctx, entry.GroupID, entry.RuleID)
				if err != nil {
					t.Error("rule fallback verification failed")
					break
				}
				if r == nil {
					break
				}
			}
			if !entry.Deleted {
				t.Error("rule absence unverified before parent cleanup")
			}
		}
		for _, id := range a.identities() {
			g, err := s.Group(ctx, id)
			if err != nil {
				t.Error("parent fallback read failed")
				continue
			}
			if g != nil {
				if err = s.DeleteGroup(ctx, id); err != nil {
					t.Error("parent fallback delete failed")
				}
			}
			if err = a.verifyDeleted(ctx, id); err != nil {
				t.Error("parent absence unverified")
			}
		}
	})
	return a
}
func ruleForgetConfig(config string) string {
	start := strings.Index(config, `resource "iwinv_security_group_ingress_rule" "test" {`)
	end := start + strings.Index(config[start:], "\n}\n") + 3
	return config[:start] + config[end:] + "\nremoved {\n from = iwinv_security_group_ingress_rule.test\n lifecycle { destroy = false }\n}\n"
}
func TestAccSecurityRules(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_TERRAFORM_RULE_WRITE") != "1" {
		t.Skip("requires TF_ACC=1, IWINV_LIVE_TERRAFORM_RULE_WRITE=1 and private journal directory")
	}
	a := newLiveRuleAPI(t)
	s := network.Service{API: a}
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	config := func(parent, desc, protocol string, from, to int, cidr string) string {
		text := ruleConfig(parent, desc, protocol, from, to, cidr)
		return strings.ReplaceAll(strings.ReplaceAll(text, "tf-rule-parent-a", "tf-ra-"+suffix), "tf-rule-parent-b", "tf-rb-"+suffix)
	}
	initial := config("first", "한글 &amp; <x>", "tcp", 8443, 8443, "192.0.2.1/32")
	updated := config("first", "수정 &#39; <x>", "udp", 9443, 9445, "192.0.2.1/24")
	cleared := config("first", "", "udp", 9443, 9445, "192.0.2.1/24")
	moved := config("second", "", "udp", 9443, 9445, "192.0.2.1/24")
	const address = "iwinv_security_group_ingress_rule.test"
	const peer = "iwinv_security_group_egress_rule.peer"
	var current, first, peerID string
	capture := func(state *terraform.State) error {
		current = state.RootModule().Resources[address].Primary.ID
		group, id, err := parseRuleIdentity(current)
		if err != nil {
			return err
		}
		a.childMu.Lock()
		defer a.childMu.Unlock()
		if a.child(group, id) == nil {
			return errors.New("state rule is outside owned fixture")
		}
		if first == "" {
			first = current
			peerID = state.RootModule().Resources[peer].Primary.ID
		}
		if state.RootModule().Resources[peer].Primary.ID != peerID {
			return errors.New("independent rule identity changed")
		}
		return nil
	}
	stable := func(*terraform.State) error {
		if current != first {
			return errors.New("in-place update replaced identity")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		for _, r := range a.children {
			if r.RuleID == "" || !r.Deleted {
				return errors.New("rule absence not verified while parent existed")
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for _, id := range a.identities() {
			if err := a.verifyDeleted(ctx, id); err != nil {
				return err
			}
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "description", "한글 &amp; <x>"), resource.TestCheckResourceAttr(peer, "description", ""))},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(capture, stable, resource.TestCheckResourceAttr(address, "ip_protocol", "udp"), resource.TestCheckResourceAttr(address, "to_port", "9445"))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{ResourceName: peer, ImportState: true, ImportStateVerify: true},
		{Config: ruleForgetConfig(updated)},
		{Config: updated, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return current, nil }},
		{Config: updated, PlanOnly: true},
		{PreConfig: func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			group, id, _ := parseRuleIdentity(current)
			rule, err := s.Rule(ctx, group, id)
			if err != nil || rule == nil {
				t.Fatal("owned drift fixture missing")
			}
			_, err = s.UpdateRule(ctx, group, id, network.RuleInput{Bound: "OUT", Protocol: rule.Protocol, Port: rule.Port, IP: rule.IP, Name: rule.Name})
			if err != nil {
				t.Fatal(err)
			}
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(capture, stable, resource.TestCheckResourceAttr(address, "direction", "ingress"))},
		{Config: cleared, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "description", ""))},
		{Config: moved, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttrPair(address, "security_group_id", "iwinv_security_group.second", "id"))},
		{PreConfig: func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			group, id, _ := parseRuleIdentity(current)
			if err := s.DeleteRule(ctx, group, id); err != nil {
				t.Fatal(err)
			}
			rule, err := s.Rule(ctx, group, id)
			if err != nil || rule != nil {
				t.Fatal("external deletion was not verified")
			}
		}, Config: moved, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: moved, Check: capture},
		{Config: moved, PlanOnly: true},
	}})
	if len(a.children) != 5 || len(a.identities()) != 2 {
		t.Fatal("unexpected number of created fixture identities")
	}
}

func TestAccSecurityEgressRule(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_TERRAFORM_RULE_WRITE") != "1" {
		t.Skip("requires explicit live Terraform rule gate and private journal directory")
	}
	a := newLiveRuleAPI(t)
	s := network.Service{API: a}
	name := "tf-egress-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	const address = "iwinv_security_group_egress_rule.test"
	config := func(desc, protocol string, from, to int, cidr string) string {
		return fmt.Sprintf(`provider "iwinv" {}
resource "iwinv_security_group" "parent" { name = %q }
resource "iwinv_security_group_egress_rule" "test" {
 security_group_id = iwinv_security_group.parent.id
 name = "tf-egress-owned"
 description = %q
 ip_protocol = %q
 from_port = %d
 to_port = %d
 cidr_ipv4 = %q
}
`, name, desc, protocol, from, to, cidr)
	}
	initial := config("", "udp", 53, 53, "192.0.2.53/32")
	updated := config("송신 &amp; <x>", "tcp", 8443, 8445, "198.51.100.1/24")
	cleared := config("", "tcp", 8443, 8445, "198.51.100.1/24")
	var current, first string
	capture := func(state *terraform.State) error {
		current = state.RootModule().Resources[address].Primary.ID
		group, id, err := parseRuleIdentity(current)
		if err != nil {
			return err
		}
		a.childMu.Lock()
		defer a.childMu.Unlock()
		if a.child(group, id) == nil {
			return errors.New("egress state is outside owned fixture")
		}
		if first == "" {
			first = current
		}
		return nil
	}
	stable := func(*terraform.State) error {
		if current != first {
			return errors.New("egress update replaced identity")
		}
		return nil
	}
	forget := fmt.Sprintf(`provider "iwinv" {}
resource "iwinv_security_group" "parent" { name = %q }
removed {
 from = iwinv_security_group_egress_rule.test
 lifecycle { destroy = false }
}
`, name)
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		for _, r := range a.children {
			if r.RuleID == "" || !r.Deleted {
				return errors.New("egress absence not verified before parent deletion")
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for _, id := range a.identities() {
			if err := a.verifyDeleted(ctx, id); err != nil {
				return err
			}
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "direction", "egress"), resource.TestCheckResourceAttr(address, "description", ""))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(capture, stable, resource.TestCheckResourceAttr(address, "ip_protocol", "tcp"), resource.TestCheckResourceAttr(address, "description", "송신 &amp; <x>"))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: forget},
		{Config: updated, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return current, nil }},
		{Config: updated, PlanOnly: true},
		{PreConfig: func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			group, id, _ := parseRuleIdentity(current)
			rule, err := s.Rule(ctx, group, id)
			if err != nil || rule == nil {
				t.Fatal("owned egress drift fixture missing")
			}
			if _, err = s.UpdateRule(ctx, group, id, network.RuleInput{Bound: "IN", Protocol: rule.Protocol, Port: rule.Port, IP: rule.IP, Name: rule.Name}); err != nil {
				t.Fatal(err)
			}
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(capture, stable, resource.TestCheckResourceAttr(address, "direction", "egress"))},
		{Config: cleared, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "description", ""))},
		{PreConfig: func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			group, id, _ := parseRuleIdentity(current)
			if err := s.DeleteRule(ctx, group, id); err != nil {
				t.Fatal(err)
			}
			rule, err := s.Rule(ctx, group, id)
			if err != nil || rule != nil {
				t.Fatal("owned egress external deletion unverified")
			}
		}, Config: cleared, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: cleared, Check: capture},
		{Config: cleared, PlanOnly: true},
	}})
	if len(a.children) != 3 || len(a.identities()) != 1 {
		t.Fatal("unexpected egress fixture identity count")
	}
}
