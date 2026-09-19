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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type sharedStorageResource struct {
	service      *hosted.NASService
	pollInterval time.Duration
}

var (
	_ resource.ResourceWithConfigure      = &sharedStorageResource{}
	_ resource.ResourceWithValidateConfig = &sharedStorageResource{}
	_ resource.ResourceWithModifyPlan     = &sharedStorageResource{}
	_ resource.ResourceWithImportState    = &sharedStorageResource{}
)

func NewSharedStorageResource() resource.Resource {
	return &sharedStorageResource{pollInterval: 2 * time.Second}
}

type sharedStorageModel struct {
	ID          types.String   `tfsdk:"id"`
	ProductID   types.String   `tfsdk:"product_id"`
	ShareName   types.String   `tfsdk:"share_name"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	SizeGB      types.Int64    `tfsdk:"size_gb"`
	AllowedIPs  types.Map      `tfsdk:"allowed_ips"`
	Status      types.String   `tfsdk:"status"`
	Address     types.String   `tfsdk:"address"`
	MountInfo   types.String   `tfsdk:"mount_info"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

var storageShareName = regexp.MustCompile(`^[A-Za-z0-9]{6,20}$`)

func (r *sharedStorageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shared_storage"
}
func (r *sharedStorageResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages one API NAS service and its complete IPv4-to-RO/RW permission map. Only allowed_ips updates in place. Other creation settings, including capacity, require destructive replacement with a fresh share name. No files, mounts, tenant credentials, backups, DNS or data migration are managed.", Attributes: map[string]schema.Attribute{
		"id":          schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, MarkdownDescription: "Exact positive decimal service_idx; import ID."},
		"product_id":  schema.StringAttribute{Required: true, MarkdownDescription: "Exact nonempty creation product ID; changes replace. Empty catalog IDs cannot create."},
		"share_name":  schema.StringAttribute{Optional: true, MarkdownDescription: "Initial 6–20 ASCII alphanumeric share name. Required to create, absent from Read; omit unknown history on import. Adding/changing/removing this history requires replacement."},
		"name":        schema.StringAttribute{Required: true, Validators: []validator.String{groupTextLength{min: 4, max: 32, summary: "Invalid NAS text"}}, MarkdownDescription: "Literal alias, 4–32 Unicode code points. Changes replace."},
		"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), Validators: []validator.String{groupTextLength{min: 0, max: 50, summary: "Invalid NAS text"}}, MarkdownDescription: "Literal description up to 50 characters; default empty is omitted on create. Changes replace."},
		"size_gb":     schema.Int64Attribute{Required: true, MarkdownDescription: "Capacity in documented GB, 100–2000. Changing capacity replaces the storage; it does not resize or copy data."},
		"allowed_ips": schema.MapAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "Authoritative nonempty map of canonical IPv4 hosts to exact RO or RW. Replaces the entire map in place, including retained-host permissions. No empty clearing, CIDR or IPv6; do not manage members elsewhere."},
		"status":      schema.StringAttribute{Computed: true, MarkdownDescription: "Observed control-plane state; active does not prove mounted storage or permission enforcement."},
		"address":     schema.StringAttribute{Computed: true, MarkdownDescription: "Observed domain string, without an inferred port, URL scheme or DNS ownership."},
		"mount_info":  schema.StringAttribute{Computed: true, MarkdownDescription: "Opaque observed mount information. The provider never executes it or derives share-name history from it."},
	}, Blocks: map[string]schema.Block{"timeouts": timeouts.BlockAll(ctx)}}
}

func (r *sharedStorageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	r.service = s.NAS
}
func (r *sharedStorageResource) configured(d *diag.Diagnostics) bool {
	if r.service == nil {
		d.AddError("Missing provider client", "Configure iwinv credentials before managing NAS.")
		return false
	}
	return true
}
func validateNASConfig(m sharedStorageModel, d *diag.Diagnostics) {
	validateGroupTimeouts(m.Timeouts, d)
	if !m.ProductID.IsNull() && !m.ProductID.IsUnknown() && m.ProductID.ValueString() == "" {
		d.AddAttributeError(path.Root("product_id"), "Invalid NAS product ID", "Use a nonempty exact creation product ID. Some catalog rows have no creation ID.")
	}
	if !m.ShareName.IsNull() && !m.ShareName.IsUnknown() && !storageShareName.MatchString(m.ShareName.ValueString()) {
		d.AddAttributeError(path.Root("share_name"), "Invalid NAS share name", "Use 6–20 ASCII letters/digits.")
	}
	if !m.SizeGB.IsNull() && !m.SizeGB.IsUnknown() && (m.SizeGB.ValueInt64() < 100 || m.SizeGB.ValueInt64() > 2000) {
		d.AddAttributeError(path.Root("size_gb"), "Invalid NAS capacity", "Use the documented 100–2000 GB range and a reviewed product. Capacity changes replace the service.")
	}
	if !m.AllowedIPs.IsNull() && !m.AllowedIPs.IsUnknown() {
		ips := map[string]string{}
		unknown := false
		for key, value := range m.AllowedIPs.Elements() {
			if value.IsNull() {
				d.AddAttributeError(path.Root("allowed_ips"), "Invalid NAS allowed IPs", "Null permissions are unsupported.")
				return
			}
			if value.IsUnknown() {
				unknown = true
				continue
			}
			ips[key] = value.(types.String).ValueString()
		}
		if !unknown {
			if err := hosted.ValidateNASAllowIPs(ips); err != nil {
				d.AddAttributeError(path.Root("allowed_ips"), "Invalid NAS allowed IPs", err.Error())
			}
		}
	}
}

