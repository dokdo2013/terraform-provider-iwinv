package provider

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type securityGroupResource struct {
	network      *network.Service
	pollInterval time.Duration
}

var (
	_ resource.ResourceWithConfigure      = &securityGroupResource{}
	_ resource.ResourceWithImportState    = &securityGroupResource{}
	_ resource.ResourceWithValidateConfig = &securityGroupResource{}
)

func NewSecurityGroupResource() resource.Resource {
	return &securityGroupResource{pollInterval: time.Second}
}

type securityGroupModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	AllowICMP   types.Bool     `tfsdk:"allow_icmp"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func (r *securityGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_group"
}

func (r *securityGroupResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages iwinv security group attributes. Rules and instance attachments are separate ownership scopes and are not managed by this resource. Empty descriptions are not supported by the verified API contract.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Exact firewall_id, also used for import.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, MarkdownDescription: "Group name (documented limit: 4–32 characters). Updated in place.", Validators: []validator.String{groupTextLength{min: 4, max: 32}}},
			"description": schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("Managed by Terraform"), MarkdownDescription: "Nonempty description (up to 50 characters). Defaults to Managed by Terraform. Empty creation and clearing are not verified and are rejected before apply.", Validators: []validator.String{groupTextLength{min: 1, max: 50}}},
			"allow_icmp":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), MarkdownDescription: "Explicit ICMP flag, default false. This is not a proof of packet behavior."},
		},
		Blocks: map[string]schema.Block{"timeouts": timeouts.BlockAll(ctx)},
	}
}

type groupTextLength struct {
	min, max int
	summary  string
}

func (v groupTextLength) Description(context.Context) string {
	return fmt.Sprintf("Must contain between %d and %d Unicode characters.", v.min, v.max)
}
func (v groupTextLength) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (v groupTextLength) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	n := utf8.RuneCountInString(req.ConfigValue.ValueString())
	if n < v.min || n > v.max {
		summary := v.summary
		if summary == "" {
			summary = "Invalid security group text"
		}
		resp.Diagnostics.AddAttributeError(req.Path, summary, v.Description(ctx))
	}
}

func (r *securityGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	service, ok := req.ProviderData.(*resourceServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected the configured iwinv network client.")
		return
	}
	r.network = service.Network
}

// Validate every configured timeout before the first write. Otherwise a valid
// create timeout could persist an invalid read timeout and prevent refresh.
func validateGroupTimeouts(value timeouts.Value, diags *diag.Diagnostics) {
	for name, value := range value.Attributes() {
		if value.IsNull() || value.IsUnknown() {
			continue
		}
		duration, err := time.ParseDuration(value.(types.String).ValueString())
		if err != nil || duration <= 0 {
			diags.AddAttributeError(path.Root("timeouts").AtName(name), "Invalid operation timeout", "Timeout must be a positive duration.")
		}
	}
}

func knownOperationTimeouts(value timeouts.Value, diags *diag.Diagnostics) bool {
	unknown := value.IsUnknown()
	for _, v := range value.Attributes() {
		unknown = unknown || v.IsUnknown()
	}
	if unknown {
		diags.AddError("Unknown operation timeouts", "Timeout values must be known before a write so the returned identity can be stored in valid Terraform state.")
		return false
	}
	return true
}

func (r *securityGroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config securityGroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if !resp.Diagnostics.HasError() {
		validateGroupTimeouts(config.Timeouts, &resp.Diagnostics)
	}
}

func (r *securityGroupResource) configured(diags *diag.Diagnostics) bool {
	if r.network == nil {
		diags.AddError("Missing provider client", "Configure iwinv credentials before managing security groups.")
		return false
	}
	return true
}

func groupOperationContext(ctx context.Context, value timeouts.Value, operation string, diags *diag.Diagnostics) (context.Context, context.CancelFunc) {
	var duration time.Duration
	var d diag.Diagnostics
	switch operation {
	case "create":
		duration, d = value.Create(ctx, 5*time.Minute)
	case "read":
		duration, d = value.Read(ctx, time.Minute)
	case "update":
		duration, d = value.Update(ctx, 5*time.Minute)
	case "delete":
		duration, d = value.Delete(ctx, 5*time.Minute)
	}
	diags.Append(d...)
	if duration <= 0 && !diags.HasError() {
		diags.AddAttributeError(path.Root("timeouts").AtName(operation), "Invalid operation timeout", "Timeout must be a positive duration.")
	}
	if diags.HasError() {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, duration)
}

func knownGroupPlan(data securityGroupModel, diags *diag.Diagnostics) bool {
	validateGroupTimeouts(data.Timeouts, diags)
	if diags.HasError() || !knownOperationTimeouts(data.Timeouts, diags) {
		return false
	}
	if data.Name.IsNull() || data.Name.IsUnknown() || data.Description.IsNull() || data.Description.IsUnknown() || data.AllowICMP.IsNull() || data.AllowICMP.IsUnknown() {
		diags.AddError("Unknown security group configuration", "Name, description and ICMP settings must be known before a write.")
		return false
	}
	return true
}

func groupMatches(g *network.Group, want *securityGroupModel) bool {
	return g != nil && g.ID == want.ID.ValueString() && g.Name == want.Name.ValueString() && g.Description != nil && *g.Description == want.Description.ValueString() && g.AllowICMP == want.AllowICMP.ValueBool()
}

// Only successful but incomplete reads are polled. API errors never become
// absence or write retries. A nil want means waiting for acknowledged deletion.
func (r *securityGroupResource) wait(ctx context.Context, id string, want *securityGroupModel) (*network.Group, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		g, err := r.network.Group(ctx, id)
		if err != nil {
			return nil, err
		}
		if (want == nil && g == nil) || (want != nil && groupMatches(g, want)) {
			return g, nil
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

func (r *securityGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var data securityGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() || !knownGroupPlan(data, &resp.Diagnostics) {
		return
	}
	opCtx, cancel := groupOperationContext(ctx, data.Timeouts, "create", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	description := data.Description.ValueString()
	created, err := r.network.CreateGroup(opCtx, network.GroupInput{Name: data.Name.ValueString(), Description: &description, AllowICMP: data.AllowICMP.ValueBool()})
	if created.ID != "" {
		data.ID = types.StringValue(created.ID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create iwinv security group", err.Error()+" No create retry was performed. A known ID is retained in state; reconcile unknown outcomes before another apply.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err = r.wait(opCtx, data.ID.ValueString(), &data); err != nil {
		resp.Diagnostics.AddError("Security group creation not verified", err.Error()+" The returned ID remains in state. Inspect it before accepting Terraform's proposed replacement of a tainted resource.")
	}
}

func (r *securityGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var data securityGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := groupOperationContext(ctx, data.Timeouts, "read", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.network.Group(opCtx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read iwinv security group", err.Error())
		return
	}
	if g == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	data.Name = types.StringValue(g.Name)
	data.Description = types.StringPointerValue(g.Description)
	data.AllowICMP = types.BoolValue(g.AllowICMP)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *securityGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var plan, prior securityGroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() || !knownGroupPlan(plan, &resp.Diagnostics) {
		return
	}
	plan.ID = prior.ID
	// Keep the previous state on every failed or uncertain update.
	resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
	opCtx, cancel := groupOperationContext(ctx, plan.Timeouts, "update", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Name.Equal(prior.Name) || !plan.Description.Equal(prior.Description) || !plan.AllowICMP.Equal(prior.AllowICMP) {
		var description *string
		if !plan.Description.Equal(prior.Description) {
			value := plan.Description.ValueString()
			description = &value
		}
		if _, err := r.network.UpdateGroup(opCtx, plan.ID.ValueString(), network.GroupInput{Name: plan.Name.ValueString(), Description: description, AllowICMP: plan.AllowICMP.ValueBool()}); err != nil {
			resp.Diagnostics.AddError("Unable to update iwinv security group", err.Error()+" No write retry was performed; refresh to reconcile the remote outcome.")
			return
		}
		if _, err := r.wait(opCtx, plan.ID.ValueString(), &plan); err != nil {
			resp.Diagnostics.AddError("Security group update not verified", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *securityGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var data securityGroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	opCtx, cancel := groupOperationContext(ctx, data.Timeouts, "delete", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.network.Group(opCtx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify security group before deletion", err.Error())
		return
	}
	if g == nil {
		return
	}
	if err := r.network.DeleteGroup(opCtx, data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete iwinv security group", err.Error()+" No delete retry was performed.")
		return
	}
	if _, err := r.wait(opCtx, data.ID.ValueString(), nil); err != nil {
		resp.Diagnostics.AddError("Security group deletion not verified", err.Error()+" The resource remains in state.")
	}
}

func (r *securityGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if err := network.ValidateGroupID(req.ID); err != nil {
		resp.Diagnostics.AddError("Invalid security group import ID", err.Error())
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
