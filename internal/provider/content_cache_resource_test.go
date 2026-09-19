package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const cacheFTPSecret = "SyntheticCache71!"

type contentCacheAPI struct {
	mu                                                                                              sync.Mutex
	rows                                                                                            map[string]map[string]any
	accounts                                                                                        map[string]bool
	creates, updates, deletes                                                                       int
	busyPuts, busyDeletes, hiddenReads, hiddenAfterCreate                                           int
	readErr, putErr, deleteErr                                                                      error
	malformedCreate, retainDeleted, ignoreUpdate, changeAfterBusy, hideAfterBusy, failReadAfterBusy bool
}

func newContentCacheAPI() *contentCacheAPI {
	return &contentCacheAPI{rows: map[string]map[string]any{}, accounts: map[string]bool{}}
}
func (a *contentCacheAPI) Get(_ context.Context, p string, _ url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.readErr != nil {
		return client.Envelope{}, a.readErr
	}
	if p != "/v1/cache" {
		return client.Envelope{}, errors.New("unexpected cache read")
	}
	rows := []map[string]any{}
	if a.hiddenReads > 0 {
		a.hiddenReads--
		return dbEnvelope(rows), nil
	}
	ids := []string{}
	for id := range a.rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rows = append(rows, a.rows[id])
	}
	return dbEnvelope(rows), nil
}
func (a *contentCacheAPI) PostJSON(_ context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.creates++
	if p != "/v1/cache" {
		return client.Envelope{}, errors.New("unexpected cache create")
	}
	b := body.(map[string]any)
	account := b["id"].(string)
	if a.accounts[account] {
		return client.Envelope{}, errors.New("synthetic account reuse restricted")
	}
	a.accounts[account] = true
	if pw, ok := b["pw"].(map[string]string); !ok || pw["FTP"] != cacheFTPSecret {
		return client.Envelope{}, errors.New("write-only password did not reach create")
	}
	if _, ok := b["allow_referer"]; ok {
		return client.Envelope{}, errors.New("ignored creation referrers must not be sent")
	}
	id := strconv.FormatInt(9007199254740992+int64(a.creates), 10)
	number, _ := strconv.ParseInt(id, 10, 64)
	desc := ""
	if v, ok := b["description"]; ok {
		desc = v.(string)
	}
	row := map[string]any{"service_idx": number, "product_id": b["product_id"], "name": b["name"], "description": desc, "id": account, "status": "active", "spec": map[string]any{"type": "SINGLE"}, "domain": account + ".example.invalid", "ip": "192.0.2.8", "allow_referer": []string{}, "pw": map[string]string{"FTP": cacheFTPSecret}}
	a.rows[id] = row
	a.hiddenReads = a.hiddenAfterCreate
	if a.malformedCreate {
		return dbEnvelope(map[string]any{"service_idx": number}), nil
	}
	receipt := map[string]any{}
	for k, v := range row {
		if k != "product_id" {
			receipt[k] = v
		}
	}
	return dbEnvelope(receipt), nil
}
func (a *contentCacheAPI) busy(id, kind string) (client.Envelope, error) {
	if a.changeAfterBusy {
		a.rows[id]["name"] = "external-change"
	}
	if a.hideAfterBusy {
		a.hiddenReads = 1
	}
	if a.failReadAfterBusy {
		a.readErr = errors.New("synthetic read failure")
	}
	return client.Envelope{}, &client.Error{Kind: kind, Status: 404, Code: "NOT_FOUND"}
}
func (a *contentCacheAPI) PutJSON(_ context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.updates++
	if a.putErr != nil {
		return client.Envelope{}, a.putErr
	}
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/cache/"), "/allow_referer")
	if p != "/v1/cache/"+id+"/allow_referer" || a.rows[id] == nil {
		return client.Envelope{}, errors.New("unexpected cache update")
	}
	if a.busyPuts > 0 {
		a.busyPuts--
		return a.busy(id, "cache_referrers_busy")
	}
	refs := body.(map[string]any)["allow_referer"].([]string)
	if len(refs) == 0 {
		return client.Envelope{}, errors.New("empty clearing unsupported")
	}
	if !a.ignoreUpdate {
		a.rows[id]["allow_referer"] = append([]string{}, refs...)
	}
	return dbEnvelope(refs), nil
}
func (a *contentCacheAPI) Delete(_ context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deletes++
	if a.deleteErr != nil {
		return client.Envelope{}, a.deleteErr
	}
	id := strings.TrimPrefix(p, "/v1/cache/")
	if p != "/v1/cache/"+id || a.rows[id] == nil {
		return client.Envelope{}, errors.New("unexpected cache delete")
	}
	if a.busyDeletes > 0 {
		a.busyDeletes--
		return a.busy(id, "cache_delete_busy")
	}
	if !a.retainDeleted {
		delete(a.rows, id)
	}
	return dbEnvelope("acknowledged"), nil
}

