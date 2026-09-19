package provider

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dbInstanceResource struct {
	service      *hosted.DBMSService
	pollInterval time.Duration
}

var (
	_ resource.ResourceWithConfigure      = &dbInstanceResource{}
	_ resource.ResourceWithValidateConfig = &dbInstanceResource{}
	_ resource.ResourceWithModifyPlan     = &dbInstanceResource{}
	_ resource.ResourceWithImportState    = &dbInstanceResource{}
)

func NewDBInstanceResource() resource.Resource {
	return &dbInstanceResource{pollInterval: 2 * time.Second}
}

type dbInstanceModel struct {
	ID            types.String   `tfsdk:"id"`
	ProductID     types.String   `tfsdk:"product_id"`
	Account       types.String   `tfsdk:"account_name"`
	Name          types.String   `tfsdk:"name"`
	Description   types.String   `tfsdk:"description"`
	AllowedIPs    types.Set      `tfsdk:"allowed_ips"`
	Status        types.String   `tfsdk:"status"`
	ProductType   types.String   `tfsdk:"product_type"`
	EngineVersion types.String   `tfsdk:"engine_version"`
	Address       types.String   `tfsdk:"address"`
	Domains       types.Map      `tfsdk:"domains"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func (r *dbInstanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_db_instance"
}
func (r *dbInstanceResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages one cloud DBMS service and its complete nonempty IPv4 allowlist. Only allowed_ips can change in place. Other creation settings replace the service and destroy stored data. Account/engine credentials, database contents, backups and connectivity are not managed.", Attributes: map[string]schema.Attribute{
		"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Exact decimal service_idx; import identifier."},
		"product_id":     schema.StringAttribute{Required: true, MarkdownDescription: "Exact nonempty creation product ID. Changes replace. Catalog IDs can span versions; the API accepts no engine-version selector."},
		"account_name":   schema.StringAttribute{Optional: true, MarkdownDescription: "Initial 6–12 ASCII-letter account; required to create. Not returned by Read, so omit when importing without creation history. Adding/changing/removing it replaces; it is not an observed remote field."},
		"name":           schema.StringAttribute{Required: true, Validators: []validator.String{groupTextLength{min: 4, max: 32, summary: "Invalid DBMS text"}}, MarkdownDescription: "Service alias, 4–32 Unicode characters. Changes replace."},
		"description":    schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Validators: []validator.String{groupTextLength{min: 0, max: 50, summary: "Invalid DBMS text"}}, MarkdownDescription: "Literal description up to 50 characters; empty is omitted on create. Changes replace."},
		"allowed_ips":    schema.SetAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Authoritative nonempty set of canonical IPv4 host addresses. Replaces the entire remote allowlist in place. Empty clearing, CIDRs and IPv6 are unsupported; never manage individual members elsewhere."},
		"status":         schema.StringAttribute{Computed: true, MarkdownDescription: "Observed control-plane status; active does not prove database connectivity."},
		"product_type":   schema.StringAttribute{Computed: true, MarkdownDescription: "Observed spec.type product tier, not an engine name."},
		"engine_version": schema.StringAttribute{Computed: true, MarkdownDescription: "Observed spec.ver string. It is not a configurable version selection or upgrade control."},
		"address":        schema.StringAttribute{Computed: true, MarkdownDescription: "Observed default domain name, without an invented port or protocol."},
		"domains":        schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Observed label-to-domain map. No DNS records are managed."},
	}, Blocks: map[string]schema.Block{"timeouts": timeouts.BlockAll(ctx)}}
}
func (r *dbInstanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	r.service = s.DBMS
}
func (r *dbInstanceResource) configured(d *diag.Diagnostics) bool {
	if r.service == nil {
		d.AddError("Missing provider client", "Configure iwinv credentials before managing DBMS.")
		return false
	}
	return true
}
func validateDBConfig(m dbInstanceModel, d *diag.Diagnostics) {
	validateGroupTimeouts(m.Timeouts, d)
	if !m.ProductID.IsNull() && !m.ProductID.IsUnknown() && m.ProductID.ValueString() == "" {
		d.AddAttributeError(path.Root("product_id"), "Invalid DBMS product ID", "Use a nonempty exact creation product ID. Some catalog rows have no creation ID.")
	}
	if !m.Account.IsNull() && !m.Account.IsUnknown() && !hostingAccountName.MatchString(m.Account.ValueString()) {
		d.AddAttributeError(path.Root("account_name"), "Invalid DBMS account", "Use 6–12 ASCII letters.")
	}
	if !m.AllowedIPs.IsNull() && !m.AllowedIPs.IsUnknown() {
		ips := []string{}
		for _, v := range m.AllowedIPs.Elements() {
			if v.IsUnknown() {
				return
			}
			if v.IsNull() {
				d.AddAttributeError(path.Root("allowed_ips"), "Invalid DBMS allowed IPs", "Null members are unsupported.")
				return
			}
			ips = append(ips, v.(types.String).ValueString())
		}
		if err := hosted.ValidateDBMSAllowIPs(ips); err != nil {
			d.AddAttributeError(path.Root("allowed_ips"), "Invalid DBMS allowed IPs", err.Error())
		}
	}
}
func (r *dbInstanceResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m dbInstanceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		validateDBConfig(m, &resp.Diagnostics)
	}
}
func dbReplacementPaths(a, b dbInstanceModel) path.Paths {
	out := path.Paths{}
	for _, p := range []struct {
		name string
		a, b attr.Value
	}{{"product_id", a.ProductID, b.ProductID}, {"account_name", a.Account, b.Account}, {"name", a.Name, b.Name}, {"description", a.Description, b.Description}} {
		if !p.a.Equal(p.b) {
			out = append(out, path.Root(p.name))
		}
	}
	return out
}
func (r *dbInstanceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan dbInstanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if req.State.Raw.IsNull() {
		if plan.Account.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("account_name"), "DBMS creation account required", "Supply an initial account name for new creation. Import may omit unknown creation history.")
		}
		return
	}
	var prior dbInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	replacements := dbReplacementPaths(prior, plan)
	if len(replacements) == 0 {
		return
	}
	resp.RequiresReplace = append(resp.RequiresReplace, replacements...)
	if plan.Account.IsNull() || plan.Account.IsUnknown() || plan.Account.Equal(prior.Account) {
		resp.Diagnostics.AddAttributeError(path.Root("account_name"), "DBMS replacement requires a new account", "The API cannot update these settings. Choose a known different account name before destructive replacement. For an imported service without account history, explicitly choose a fresh account; Read cannot verify whether it was previously used. No write was performed.")
	}
}
func knownDBPlan(ctx context.Context, m dbInstanceModel, create bool, d *diag.Diagnostics) ([]string, bool) {
	validateDBConfig(m, d)
	if d.HasError() || !knownOperationTimeouts(m.Timeouts, d) {
		return nil, false
	}
	values := []attr.Value{m.ProductID, m.Name, m.Description, m.AllowedIPs}
	if create {
		values = append(values, m.Account)
	}
	for _, v := range values {
		if v.IsUnknown() || v.IsNull() {
			d.AddError("Incomplete DBMS configuration", "All required creation/update inputs must be known before writing.")
			return nil, false
		}
	}
	ips := []string{}
	d.Append(m.AllowedIPs.ElementsAs(ctx, &ips, false)...)
	if d.HasError() {
		return nil, false
	}
	sort.Strings(ips)
	return ips, true
}
func setDBObserved(ctx context.Context, m *dbInstanceModel, g *hosted.DBMS, d *diag.Diagnostics) {
	if g.Domains["default"] == "" {
		d.AddError("Unverified DBMS domain contract", "The default address is missing. No endpoint or creation account is inferred.")
		return
	}
	m.ID = types.StringValue(g.ID)
	m.ProductID = types.StringValue(g.ProductID)
	m.Name = types.StringValue(g.Name)
	desc := ""
	if g.Description != nil {
		desc = *g.Description
	}
	m.Description = types.StringValue(desc)
	m.Status = types.StringValue(g.Status)
	m.ProductType = types.StringValue(g.Tier)
	m.EngineVersion = types.StringValue(g.Version)
	m.Address = types.StringValue(g.Domains["default"])
	var ds diag.Diagnostics
	m.AllowedIPs, ds = types.SetValueFrom(ctx, types.StringType, g.AllowIPs)
	d.Append(ds...)
	m.Domains, ds = types.MapValueFrom(ctx, types.StringType, g.Domains)
	d.Append(ds...)
	// Account is historical create-only input; preserve it, never derive from DNS.
}
func dbMatches(ctx context.Context, g *hosted.DBMS, m *dbInstanceModel) bool {
	if g == nil || g.ID != m.ID.ValueString() || g.ProductID != m.ProductID.ValueString() || g.Name != m.Name.ValueString() || g.Domains["default"] == "" {
		return false
	}
	desc := ""
	if g.Description != nil {
		desc = *g.Description
	}
	if desc != m.Description.ValueString() {
		return false
	}
	ips := []string{}
	if ds := m.AllowedIPs.ElementsAs(ctx, &ips, false); ds.HasError() {
		return false
	}
	sort.Strings(ips)
	return reflect.DeepEqual(ips, g.AllowIPs)
}
func (r *dbInstanceResource) wait(ctx context.Context, id string, want *dbInstanceModel) (*hosted.DBMS, error) {
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
			if row.Status == "active" && dbMatches(ctx, row, want) {
				return row, nil
			}
			if row.Status != "active" && row.Status != "pending" && row.Status != "waiting" {
				return nil, errors.New("unverified or failed DBMS status; reconcile before another apply")
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
func (r *dbInstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m dbInstanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ips, ok := knownDBPlan(ctx, m, true, &resp.Diagnostics)
	if !ok {
		return
	}
	in := hosted.DBMSInput{ProductID: m.ProductID.ValueString(), Account: m.Account.ValueString(), Name: m.Name.ValueString(), AllowIPs: ips}
	if m.Description.ValueString() != "" {
		v := m.Description.ValueString()
		in.Description = &v
	}
	opCtx, cancel := groupOperationContext(ctx, m.Timeouts, "create", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.service.Create(opCtx, in)
	if created.ID != "" {
		m.ID = types.StringValue(created.ID)
		m.Status = types.StringNull()
		m.ProductType = types.StringNull()
		m.EngineVersion = types.StringNull()
		m.Address = types.StringNull()
		m.Domains = types.MapNull(types.StringType)
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create iwinv DBMS", err.Error()+" No create retry was performed. A known ID remains in state; reconcile an unknown outcome before another apply.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	row, err := r.wait(opCtx, created.ID, &m)
	if err != nil {
		resp.Diagnostics.AddError("DBMS creation not verified", err.Error()+" The identity is retained. Inspect it before accepting a tainted replacement.")
		return
	}
	setDBObserved(ctx, &m, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *dbInstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m dbInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := groupOperationContext(ctx, m.Timeouts, "read", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	row, err := r.service.Read(opCtx, m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read iwinv DBMS", err.Error())
		return
	}
	if row == nil {
		if m.Status.IsNull() || m.Status.ValueString() != "active" {
			resp.Diagnostics.AddError("DBMS identity not yet verified", "The service is missing from this successful list but has not previously been observed active. Retain and reconcile the ID before replacement.")
			return
		}
		resp.State.RemoveResource(ctx)
		return
	}
	setDBObserved(ctx, &m, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *dbInstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var prior, plan dbInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
	ips, ok := knownDBPlan(ctx, plan, false, &resp.Diagnostics)
	if !ok {
		return
	}
	if len(dbReplacementPaths(prior, plan)) > 0 {
		resp.Diagnostics.AddError("Unsupported DBMS update", "Only allowed_ips can change remotely in place. Other creation attributes require replacement. No write was performed.")
		return
	}
	plan.ID = prior.ID
	opCtx, cancel := groupOperationContext(ctx, plan.Timeouts, "update", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	if !prior.AllowedIPs.Equal(plan.AllowedIPs) {
		if err := r.service.ReplaceAllowIPs(opCtx, prior.ID.ValueString(), ips); err != nil {
			resp.Diagnostics.AddError("Unable to update DBMS allowed IPs", err.Error()+" No write retry was performed; refresh to reconcile the remote outcome.")
			return
		}
	}
	row, err := r.wait(opCtx, prior.ID.ValueString(), &plan)
	if err != nil {
		resp.Diagnostics.AddError("DBMS update not verified", err.Error()+" Prior state is retained.")
		return
	}
	setDBObserved(ctx, &plan, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *dbInstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m dbInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := groupOperationContext(ctx, m.Timeouts, "delete", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	row, err := r.service.Read(opCtx, m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify DBMS before deletion", err.Error())
		return
	}
	if row == nil && !m.Status.IsNull() && m.Status.ValueString() == "active" {
		return
	}
	if err = r.service.Delete(opCtx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete iwinv DBMS", err.Error()+" No delete retry was performed; state is retained.")
		return
	}
	if _, err = r.wait(opCtx, m.ID.ValueString(), nil); err != nil {
		resp.Diagnostics.AddError("DBMS deletion not verified", err.Error()+" The identity remains in state.")
	}
}
func (r *dbInstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if err := hosted.ValidateServiceID(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid DBMS import ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