func (r *sharedStorageResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m sharedStorageModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		validateNASConfig(m, &resp.Diagnostics)
	}
}
func nasReplacementPaths(a, b sharedStorageModel) path.Paths {
	out := path.Paths{}
	for _, p := range []struct {
		name string
		a, b attr.Value
	}{{"product_id", a.ProductID, b.ProductID}, {"share_name", a.ShareName, b.ShareName}, {"name", a.Name, b.Name}, {"description", a.Description, b.Description}, {"size_gb", a.SizeGB, b.SizeGB}} {
		if !p.a.Equal(p.b) {
			out = append(out, path.Root(p.name))
		}
	}
	return out
}
func (r *sharedStorageResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan sharedStorageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if req.State.Raw.IsNull() {
		if plan.ShareName.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("share_name"), "NAS creation share name required", "Supply an initial share name for new creation. Import may omit unknown creation history.")
		}
		return
	}
	var prior sharedStorageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	replacements := nasReplacementPaths(prior, plan)
	if len(replacements) == 0 {
		return
	}
	resp.RequiresReplace = append(resp.RequiresReplace, replacements...)
	if plan.ShareName.IsNull() || plan.ShareName.IsUnknown() || plan.ShareName.Equal(prior.ShareName) {
		resp.Diagnostics.AddAttributeError(path.Root("share_name"), "NAS replacement requires a new share name", "The API cannot update these settings. Choose a known different share name before destructive replacement. For an imported service without share-name history, explicitly choose a fresh share name; Read cannot verify whether it was previously used. No write was performed.")
	}
}
func knownNASPlan(ctx context.Context, m sharedStorageModel, create bool, d *diag.Diagnostics) (map[string]string, bool) {
	validateNASConfig(m, d)
	if d.HasError() || !knownOperationTimeouts(m.Timeouts, d) {
		return nil, false
	}
	values := []attr.Value{m.ProductID, m.Name, m.Description, m.SizeGB, m.AllowedIPs}
	if create {
		values = append(values, m.ShareName)
	}
	for _, v := range values {
		if v.IsUnknown() || v.IsNull() {
			d.AddError("Incomplete NAS configuration", "All required creation/update inputs must be known before writing.")
			return nil, false
		}
	}
	ips := map[string]string{}
	d.Append(m.AllowedIPs.ElementsAs(ctx, &ips, false)...)
	if d.HasError() {
		return nil, false
	}
	return ips, true
}
func setNASObserved(ctx context.Context, m *sharedStorageModel, g *hosted.NAS, d *diag.Diagnostics) {
	m.ID = types.StringValue(g.ID)
	m.ProductID = types.StringValue(g.ProductID)
	m.Name = types.StringValue(g.Name)
	desc := ""
	if g.Description != nil {
		desc = *g.Description
	}
	m.Description = types.StringValue(desc)
	m.Status = types.StringValue(g.Status)
	m.SizeGB = types.Int64Value(g.DiskGB)
	m.Address = types.StringValue(g.Domain)
	m.MountInfo = types.StringValue(g.MountInfo)
	var ds diag.Diagnostics
	m.AllowedIPs, ds = types.MapValueFrom(ctx, types.StringType, g.AllowIPs)
	d.Append(ds...)
	// ShareName is historical create-only input; never infer it from domain/mount text.
}

