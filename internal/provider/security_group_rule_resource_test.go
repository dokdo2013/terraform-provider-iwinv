package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type ruleAPI struct {
	*groupAPI
	ruleMu                                sync.Mutex
	rules                                 map[string]map[string]map[string]string
	nextID                                int64
	malformedRuleCreate                   bool
	ruleReadError, ruleWriteError         error
	postDeleteReadError                   bool
	keepDeletedRule                       bool
	ruleCreates, ruleUpdates, ruleDeletes int
}

func newRuleAPI() *ruleAPI {
	return &ruleAPI{groupAPI: newGroupAPI(), rules: map[string]map[string]map[string]string{}, nextID: 9007199254740993}
}
func ruleAPIPath(p string) (string, string, bool) {
	parts := strings.Split(strings.TrimPrefix(p, "/v1/security-groups/"), "/")
	if len(parts) < 2 || parts[1] != "rules" {
		return "", "", false
	}
	id := ""
	if len(parts) == 3 {
		id = parts[2]
	}
	return parts[0], id, true
}
func ruleAPIEnvelope(rules map[string]map[string]string) client.Envelope {
	rows := []map[string]any{}
	for id, b := range rules {
		n, _ := strconv.ParseInt(id, 10, 64)
		var desc any
		if b["content"] != "" {
			desc = b["content"]
		}
		rows = append(rows, map[string]any{"rule_id": n, "bound": b["bound"], "protocol": b["protocol"], "port": b["port"], "ip": b["ip"], "title": b["title"], "content": desc})
	}
	b, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: b, Count: json.RawMessage(fmt.Sprint(len(rows)))}
}
func (a *ruleAPI) Get(ctx context.Context, p string, q url.Values) (client.Envelope, error) {
	group, _, ok := ruleAPIPath(p)
	if !ok {
		return a.groupAPI.Get(ctx, p, q)
	}
	a.ruleMu.Lock()
	defer a.ruleMu.Unlock()
	if a.ruleReadError != nil {
		return client.Envelope{}, a.ruleReadError
	}
	return ruleAPIEnvelope(a.rules[group]), nil
}
func (a *ruleAPI) PostJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	group, _, ok := ruleAPIPath(p)
	if !ok {
		return a.groupAPI.PostJSON(ctx, p, body)
	}
	a.ruleMu.Lock()
	defer a.ruleMu.Unlock()
	b := body.(map[string]string)
	if a.rules[group] == nil {
		a.rules[group] = map[string]map[string]string{}
	}
	for _, existing := range a.rules[group] {
		if sameRuleTuple(existing, b) {
			return client.Envelope{}, errors.New("synthetic duplicate rule")
		}
	}
	id := strconv.FormatInt(a.nextID, 10)
	a.nextID++
	a.ruleCreates++
	stored := map[string]string{}
	for k, v := range b {
		stored[k] = v
	}
	a.rules[group][id] = stored
	e := ruleAPIEnvelope(map[string]map[string]string{id: stored})
	if a.malformedRuleCreate {
		e.Count = json.RawMessage(`99`)
	}
	return e, nil
}
func sameRuleTuple(a, b map[string]string) bool {
	for _, k := range []string{"bound", "protocol", "port", "ip"} {
		if a[k] != b[k] {
			return false
		}
	}
	return true
}
func (a *ruleAPI) PutJSON(ctx context.Context, p string, body any) (client.Envelope, error) {
	group, id, ok := ruleAPIPath(p)
	if !ok {
		return a.groupAPI.PutJSON(ctx, p, body)
	}
	a.ruleMu.Lock()
	defer a.ruleMu.Unlock()
	if a.ruleWriteError != nil {
		return client.Envelope{}, a.ruleWriteError
	}
	stored := a.rules[group][id]
	if stored == nil {
		return client.Envelope{}, errors.New("synthetic rule missing")
	}
	a.ruleUpdates++
	for k, v := range body.(map[string]string) {
		if k == "content" && v == "" {
			continue
		}
		stored[k] = v
	}
	return ruleAPIEnvelope(map[string]map[string]string{id: stored}), nil
}
func (a *ruleAPI) Delete(ctx context.Context, p string) (client.Envelope, error) {
	group, id, ok := ruleAPIPath(p)
	a.ruleMu.Lock()
	defer a.ruleMu.Unlock()
	if !ok {
		group = strings.TrimPrefix(p, "/v1/security-groups/")
		if len(a.rules[group]) != 0 {
			return client.Envelope{}, errors.New("parent deleted before its independently managed rules")
		}
		delete(a.rules, group)
		return a.groupAPI.Delete(ctx, p)
	}
	a.ruleDeletes++
	if a.ruleWriteError != nil {
		return client.Envelope{}, a.ruleWriteError
	}
	if !a.keepDeletedRule {
		delete(a.rules[group], id)
	}
	if a.postDeleteReadError {
		a.ruleReadError = errors.New("synthetic post-delete read error")
	}
	return client.Envelope{Status: 200, Result: json.RawMessage(`"deleted"`)}, nil
}
func ruleConfig(parent, description, protocol string, from, to int, cidr string) string {
	return fmt.Sprintf(`provider "iwinv" {}
resource "iwinv_security_group" "first" { name = "tf-rule-parent-a" }
resource "iwinv_security_group" "second" { name = "tf-rule-parent-b" }
resource "iwinv_security_group_ingress_rule" "test" {
 security_group_id = iwinv_security_group.%s.id
 name = "tf-ingress"
 description = %q
 ip_protocol = %q
 from_port = %d
 to_port = %d
 cidr_ipv4 = %q
}
resource "iwinv_security_group_egress_rule" "peer" {
 security_group_id = iwinv_security_group.first.id
 name = "tf-egress"
 ip_protocol = "udp"
 from_port = 53
 to_port = 53
 cidr_ipv4 = "192.0.2.0/24"
}
`, parent, description, protocol, from, to, cidr)
}
func TestProtocolSecurityRulesLifecycle(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newRuleAPI()
	const address = "iwinv_security_group_ingress_rule.test"
	const peer = "iwinv_security_group_egress_rule.peer"
	initial := ruleConfig("first", "한글 &amp; <x>", "tcp", 8443, 8443, "192.0.2.1/32")
	updated := ruleConfig("first", "수정 &#39; <x>", "udp", 9443, 9445, "192.0.2.1/24")
	cleared := ruleConfig("first", "", "udp", 9443, 9445, "192.0.2.1/24")
	moved := ruleConfig("second", "", "udp", 9443, 9445, "192.0.2.1/24")
	var firstID, peerID, currentID string
	capture := func(s *terraform.State) error {
		currentID = s.RootModule().Resources[address].Primary.ID
		if firstID == "" {
			firstID = currentID
			peerID = s.RootModule().Resources[peer].Primary.ID
		}
		if s.RootModule().Resources[peer].Primary.ID != peerID {
			return errors.New("independent rule identity changed")
		}
		return nil
	}
	checkStable := func(*terraform.State) error {
		if currentID != firstID {
			return errors.New("in-place update replaced rule")
		}
		return nil
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		a.ruleMu.Lock()
		defer a.ruleMu.Unlock()
		if len(a.rules) != 0 {
			return errors.New("synthetic rules leaked")
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(a.groups) != 0 {
			return errors.New("synthetic parents leaked")
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: initial, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr(address, "direction", "ingress"), resource.TestCheckResourceAttr(peer, "direction", "egress"), resource.TestCheckResourceAttr(peer, "description", ""), resource.TestCheckResourceAttr(address, "description", "한글 &amp; <x>"))},
		{Config: initial, PlanOnly: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(capture, checkStable, resource.TestCheckResourceAttr(address, "ip_protocol", "udp"), resource.TestCheckResourceAttr(address, "to_port", "9445"))},
		{ResourceName: address, ImportState: true, ImportStateVerify: true},
		{ResourceName: peer, ImportState: true, ImportStateVerify: true},
		{Config: updated, PlanOnly: true},
		{PreConfig: func() {
			a.ruleMu.Lock()
			defer a.ruleMu.Unlock()
			group, id, _ := parseRuleIdentity(currentID)
			a.rules[group][id]["bound"] = "OUT"
		}, Config: updated, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: updated, Check: resource.ComposeAggregateTestCheckFunc(capture, checkStable, resource.TestCheckResourceAttr(address, "direction", "ingress"))},
		{Config: cleared, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.ComposeAggregateTestCheckFunc(capture, func(*terraform.State) error {
			if currentID == firstID {
				return errors.New("clear did not replace")
			}
			return nil
		}, resource.TestCheckResourceAttr(address, "description", ""))},
		{Config: moved, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttrPair(address, "security_group_id", "iwinv_security_group.second", "id"))},
		{PreConfig: func() {
			a.ruleMu.Lock()
			defer a.ruleMu.Unlock()
			group, id, _ := parseRuleIdentity(currentID)
			delete(a.rules[group], id)
		}, Config: moved, PlanOnly: true, ExpectNonEmptyPlan: true},
		{Config: moved, Check: capture},
		{Config: moved, PlanOnly: true},
	}})
	if a.ruleCreates != 5 || a.ruleUpdates != 2 || a.ruleDeletes != 4 {
		t.Fatalf("unexpected rule calls: create=%d update=%d delete=%d", a.ruleCreates, a.ruleUpdates, a.ruleDeletes)
	}
}
func TestProtocolSecurityRuleFailedCreateRecovery(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newRuleAPI()
	a.malformedRuleCreate = true
	// One rule and one parent isolate the failed-create cleanup contract.
	config := `provider "iwinv" {}
resource "iwinv_security_group" "first" { name = "tf-failed-rule" }
resource "iwinv_security_group_ingress_rule" "test" {
 security_group_id = iwinv_security_group.first.id
 name = "tf-failed"
 ip_protocol = "tcp"
 from_port = 80
 to_port = 80
 cidr_ipv4 = "192.0.2.1/32"
}`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: func(*terraform.State) error {
		if len(a.groups) != 0 || len(a.rules) != 0 {
			return errors.New("failed-create fixture leaked")
		}
		return nil
	}, Steps: []resource.TestStep{
		{Config: config, ExpectError: regexp.MustCompile("Unable to create iwinv security rule")},
		{Config: config, Destroy: true, Check: resource.TestCheckResourceAttr("iwinv_security_group_ingress_rule.test", "rule_id", "9007199254740993")},
	}})
	if a.ruleCreates != 1 || a.ruleDeletes != 1 {
		t.Fatal("failed create lost identity or was replayed")
	}
}

