package provider

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type webhostingResource struct {
	service      *hosted.WebhostingService
	pollInterval time.Duration
}

var (
	_ resource.ResourceWithConfigure      = &webhostingResource{}
	_ resource.ResourceWithValidateConfig = &webhostingResource{}
	_ resource.ResourceWithModifyPlan     = &webhostingResource{}
	_ resource.ResourceWithImportState    = &webhostingResource{}
)

func NewWebhostingResource() resource.Resource {
	return &webhostingResource{pollInterval: 2 * time.Second}
}

type webhostingModel struct {
	ID               types.String   `tfsdk:"id"`
	ProductID        types.String   `tfsdk:"product_id"`
	ServerID         types.String   `tfsdk:"server_id"`
	Account          types.String   `tfsdk:"account_name"`
	Name             types.String   `tfsdk:"name"`
	Description      types.String   `tfsdk:"description"`
	Firewall         types.Bool     `tfsdk:"web_firewall_enabled"`
	CustomDomains    types.Map      `tfsdk:"custom_domains"`
	FTPPassword      types.String   `tfsdk:"ftp_password_wo"`
	DatabasePassword types.String   `tfsdk:"database_password_wo"`
	PasswordVersion  types.Int64    `tfsdk:"password_wo_version"`
	Status           types.String   `tfsdk:"status"`
	IP               types.String   `tfsdk:"ip_address"`
	DefaultDomain    types.String   `tfsdk:"default_domain"`
	Domains          types.Map      `tfsdk:"domains"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *webhostingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhosting"
}
func (r *webhostingResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages one webhosting account. The public API has no update operation: remote configuration changes require replacement with a new account name. Deletion destroys hosted data and the vendor restricts account-name reuse for 24 hours. Passwords are initial write-only inputs, not in-place rotation.", Attributes: map[string]schema.Attribute{
		"id":                   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Exact decimal service_idx; import identifier."},
		"product_id":           schema.StringAttribute{Required: true, MarkdownDescription: "Exact hosting product ID. Changes replace the service."},
		"server_id":            schema.StringAttribute{Optional: true, MarkdownDescription: "Exact server catalog idx. Required to create; omit after import if creation history is unknown. Read cannot verify this historical input. Adding, changing or removing it requires replacement."},
		"account_name":         schema.StringAttribute{Required: true, MarkdownDescription: "Login account, 6–12 ASCII letters. Distinct from service ID. Changes replace the service; use a fresh account name for every replacement."},
		"name":                 schema.StringAttribute{Required: true, Validators: []validator.String{groupTextLength{min: 4, max: 32, summary: "Invalid hosting text"}}, MarkdownDescription: "Service alias, 4–32 Unicode characters. Changes replace the service."},
		"description":          schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Validators: []validator.String{groupTextLength{min: 0, max: 50, summary: "Invalid hosting text"}}, MarkdownDescription: "Literal description, up to 50 characters. Empty is omitted on create and reads as empty. Changes replace the service."},
		"web_firewall_enabled": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), MarkdownDescription: "Explicit Y/N web firewall flag, default true. Changes replace; not a packet-level verification."},
		"custom_domains":       schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})), MarkdownDescription: "Complete custom domain-to-folder map, excluding account_name.iwinv.net. Default empty. Changes, including external drift, require replacement. DNS and files/folders are not managed."},
		"ftp_password_wo":      schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: "Initial FTP password, required only for creation. Never stored in plan/state; read from configuration during create. Changing this alone does not trigger a plan."},
		"database_password_wo": schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: "Distinct initial database password, required only for creation. Never stored in plan/state. No in-place password update API is available."},
		"password_wo_version":  schema.Int64Attribute{Optional: true, MarkdownDescription: "Optional positive local version to signal new initial passwords. A change replaces the entire service and requires a fresh account name; not in-place rotation. Omit on import when unknown."},
		"status":               schema.StringAttribute{Computed: true, MarkdownDescription: "Observed control-plane status. Active is not proof of HTTP/FTP/database connectivity."},
		"ip_address":           schema.StringAttribute{Computed: true, MarkdownDescription: "Observed hosting IP."},
		"default_domain":       schema.StringAttribute{Computed: true, MarkdownDescription: "Verified default domain account_name.iwinv.net, present in the API response."},
		"domains":              schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "All observed domain-to-folder mappings, including the service-managed default domain."},
	}, Blocks: map[string]schema.Block{"timeouts": timeouts.BlockAll(ctx)}}
}
func (r *webhostingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv resource services.")
		return
	}
	r.service = s.Hosting
}
func (r *webhostingResource) configured(d *diag.Diagnostics) bool {
	if r.service == nil {
		d.AddError("Missing provider client", "Configure iwinv credentials before managing webhosting.")
		return false
	}
	return true
}

var hostingAccountName = regexp.MustCompile(`^[A-Za-z]{6,12}$`)

func validateHostingConfig(m webhostingModel, d *diag.Diagnostics) {
	validateGroupTimeouts(m.Timeouts, d)
	if !m.FTPPassword.IsNull() && !m.FTPPassword.IsUnknown() && !m.DatabasePassword.IsNull() && !m.DatabasePassword.IsUnknown() {
		if err := hosted.ValidateWebhostingPasswords(m.FTPPassword.ValueString(), m.DatabasePassword.ValueString()); err != nil {
			d.AddError("Invalid initial hosting passwords", err.Error())
		}
	}

	if !m.ProductID.IsNull() && !m.ProductID.IsUnknown() && m.ProductID.ValueString() == "" {
		d.AddAttributeError(path.Root("product_id"), "Invalid product ID", "Product ID must not be empty.")
	}
	if !m.ServerID.IsNull() && !m.ServerID.IsUnknown() {
		if err := hosted.ValidateServiceID(m.ServerID.ValueString()); err != nil {
			d.AddAttributeError(path.Root("server_id"), "Invalid server ID", err.Error())
		}
	}
	if !m.Account.IsNull() && !m.Account.IsUnknown() && !hostingAccountName.MatchString(m.Account.ValueString()) {
		d.AddAttributeError(path.Root("account_name"), "Invalid hosting account", "Use 6–12 ASCII letters.")
	}
	if !m.PasswordVersion.IsNull() && !m.PasswordVersion.IsUnknown() && m.PasswordVersion.ValueInt64() < 1 {
		d.AddAttributeError(path.Root("password_wo_version"), "Invalid password version", "Use a positive version when supplied.")
	}
	if !m.CustomDomains.IsNull() && !m.CustomDomains.IsUnknown() {
		for domain, v := range m.CustomDomains.Elements() {
			if domain == "" || v.IsNull() || (!v.IsUnknown() && v.(types.String).ValueString() == "") {
				d.AddAttributeError(path.Root("custom_domains"), "Invalid custom domain mapping", "Domain names and folder values must not be empty or null.")
			}
			if !m.Account.IsUnknown() && !m.Account.IsNull() && domain == m.Account.ValueString()+".iwinv.net" {
				d.AddAttributeError(path.Root("custom_domains"), "Default domain is service-managed", "Exclude account_name.iwinv.net from custom_domains.")
			}
		}
	}
}
func (r *webhostingResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m webhostingModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		validateHostingConfig(m, &resp.Diagnostics)
	}
}
func hostingReplacementPaths(before, after webhostingModel) path.Paths {
	pairs := []struct {
		name string
		a, b attr.Value
	}{
		{"product_id", before.ProductID, after.ProductID}, {"server_id", before.ServerID, after.ServerID}, {"account_name", before.Account, after.Account},
		{"name", before.Name, after.Name}, {"description", before.Description, after.Description}, {"web_firewall_enabled", before.Firewall, after.Firewall},
		{"custom_domains", before.CustomDomains, after.CustomDomains}, {"password_wo_version", before.PasswordVersion, after.PasswordVersion},
	}
	out := path.Paths{}
	for _, p := range pairs {
		if !p.a.Equal(p.b) {
			out = append(out, path.Root(p.name))
		}
	}
	return out
}
func (r *webhostingResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan webhostingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config webhostingModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	requirePasswords := func() {
		if config.FTPPassword.IsNull() || config.DatabasePassword.IsNull() {
			resp.Diagnostics.AddError("Initial hosting passwords required", "Set both write-only passwords before creating or replacing an account. Import does not require passwords.")
		}
	}
	if req.State.Raw.IsNull() {
		requirePasswords()

		if plan.ServerID.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("server_id"), "Server choice required for creation", "Set an exact server catalog ID for a new account. It may be omitted only when managing an imported account without this history.")
		}
		return
	}
	var prior webhostingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	replacements := hostingReplacementPaths(prior, plan)
	if len(replacements) == 0 {
		return
	}
	resp.RequiresReplace = append(resp.RequiresReplace, replacements...)
	requirePasswords()
	if plan.Account.IsUnknown() || plan.Account.IsNull() || plan.Account.Equal(prior.Account) {
		resp.Diagnostics.AddAttributeError(path.Root("account_name"), "Replacement requires a new hosting account", "The public API cannot update this configuration. Deletion destroys data, and the vendor restricts reusing the account name for 24 hours. Supply a known, different account_name before replacement, or reconcile configuration with the existing service. No write was performed.")
	}
	if plan.ServerID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("server_id"), "Server choice required for replacement", "Read cannot recover the creation server selection. Supply an exact server catalog ID for the replacement account.")
	}
}
func hostingInput(ctx context.Context, plan, config webhostingModel, d *diag.Diagnostics) (hosted.WebhostingInput, bool) {
	validateHostingConfig(plan, d)
	if d.HasError() || !knownOperationTimeouts(plan.Timeouts, d) {
		return hosted.WebhostingInput{}, false
	}
	for _, v := range []attr.Value{plan.ProductID, plan.ServerID, plan.Account, plan.Name, plan.Description, plan.Firewall, plan.CustomDomains, config.FTPPassword, config.DatabasePassword} {
		if v.IsNull() || v.IsUnknown() {
			d.AddError("Incomplete hosting creation configuration", "Product, server, account, name, description, firewall, domains and both initial write-only passwords must be known before creation.")
			return hosted.WebhostingInput{}, false
		}
	}
	if plan.PasswordVersion.IsUnknown() {
		d.AddError("Unknown password version", "The local password version must be known before creation.")
		return hosted.WebhostingInput{}, false
	}
	var domains map[string]string
	d.Append(plan.CustomDomains.ElementsAs(ctx, &domains, false)...)
	if d.HasError() {
		return hosted.WebhostingInput{}, false
	}
	in := hosted.WebhostingInput{ProductID: plan.ProductID.ValueString(), ServerID: plan.ServerID.ValueString(), Account: plan.Account.ValueString(), Name: plan.Name.ValueString(), WebFirewall: plan.Firewall.ValueBool(), FTPPassword: config.FTPPassword.ValueString(), DatabasePassword: config.DatabasePassword.ValueString()}
	if plan.Description.ValueString() != "" {
		v := plan.Description.ValueString()
		in.Description = &v
	}
	if len(domains) > 0 {
		in.Domains = domains
	}
	return in, true
}
func hostingCustomDomains(g *hosted.Webhosting) (map[string]string, error) {
	expected := g.Account + ".iwinv.net"
	if _, ok := g.Domains[expected]; !ok {
		return nil, errors.New("expected service-managed default domain is absent; domain ownership cannot be established")
	}
	out := map[string]string{}
	for domain, folder := range g.Domains {
		if domain != expected {
			out[domain] = folder
		}
	}
	return out, nil
}
func setHostingObserved(ctx context.Context, m *webhostingModel, g *hosted.Webhosting, d *diag.Diagnostics) {
	custom, err := hostingCustomDomains(g)
	if err != nil {
		d.AddError("Unverified hosting domain contract", err.Error())
		return
	}
	m.ID = types.StringValue(g.ID)
	m.ProductID = types.StringValue(g.ProductID)
	m.Account = types.StringValue(g.Account)
	m.Name = types.StringValue(g.Name)
	description := ""
	if g.Description != nil {
		description = *g.Description
	}
	m.Description = types.StringValue(description)
	m.Firewall = types.BoolValue(g.WebFirewall)
	m.Status = types.StringValue(g.Status)
	m.IP = types.StringValue(g.IP)
	m.DefaultDomain = types.StringValue(g.Account + ".iwinv.net")
	var ds diag.Diagnostics
	m.CustomDomains, ds = types.MapValueFrom(ctx, types.StringType, custom)
	d.Append(ds...)
	m.Domains, ds = types.MapValueFrom(ctx, types.StringType, g.Domains)
	d.Append(ds...)
	m.FTPPassword = types.StringNull()
	m.DatabasePassword = types.StringNull()
}
func hostingMatches(ctx context.Context, g *hosted.Webhosting, want *webhostingModel) bool {
	if g == nil || g.ID != want.ID.ValueString() || g.ProductID != want.ProductID.ValueString() || g.Account != want.Account.ValueString() || g.Name != want.Name.ValueString() || g.WebFirewall != want.Firewall.ValueBool() {
		return false
	}
	description := ""
	if g.Description != nil {
		description = *g.Description
	}
	if description != want.Description.ValueString() {
		return false
	}
	custom, err := hostingCustomDomains(g)
	if err != nil {
		return false
	}
	var expected map[string]string
	if ds := want.CustomDomains.ElementsAs(ctx, &expected, false); ds.HasError() {
		return false
	}
	return reflect.DeepEqual(custom, expected)
}
func (r *webhostingResource) wait(ctx context.Context, id string, want *webhostingModel) (*hosted.Webhosting, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		row, err := r.service.Read(ctx, id)
		if err != nil {
			return nil, err
		}
		if want == nil && row == nil {
			return nil, nil
		}
		if want != nil && row != nil {
			if row.Status == "active" && hostingMatches(ctx, row, want) {
				return row, nil
			}
			if row.Status != "active" && row.Status != "pending" && row.Status != "waiting" {
				return nil, errors.New("unverified or failed hosting status; inspect the service before another apply")
			}
		}
		timer := time.NewTimer(r.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
func (r *webhostingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var plan, config webhostingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	in, ok := hostingInput(ctx, plan, config, &resp.Diagnostics)
	if !ok {
		return
	}
	opCtx, cancel := groupOperationContext(ctx, plan.Timeouts, "create", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.service.Create(opCtx, in)
	if created.ID != "" {
		plan.ID = types.StringValue(created.ID)
		plan.Status = types.StringNull()
		plan.IP = types.StringNull()
		plan.DefaultDomain = types.StringNull()
		plan.Domains = types.MapNull(types.StringType)
		plan.FTPPassword = types.StringNull()
		plan.DatabasePassword = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create iwinv webhosting", err.Error()+" No create retry was performed. A known ID remains in state; reconcile an unknown outcome before applying again.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.wait(opCtx, created.ID, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Hosting creation not verified", err.Error()+" The identity is retained. Do not blindly replace a tainted account: the deleted account name cannot be immediately reused.")
		return
	}
	setHostingObserved(ctx, &plan, g, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *webhostingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m webhostingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := groupOperationContext(ctx, m.Timeouts, "read", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.service.Read(opCtx, m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read iwinv webhosting", err.Error())
		return
	}
	if g == nil {
		if m.Status.IsNull() || m.Status.ValueString() != "active" {
			resp.Diagnostics.AddError("Hosting identity is not yet verified", "The service is not in the successful list, but it has not previously been observed active. Its ID is retained to avoid losing a delayed or failed create. Reconcile it before replacement or import.")
			return
		}
		resp.State.RemoveResource(ctx)
		return
	}
	setHostingObserved(ctx, &m, g, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *webhostingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var prior, plan webhostingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
	validateHostingConfig(plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || !knownOperationTimeouts(plan.Timeouts, &resp.Diagnostics) {
		return
	}
	if len(hostingReplacementPaths(prior, plan)) > 0 {
		resp.Diagnostics.AddError("Unsupported hosting update", "Remote configuration changes require replacement with a new account name. No update API was called.")
		return
	}
	opCtx, cancel := groupOperationContext(ctx, plan.Timeouts, "update", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.service.Read(opCtx, prior.ID.ValueString())
	if err != nil || g == nil {
		resp.Diagnostics.AddError("Hosting local update not verified", "The existing service could not be read. Prior state is retained; no remote update was attempted.")
		return
	}
	setHostingObserved(ctx, &plan, g, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *webhostingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m webhostingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := groupOperationContext(ctx, m.Timeouts, "delete", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.service.Read(opCtx, m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify hosting before deletion", err.Error())
		return
	}
	// An unverified create may be hidden. Do not drop its ID just because a
	// pre-delete list is empty; explicitly address the known service once.
	if g == nil && !m.Status.IsNull() && m.Status.ValueString() == "active" {
		return
	}
	if err = r.service.Delete(opCtx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete iwinv webhosting", err.Error()+" No delete retry was performed; state is retained.")
		return
	}
	if _, err = r.wait(opCtx, m.ID.ValueString(), nil); err != nil {
		resp.Diagnostics.AddError("Hosting deletion not verified", err.Error()+" The identity remains in state.")
	}
}
func (r *webhostingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if err := hosted.ValidateServiceID(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid hosting import ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
