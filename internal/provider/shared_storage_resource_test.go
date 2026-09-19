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

type storageAPI struct {
	mu                           sync.Mutex
	rows                         map[string]map[string]any
	accounts                     map[string]bool
	creates, updates, deletes    int
	malformedCreate              bool
	hiddenReads                  int
	readErr, writeErr, deleteErr error
	retainDeleted, ignoreUpdate  bool
}

func newStorageAPI() *storageAPI {
	return &storageAPI{rows: map[string]map[string]any{}, accounts: map[string]bool{}}
}
func storageEnvelope(v any) client.Envelope {
	b, _ := json.Marshal(v)
	return client.Envelope{Status: 200, Result: b}
}
func (a *storageAPI) Get(_ context.Context, p string, _ url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.readErr != nil {
		return client.Envelope{}, a.readErr
	}
	if p != "/v1/apinas" {
		return client.Envelope{}, errors.New("unexpected NAS read")
	}
	rows := []map[string]any{}
	if a.hiddenReads > 0 {
		a.hiddenReads--
		return storageEnvelope(rows), nil
	}
	ids := []string{}
	for id := range a.rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rows = append(rows, a.rows[id])
	}
	return storageEnvelope(rows), nil
}
func (a *storageAPI) PostJSON(_ context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.creates++
	if a.writeErr != nil {
		return client.Envelope{}, a.writeErr
	}
	if p != "/v1/apinas" {
		return client.Envelope{}, errors.New("unexpected NAS create")
	}
	b := body.(map[string]any)
	account := b["sharename"].(string)
	if a.accounts[account] {
		return client.Envelope{}, errors.New("synthetic account reuse rejected")
	}
	a.accounts[account] = true
	id := strconv.FormatInt(9007199254740992+int64(a.creates), 10)
	number, _ := strconv.ParseInt(id, 10, 64)
	desc := ""
	if v, ok := b["description"]; ok {
		desc = v.(string)
	}
	row := map[string]any{"service_idx": number, "product_id": b["product_id"], "name": b["name"], "description": desc, "status": "active", "spec": map[string]any{"disk": b["hdd"]}, "domain": "nas.example.invalid", "mount_info": "nas.example.invalid:/opaque/path", "allowip": b["allowip"]}

	a.rows[id] = row
	if a.malformedCreate {
		return storageEnvelope(map[string]any{"service_idx": number}), nil
	}
	receipt := map[string]any{}
	for k, v := range row {
		if k != "mount_info" {
			receipt[k] = v
		}
	}
	receipt["status"] = "pending"
	return storageEnvelope(receipt), nil
}
func (a *storageAPI) PutJSON(_ context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.updates++
	if a.writeErr != nil {
		return client.Envelope{}, a.writeErr
	}
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/apinas/"), "/allowip")
	if p != "/v1/apinas/"+id+"/allowip" || a.rows[id] == nil {
		return client.Envelope{}, errors.New("unexpected NAS update")
	}
	ips := body.(map[string]any)["allowip"].(map[string]string)
	if len(ips) == 0 {
		return client.Envelope{}, errors.New("empty clearing unsupported")
	}
	if !a.ignoreUpdate {
		cloned := map[string]string{}
		for k, v := range ips {
			cloned[k] = v
		}
		a.rows[id]["allowip"] = cloned
	}
	ack := []map[string]string{}
	for ip, mode := range ips {
		ack = append(ack, map[string]string{"ip": ip, "acl": mode})
	}
	return storageEnvelope(ack), nil
}
func (a *storageAPI) Delete(_ context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deletes++
	if a.deleteErr != nil {
		return client.Envelope{}, a.deleteErr
	}
	id := strings.TrimPrefix(p, "/v1/apinas/")
	if a.rows[id] == nil {
		return client.Envelope{}, errors.New("synthetic NAS missing")
	}
	if !a.retainDeleted {
		delete(a.rows, id)
	}
	return storageEnvelope("accepted"), nil
}
func storageConfig(account, ips, extra string) string {
	history := ""
	if account != "" {
		history = fmt.Sprintf("share_name = %q\n", account)
	}
	return `provider "iwinv" {}
resource "iwinv_shared_storage" "test" {
 product_id = "synthetic_nas"
 name = "tf-nas-basic"
 size_gb = 100
 ` + history + ` allowed_ips = ` + ips + "\n" + extra + "\n}\n"
}
func storageProtocolEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
}
func storageDestroyCheck(a *storageAPI) resource.TestCheckFunc {
	return func(*terraform.State) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(a.rows) > 0 {
			return errors.New("owned NAS remained")
		}
		return nil
	}
}
func TestProtocolSharedStorageLifecycle(t *testing.T) {
	storageProtocolEnv(t)
	a := newStorageAPI()
	address := "iwinv_shared_storage.test"
	id := ""
	initial := storageConfig("tfexamplea", `{"192.0.2.1" = "RO"}`, "")
	updated := storageConfig("tfexamplea", `{"192.0.2.3" = "RO", "192.0.2.2" = "RW"}`, "")
	capture := func(s *terraform.State) error { id = s.RootModule().Resources[address].Primary.ID; return nil }
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: storageDestroyCheck(a), Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "description", ""), resource.TestCheckResourceAttr(address, "id", "9007199254740993"), resource.TestCheckResourceAttr(address, "size_gb", "100"), resource.TestCheckResourceAttr(address, "address", "nas.example.invalid"))},
		{Config: initial, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "id", "9007199254740993"), resource.TestCheckResourceAttr(address, "allowed_ips.%", "2"))},
		{Config: storageConfig("tfexamplea", `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, ""), PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: updated, ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"share_name"}},
		{Config: updated, PreConfig: func() { a.mu.Lock(); defer a.mu.Unlock(); a.rows[id]["allowip"] = map[string]string{"192.0.2.4": "RO"} }, Check: resource.TestCheckResourceAttr(address, "allowed_ips.%", "2")},
		{Config: storageConfig("tfexamplea", `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, `timeouts { update = "2m" }`), Check: func(*terraform.State) error {
			if a.updates != 2 {
				return errors.New("timeout-only update wrote to API")
			}
			return nil
		}},
		{Config: storageConfig("tfexamplea", `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, `description = "changed"`), ExpectError: regexp.MustCompile("NAS replacement requires a new share name")},
		{Config: storageConfig("tfexampleb", `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, `description = "설명 &amp; + %"`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: capture},
		{Config: storageConfig("tfexampleb", `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, `description = "설명 &amp; + %"`), PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: `removed {
 from = iwinv_shared_storage.test
 lifecycle { destroy = false }
}
provider "iwinv" {}`},
		{Config: storageConfig("", `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, `description = "설명 &amp; + %"`), ResourceName: address, ImportState: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return id, nil }, ImportStatePersist: true},
		{Config: storageConfig("", `{"192.0.2.2" = "RW", "192.0.2.3" = "RO"}`, `description = "설명 &amp; + %"`), PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: storageConfig("tfexamplec", `{"192.0.2.1" = "RO"}`, ""), PreConfig: func() { a.mu.Lock(); defer a.mu.Unlock(); delete(a.rows, id) }, Check: capture},
	}})
	if a.creates != 3 || a.updates != 2 || a.deletes != 2 {
		t.Fatal("unexpected NAS lifecycle operation counts")
	}
}
func TestProtocolSharedStorageFailedCreateRecovery(t *testing.T) {
	storageProtocolEnv(t)
	a := newStorageAPI()
	a.malformedCreate = true
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: storageDestroyCheck(a), Steps: []resource.TestStep{{Config: storageConfig("tfexamplea", `{"192.0.2.1" = "RO"}`, ""), ExpectError: regexp.MustCompile("Unable to create iwinv NAS")}}})
	if a.creates != 1 || a.deletes != 1 {
		t.Fatal("failed create was replayed or its ID was not cleaned up")
	}
}
func TestProtocolSharedStorageInvalidInputs(t *testing.T) {
	storageProtocolEnv(t)
	for _, tc := range []struct{ config, message string }{
		{storageConfig("", `{"192.0.2.1" = "RO"}`, ""), "NAS creation share name required"},
		{storageConfig("bad!23", `{"192.0.2.1" = "RO"}`, ""), "Invalid NAS share name"},
		{storageConfig("tfexamplea", `{}`, ""), "Invalid NAS allowed IPs"},
		{storageConfig("tfexamplea", `{"192.0.2.1/32" = "RO"}`, ""), "Invalid NAS allowed IPs"},
		{storageConfig("tfexamplea", `{"192.0.2.1" = null}`, ""), "Invalid NAS allowed IPs"},
		{storageConfig("tfexamplea", `{"192.0.2.1" = "RO"}`, `timeouts { create = "0s" }`), "Invalid operation timeout"},
	} {
		a := newStorageAPI()
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: tc.config, ExpectError: regexp.MustCompile(tc.message)}}})
		if a.creates != 0 || a.updates != 0 || a.deletes != 0 {
			t.Fatal("invalid config reached a write")
		}
	}
}
func TestProtocolSharedStorageUnknownReplacement(t *testing.T) {
	storageProtocolEnv(t)
	a := newStorageAPI()
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: storageDestroyCheck(a), Steps: []resource.TestStep{
		{Config: storageConfig("tfexamplea", `{"192.0.2.1" = "RO"}`, "")},
		{Config: storageConfig("tfexamplea", `{"192.0.2.1" = "RO"}`, `description = terraform_data.value.output`) + `resource "terraform_data" "value" { input = "changed" }`, ExpectError: regexp.MustCompile("NAS replacement requires a new share name")},
		{Config: storageConfig("tfexampleb", `{"192.0.2.1" = "RO"}`, `description = terraform_data.value.output`) + `resource "terraform_data" "value" { input = "changed" }`, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("iwinv_shared_storage.test", plancheck.ResourceActionDestroyBeforeCreate)}}},
	}})
	if a.creates != 2 || a.deletes != 2 {
		t.Fatal("unknown replacement plan changed during apply")
	}
}
func storageTestState(t *testing.T, r *sharedStorageResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	schemaResp := frameworkresource.SchemaResponse{}
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	tm := timeouts.Value{Object: types.ObjectValueMust(map[string]attr.Type{"create": types.StringType, "read": types.StringType, "update": types.StringType, "delete": types.StringType}, map[string]attr.Value{"create": types.StringValue("10ms"), "read": types.StringValue("10ms"), "update": types.StringValue("10ms"), "delete": types.StringValue("10ms")})}
	m := sharedStorageModel{ID: types.StringValue("9007199254740993"), ProductID: types.StringValue("synthetic_nas"), ShareName: types.StringValue("tfexamplea"), Name: types.StringValue("tf-nas-basic"), Description: types.StringValue(""), AllowedIPs: types.MapValueMust(types.StringType, map[string]attr.Value{"192.0.2.1": types.StringValue("RO")}), Status: types.StringValue("active"), SizeGB: types.Int64Value(100), Address: types.StringValue("nas.example.invalid"), MountInfo: types.StringValue("nas.example.invalid:/opaque/path"), Timeouts: tm}
	state := tfsdk.State{Schema: schemaResp.Schema}
	if d := state.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	return state
}
func TestSharedStorageFailureStateAndUnknownInputs(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"read-error", "unverified-missing", "update-error", "update-timeout", "delete-error", "delete-timeout"} {
		t.Run(kind, func(t *testing.T) {
			a := newStorageAPI()
			r := &sharedStorageResource{service: &hosted.NASService{API: a}, pollInterval: time.Millisecond}
			state := storageTestState(t, r)
			_, err := a.PostJSON(ctx, "/v1/apinas", map[string]any{"product_id": "synthetic_nas", "name": "tf-nas-basic", "sharename": "tfexamplea", "hdd": int64(100), "allowip": map[string]string{"192.0.2.1": "RO"}})
			if err != nil {
				t.Fatal(err)
			}
			var m sharedStorageModel
			state.Get(ctx, &m)
			switch kind {
			case "read-error", "unverified-missing":
				if kind == "read-error" {
					a.readErr = errors.New("synthetic 404")
				} else {
					a.rows = map[string]map[string]any{}
					m.Status = types.StringNull()
					state.Set(ctx, &m)
				}
				resp := frameworkresource.ReadResponse{State: state}
				r.Read(ctx, frameworkresource.ReadRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
					t.Fatal("unverified read lost state")
				}
			case "update-error", "update-timeout":
				m.AllowedIPs = types.MapValueMust(types.StringType, map[string]attr.Value{"192.0.2.2": types.StringValue("RW")})
				plan := tfsdk.Plan{Schema: state.Schema}
				plan.Set(ctx, &m)
				if kind == "update-error" {
					a.writeErr = errors.New("synthetic failed update")
				} else {
					a.ignoreUpdate = true
				}
				resp := frameworkresource.UpdateResponse{State: state}
				r.Update(ctx, frameworkresource.UpdateRequest{State: state, Plan: plan}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) || a.updates != 1 {
					t.Fatal("update failure lost state or replayed write")
				}
			default:
				if kind == "delete-error" {
					a.deleteErr = errors.New("synthetic delete error")
				} else {
					a.retainDeleted = true
				}
				resp := frameworkresource.DeleteResponse{State: state}
				r.Delete(ctx, frameworkresource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) || a.deletes != 1 {
					t.Fatal("delete failure lost state or replayed write")
				}
			}
		})
	}
	for _, kind := range []string{"account", "set", "member", "capacity", "timeouts"} {
		t.Run("unknown-"+kind, func(t *testing.T) {
			a := newStorageAPI()
			r := &sharedStorageResource{service: &hosted.NASService{API: a}, pollInterval: time.Millisecond}
			state := storageTestState(t, r)
			var m sharedStorageModel
			state.Get(ctx, &m)
			switch kind {
			case "account":
				m.ShareName = types.StringUnknown()
			case "set":
				m.AllowedIPs = types.MapUnknown(types.StringType)
			case "member":
				m.AllowedIPs = types.MapValueMust(types.StringType, map[string]attr.Value{"192.0.2.1": types.StringUnknown()})
			case "capacity":
				m.SizeGB = types.Int64Unknown()
			case "timeouts":
				m.Timeouts.Object = types.ObjectUnknown(m.Timeouts.AttributeTypes(ctx))
			}
			plan := tfsdk.Plan{Schema: state.Schema}
			plan.Set(ctx, &m)
			resp := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
			r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &resp)
			if !resp.Diagnostics.HasError() || a.creates != 0 {
				t.Fatal("unknown input reached create")
			}
		})
	}
}

