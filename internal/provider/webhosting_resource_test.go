package provider

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

const hostingFTPSecret = "SyntheticFtp71!"
const hostingDBSecret = "SyntheticDb82!"

type hostingAPI struct {
	mu                             sync.Mutex
	rows                           map[string]map[string]any
	usedAccounts                   map[string]bool
	creates, deletes               int
	malformedCreate                bool
	hiddenAfterCreate, hiddenReads int
	readError, writeError          error
	retainDeleted                  bool
}

func newHostingAPI() *hostingAPI {
	return &hostingAPI{rows: map[string]map[string]any{}, usedAccounts: map[string]bool{}}
}
func (a *hostingAPI) Get(_ context.Context, p string, _ url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.readError != nil {
		return client.Envelope{}, a.readError
	}
	if p != "/v1/webhosting" {
		return client.Envelope{}, errors.New("unexpected synthetic hosting read")
	}
	rows := []map[string]any{}
	if a.hiddenReads > 0 {
		a.hiddenReads--
	} else {
		ids := []string{}
		for id := range a.rows {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			rows = append(rows, a.rows[id])
		}
	}
	raw, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: raw}, nil
}
func (a *hostingAPI) PostJSON(_ context.Context, p string, b any) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.creates++
	if a.writeError != nil {
		return client.Envelope{}, a.writeError
	}
	if p != "/v1/webhosting" {
		return client.Envelope{}, errors.New("unexpected synthetic hosting create")
	}
	body := b.(map[string]any)
	account := body["id"].(string)
	if a.usedAccounts[account] {
		return client.Envelope{}, errors.New("synthetic account reuse restricted")
	}
	a.usedAccounts[account] = true
	// Passwords must come from ephemeral configuration, not the null plan.
	if body["ftppw"] != hostingFTPSecret || body["dbpw"] != hostingDBSecret {
		return client.Envelope{}, errors.New("write-only credentials not received correctly")
	}
	if value, ok := body["description"]; ok && value == "" {
		return client.Envelope{}, errors.New("synthetic empty description rejected")
	}
	id := int64(9007199254740992) + int64(a.creates)
	key := fmt.Sprint(id)
	domains := map[string]string{account + ".iwinv.net": "/"}
	if custom, ok := body["domain"].(map[string]string); ok {
		for domain, folder := range custom {
			domains[domain] = folder
		}
	}
	description := ""
	if value, ok := body["description"].(string); ok {
		description = value
	}
	row := map[string]any{"service_idx": id, "product_id": body["product_id"], "name": body["name"], "description": description, "id": account, "status": "active", "ip": "192.0.2.10", "domain": domains, "security": body["security"], "ftppw": hostingFTPSecret, "dbpw": hostingDBSecret}
	a.rows[key] = row
	a.hiddenReads = a.hiddenAfterCreate
	raw, _ := json.Marshal(row)
	e := client.Envelope{Status: 200, Result: raw}
	if a.malformedCreate {
		e.Count = json.RawMessage(`99`)
	}
	return e, nil
}
func (a *hostingAPI) Delete(_ context.Context, p string) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.deletes++
	if a.writeError != nil {
		return client.Envelope{}, a.writeError
	}
	id := strings.TrimPrefix(p, "/v1/webhosting/")
	if a.rows[id] == nil {
		return client.Envelope{}, errors.New("synthetic hosting deletion target missing")
	}
	if !a.retainDeleted {
		delete(a.rows, id)
	}
	return client.Envelope{Status: 200, Result: json.RawMessage(`"acknowledged"`)}, nil
}

const hostingVariables = `provider "iwinv" {}
variable "hosting_ftp" {
 type = string
 sensitive = true
 ephemeral = true
}
variable "hosting_db" {
 type = string
 sensitive = true
 ephemeral = true
}
`

func hostingConfig(account, extra string, createInputs bool) string {
	inputs := ""
	if createInputs {
		inputs = `server_id = "42"
 ftp_password_wo = var.hosting_ftp
 database_password_wo = var.hosting_db
 password_wo_version = 1
`
	}
	return hostingVariables + fmt.Sprintf(`resource "iwinv_webhosting" "test" {
 product_id = "synthetic_hosting"
 account_name = %q
 name = "tf-hosting"
 %s
 %s
}
`, account, inputs, extra)
}
func hostingProtocolEnv(t *testing.T) {
	t.Helper()
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	t.Setenv("TF_VAR_hosting_ftp", hostingFTPSecret)
	t.Setenv("TF_VAR_hosting_db", hostingDBSecret)
}

