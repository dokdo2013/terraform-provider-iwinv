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

type dbAPI struct {
	mu                           sync.Mutex
	rows                         map[string]map[string]any
	accounts                     map[string]bool
	creates, updates, deletes    int
	malformedCreate              bool
	hiddenReads                  int
	readErr, writeErr, deleteErr error
	retainDeleted, ignoreUpdate  bool
}

func newDBAPI() *dbAPI { return &dbAPI{rows: map[string]map[string]any{}, accounts: map[string]bool{}} }
func dbEnvelope(v any) client.Envelope {
	b, _ := json.Marshal(v)
	return client.Envelope{Status: 200, Result: b}
}
func (a *dbAPI) Get(_ context.Context, p string, _ url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.readErr != nil {
		return client.Envelope{}, a.readErr
	}
	if p != "/v1/dbms" {
		return client.Envelope{}, errors.New("unexpected DBMS read")
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
func (a *dbAPI) PostJSON(_ context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.creates++
	if a.writeErr != nil {
		return client.Envelope{}, a.writeErr
	}
	if p != "/v1/dbms" {
		return client.Envelope{}, errors.New("unexpected DBMS create")
	}
	b := body.(map[string]any)
	account := b["id"].(string)
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
	row := map[string]any{"service_idx": number, "product_id": b["product_id"], "name": b["name"], "description": desc, "status": "active", "spec": map[string]any{"type": "STD", "ver": "7"}, "domain": map[string]string{"default": "db.example.invalid"}, "allowip": b["allowip"]}
	a.rows[id] = row
	if a.malformedCreate {
		return dbEnvelope(map[string]any{"service_idx": number}), nil
	}
	return dbEnvelope(map[string]any{"service_idx": number, "name": b["name"], "description": desc, "status": "WAIT", "domain": "db.example.invalid", "allowip": b["allowip"]}), nil
}
func (a *dbAPI) PutJSON(_ context.Context, p string, body any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.updates++
	if a.writeErr != nil {
		return client.Envelope{}, a.writeErr
	}
	id := strings.TrimSuffix(strings.TrimPrefix(p, "/v1/dbms/"), "/allowip")
	if p != "/v1/dbms/"+id+"/allowip" || a.rows[id] == nil {
		return client.Envelope{}, errors.New("unexpected DBMS update")
	}
	ips := body.(map[string]any)["allowip"].([]string)
	if len(ips) == 0 {
		return client.Envelope{}, errors.New("empty clearing unsupported")
	}
	if !a.ignoreUpdate {
		a.rows[id]["allowip"] = append([]string{}, ips...)
	}
	return dbEnvelope(ips), nil
}
func (a *dbAPI) Delete(_ context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deletes++
	if a.deleteErr != nil {
		return client.Envelope{}, a.deleteErr
	}
	id := strings.TrimPrefix(p, "/v1/dbms/")
	if a.rows[id] == nil {
		return client.Envelope{}, errors.New("synthetic DBMS missing")
	}
	if !a.retainDeleted {
		delete(a.rows, id)
	}
	return dbEnvelope("accepted"), nil
}
func dbConfig(account, ips, extra string) string {
	history := ""
	if account != "" {
		history = fmt.Sprintf("account_name = %q\n", account)
	}
	return `provider "iwinv" {}
resource "iwinv_db_instance" "test" {
 product_id = "synthetic_redis"
 name = "tf-dbms-basic"
 ` + history + ` allowed_ips = ` + ips + "\n" + extra + "\n}\n"
}
func dbProtocolEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
}
func dbDestroyCheck(a *dbAPI) resource.TestCheckFunc {
	return func(*terraform.State) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(a.rows) > 0 {
			return errors.New("owned DBMS remained")
		}
		return nil
	}
}
func TestProtocolDBInstanceLifecycle(t *testing.T) {
	dbProtocolEnv(t)
	a := newDBAPI()
	address := "iwinv_db_instance.test"
	id := ""
	initial := dbConfig("tfexamplea", `["192.0.2.1"]`, "")
	updated := dbConfig("tfexamplea", `["192.0.2.3","192.0.2.2"]`, "")
	capture := func(s *terraform.State) error { id = s.RootModule().Resources[address].Primary.ID; return nil }
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: dbDestroyCheck(a), Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "description", ""), resource.TestCheckResourceAttr(address, "id", "9007199254740993"), resource.TestCheckResourceAttr(address, "engine_version", "7"), resource.TestCheckResourceAttr(address, "address", "db.example.invalid"))},
		{Config: initial, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "id", "9007199254740993"), resource.TestCheckResourceAttr(address, "allowed_ips.#", "2"))},
		{Config: dbConfig("tfexamplea", `["192.0.2.2","192.0.2.3"]`, ""), PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: updated, ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"account_name", "timeouts"}},
		{Config: updated, PreConfig: func() { a.mu.Lock(); defer a.mu.Unlock(); a.rows[id]["allowip"] = []string{"192.0.2.4"} }, Check: resource.TestCheckResourceAttr(address, "allowed_ips.#", "2")},
		{Config: dbConfig("tfexamplea", `["192.0.2.2","192.0.2.3"]`, `timeouts { update = "2m" }`), Check: func(*terraform.State) error {
			if a.updates != 2 {
				return errors.New("timeout-only update wrote to API")
			}
			return nil
		}},
		{Config: dbConfig("tfexamplea", `["192.0.2.2","192.0.2.3"]`, `description = "changed"`), ExpectError: regexp.MustCompile("DBMS replacement requires a new account")},
		{Config: dbConfig("tfexampleb", `["192.0.2.2","192.0.2.3"]`, `description = "설명 &amp; + %"`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: capture},
		{Config: dbConfig("tfexampleb", `["192.0.2.2","192.0.2.3"]`, `description = "설명 &amp; + %"`), PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: `removed {
 from = iwinv_db_instance.test
 lifecycle { destroy = false }
}
provider "iwinv" {}`},
		{Config: dbConfig("", `["192.0.2.2","192.0.2.3"]`, `description = "설명 &amp; + %"`), ResourceName: address, ImportState: true, ImportStateIdFunc: func(*terraform.State) (string, error) { return id, nil }, ImportStatePersist: true},
		{Config: dbConfig("", `["192.0.2.2","192.0.2.3"]`, `description = "설명 &amp; + %"`), PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: dbConfig("tfexamplec", `["192.0.2.1"]`, ""), PreConfig: func() { a.mu.Lock(); defer a.mu.Unlock(); delete(a.rows, id) }, Check: capture},
	}})
	if a.creates != 3 || a.updates != 2 || a.deletes != 2 {
		t.Fatal("unexpected DBMS lifecycle operation counts")
	}
}
func TestProtocolDBInstanceFailedCreateRecovery(t *testing.T) {
	dbProtocolEnv(t)
	a := newDBAPI()
	a.malformedCreate = true
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: dbDestroyCheck(a), Steps: []resource.TestStep{{Config: dbConfig("tfexamplea", `["192.0.2.1"]`, ""), ExpectError: regexp.MustCompile("Unable to create iwinv DBMS")}}})
	if a.creates != 1 || a.deletes != 1 {
		t.Fatal("failed create was replayed or its ID was not cleaned up")
	}
}
func TestProtocolDBInstanceInvalidInputs(t *testing.T) {
	dbProtocolEnv(t)
	for _, tc := range []struct{ config, message string }{
		{dbConfig("", `["192.0.2.1"]`, ""), "DBMS creation account required"},
		{dbConfig("bad123", `["192.0.2.1"]`, ""), "Invalid DBMS account"},
		{dbConfig("tfexamplea", `[]`, ""), "Invalid DBMS allowed IPs"},
		{dbConfig("tfexamplea", `["192.0.2.1/32"]`, ""), "Invalid DBMS allowed IPs"},
		{dbConfig("tfexamplea", `[null]`, ""), "Invalid DBMS allowed IPs"},
		{dbConfig("tfexamplea", `["192.0.2.1"]`, `timeouts { create = "0s" }`), "Invalid operation timeout"},
	} {
		a := newDBAPI()
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: tc.config, ExpectError: regexp.MustCompile(tc.message)}}})
		if a.creates != 0 || a.updates != 0 || a.deletes != 0 {
			t.Fatal("invalid config reached a write")
		}
	}
}
func TestProtocolDBInstanceUnknownReplacement(t *testing.T) {
	dbProtocolEnv(t)
	a := newDBAPI()
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: dbDestroyCheck(a), Steps: []resource.TestStep{
		{Config: dbConfig("tfexamplea", `["192.0.2.1"]`, "")},
		{Config: dbConfig("tfexamplea", `["192.0.2.1"]`, `description = terraform_data.value.output`) + `resource "terraform_data" "value" { input = "changed" }`, ExpectError: regexp.MustCompile("DBMS replacement requires a new account")},
		{Config: dbConfig("tfexampleb", `["192.0.2.1"]`, `description = terraform_data.value.output`) + `resource "terraform_data" "value" { input = "changed" }`, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("iwinv_db_instance.test", plancheck.ResourceActionDestroyBeforeCreate)}}},
	}})
	if a.creates != 2 || a.deletes != 2 {
		t.Fatal("unknown replacement plan changed during apply")
	}
}
func dbTestState(t *testing.T, r *dbInstanceResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	schemaResp := frameworkresource.SchemaResponse{}
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResp)
	tm := timeouts.Value{Object: types.ObjectValueMust(map[string]attr.Type{"create": types.StringType, "read": types.StringType, "update": types.StringType, "delete": types.StringType}, map[string]attr.Value{"create": types.StringValue("10ms"), "read": types.StringValue("10ms"), "update": types.StringValue("10ms"), "delete": types.StringValue("10ms")})}
	m := dbInstanceModel{ID: types.StringValue("9007199254740993"), ProductID: types.StringValue("synthetic_redis"), Account: types.StringValue("tfexamplea"), Name: types.StringValue("tf-dbms-basic"), Description: types.StringValue(""), AllowedIPs: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("192.0.2.1")}), Status: types.StringValue("active"), ProductType: types.StringValue("STD"), EngineVersion: types.StringValue("7"), Address: types.StringValue("db.example.invalid"), Domains: types.MapValueMust(types.StringType, map[string]attr.Value{"default": types.StringValue("db.example.invalid")}), Timeouts: tm}
	state := tfsdk.State{Schema: schemaResp.Schema}
	if d := state.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	return state
}
func TestDBInstanceFailureStateAndUnknownInputs(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"read-error", "unverified-missing", "update-error", "update-timeout", "delete-error", "delete-timeout"} {
		t.Run(kind, func(t *testing.T) {
			a := newDBAPI()
			r := &dbInstanceResource{service: &hosted.DBMSService{API: a}, pollInterval: time.Millisecond}
			state := dbTestState(t, r)
			_, err := a.PostJSON(ctx, "/v1/dbms", map[string]any{"product_id": "synthetic_redis", "name": "tf-dbms-basic", "id": "tfexamplea", "allowip": []string{"192.0.2.1"}})
			if err != nil {
				t.Fatal(err)
			}
			var m dbInstanceModel
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
				m.AllowedIPs = types.SetValueMust(types.StringType, []attr.Value{types.StringValue("192.0.2.2")})
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
	for _, kind := range []string{"account", "set", "member", "timeouts"} {
		t.Run("unknown-"+kind, func(t *testing.T) {
			a := newDBAPI()
			r := &dbInstanceResource{service: &hosted.DBMSService{API: a}, pollInterval: time.Millisecond}
			state := dbTestState(t, r)
			var m dbInstanceModel
			state.Get(ctx, &m)
			switch kind {
			case "account":
				m.Account = types.StringUnknown()
			case "set":
				m.AllowedIPs = types.SetUnknown(types.StringType)
			case "member":
				m.AllowedIPs = types.SetValueMust(types.StringType, []attr.Value{types.StringUnknown()})
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

func TestDBInstanceDelayedCreationPreservesIdentity(t *testing.T) {
	ctx := context.Background()
	a := newDBAPI()
	a.hiddenReads = 100
	r := &dbInstanceResource{service: &hosted.DBMSService{API: a}, pollInterval: time.Millisecond}
	state := dbTestState(t, r)
	var m dbInstanceModel
	state.Get(ctx, &m)
	m.ID = types.StringUnknown()
	plan := tfsdk.Plan{Schema: state.Schema}
	plan.Set(ctx, &m)
	created := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &created)
	var saved dbInstanceModel
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
func TestProtocolDBInstanceExplicitReplacementPlans(t *testing.T) {
	dbProtocolEnv(t)
	for _, mode := range []string{"taint", "replace-flag"} {
		t.Run(mode, func(t *testing.T) {
			a := newDBAPI()
			config := dbConfig("tfexamplea", `["192.0.2.1"]`, "")
			address := "iwinv_db_instance.test"
			step := resource.TestStep{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true, ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPreRefresh: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}}
			if mode == "taint" {
				step.Taint = []string{address}
			} else {
				step.PreConfig = func() { t.Setenv("TF_CLI_ARGS_plan", "-replace="+address) }
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: dbDestroyCheck(a), Steps: []resource.TestStep{{Config: config}, step, {Config: config, Destroy: true, PreConfig: func() { t.Setenv("TF_CLI_ARGS_plan", "") }}}})
			if a.creates != 1 || a.deletes != 1 {
				t.Fatal("explicit plan unexpectedly recreated an account")
			}
		})
	}
}