func TestSharedStorageDelayedCreationPreservesIdentity(t *testing.T) {
	ctx := context.Background()
	a := newStorageAPI()
	a.hiddenReads = 100
	r := &sharedStorageResource{service: &hosted.NASService{API: a}, pollInterval: time.Millisecond}
	state := storageTestState(t, r)
	var m sharedStorageModel
	state.Get(ctx, &m)
	m.ID = types.StringUnknown()
	plan := tfsdk.Plan{Schema: state.Schema}
	plan.Set(ctx, &m)
	created := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &created)
	var saved sharedStorageModel
	created.State.Get(ctx, &saved)
	if !created.Diagnostics.HasError() || saved.ID.ValueString() != "9007199254740993" || !saved.Status.IsNull() || a.creates != 1 {
		t.Fatal("failed delayed creation lost identity")
	}
	read := frameworkresource.ReadResponse{State: created.State}
	r.Read(ctx, frameworkresource.ReadRequest{State: created.State}, &read)
	if !read.Diagnostics.HasError() || !read.State.Raw.Equal(created.State.Raw) {
		t.Fatal("empty post-create list erased unverified identity")
	}
	deleted := frameworkresource.DeleteResponse{State: created.State}
	r.Delete(ctx, frameworkresource.DeleteRequest{State: created.State}, &deleted)
	if deleted.Diagnostics.HasError() || a.deletes != 1 || len(a.rows) != 0 {
		t.Fatal("known hidden creation was not explicitly cleaned up")
	}
}
func TestProtocolSharedStorageExplicitReplacementPlans(t *testing.T) {
	storageProtocolEnv(t)
	for _, mode := range []string{"taint", "replace-flag"} {
		t.Run(mode, func(t *testing.T) {
			a := newStorageAPI()
			config := storageConfig("tfexamplea", `{"192.0.2.1" = "RO"}`, "")
			address := "iwinv_shared_storage.test"
			step := resource.TestStep{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true, ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPreRefresh: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}}
			if mode == "taint" {
				step.Taint = []string{address}
			} else {
				step.PreConfig = func() { t.Setenv("TF_CLI_ARGS_plan", "-replace="+address) }
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: storageDestroyCheck(a), Steps: []resource.TestStep{{Config: config}, step, {Config: config, Destroy: true, PreConfig: func() { t.Setenv("TF_CLI_ARGS_plan", "") }}}})
			if a.creates != 1 || a.deletes != 1 {
				t.Fatal("explicit plan unexpectedly recreated an account")
			}
		})
	}
}