const cacheVariables = `provider "iwinv" {}
variable "cache_ftp" {
 type = string
 sensitive = true
 ephemeral = true
}
`

func cacheConfig(account, refs, extra string, credentials bool) string {
	secret := ""
	if credentials {
		secret = "ftp_password_wo = var.cache_ftp\npassword_wo_version = 1\n"
	}
	return cacheVariables + fmt.Sprintf("resource \"iwinv_content_cache\" \"test\" {\nproduct_id = \"synthetic_cache\"\naccount_name = %q\nname = \"tf-cache\"\nallowed_referrers = %s\n%s\n%s\n}\n", account, refs, secret, extra)
}
func cacheProtocolEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	t.Setenv("TF_VAR_cache_ftp", cacheFTPSecret)
}
func cacheDestroyCheck(a *contentCacheAPI) resource.TestCheckFunc {
	return func(*terraform.State) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(a.rows) > 0 {
			return errors.New("synthetic cache leaked")
		}
		return nil
	}
}
func TestProtocolContentCacheLifecycle(t *testing.T) {
	cacheProtocolEnv(t)
	a := newContentCacheAPI()
	a.busyPuts = 1
	a.busyDeletes = 1
	a.hiddenAfterCreate = 1
	dir := t.TempDir()
	address := "iwinv_content_cache.test"
	initial := cacheConfig("tfexamplea", `["a.example.invalid"]`, "", true)
	updated := cacheConfig("tfexamplea", `["b.example.invalid","c.example.invalid"]`, "", true)
	cleared := cacheConfig("tfexampleb", `[]`, `lifecycle { create_before_destroy = true }`, true)
	replaced := strings.Replace(cacheConfig("tfexamplec", `["a.example.invalid"]`, "", true), "password_wo_version = 1", "password_wo_version = 2", 1)
	imported := cacheConfig("tfexamplec", `["a.example.invalid"]`, "", false)
	secretCheck := func(s *terraform.State) error {
		b, _ := json.Marshal(s)
		if strings.Contains(string(b), cacheFTPSecret) {
			return errors.New("cache password in state")
		}
		return scanHostingArtifacts(dir, cacheFTPSecret)
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, WorkingDir: dir, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: cacheDestroyCheck(a), Steps: []resource.TestStep{
		{Config: initial, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{hostingSecretPlanCheck{secrets: []string{cacheFTPSecret}, directory: dir}}}, Check: resource.ComposeAggregateTestCheckFunc(secretCheck, resource.TestCheckResourceAttr(address, "id", "9007199254740993"), resource.TestCheckResourceAttr(address, "allowed_referrers.#", "1"), resource.TestCheckResourceAttr(address, "product_type", "SINGLE"), resource.TestCheckNoResourceAttr(address, "ftp_password_wo"))},
		{Config: initial, PlanOnly: true},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"password_wo_version"}},
		{Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeAggregateTestCheckFunc(secretCheck, resource.TestCheckResourceAttr(address, "id", "9007199254740993"), resource.TestCheckResourceAttr(address, "allowed_referrers.#", "2"))},
		{Config: strings.Replace(updated, `["b.example.invalid","c.example.invalid"]`, `["c.example.invalid","b.example.invalid"]`, 1), PlanOnly: true},
		{PreConfig: func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			a.rows["9007199254740993"]["allow_referer"] = []string{"external.example.invalid"}
		}, Config: updated, Check: resource.TestCheckResourceAttr(address, "allowed_referrers.#", "2")},
		{Config: cacheConfig("tfexamplea", `[]`, "", true), ExpectError: regexp.MustCompile("Cache replacement requires a new account")},
		{Config: cleared, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionCreateBeforeDestroy)}}, Check: resource.ComposeAggregateTestCheckFunc(secretCheck, resource.TestCheckResourceAttr(address, "allowed_referrers.#", "0"), resource.TestCheckResourceAttr(address, "id", "9007199254740994"))},
		{Config: replaced, Check: resource.ComposeAggregateTestCheckFunc(secretCheck, resource.TestCheckResourceAttr(address, "id", "9007199254740995"))},
		{Config: `provider "iwinv" {}
removed {
 from = iwinv_content_cache.test
 lifecycle { destroy = false }
}`},
		{Config: imported, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateId: "9007199254740995"},
		{Config: imported, PlanOnly: true},
		{Config: cacheConfig("tfexamplec", `["a.example.invalid"]`, `timeouts { read = "20s" }`, false), Check: secretCheck},
		{PreConfig: func() { a.mu.Lock(); defer a.mu.Unlock(); delete(a.rows, "9007199254740995") }, Config: cacheConfig("tfexampled", `["a.example.invalid"]`, "", true), Check: resource.ComposeAggregateTestCheckFunc(secretCheck, resource.TestCheckResourceAttr(address, "id", "9007199254740996"))},
	}})
	if a.creates != 4 || a.updates != 6 || a.deletes != 4 {
		t.Fatalf("unexpected writes %d/%d/%d", a.creates, a.updates, a.deletes)
	}
}
func TestProtocolContentCacheFailedSetupRecovery(t *testing.T) {
	cacheProtocolEnv(t)
	for _, receipt := range []bool{false, true} {
		a := newContentCacheAPI()
		message := "Initial cache referrers not applied"
		if receipt {
			a.malformedCreate = true
			message = "Unable to create iwinv content cache"
		} else {
			a.putErr = &client.Error{Kind: "transport"}
		}
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: cacheDestroyCheck(a), Steps: []resource.TestStep{{Config: cacheConfig("tfexamplea", `["a.example.invalid"]`, "", true), ExpectError: regexp.MustCompile(message)}}})
		if a.creates != 1 || a.deletes != 1 {
			t.Fatal("failed setup lost identity or replayed creation")
		}
	}
}
func TestProtocolContentCacheInvalidAndReplacementInputs(t *testing.T) {
	cacheProtocolEnv(t)
	for _, tc := range []struct{ config, message string }{
		{cacheConfig("tfexamplea", `[]`, "", false), "Initial cache password required"},
		{cacheConfig("bad", `[]`, "", true), "Invalid cache account"},
		{cacheConfig("tfexamplea", `[null]`, "", true), "Invalid cache referrers"},
		{cacheConfig("tfexamplea", `["*.example.invalid"]`, "", true), "Invalid cache referrers"},
		{cacheConfig("tfexamplea", `["https://example.invalid"]`, "", true), "Invalid cache referrers"},
		{cacheConfig("tfexamplea", `[]`, `timeouts { create = "0s" }`, true), "Invalid operation timeout"},
	} {
		a := newContentCacheAPI()
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: tc.config, ExpectError: regexp.MustCompile(tc.message)}}})
		if a.creates != 0 || a.updates != 0 || a.deletes != 0 {
			t.Fatal("invalid input made remote writes")
		}
	}
	for _, config := range []string{
		strings.Replace(cacheConfig("tfexamplea", `["a.example.invalid"]`, "", true), "password_wo_version = 1", "password_wo_version = 2", 1),
		cacheConfig("tfexamplea", `terraform_data.next.output`, "", true) + `resource "terraform_data" "next" { input = [] }`,
		cacheConfig("tfexamplea", `["a.example.invalid"]`, `description = terraform_data.next.output`, true) + `resource "terraform_data" "next" { input = "changed" }`,
	} {
		a := newContentCacheAPI()
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: cacheDestroyCheck(a), Steps: []resource.TestStep{{Config: cacheConfig("tfexamplea", `["a.example.invalid"]`, "", true)}, {Config: config, ExpectError: regexp.MustCompile("Cache replacement requires a new account")}}})
		if a.creates != 1 || a.deletes != 1 {
			t.Fatal("replacement guard caused writes")
		}
	}
}
func cacheTestState(t *testing.T, r *contentCacheResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	sr := frameworkresource.SchemaResponse{}
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &sr)
	tt := map[string]attr.Type{"create": types.StringType, "read": types.StringType, "update": types.StringType, "delete": types.StringType}
	tv := map[string]attr.Value{}
	for k := range tt {
		tv[k] = types.StringValue("10ms")
	}
	m := contentCacheModel{ID: types.StringValue("9007199254740993"), ProductID: types.StringValue("synthetic_cache"), Account: types.StringValue("tfexamplea"), Name: types.StringValue("tf-cache"), Description: types.StringValue(""), Referrers: types.SetValueMust(types.StringType, []attr.Value{}), FTPPassword: types.StringNull(), PasswordVersion: types.Int64Value(1), Status: types.StringValue("active"), IP: types.StringValue("192.0.2.8"), Domain: types.StringValue("tfexamplea.example.invalid"), ProductType: types.StringValue("SINGLE"), Timeouts: timeouts.Value{Object: types.ObjectValueMust(tt, tv)}}
	state := tfsdk.State{Schema: sr.Schema}
	if d := state.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	return state
}
func TestContentCacheFailureStateAndRetryBoundaries(t *testing.T) {
	for _, scenario := range []string{"generic-put", "busy-put-change", "busy-put-hide", "busy-put-read-error", "busy-put-timeout", "put-wait-timeout", "generic-delete", "busy-delete-change", "busy-delete-timeout", "delete-wait-timeout", "read-error", "unverified-missing"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			a := newContentCacheAPI()
			r := &contentCacheResource{service: &hosted.CacheService{API: a}, pollInterval: time.Millisecond}
			state := cacheTestState(t, r)
			_, err := r.service.Create(ctx, hosted.CacheInput{ProductID: "synthetic_cache", Account: "tfexamplea", Name: "tf-cache", FTPPassword: cacheFTPSecret})
			if err != nil {
				t.Fatal(err)
			}
			var m contentCacheModel
			state.Get(ctx, &m)
			if strings.Contains(scenario, "put") {
				switch scenario {
				case "generic-put":
					a.putErr = &client.Error{Kind: "http_status", Status: 404, Code: "NOT_FOUND"}
				case "busy-put-change":
					a.busyPuts = 1
					a.changeAfterBusy = true
				case "busy-put-hide":
					a.busyPuts = 1
					a.hideAfterBusy = true
				case "busy-put-read-error":
					a.busyPuts = 1
					a.failReadAfterBusy = true
				case "busy-put-timeout":
					a.busyPuts = 100
				case "put-wait-timeout":
					a.ignoreUpdate = true
				}
				m.Referrers = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("a.example.invalid")})
				plan := tfsdk.Plan{Schema: state.Schema}
				plan.Set(ctx, &m)
				resp := frameworkresource.UpdateResponse{State: state}
				r.Update(ctx, frameworkresource.UpdateRequest{State: state, Plan: plan}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
					t.Fatal("update failure erased or changed prior state")
				}
				if scenario != "busy-put-timeout" && a.updates != 1 {
					t.Fatal("uncertain/changed/accepted PUT was retried")
				}
			} else if strings.Contains(scenario, "delete") {
				switch scenario {
				case "generic-delete":
					a.deleteErr = &client.Error{Kind: "http_status", Status: 404, Code: "NOT_FOUND"}
				case "busy-delete-change":
					a.busyDeletes = 1
					a.changeAfterBusy = true
				case "busy-delete-timeout":
					a.busyDeletes = 100
				case "delete-wait-timeout":
					a.retainDeleted = true
				}
				resp := frameworkresource.DeleteResponse{State: state}
				r.Delete(ctx, frameworkresource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
					t.Fatal("delete failure lost prior state")
				}
				if scenario != "busy-delete-timeout" && a.deletes != 1 {
					t.Fatal("uncertain/changed/accepted DELETE was retried")
				}
			} else {
				if scenario == "read-error" {
					a.readErr = errors.New("synthetic failure")
				} else {
					a.hiddenReads = 1
					m.Status = types.StringNull()
					state.Set(ctx, &m)
				}
				resp := frameworkresource.ReadResponse{State: state}
				r.Read(ctx, frameworkresource.ReadRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
					t.Fatal("unverified absence/read error lost identity")
				}
			}
		})
	}
}
func TestContentCacheUnknownCreationInputs(t *testing.T) {
	for _, scenario := range []string{"password", "referrers", "referrer-member", "version", "timeouts"} {
		ctx := context.Background()
		a := newContentCacheAPI()
		r := &contentCacheResource{service: &hosted.CacheService{API: a}, pollInterval: time.Millisecond}
		state := cacheTestState(t, r)
		var m contentCacheModel
		state.Get(ctx, &m)
		m.FTPPassword = types.StringValue(cacheFTPSecret)
		switch scenario {
		case "password":
			m.FTPPassword = types.StringUnknown()
		case "referrers":
			m.Referrers = types.SetUnknown(types.StringType)
		case "referrer-member":
			m.Referrers = types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()})
		case "version":
			m.PasswordVersion = types.Int64Unknown()
		case "timeouts":
			m.Timeouts = timeouts.Value{Object: types.ObjectUnknown(m.Timeouts.AttributeTypes(ctx))}
		}
		configState := tfsdk.State{Schema: state.Schema}
		configState.Set(ctx, &m)
		plan := tfsdk.Plan{Schema: state.Schema}
		plan.Set(ctx, &m)
		resp := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
		r.Create(ctx, frameworkresource.CreateRequest{Plan: plan, Config: tfsdk.Config{Schema: state.Schema, Raw: configState.Raw}}, &resp)
		if !resp.Diagnostics.HasError() || a.creates != 0 {
			t.Fatal("unknown input reached create")
		}
	}
}

