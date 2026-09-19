package provider

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"sort"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type contentCacheResource struct {
	service      *hosted.CacheService
	pollInterval time.Duration
}

var (
	_ resource.ResourceWithConfigure      = &contentCacheResource{}
	_ resource.ResourceWithValidateConfig = &contentCacheResource{}
	_ resource.ResourceWithModifyPlan     = &contentCacheResource{}
	_ resource.ResourceWithImportState    = &contentCacheResource{}
)

func NewContentCacheResource() resource.Resource {
	return &contentCacheResource{pollInterval: 2 * time.Second}
}

type contentCacheModel struct {
	ID              types.String   `tfsdk:"id"`
	ProductID       types.String   `tfsdk:"product_id"`
	Account         types.String   `tfsdk:"account_name"`
	Name            types.String   `tfsdk:"name"`
	Description     types.String   `tfsdk:"description"`
	Referrers       types.Set      `tfsdk:"allowed_referrers"`
	FTPPassword     types.String   `tfsdk:"ftp_password_wo"`
	PasswordVersion types.Int64    `tfsdk:"password_wo_version"`
	Status          types.String   `tfsdk:"status"`
	IP              types.String   `tfsdk:"ip_address"`
	Domain          types.String   `tfsdk:"domain_name"`
	ProductType     types.String   `tfsdk:"product_type"`
	Timeouts        timeouts.Value `tfsdk:"timeouts"`
}

func (r *contentCacheResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_content_cache"
}
func (r *contentCacheResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages one content-cache service and its complete referrer set. Nonempty referrer sets update in place. Clearing or changing other creation settings requires destructive replacement with a fresh account; account names cannot be reused for 24 hours after deletion. Initial passwords are write-only; tenant APIs, content, DNS and purge are not managed.", Attributes: map[string]schema.Attribute{
		"id":                  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Exact decimal service_idx; import identifier."},
		"product_id":          schema.StringAttribute{Required: true, MarkdownDescription: "Exact nonempty creation product ID. Changes replace the service; null catalog IDs cannot create a service."},
		"account_name":        schema.StringAttribute{Required: true, MarkdownDescription: "Observed login account, 6–12 ASCII letters/digits. Changes replace. Choose a fresh name before every replacement."},
		"name":                schema.StringAttribute{Required: true, Validators: []validator.String{groupTextLength{min: 4, max: 32, summary: "Invalid cache text"}}, MarkdownDescription: "Service alias, 4–32 Unicode characters. Changes replace."},
		"description":         schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Validators: []validator.String{groupTextLength{min: 0, max: 50, summary: "Invalid cache text"}}, MarkdownDescription: "Literal description up to 50 characters. Empty is omitted on creation. Changes replace."},
		"allowed_referrers":   schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: setdefault.StaticValue(types.SetValueMust(types.StringType, []attr.Value{})), MarkdownDescription: "Authoritative set of lowercase ASCII DNS hostnames; default empty. Nonempty sets replace the remote list in place. Clearing an existing nonempty set replaces the service because the API rejects empty PUTs. No wildcard, URL/path/port, IDN or IP support is implied."},
		"ftp_password_wo":     schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, MarkdownDescription: "Initial FTP password required for creation. Read from configuration, never stored in plan/state. Changing this alone does not trigger an update."},
		"password_wo_version": schema.Int64Attribute{Optional: true, MarkdownDescription: "Positive local version signaling a new initial password. Changes replace the whole service with a fresh account, not in-place rotation. Omit unknown history when importing."},
		"status":              schema.StringAttribute{Computed: true, MarkdownDescription: "Observed control-plane status. Active does not guarantee a cleared write lock or content/FTP connectivity."},
		"ip_address":          schema.StringAttribute{Computed: true, MarkdownDescription: "Observed service IP address."},
		"domain_name":         schema.StringAttribute{Computed: true, MarkdownDescription: "Observed domain string; no port, URL scheme or DNS management is inferred."},
		"product_type":        schema.StringAttribute{Computed: true, MarkdownDescription: "Observed spec.type. The API can report SINGLE for a SHARE catalog product; this is not an isolation guarantee."},
	}, Blocks: map[string]schema.Block{"timeouts": timeouts.BlockAll(ctx)}}
}
func (r *contentCacheResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	r.service = s.Cache
}
func (r *contentCacheResource) configured(d *diag.Diagnostics) bool {
	if r.service == nil {
		d.AddError("Missing provider client", "Configure iwinv credentials before managing content cache.")
		return false
	}
	return true
}