func TestProtocolSharedStorageCapacityAndPermissions(t *testing.T) {
	storageProtocolEnv(t)
	a := newStorageAPI()
	address := "iwinv_shared_storage.test"
	initial := storageConfig("tfexamplea", `{"192.0.2.1" = "RO"}`, "")
	unknownPermission := storageConfig("tfexamplea", `{"192.0.2.1" = terraform_data.role.output}`, "") + `resource "terraform_data" "role" { input = "RW" }`
	resized := strings.Replace(storageConfig("tfexampleb", `{"192.0.2.1" = "RW"}`, `lifecycle { create_before_destroy = true }`), "size_gb = 100", "size_gb = terraform_data.capacity.output", 1) + `resource "terraform_data" "capacity" { input = 200 }
resource "terraform_data" "role" { input = "RW" }`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: storageDestroyCheck(a), Steps: []resource.TestStep{
		{Config: initial},
		{Config: unknownPermission, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "id", "9007199254740993"), resource.TestCheckResourceAttr(address, "allowed_ips.192.0.2.1", "RW"))},
		{Config: strings.Replace(initial, "size_gb = 100", "size_gb = 200", 1), ExpectError: regexp.MustCompile("NAS replacement requires a new share name")},
		{Config: resized, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionCreateBeforeDestroy)}}, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "size_gb", "200"), resource.TestCheckResourceAttr(address, "id", "9007199254740994"))},
		{Config: resized, PlanOnly: true},
	}})
	if a.creates != 2 || a.updates != 1 || a.deletes != 2 {
		t.Fatal("capacity replacement or permission update write count changed")
	}
	for _, bad := range []string{"99", "2001", "0"} {
		a := newStorageAPI()
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: strings.Replace(initial, "size_gb = 100", "size_gb = "+bad, 1), ExpectError: regexp.MustCompile("Invalid NAS capacity")}}})
		if a.creates != 0 {
			t.Fatal("invalid capacity reached API")
		}
	}
	for _, bad := range []string{`{"192.0.2.1" = "rw"}`, `{"2001:db8::1" = "RO"}`, `{"192.0.2.1" = "ADMIN"}`} {
		a := newStorageAPI()
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: storageConfig("tfexamplea", bad, ""), ExpectError: regexp.MustCompile("Invalid NAS allowed IPs")}}})
		if a.creates != 0 {
			t.Fatal("invalid permissions reached API")
		}
	}
}
func TestSharedStorageConcurrentChangeStopsUpdate(t *testing.T) {
	for _, mode := range []string{"changed", "missing", "read-error"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			a := newStorageAPI()
			r := &sharedStorageResource{service: &hosted.NASService{API: a}, pollInterval: time.Millisecond}
			state := storageTestState(t, r)
			_, err := a.PostJSON(ctx, "/v1/apinas", map[string]any{"product_id": "synthetic_nas", "name": "tf-nas-basic", "sharename": "tfexamplea", "hdd": int64(100), "allowip": map[string]string{"192.0.2.1": "RO"}})
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "changed":
				a.rows["9007199254740993"]["name"] = "externally-changed"
			case "missing":
				a.rows = map[string]map[string]any{}
			case "read-error":
				a.readErr = errors.New("synthetic read failure")
			}
			var m sharedStorageModel
			state.Get(ctx, &m)
			m.AllowedIPs = types.MapValueMust(types.StringType, map[string]attr.Value{"192.0.2.1": types.StringValue("RW")})
			plan := tfsdk.Plan{Schema: state.Schema}
			plan.Set(ctx, &m)
			resp := frameworkresource.UpdateResponse{State: state}
			r.Update(ctx, frameworkresource.UpdateRequest{State: state, Plan: plan}, &resp)
			if !resp.Diagnostics.HasError() || a.updates != 0 || !resp.State.Raw.Equal(state.Raw) {
				t.Fatal("concurrent parent change led to write or state loss")
			}
		})
	}
}