type hostingSecretPlanCheck struct {
	secrets   []string
	directory string
}

func (check hostingSecretPlanCheck) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	raw, err := json.Marshal(req.Plan)
	if err != nil {
		resp.Error = err
		return
	}
	secrets := check.secrets
	if len(secrets) == 0 {
		secrets = []string{hostingFTPSecret, hostingDBSecret}
	}
	for _, secret := range secrets {
		if bytes.Contains(raw, []byte(secret)) {
			resp.Error = errors.New("write-only credential entered parsed plan")
			return
		}
	}
	if check.directory != "" {
		resp.Error = scanHostingArtifactFiles(check.directory, secrets, true)
	}
}
func hostingStateHasNoSecrets(s *terraform.State) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if bytes.Contains(raw, []byte(hostingFTPSecret)) || bytes.Contains(raw, []byte(hostingDBSecret)) {
		return errors.New("write-only credential entered Terraform state")
	}
	return nil
}

// Inspect real Terraform artifacts, including the compressed contents of saved
// plans. Ephemeral variable values must be absent, not merely sensitive-marked.
func scanHostingArtifacts(root string, secrets ...string) error {
	return scanHostingArtifactFiles(root, secrets, false)
}
func scanHostingArtifactFiles(root string, secrets []string, requirePlan bool) error {
	if len(secrets) == 0 {
		secrets = []string{hostingFTPSecret, hostingDBSecret}
	}
	plans := 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > 32<<20 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		check := func(raw []byte) error {
			for _, secret := range secrets {
				if bytes.Contains(raw, []byte(secret)) {
					return errors.New("write-only credential found in a Terraform artifact")
				}
			}
			return nil
		}
		if err = check(b); err != nil {
			return err
		}
		if z, err := zip.NewReader(bytes.NewReader(b), int64(len(b))); err == nil {
			for _, f := range z.File {
				if f.Name == "tfplan" {
					plans++
				}
				reader, err := f.Open()
				if err != nil {
					return err
				}
				raw, err := io.ReadAll(io.LimitReader(reader, 32<<20))
				reader.Close()
				if err != nil {
					return err
				}
				if err = check(raw); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if requirePlan && plans == 0 {
		return errors.New("saved Terraform plan archive was not inspected")
	}
	return nil
}
func hostingDestroyCheck(a *hostingAPI) resource.TestCheckFunc {
	return func(*terraform.State) error {
		a.mu.Lock()
		defer a.mu.Unlock()
		if len(a.rows) > 0 {
			return errors.New("synthetic hosting leaked")
		}
		return nil
	}
}
func TestProtocolWebhostingLifecycle(t *testing.T) {
	hostingProtocolEnv(t)
	a := newHostingAPI()
	a.hiddenAfterCreate = 1
	dir := t.TempDir()
	const address = "iwinv_webhosting.test"
	initial := hostingConfig("tfexamplea", "", true)
	fields := `description = "한글 &amp; + %"
 web_firewall_enabled = false
 custom_domains = { "custom.example.invalid" = "/" }`
	replacement := hostingConfig("tfexampleb", fields, true)
	imported := hostingConfig("tfexampleb", fields, false)
	timed := strings.Replace(imported, fields, fields+`
timeouts { read = "20s" }`, 1)
	captureSecrets := func(s *terraform.State) error {
		if err := hostingStateHasNoSecrets(s); err != nil {
			return err
		}
		return scanHostingArtifacts(dir)
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, WorkingDir: dir, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: hostingDestroyCheck(a), Steps: []resource.TestStep{
		{Config: initial, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{hostingSecretPlanCheck{directory: dir}}}, Check: resource.ComposeAggregateTestCheckFunc(captureSecrets, resource.TestCheckResourceAttr(address, "id", "9007199254740993"), resource.TestCheckResourceAttr(address, "custom_domains.%", "0"), resource.TestCheckResourceAttr(address, "default_domain", "tfexamplea.iwinv.net"), resource.TestCheckResourceAttr(address, "domains.%", "1"), resource.TestCheckNoResourceAttr(address, "ftp_password_wo"))},
		{Config: initial, PlanOnly: true},
		{Config: hostingConfig("tfexamplea", `description = "changed"`, true), ExpectError: regexp.MustCompile("Replacement requires a new hosting account")},
		{Config: replacement, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate), hostingSecretPlanCheck{directory: dir}}}, Check: resource.ComposeAggregateTestCheckFunc(captureSecrets, resource.TestCheckResourceAttr(address, "id", "9007199254740994"), resource.TestCheckResourceAttr(address, "custom_domains.%", "1"), resource.TestCheckResourceAttr(address, "domains.%", "2"), resource.TestCheckResourceAttr(address, "web_firewall_enabled", "false"))},
		{Config: replacement, PlanOnly: true},
		{ResourceName: address, ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"server_id", "password_wo_version"}},
		{Config: `provider "iwinv" {}
removed {
 from = iwinv_webhosting.test
 lifecycle { destroy = false }
}`},
		{Config: imported, ResourceName: address, ImportState: true, ImportStatePersist: true, ImportStateId: "9007199254740994"},
		{Config: imported, PlanOnly: true},
		{Config: timed, Check: captureSecrets},
		{PreConfig: func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			a.rows["9007199254740994"]["domain"].(map[string]string)["external.example.invalid"] = "/extra"
		}, Config: timed, PlanOnly: true, ExpectError: regexp.MustCompile("Replacement requires a new hosting account")},
		{PreConfig: func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			delete(a.rows["9007199254740994"]["domain"].(map[string]string), "external.example.invalid")
		}, Config: timed, Check: captureSecrets},
		{PreConfig: func() { a.mu.Lock(); defer a.mu.Unlock(); delete(a.rows, "9007199254740994") }, Config: hostingConfig("tfexamplec", "", true), Check: resource.ComposeAggregateTestCheckFunc(captureSecrets, resource.TestCheckResourceAttr(address, "id", "9007199254740995"))},
	}})
	if a.creates != 3 || a.deletes != 2 {
		t.Fatalf("unexpected create/delete counts: %d/%d", a.creates, a.deletes)
	}
}
func TestProtocolWebhostingFailedCreateRecovery(t *testing.T) {
	hostingProtocolEnv(t)
	a := newHostingAPI()
	a.malformedCreate = true
	config := hostingConfig("tfexamplea", "", true)
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: hostingDestroyCheck(a), Steps: []resource.TestStep{
		{Config: config, ExpectError: regexp.MustCompile("Unable to create iwinv webhosting")},
		{Config: config, Destroy: true, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("iwinv_webhosting.test", "id", "9007199254740993"), hostingStateHasNoSecrets)},
	}})
	if a.creates != 1 || a.deletes != 1 {
		t.Fatal("failed create lost identity or replayed a write")
	}
}
func TestProtocolWebhostingReplacementGuard(t *testing.T) {
	hostingProtocolEnv(t)
	for _, tc := range []struct{ name, config string }{
		{"password-version", strings.Replace(hostingConfig("tfexamplea", "", true), "password_wo_version = 1", "password_wo_version = 2", 1)},
		{"unknown-account", strings.Replace(hostingConfig("tfexamplea", `description = "changed"`, true), `account_name = "tfexamplea"`, `account_name = terraform_data.next.output`, 1) + `resource "terraform_data" "next" { input = "tfexampleb" }`},
		{"unknown-description", hostingConfig("tfexamplea", `description = terraform_data.next.output`, true) + `resource "terraform_data" "next" { input = "changed" }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newHostingAPI()
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: hostingConfig("tfexamplea", "", true)}, {Config: tc.config, ExpectError: regexp.MustCompile("Replacement requires a new hosting account")}}})
			if a.creates != 1 || a.deletes != 1 {
				t.Fatal("guard changed a remote service")
			}
		})
	}
}
func hostingTestState(t *testing.T, r *webhostingResource) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	var schema frameworkresource.SchemaResponse
	r.Schema(ctx, frameworkresource.SchemaRequest{}, &schema)
	timeoutTypes := map[string]attr.Type{"create": types.StringType, "read": types.StringType, "update": types.StringType, "delete": types.StringType}
	timeoutValues := map[string]attr.Value{}
	for k := range timeoutTypes {
		timeoutValues[k] = types.StringValue("10ms")
	}
	m := webhostingModel{ID: types.StringValue("9007199254740993"), ProductID: types.StringValue("synthetic_hosting"), ServerID: types.StringValue("42"), Account: types.StringValue("tfexamplea"), Name: types.StringValue("tf-hosting"), Description: types.StringValue(""), Firewall: types.BoolValue(true), CustomDomains: types.MapValueMust(types.StringType, map[string]attr.Value{}), FTPPassword: types.StringNull(), DatabasePassword: types.StringNull(), PasswordVersion: types.Int64Value(1), Status: types.StringValue("active"), IP: types.StringValue("192.0.2.10"), DefaultDomain: types.StringValue("tfexamplea.iwinv.net"), Domains: types.MapValueMust(types.StringType, map[string]attr.Value{"tfexamplea.iwinv.net": types.StringValue("/")}), Timeouts: timeouts.Value{Object: types.ObjectValueMust(timeoutTypes, timeoutValues)}}
	s := tfsdk.State{Schema: schema.Schema}
	if d := s.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	return s
}
func TestWebhostingUnknownCreationAndFailureState(t *testing.T) {
	for _, scenario := range []string{"unknown-password", "unknown-server", "unknown-timeouts", "read-error", "unverified-absence", "active-absence", "delete-error", "delete-retained", "missing-default-domain"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			a := newHostingAPI()
			r := &webhostingResource{service: &hosted.WebhostingService{API: a}, pollInterval: time.Millisecond}
			state := hostingTestState(t, r)
			var m webhostingModel
			state.Get(ctx, &m)
			in := hosted.WebhostingInput{ProductID: "synthetic_hosting", ServerID: "42", Account: "tfexamplea", Name: "tf-hosting", WebFirewall: true, FTPPassword: hostingFTPSecret, DatabasePassword: hostingDBSecret}
			if strings.HasPrefix(scenario, "unknown-") {
				m.FTPPassword = types.StringValue(hostingFTPSecret)
				m.DatabasePassword = types.StringValue(hostingDBSecret)
				if scenario == "unknown-password" {
					m.FTPPassword = types.StringUnknown()
				}
				if scenario == "unknown-server" {
					m.ServerID = types.StringUnknown()
				}
				if scenario == "unknown-timeouts" {
					m.Timeouts.Object = types.ObjectUnknown(m.Timeouts.AttributeTypes(ctx))
				}
				config := tfsdk.Config{Schema: state.Schema}
				tmp := tfsdk.State{Schema: state.Schema}
				tmp.Set(ctx, &m)
				config.Raw = tmp.Raw
				m.FTPPassword = types.StringNull()
				m.DatabasePassword = types.StringNull()
				plan := tfsdk.Plan{Schema: state.Schema}
				plan.Set(ctx, &m)
				resp := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
				r.Create(ctx, frameworkresource.CreateRequest{Config: config, Plan: plan}, &resp)
				if !resp.Diagnostics.HasError() || a.creates != 0 {
					t.Fatal("unknown input reached a write")
				}
				return
			}
			if scenario != "unverified-absence" && scenario != "active-absence" {
				if _, err := r.service.Create(ctx, in); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "read-error":
				a.readError = &client.Error{Kind: "http_status", Status: 404, Code: "NOT_FOUND"}
			case "unverified-absence":
				m.Status = types.StringNull()
				state.Set(ctx, &m)
			case "delete-error":
				a.writeError = errors.New("synthetic failure")
			case "delete-retained":
				a.retainDeleted = true
			case "missing-default-domain":
				delete(a.rows[m.ID.ValueString()]["domain"].(map[string]string), m.DefaultDomain.ValueString())
				a.rows[m.ID.ValueString()]["domain"].(map[string]string)["other.invalid"] = "/"
			}
			if strings.HasPrefix(scenario, "delete-") {
				resp := frameworkresource.DeleteResponse{State: state}
				r.Delete(ctx, frameworkresource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() || a.deletes != 1 {
					t.Fatal("uncertain deletion lost identity or retried")
				}
				return
			}
			resp := frameworkresource.ReadResponse{State: state}
			r.Read(ctx, frameworkresource.ReadRequest{State: state}, &resp)
			if scenario == "active-absence" {
				if resp.Diagnostics.HasError() || !resp.State.Raw.IsNull() {
					t.Fatal("confirmed active service absence not removed")
				}
				return
			}
			if !resp.Diagnostics.HasError() || !resp.State.Raw.Equal(state.Raw) {
				t.Fatal("unverified read lost prior state")
			}
		})
	}
}

// Explicit Core replacement mechanisms are distinct from ordinary attribute
// changes. Show their destructive plan without executing it under a reused name.
func TestProtocolWebhostingExplicitReplacementPlans(t *testing.T) {
	hostingProtocolEnv(t)
	for _, mode := range []string{"taint", "replace-flag"} {
		t.Run(mode, func(t *testing.T) {
			a := newHostingAPI()
			config := hostingConfig("tfexamplea", "", true)
			const address = "iwinv_webhosting.test"
			step := resource.TestStep{Config: config, PlanOnly: true, ExpectNonEmptyPlan: true, ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPreRefresh: []plancheck.PlanCheck{plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate)}}}
			if mode == "taint" {
				step.Taint = []string{address}
			} else {
				step.PreConfig = func() { t.Setenv("TF_CLI_ARGS_plan", "-replace="+address) }
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: hostingDestroyCheck(a), Steps: []resource.TestStep{
				{Config: config}, step, {PreConfig: func() { t.Setenv("TF_CLI_ARGS_plan", "") }, Config: config, Destroy: true},
			}})
			if a.creates != 1 || a.deletes != 1 {
				t.Fatal("explicit replacement plan executed an unintended create")
			}
		})
	}
}
func TestProtocolWebhostingUnknownReplacementValue(t *testing.T) {
	hostingProtocolEnv(t)
	a := newHostingAPI()
	config := hostingConfig("tfexampleb", `description = terraform_data.value.output`, true) + `resource "terraform_data" "value" { input = "changed" }`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), CheckDestroy: hostingDestroyCheck(a), Steps: []resource.TestStep{
		{Config: hostingConfig("tfexamplea", "", true)},
		{Config: config, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("iwinv_webhosting.test", plancheck.ResourceActionDestroyBeforeCreate)}}, Check: resource.TestCheckResourceAttr("iwinv_webhosting.test", "description", "changed")},
	}})
	if a.creates != 2 || a.deletes != 2 {
		t.Fatal("unknown replacement value changed operation counts")
	}
}
func TestProtocolWebhostingMissingReplacementInputs(t *testing.T) {
	hostingProtocolEnv(t)
	a := newHostingAPI()
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{
		{Config: hostingConfig("tfexamplea", "", true)},
		{Config: hostingConfig("tfexampleb", "", false), ExpectError: regexp.MustCompile("Initial hosting passwords required")},
	}})
	if a.creates != 1 || a.deletes != 1 {
		t.Fatal("missing replacement input destroyed an existing service")
	}
}
func TestWebhostingDelayedCreateKeepsIdentityUntilCleanup(t *testing.T) {
	ctx := context.Background()
	a := newHostingAPI()
	a.hiddenAfterCreate = 100
	r := &webhostingResource{service: &hosted.WebhostingService{API: a}, pollInterval: time.Millisecond}
	state := hostingTestState(t, r)
	var m webhostingModel
	state.Get(ctx, &m)
	m.FTPPassword = types.StringValue(hostingFTPSecret)
	m.DatabasePassword = types.StringValue(hostingDBSecret)
	configState := tfsdk.State{Schema: state.Schema}
	configState.Set(ctx, &m)
	config := tfsdk.Config{Schema: state.Schema, Raw: configState.Raw}
	m.FTPPassword = types.StringNull()
	m.DatabasePassword = types.StringNull()
	m.ID = types.StringUnknown()
	plan := tfsdk.Plan{Schema: state.Schema}
	plan.Set(ctx, &m)
	created := frameworkresource.CreateResponse{State: tfsdk.State{Schema: state.Schema}}
	r.Create(ctx, frameworkresource.CreateRequest{Config: config, Plan: plan}, &created)
	if !created.Diagnostics.HasError() {
		t.Fatal("delayed creation unexpectedly verified")
	}
	var saved webhostingModel
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