func ruleTestState(t *testing.T, r *securityGroupRuleResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	var response frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &response)
	groupState := groupTestState(t, &securityGroupResource{}, "FIREWALL-test")
	var group securityGroupModel
	if d := groupState.Get(ctx, &group); d.HasError() {
		t.Fatal(d)
	}
	model := securityGroupRuleModel{ID: types.StringValue("FIREWALL-test/123"), RuleID: types.StringValue("123"), GroupID: types.StringValue("FIREWALL-test"), Direction: types.StringValue("ingress"), Protocol: types.StringValue("tcp"), FromPort: types.Int64Value(80), ToPort: types.Int64Value(80), CIDR: types.StringValue("192.0.2.1/32"), Name: types.StringValue("tf-rule"), Description: types.StringValue("initial"), Timeouts: group.Timeouts}
	state := tfsdk.State{Schema: response.Schema}
	if d := state.Set(ctx, &model); d.HasError() {
		t.Fatal(d)
	}
	return state
}
func TestSecurityRuleFailureState(t *testing.T) {
	for _, scenario := range []string{"create-read-error", "create-malformed", "update-error", "update-read-error", "delete-error", "delete-read-error", "delete-timeout", "read-error", "parent-absent"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			a := newRuleAPI()
			a.nextID = 123
			a.groups["FIREWALL-test"] = map[string]string{"title": "tf-parent", "content": "nonempty", "icmp": "N"}
			r := &securityGroupRuleResource{network: &network.Service{API: a}, bound: "IN", pollInterval: time.Millisecond}
			state := ruleTestState(t, r)
			plan := tfsdk.Plan{Schema: state.Schema, Raw: state.Raw}
			check := func(got tfsdk.State) {
				var m securityGroupRuleModel
				if d := got.Get(ctx, &m); d.HasError() {
					t.Fatal(d)
				}
				if m.ID.ValueString() != "FIREWALL-test/123" || m.RuleID.ValueString() != "123" || m.Name.ValueString() != "tf-rule" {
					t.Fatal("rule identity or prior values lost")
				}
			}
			if strings.HasPrefix(scenario, "create") {
				if scenario == "create-read-error" {
					a.ruleReadError = errors.New("synthetic read error")
				} else {
					a.malformedRuleCreate = true
				}
				response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
				r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("failed create accepted")
				}
				check(response.State)
				if a.ruleCreates != 1 {
					t.Fatal("create replayed")
				}
				return
			}
			a.rules["FIREWALL-test"] = map[string]map[string]string{"123": {"bound": "IN", "protocol": "TCP", "port": "80", "ip": "192.0.2.1/32", "title": "tf-rule", "content": "initial"}}
			if strings.HasPrefix(scenario, "update") {
				var m securityGroupRuleModel
				plan.Get(ctx, &m)
				m.Name = types.StringValue("tf-updated")
				plan.Set(ctx, &m)
				if scenario == "update-error" {
					a.ruleWriteError = errors.New("synthetic update error")
				} else {
					a.ruleReadError = errors.New("synthetic read error")
				}
				response := frameworkresource.UpdateResponse{State: state}
				r.Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: state}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("failed update accepted")
				}
				check(response.State)
				return
			}
			if strings.HasPrefix(scenario, "delete") {
				switch scenario {
				case "delete-error":
					a.ruleWriteError = errors.New("synthetic delete error")
				case "delete-read-error":
					a.postDeleteReadError = true
				case "delete-timeout":
					a.keepDeletedRule = true
				}
				response := frameworkresource.DeleteResponse{State: state}
				r.Delete(ctx, frameworkresource.DeleteRequest{State: state}, &response)
				if !response.Diagnostics.HasError() {
					t.Fatal("failed delete accepted")
				}
				check(response.State)
				if a.ruleDeletes != 1 {
					t.Fatal("delete replayed")
				}
				return
			}
			if scenario == "read-error" {
				a.ruleReadError = &client.Error{Status: 400, Kind: "response", Code: "CHECK_PARAM"}
			} else {
				delete(a.groups, "FIREWALL-test")
			}
			response := frameworkresource.ReadResponse{State: state}
			r.Read(ctx, frameworkresource.ReadRequest{State: state}, &response)
			if scenario == "read-error" {
				if !response.Diagnostics.HasError() {
					t.Fatal("API error erased state")
				}
				check(response.State)
			} else if !response.State.Raw.IsNull() {
				t.Fatal("verified parent absence did not remove rule state")
			}
		})
	}
}
func TestProtocolSecurityRuleInvalidInput(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	for _, input := range []struct {
		protocol, cidr string
		from, to       int
		message        string
	}{{"tcp", "192.0.2.1/32", 0, 80, "Invalid rule port"}, {"tcp", "192.0.2.1/32", 81, 80, "Invalid rule port range"}, {"icmp", "192.0.2.1/32", 80, 80, "Unsupported rule protocol"}, {"tcp", "192.0.2.1", 80, 80, "Unsupported rule address"}, {"tcp", "2001:db8::/64", 80, 80, "Unsupported rule address"}} {
		t.Run(input.message+input.cidr, func(t *testing.T) {
			a := newRuleAPI()
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: ruleConfig("first", "", input.protocol, input.from, input.to, input.cidr), ExpectError: regexp.MustCompile(input.message)}}})
			if a.creates != 0 || a.ruleCreates != 0 {
				t.Fatal("invalid plan mutated cloud fixture")
			}
		})
	}
}
func TestProtocolSecurityRuleTimeoutOnly(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newRuleAPI()
	config := ruleConfig("first", "", "tcp", 80, 80, "192.0.2.1/32")
	config = strings.Replace(config, ` name = "tf-ingress"`, " timeouts { update = \"10s\" }\n"+` name = "tf-ingress"`, 1)
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: config}, {Config: strings.Replace(config, "10s", "20s", 1)}, {Config: strings.Replace(config, "10s", "20s", 1), PlanOnly: true}}})
	if a.ruleUpdates != 0 {
		t.Fatal("timeout-only change wrote rule attributes")
	}
}
func TestRuleImportIdentityValidation(t *testing.T) {
	for _, id := range []string{"123", "FIREWALL-test/0", "FIREWALL-test/01", "FIREWALL-test/1/2", "FIREWALL-test/9223372036854775808", "FIREWALL-test/1?x"} {
		if _, _, err := parseRuleIdentity(id); err == nil {
			t.Fatal("invalid compound identity accepted")
		}
	}
	group, id, err := parseRuleIdentity("FIREWALL-test/9223372036854775807")
	if err != nil || group != "FIREWALL-test" || id != "9223372036854775807" {
		t.Fatal("exact identity was not preserved")
	}
}