var cacheAccountName = regexp.MustCompile(`^[A-Za-z0-9]{6,12}$`)

func validateCacheConfig(m contentCacheModel, d *diag.Diagnostics) {
	validateGroupTimeouts(m.Timeouts, d)
	if !m.ProductID.IsNull() && !m.ProductID.IsUnknown() && m.ProductID.ValueString() == "" {
		d.AddAttributeError(path.Root("product_id"), "Invalid cache product ID", "Choose a nonempty exact creation product ID.")
	}
	if !m.Account.IsNull() && !m.Account.IsUnknown() && !cacheAccountName.MatchString(m.Account.ValueString()) {
		d.AddAttributeError(path.Root("account_name"), "Invalid cache account", "Use 6–12 ASCII letters/digits.")
	}
	if !m.PasswordVersion.IsNull() && !m.PasswordVersion.IsUnknown() && m.PasswordVersion.ValueInt64() < 1 {
		d.AddAttributeError(path.Root("password_wo_version"), "Invalid password version", "Use a positive version when supplied.")
	}
	if !m.FTPPassword.IsNull() && !m.FTPPassword.IsUnknown() {
		if err := hosted.ValidateCachePassword(m.FTPPassword.ValueString()); err != nil {
			d.AddAttributeError(path.Root("ftp_password_wo"), "Invalid initial cache password", err.Error())
		}
	}
	if !m.Referrers.IsNull() && !m.Referrers.IsUnknown() {
		refs := []string{}
		unknown := false
		for _, v := range m.Referrers.Elements() {
			if v.IsNull() {
				d.AddAttributeError(path.Root("allowed_referrers"), "Invalid cache referrers", "Null members are unsupported.")
				return
			}
			if v.IsUnknown() {
				unknown = true
				continue
			}
			refs = append(refs, v.(types.String).ValueString())
		}
		if !unknown && len(refs) > 0 {
			if err := hosted.ValidateCacheReferrers(refs); err != nil {
				d.AddAttributeError(path.Root("allowed_referrers"), "Invalid cache referrers", err.Error())
			}
		}
	}
}
func (r *contentCacheResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m contentCacheModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		validateCacheConfig(m, &resp.Diagnostics)
	}
}
func cacheReplacementPaths(before, after contentCacheModel) path.Paths {
	out := path.Paths{}
	for _, v := range []struct {
		name string
		a, b attr.Value
	}{{"product_id", before.ProductID, after.ProductID}, {"account_name", before.Account, after.Account}, {"name", before.Name, after.Name}, {"description", before.Description, after.Description}, {"password_wo_version", before.PasswordVersion, after.PasswordVersion}} {
		if !v.a.Equal(v.b) {
			out = append(out, path.Root(v.name))
		}
	}
	if !before.Referrers.Equal(after.Referrers) && len(before.Referrers.Elements()) > 0 && (after.Referrers.IsUnknown() || after.Referrers.IsNull() || len(after.Referrers.Elements()) == 0) {
		out = append(out, path.Root("allowed_referrers"))
	}
	return out
}
func (r *contentCacheResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan, config contentCacheModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	passwordRequired := func() {
		if config.FTPPassword.IsNull() {
			resp.Diagnostics.AddError("Initial cache password required", "Supply ftp_password_wo for new creation or replacement. Import and in-place referrer updates do not require it.")
		}
	}
	if req.State.Raw.IsNull() {
		passwordRequired()
		return
	}
	var prior contentCacheModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	replacements := cacheReplacementPaths(prior, plan)
	if len(replacements) == 0 {
		return
	}
	resp.RequiresReplace = append(resp.RequiresReplace, replacements...)
	passwordRequired()
	if plan.Account.IsUnknown() || plan.Account.IsNull() || plan.Account.Equal(prior.Account) {
		resp.Diagnostics.AddAttributeError(path.Root("account_name"), "Cache replacement requires a new account", "Clearing a nonempty referrer set or changing creation settings replaces the entire service. Deletion destroys data and blocks reuse of the account name for 24 hours. Choose a known different account and inspect the plan before replacement.")
	}
}
func knownCachePlan(ctx context.Context, m contentCacheModel, d *diag.Diagnostics) ([]string, bool) {
	validateCacheConfig(m, d)
	if d.HasError() || !knownOperationTimeouts(m.Timeouts, d) {
		return nil, false
	}
	for _, v := range []attr.Value{m.ProductID, m.Account, m.Name, m.Description, m.Referrers} {
		if v.IsNull() || v.IsUnknown() {
			d.AddError("Incomplete cache configuration", "Required inputs must be known before writing.")
			return nil, false
		}
	}
	if m.PasswordVersion.IsUnknown() {
		d.AddError("Unknown password version", "Resolve the local version before writing.")
		return nil, false
	}
	refs := []string{}
	d.Append(m.Referrers.ElementsAs(ctx, &refs, false)...)
	if d.HasError() {
		return nil, false
	}
	sort.Strings(refs)
	return refs, true
}
func setCacheObserved(ctx context.Context, m *contentCacheModel, g *hosted.Cache, d *diag.Diagnostics) {
	m.ID = types.StringValue(g.ID)
	m.ProductID = types.StringValue(g.ProductID)
	m.Account = types.StringValue(g.Account)
	m.Name = types.StringValue(g.Name)
	desc := ""
	if g.Description != nil {
		desc = *g.Description
	}
	m.Description = types.StringValue(desc)
	m.Status = types.StringValue(g.Status)
	m.IP = types.StringValue(g.IP)
	m.Domain = types.StringValue(g.Domain)
	m.ProductType = types.StringValue(g.Type)
	var ds diag.Diagnostics
	m.Referrers, ds = types.SetValueFrom(ctx, types.StringType, g.Referrers)
	d.Append(ds...)
	m.FTPPassword = types.StringNull()
}
func cacheMatches(ctx context.Context, g *hosted.Cache, m *contentCacheModel) bool {
	if g == nil || g.ID != m.ID.ValueString() || g.ProductID != m.ProductID.ValueString() || g.Account != m.Account.ValueString() || g.Name != m.Name.ValueString() {
		return false
	}
	desc := ""
	if g.Description != nil {
		desc = *g.Description
	}
	if desc != m.Description.ValueString() {
		return false
	}
	refs := []string{}
	if ds := m.Referrers.ElementsAs(ctx, &refs, false); ds.HasError() {
		return false
	}
	sort.Strings(refs)
	return reflect.DeepEqual(refs, g.Referrers)
}
func (r *contentCacheResource) pause(ctx context.Context) error {
	timer := time.NewTimer(r.pollInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (r *contentCacheResource) wait(ctx context.Context, id string, want *contentCacheModel) (*hosted.Cache, error) {
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
			if row.Status == "active" && cacheMatches(ctx, row, want) {
				return row, nil
			}
			if row.Status != "active" && row.Status != "pending" && row.Status != "waiting" {
				return nil, errors.New("unverified or failed cache status; reconcile the service before another apply")
			}
		}
		if err = r.pause(ctx); err != nil {
			return nil, err
		}
	}
}
func cacheBusy(err error, kind string) bool {
	var e *client.Error
	return errors.As(err, &e) && e.Kind == kind && e.Status == 404 && e.Code == "NOT_FOUND"
}