// Explicit Core replacement mechanisms are distinct from ordinary attribute
// changes. Show their destructive plan without executing it under a reused name.
func TestProtocolContentCacheExplicitReplacementPlans(t *testing.T) {
	cacheProtocolEnv(t)
	for _, mode := range []string{"taint", "replace-flag"} {
		t.Run(mode, func(t *testing.T) {
			a := newContentCacheAPI()
			config := cacheConfig("tfexamplea", `[]`, "", true)
			const address = "iwinv_content_cache.test"
			step := resource.TestStep{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true, ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPreRefresh: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}}
			if mode == "taint" {
				step.Taint = []string{address}
			} else {
				step.PreConfig = func() { t.Setenv("TF_CLI_ARGS_plan", "-replace="+address) }
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: cacheDestroyCheck(a), Steps: []resource.TestStep{
				{Config: config}, step, {PreConfig: func() { t.Setenv("TF_CLI_ARGS_plan", "") }, Config: config, Destroy: true},
			}})
			if a.creates != 1 || a.deletes != 1 {
				t.Fatal("explicit replacement plan executed an unintended create")
			}
		})
	}
}

func TestProtocolContentCacheUnknownReferrerReplacement(t *testing.T) {
	cacheProtocolEnv(t)
	a := newContentCacheAPI()
	config := cacheConfig("tfexampleb", `terraform_data.next.output`, "", true) + `resource "terraform_data" "next" { input = [] }`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: cacheDestroyCheck(a), Steps: []resource.TestStep{
		{Config: cacheConfig("tfexamplea", `["a.example.invalid"]`, "", true)},
		{Config: config, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("iwinv_content_cache.test", plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.TestCheckResourceAttr("iwinv_content_cache.test", "allowed_referrers.#", "0")},
		{Config: config, PlanOnly: true},
	}})
	if a.creates != 2 || a.updates != 1 || a.deletes != 2 {
		t.Fatal("unknown referrer replacement made unexpected writes")
	}
}