func nasMatches(ctx context.Context, g *hosted.NAS, m *sharedStorageModel) bool {
	if g == nil || g.ID != m.ID.ValueString() || g.ProductID != m.ProductID.ValueString() || g.Name != m.Name.ValueString() || g.DiskGB != m.SizeGB.ValueInt64() {
		return false
	}
	desc := ""
	if g.Description != nil {
		desc = *g.Description
	}
	if desc != m.Description.ValueString() {
		return false
	}
	ips := map[string]string{}
	if ds := m.AllowedIPs.ElementsAs(ctx, &ips, false); ds.HasError() {
		return false
	}
	return reflect.DeepEqual(ips, g.AllowIPs)
}
func (r *sharedStorageResource) wait(ctx context.Context, id string, want *sharedStorageModel) (*hosted.NAS, error) {
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
			if row.Status == "active" && nasMatches(ctx, row, want) {
				return row, nil
			}
			if row.Status != "active" && row.Status != "pending" && row.Status != "waiting" {
				return nil, errors.New("unverified or failed NAS status; reconcile before another apply")
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
func (r *sharedStorageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m sharedStorageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ips, ok := knownNASPlan(ctx, m, true, &resp.Diagnostics)
	if !ok {
		return
	}
	in := hosted.NASInput{ProductID: m.ProductID.ValueString(), ShareName: m.ShareName.ValueString(), Name: m.Name.ValueString(), DiskGB: m.SizeGB.ValueInt64(), AllowIPs: ips}
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
		m.Address = types.StringNull()
		m.MountInfo = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create iwinv NAS", err.Error()+" No create retry was performed. A known ID remains in state; reconcile an unknown outcome before another apply.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	row, err := r.wait(opCtx, created.ID, &m)
	if err != nil {
		resp.Diagnostics.AddError("NAS creation not verified", err.Error()+" The identity is retained. Inspect it before accepting a tainted replacement.")
		return
	}
	setNASObserved(ctx, &m, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *sharedStorageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m sharedStorageModel
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
		resp.Diagnostics.AddError("Unable to read iwinv NAS", err.Error())
		return
	}
	if row == nil {
		if m.Status.IsNull() || m.Status.ValueString() != "active" {
			resp.Diagnostics.AddError("NAS identity not yet verified", "The service is missing from this successful list but has not previously been observed active. Retain and reconcile the ID before replacement.")
			return
		}
		resp.State.RemoveResource(ctx)
		return
	}
	setNASObserved(ctx, &m, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
func (r *sharedStorageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var prior, plan sharedStorageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
	ips, ok := knownNASPlan(ctx, plan, false, &resp.Diagnostics)
	if !ok {
		return
	}
	if len(nasReplacementPaths(prior, plan)) > 0 {
		resp.Diagnostics.AddError("Unsupported NAS update", "Only allowed_ips can change remotely in place. Other creation attributes require replacement. No write was performed.")
		return
	}
	plan.ID = prior.ID
	opCtx, cancel := groupOperationContext(ctx, plan.Timeouts, "update", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	if !prior.AllowedIPs.Equal(plan.AllowedIPs) {
		observed, err := r.service.Read(opCtx, prior.ID.ValueString())
		if err != nil || observed == nil {
			resp.Diagnostics.AddError("Unable to verify NAS before update", "Read the parent successfully before retrying; prior state is retained and no write was attempted.")
			return
		}
		if !nasMatches(ctx, observed, &prior) {
			resp.Diagnostics.AddError("NAS changed before update", "Refresh and re-plan; the current parent differs from prior state. No write was performed.")
			return
		}
		if err := r.service.ReplaceAllowIPs(opCtx, prior.ID.ValueString(), ips); err != nil {
			resp.Diagnostics.AddError("Unable to update NAS allowed IPs", err.Error()+" No write retry was performed; refresh to reconcile the remote outcome.")
			return
		}
	}
	row, err := r.wait(opCtx, prior.ID.ValueString(), &plan)
	if err != nil {
		resp.Diagnostics.AddError("NAS update not verified", err.Error()+" Prior state is retained.")
		return
	}
	setNASObserved(ctx, &plan, row, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}
func (r *sharedStorageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m sharedStorageModel
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
		resp.Diagnostics.AddError("Unable to verify NAS before deletion", err.Error())
		return
	}
	if row == nil && !m.Status.IsNull() && m.Status.ValueString() == "active" {
		return
	}
	if err = r.service.Delete(opCtx, m.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete iwinv NAS", err.Error()+" No delete retry was performed; state is retained.")
		return
	}
	if _, err = r.wait(opCtx, m.ID.ValueString(), nil); err != nil {
		resp.Diagnostics.AddError("NAS deletion not verified", err.Error()+" The identity remains in state.")
	}
}
func (r *sharedStorageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if err := hosted.ValidateServiceID(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid NAS import ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