// Only an explicitly rejected busy request can be attempted again. Each retry
// requires the entire parent to remain unchanged; accepted/uncertain writes exit.
func (r *contentCacheResource) write(ctx context.Context, prior *hosted.Cache, id string, refs []string, deleting bool) error {
	kind := "cache_referrers_busy"
	if deleting {
		kind = "cache_delete_busy"
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		if deleting {
			err = r.service.Delete(ctx, id)
		} else {
			err = r.service.ReplaceReferrers(ctx, id, refs)
		}
		if err == nil || !cacheBusy(err, kind) {
			return err
		}
		if prior == nil {
			return errors.New("busy operation cannot be reconciled without a verified parent; identity is retained")
		}
		current, readErr := r.service.Read(ctx, id)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, prior) {
			return errors.New("cache changed or disappeared after a busy rejection; no further write attempted")
		}
		if err = r.pause(ctx); err != nil {
			return err
		}
	}
}
func (r *contentCacheResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var plan, config contentCacheModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	refs, ok := knownCachePlan(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}
	if config.FTPPassword.IsNull() || config.FTPPassword.IsUnknown() {
		resp.Diagnostics.AddError("Incomplete cache password", "Resolve the initial write-only password before creation.")
		return
	}
	in := hosted.CacheInput{ProductID: plan.ProductID.ValueString(), Account: plan.Account.ValueString(), Name: plan.Name.ValueString(), FTPPassword: config.FTPPassword.ValueString()}
	if plan.Description.ValueString() != "" {
		v := plan.Description.ValueString()
		in.Description = &v
	}
	opCtx, cancel := groupOperationContext(ctx, plan.Timeouts, "create", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.service.Create(opCtx, in)
	if created.ID != "" {
		plan.ID = types.StringValue(created.ID)
		pending := plan
		pending.Status = types.StringNull()
		pending.IP = types.StringNull()
		pending.Domain = types.StringNull()
		pending.ProductType = types.StringNull()
		pending.Referrers = types.SetNull(types.StringType)
		pending.FTPPassword = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &pending)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create iwinv content cache", err.Error()+" No create retry was performed. Preserve a returned ID and reconcile unknown outcomes before applying again.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	initial := plan
	initial.Referrers = types.SetValueMust(types.StringType, []attr.Value{})
	row, err := r.wait(opCtx, created.ID, &initial)
	if err != nil {
		resp.Diagnostics.AddError("Cache creation not verified", err.Error()+" The identity is retained; inspect it before accepting a tainted replacement.")
		return
	}
	setCacheObserved(ctx, &initial, row, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &initial)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(refs) > 0 {
		if err = r.write(opCtx, row, created.ID, refs, false); err != nil {
			resp.Diagnostics.AddError("Initial cache referrers not applied", err.Error()+" The verified parent identity and observed empty set remain in state. No accepted or uncertain write was replayed.")
			return
		}
		row, err = r.wait(opCtx, created.ID, &plan)
		if err != nil {
			resp.Diagnostics.AddError("Initial cache referrers not verified", err.Error()+" The parent identity is retained; refresh to reconcile the result.")
			return
		}
	}
	setCacheObserved(ctx, &plan, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *contentCacheResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m contentCacheModel
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
		resp.Diagnostics.AddError("Unable to read iwinv content cache", err.Error())
		return
	}
	if row == nil {
		if m.Status.IsNull() || m.Status.ValueString() != "active" {
			resp.Diagnostics.AddError("Cache identity not yet verified", "A successful list omits this never-verified identity. Preserve the ID and reconcile delayed creation before replacement.")
			return
		}
		resp.State.RemoveResource(ctx)
		return
	}
	setCacheObserved(ctx, &m, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *contentCacheResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var prior, plan contentCacheModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
	refs, ok := knownCachePlan(ctx, plan, &resp.Diagnostics)
	if !ok {
		return
	}
	if len(cacheReplacementPaths(prior, plan)) > 0 {
		resp.Diagnostics.AddError("Unsupported cache update", "Only nonempty referrer sets can change remotely in place. Other changes require replacement. No write was performed.")
		return
	}
	plan.ID = prior.ID
	opCtx, cancel := groupOperationContext(ctx, plan.Timeouts, "update", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	if !prior.Referrers.Equal(plan.Referrers) {
		row, err := r.service.Read(opCtx, prior.ID.ValueString())
		if err != nil || row == nil {
			resp.Diagnostics.AddError("Unable to verify cache before update", "The parent could not be read; prior state is retained and no write was attempted.")
			return
		}
		if !cacheMatches(ctx, row, &prior) {
			resp.Diagnostics.AddError("Cache changed before update", "Refresh and re-plan because the parent no longer matches the prior state. No write was attempted.")
			return
		}
		if err = r.write(opCtx, row, prior.ID.ValueString(), refs, false); err != nil {
			resp.Diagnostics.AddError("Unable to update cache referrers", err.Error()+" Prior state is retained; refresh to reconcile the remote result.")
			return
		}
	}
	row, err := r.wait(opCtx, prior.ID.ValueString(), &plan)
	if err != nil {
		resp.Diagnostics.AddError("Cache update not verified", err.Error()+" Prior state is retained.")
		return
	}
	setCacheObserved(ctx, &plan, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *contentCacheResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m contentCacheModel
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
		resp.Diagnostics.AddError("Unable to verify cache before deletion", err.Error())
		return
	}
	if row == nil && !m.Status.IsNull() && m.Status.ValueString() == "active" {
		return
	}
	if err = r.write(opCtx, row, m.ID.ValueString(), nil, true); err != nil {
		resp.Diagnostics.AddError("Unable to delete iwinv content cache", err.Error()+" No accepted or uncertain deletion was replayed; state is retained.")
		return
	}
	if _, err = r.wait(opCtx, m.ID.ValueString(), nil); err != nil {
		resp.Diagnostics.AddError("Cache deletion not verified", err.Error()+" The identity remains in state.")
	}
}
func (r *contentCacheResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if err := hosted.ValidateServiceID(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid cache import ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