func TestContentCacheDelayedCreateKeepsIdentityUntilCleanup(t *testing.T) {
	ctx := context.Background()
	a := newContentCacheAPI()
	a.hiddenAfterCreate = 100
	r := &contentCacheResource{service: &hosted.CacheService{API: a}, pollInterval: time.Millisecond}
	state := cacheTestState(t, r)
	var m contentCacheModel
	state.Get(ctx, &m)
	m.FTPPassword = types.StringValue(cacheFTPSecret)
	configState := tfsdk.State{Schema: state.Schema}
	configState.Set(ctx, &m)
	config := tfsdk.Config{Schema: state.Schema, Raw: configState.Raw}
	m.FTPPassword = types.StringNull()
	m.ID = types.StringUnknown()
	plan := tfsdk.Plan{Schema: state.Schema}
	plan.Set(ctx, &m)
	created := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, frameworkresource.CreateRequest{Config: config, Plan: plan}, &created)
	if !created.Diagnostics.HasError() {
		t.Fatal("delayed creation unexpectedly verified")
	}
	var saved contentCacheModel
	created.State.Get(ctx, &saved)
	if saved.ID.ValueString() != "9007199254740993" || !saved.Status.IsNull() || !saved.FTPPassword.IsNull() || a.creates != 1 {
		t.Fatal("failed creation identity or write-only state invalid")
	}
	read := frameworkresource.ReadResponse{State: created.State}
	r.Read(ctx, frameworkresource.ReadRequest{State: created.State}, &read)
	if !read.Diagnostics.HasError() || !read.State.Raw.Equal(created.State.Raw) {
		t.Fatal("post-failure empty list lost creation identity")
	}
	deleted := frameworkresource.DeleteResponse{State: created.State}
	r.Delete(ctx, frameworkresource.DeleteRequest{State: created.State}, &deleted)
	if deleted.Diagnostics.HasError() || a.deletes != 1 || len(a.rows) != 0 {
		t.Fatal("known hidden create was not explicitly cleaned up")
	}
}