func TestProtocolSecurityRuleUnknownDescription(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := newRuleAPI()
	const address = "iwinv_security_group_ingress_rule.test"
	config := func(value string) string {
		base := ruleConfig("first", "placeholder", "tcp", 80, 80, "192.0.2.1/32")
		base = strings.Replace(base, `description = "placeholder"`, `description = terraform_data.note.output`, 1)
		return base + fmt.Sprintf("\nresource \"terraform_data\" \"note\" { input = %q }\n", value)
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{
		{Config: config("initial")},
		{Config: config(""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.TestCheckResourceAttr(address, "description", "")},
		{Config: config("next"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionUpdate)}}, Check: resource.TestCheckResourceAttr(address, "description", "next")},
		{Config: config("another"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.TestCheckResourceAttr(address, "description", "another")},
	}})
}

func TestSecurityRuleUnknownTimeoutBeforeWrite(t *testing.T) {
	a := newRuleAPI()
	r := &securityGroupRuleResource{network: &network.Service{API: a}, bound: "IN"}
	ctx := context.Background()
	state := ruleTestState(t, r)
	var m securityGroupRuleModel
	state.Get(ctx, &m)
	attrs := m.Timeouts.Attributes()
	attrs["read"] = types.StringUnknown()
	m.Timeouts.Object = types.ObjectValueMust(m.Timeouts.AttributeTypes(ctx), attrs)
	plan := tfsdk.Plan{Schema: state.Schema}
	if d := plan.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &response)
	if !response.Diagnostics.HasError() || a.ruleCreates != 0 {
		t.Fatal("unknown timeouts reached a write and could invalidate returned state")
	}
}
