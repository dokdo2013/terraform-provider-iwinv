package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// This fake models the observed wire contract, including description-only HTML
// escaping. It has no credentials, network transport or account fixtures.
type groupAPI struct {
	mu                        sync.Mutex
	groups                    map[string]map[string]string
	creates, updates, deletes int
	readError, writeError     error
	malformedCreate           bool
	hiddenReads               int
	retainDeleted             bool
	afterDeleteReadError      bool
}

func newGroupAPI() *groupAPI { return &groupAPI{groups: map[string]map[string]string{}} }
func groupEnvelope(id string, body map[string]string) client.Envelope {
	rows := []map[string]string{}
	if body != nil {
		rows = append(rows, map[string]string{"firewall_id": id, "title": body["title"], "content": html.EscapeString(body["content"]), "icmp": body["icmp"]})
	}
	b, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: b, Count: json.RawMessage(fmt.Sprint(len(rows)))}
}
func (a *groupAPI) Get(_ context.Context, p string, _ url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.readError != nil {
		return client.Envelope{}, a.readError
	}
	if a.hiddenReads > 0 {
		a.hiddenReads--
		return groupEnvelope("", nil), nil
	}
	return groupEnvelope(strings.TrimPrefix(p, "/v1/security-groups/"), a.groups[strings.TrimPrefix(p, "/v1/security-groups/")]), nil
}
func (a *groupAPI) PostJSON(_ context.Context, p string, b any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.creates++
	if a.writeError != nil {
		return client.Envelope{}, a.writeError
	}
	if p != "/v1/security-groups" {
		return client.Envelope{}, errors.New("unexpected create path")
	}
	id := fmt.Sprintf("FIREWALL-synthetic-%d", a.creates)
	a.groups[id] = map[string]string{}
	for k, v := range b.(map[string]string) {
		a.groups[id][k] = v
	}
	e := groupEnvelope(id, a.groups[id])
	if a.malformedCreate {
		e.Count = json.RawMessage(`99`)
	}
	return e, nil
}
func (a *groupAPI) PutJSON(_ context.Context, p string, b any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.updates++
	if a.writeError != nil {
		return client.Envelope{}, a.writeError
	}
	id := strings.TrimPrefix(p, "/v1/security-groups/")
	if a.groups[id] == nil {
		return client.Envelope{}, errors.New("synthetic group missing")
	}
	for k, v := range b.(map[string]string) {
		a.groups[id][k] = v
	}
	return groupEnvelope(id, a.groups[id]), nil
}
func (a *groupAPI) Delete(_ context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deletes++
	if a.writeError != nil {
		return client.Envelope{}, a.writeError
	}
	if !a.retainDeleted {
		delete(a.groups, strings.TrimPrefix(p, "/v1/security-groups/"))
	}
	if a.afterDeleteReadError {
		a.readError = errors.New("synthetic post-delete read error")
	}
	return client.Envelope{Status: 200, Result: json.RawMessage(`"deleted"`)}, nil
}
func groupFactories(a compute.API) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(&IwinvProvider{version: "test", newClient: func(string, string) (compute.API, error) { return a, nil }})}
}
func groupConfig(name, description string, icmp bool) string {
	return fmt.Sprintf("provider \"iwinv\" {}\nresource \"iwinv_security_group\" \"test\" {\n name = %q\n description = %q\n allow_icmp = %t\n}\n", name, description, icmp)
}
func TestProtocolSecurityGroupLifecycle(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newGroupAPI()
	initial := groupConfig("tf-synthetic", "한글 &amp; <value> + %", false)
	updated := groupConfig("tf-renamed", "수정 &#39; &lt;", true)
	const address = "iwinv_security_group.test"
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(a.groups) != 0 {
			return errors.New("synthetic group leaked")
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "id", "FIREWALL-synthetic-1"), resource.TestCheckResourceAttr(address, "description", "한글 &amp; <value> + %"))},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr(address, "id", "FIREWALL-synthetic-1"), resource.TestCheckResourceAttr(address, "name", "tf-renamed"), resource.TestCheckResourceAttr(address, "allow_icmp", "true"))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{Config: groupForgetConfig},
		{Config: updated, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateId: "FIREWALL-synthetic-1"},
		{Config: updated, PlanOnly: true},
		{PreConfig: func() { a.mu.Lock(); defer a.mu.Unlock(); a.groups["FIREWALL-synthetic-1"]["title"] = "external-drift" }, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: resource.TestCheckResourceAttr(address, "name", "tf-renamed")},
		{PreConfig: func() { a.mu.Lock(); defer a.mu.Unlock(); delete(a.groups, "FIREWALL-synthetic-1") }, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: resource.TestCheckResourceAttr(address, "id", "FIREWALL-synthetic-2")},
		{Config: updated, PlanOnly: true},
	}})
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.creates != 2 || a.updates != 2 || a.deletes != 1 {
		t.Fatalf("unexpected writes: create=%d update=%d delete=%d", a.creates, a.updates, a.deletes)
	}
}
func TestProtocolSecurityGroupDefaultsAndTimeouts(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newGroupAPI()
	config := `provider "iwinv" {}
resource "iwinv_security_group" "test" {
 name = "tf-defaults"
 timeouts {
  create = "20s"
  read = "10s"
  update = "20s"
  delete = "20s"
 }
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("iwinv_security_group.test", "description", "Managed by Terraform"), resource.TestCheckResourceAttr("iwinv_security_group.test", "allow_icmp", "false"))},
		{Config: config, PlanOnly: true},
		{Config: strings.ReplaceAll(config, "20s", "30s"), Check: resource.TestCheckResourceAttr("iwinv_security_group.test", "timeouts.create", "30s")},
	}})
	if a.creates != 1 || a.updates != 0 || a.deletes != 1 {
		t.Fatal("timeout-only update issued a remote write")
	}
}
func TestProtocolSecurityGroupInvalidDescription(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newGroupAPI()
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: groupConfig("tf-empty", "", false), ExpectError: regexp.MustCompile("Invalid security group text")}}})
	if a.creates != 0 {
		t.Fatal("invalid configuration performed a write")
	}
}

func groupTestState(t *testing.T, r *securityGroupResource, id string) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	var s frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &s)
	timeoutTypes := map[string]attr.Type{"create": types.StringType, "read": types.StringType, "update": types.StringType, "delete": types.StringType}
	values := map[string]attr.Value{}
	for key := range timeoutTypes {
		values[key] = types.StringValue("10ms")
	}
	m := securityGroupModel{ID: types.StringValue(id), Name: types.StringValue("tf-test"), Description: types.StringValue("nonempty"), AllowICMP: types.BoolValue(false), Timeouts: timeouts.Value{Object: types.ObjectValueMust(timeoutTypes, values)}}
	state := tfsdk.State{Schema: s.Schema}
	if d := state.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	return state
}
func assertGroupState(t *testing.T, state tfsdk.State, id, name string) {
	t.Helper()
	var m securityGroupModel
	if d := state.Get(context.Background(), &m); d.HasError() {
		t.Fatal(d)
	}
	if m.ID.ValueString() != id || m.Name.ValueString() != name {
		t.Fatal("resource identity/prior values not preserved")
	}
}
func TestSecurityGroupFailureState(t *testing.T) {
	for _, scenario := range []string{"malformed-create", "create-read-error", "create-timeout", "create-hidden-then-visible", "update-error", "update-read-error", "delete-error", "delete-timeout", "delete-read-error", "delete-post-read-error", "read-error", "read-absent", "delete-absent"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			a := newGroupAPI()
			r := &securityGroupResource{network: &network.Service{API: a}, pollInterval: time.Millisecond}
			state := groupTestState(t, r, "FIREWALL-synthetic-1")
			plan := tfsdk.Plan{Schema: state.Schema, Raw: state.Raw}
			if strings.HasPrefix(scenario, "create") || scenario == "malformed-create" {
				switch scenario {
				case "malformed-create":
					a.malformedCreate = true
				case "create-read-error":
					a.readError = errors.New("synthetic read error")
				case "create-timeout":
					a.hiddenReads = 1000
				case "create-hidden-then-visible":
					a.hiddenReads = 2
				}
				resp := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
				r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &resp)
				if resp.Diagnostics.HasError() != (scenario != "create-hidden-then-visible") {
					t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
				}
				assertGroupState(t, resp.State, "FIREWALL-synthetic-1", "tf-test")
				if a.creates != 1 {
					t.Fatal("create was replayed")
				}
				return
			}
			a.groups["FIREWALL-synthetic-1"] = map[string]string{"title": "tf-test", "content": "nonempty", "icmp": "N"}
			if strings.HasPrefix(scenario, "update") {
				var m securityGroupModel
				plan.Get(ctx, &m)
				m.Name = types.StringValue("tf-updated")
				plan.Set(ctx, &m)
				if scenario == "update-error" {
					a.writeError = errors.New("synthetic write error")
				} else {
					a.readError = errors.New("synthetic read error")
				}
				resp := frameworkresource.UpdateResponse{State: state}
				r.Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: state}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("uncertain update accepted")
				}
				assertGroupState(t, resp.State, "FIREWALL-synthetic-1", "tf-test")
				if a.updates != 1 {
					t.Fatal("update replayed")
				}
				return
			}
			if strings.HasPrefix(scenario, "delete") {
				switch scenario {
				case "delete-error":
					a.writeError = errors.New("synthetic write error")
				case "delete-timeout":
					a.retainDeleted = true
				case "delete-post-read-error":
					a.afterDeleteReadError = true
				case "delete-read-error":
					a.readError = errors.New("synthetic read error")
				case "delete-absent":
					delete(a.groups, "FIREWALL-synthetic-1")
				}
				resp := frameworkresource.DeleteResponse{State: state}
				r.Delete(ctx, frameworkresource.DeleteRequest{State: state}, &resp)
				if resp.Diagnostics.HasError() != (scenario != "delete-absent") {
					t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
				}
				if scenario != "delete-absent" {
					assertGroupState(t, resp.State, "FIREWALL-synthetic-1", "tf-test")
				}
				expected := 1
				if scenario == "delete-absent" || scenario == "delete-read-error" {
					expected = 0
				}
				if a.deletes != expected {
					t.Fatal("unsafe/replayed delete")
				}
				return
			}
			if scenario == "read-error" {
				a.readError = errors.New("synthetic read error")
			} else {
				delete(a.groups, "FIREWALL-synthetic-1")
			}
			resp := frameworkresource.ReadResponse{State: state}
			r.Read(ctx, frameworkresource.ReadRequest{State: state}, &resp)
			if scenario == "read-error" {
				if !resp.Diagnostics.HasError() {
					t.Fatal("read error accepted")
				}
				assertGroupState(t, resp.State, "FIREWALL-synthetic-1", "tf-test")
			} else if !resp.State.Raw.IsNull() {
				t.Fatal("confirmed absence not removed")
			}
		})
	}
}

// Exercise Terraform Core's failure state, not just the Go response struct:
// a valid create ID survives an error and can be destroyed without another POST.
func TestProtocolSecurityGroupFailedCreateRecovery(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newGroupAPI()
	a.malformedCreate = true
	config := groupConfig("tf-failed-create", "nonempty", false)
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(a.groups) != 0 {
			return errors.New("failed create leaked")
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: config, ExpectError: regexp.MustCompile("Unable to create iwinv security group")},
		{Config: config, Destroy: true, Check: resource.TestCheckResourceAttr("iwinv_security_group.test", "id", "FIREWALL-synthetic-1")},
	}})
	if a.creates != 1 || a.deletes != 1 {
		t.Fatal("failure recovery lost identity or replayed create")
	}
}
func TestSecurityGroupReadErrorsPreserveState(t *testing.T) {
	for _, status := range []int{401, 403, 404, 429, 500, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			a := newGroupAPI()
			a.readError = &client.Error{Kind: "response", Status: status, Code: "NOT_FOUND"}
			r := &securityGroupResource{network: &network.Service{API: a}}
			state := groupTestState(t, r, "FIREWALL-synthetic-1")
			resp := frameworkresource.ReadResponse{State: state}
			r.Read(context.Background(), frameworkresource.ReadRequest{State: state}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("API error accepted as absence")
			}
			assertGroupState(t, resp.State, "FIREWALL-synthetic-1", "tf-test")
		})
	}
}
func TestSecurityGroupInvalidTimeoutBeforeWrite(t *testing.T) {
	for _, duration := range []string{"0s", "-1s", "invalid"} {
		t.Run(duration, func(t *testing.T) {
			a := newGroupAPI()
			r := &securityGroupResource{network: &network.Service{API: a}}
			ctx := context.Background()
			state := groupTestState(t, r, "FIREWALL-synthetic-1")
			var m securityGroupModel
			state.Get(ctx, &m)
			values := m.Timeouts.Attributes()
			values["create"] = types.StringValue(duration)
			m.Timeouts = timeouts.Value{Object: types.ObjectValueMust(m.Timeouts.AttributeTypes(ctx), values)}
			plan := tfsdk.Plan{Schema: state.Schema}
			if d := plan.Set(ctx, &m); d.HasError() {
				t.Fatal(d)
			}
			resp := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
			r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &resp)
			if !resp.Diagnostics.HasError() || a.creates != 0 {
				t.Fatal("invalid timeout reached a write")
			}
		})
	}
}
func TestSecurityGroupUnknownPlanBeforeWrite(t *testing.T) {
	a := newGroupAPI()
	r := &securityGroupResource{network: &network.Service{API: a}}
	ctx := context.Background()
	state := groupTestState(t, r, "FIREWALL-synthetic-1")
	var m securityGroupModel
	state.Get(ctx, &m)
	m.Name = types.StringUnknown()
	plan := tfsdk.Plan{Schema: state.Schema}
	if d := plan.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	resp := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &resp)
	if !resp.Diagnostics.HasError() || a.creates != 0 {
		t.Fatal("unknown configuration reached a write")
	}
}

// Explicitly relinquish only Terraform ownership, without deleting the object,
// before importing into the same persisted state for a subsequent no-op plan.
const groupForgetConfig = `provider "iwinv" {}
removed {
 from = iwinv_security_group.test
 lifecycle { destroy = false }
}`

func TestSecurityGroupImportIDValidation(t *testing.T) {
	for _, id := range []string{"", "name-not-id", "FIREWALL-", "FIREWALL-test/other", "FIREWALL-test?query", "FIREWALL-test%2Fother"} {
		t.Run(id, func(t *testing.T) {
			r := &securityGroupResource{}
			state := groupTestState(t, r, "FIREWALL-synthetic-1")
			resp := frameworkresource.ImportStateResponse{State: tfsdk.State{Schema: state.Schema}}
			r.ImportState(context.Background(), frameworkresource.ImportStateRequest{ID: id}, &resp)
			if !resp.Diagnostics.HasError() {
				t.Fatal("invalid import accepted")
			}
		})
	}
}

func TestProtocolSecurityGroupInvalidReadTimeout(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newGroupAPI()
	config := `provider "iwinv" {}
resource "iwinv_security_group" "test" {
 name = "tf-bad-timeout"
 timeouts {
  create = "1m"
  read = "0s"
 }
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: config, ExpectError: regexp.MustCompile("Invalid operation timeout")}}})
	if a.creates != 0 {
		t.Fatal("invalid future read timeout was persisted by create")
	}
}

func TestSecurityGroupUnknownTimeoutBeforeWrite(t *testing.T) {
	a := newGroupAPI()
	r := &securityGroupResource{network: &network.Service{API: a}}
	ctx := context.Background()
	state := groupTestState(t, r, "FIREWALL-synthetic-1")
	var m securityGroupModel
	state.Get(ctx, &m)
	m.Timeouts.Object = types.ObjectUnknown(m.Timeouts.AttributeTypes(ctx))
	plan := tfsdk.Plan{Schema: state.Schema}
	if d := plan.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &response)
	if !response.Diagnostics.HasError() || a.creates != 0 {
		t.Fatal("unknown timeouts reached a write and could invalidate returned state")
	}
}
