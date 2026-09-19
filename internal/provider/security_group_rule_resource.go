package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
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

type securityGroupRuleResource struct {
	network      *network.Service
	bound        string
	pollInterval time.Duration
}

var (
	_ resource.ResourceWithConfigure      = &securityGroupRuleResource{}
	_ resource.ResourceWithValidateConfig = &securityGroupRuleResource{}
	_ resource.ResourceWithModifyPlan     = &securityGroupRuleResource{}
	_ resource.ResourceWithImportState    = &securityGroupRuleResource{}
)

func NewSecurityGroupIngressRuleResource() resource.Resource {
	return &securityGroupRuleResource{bound: "IN", pollInterval: time.Second}
}
func NewSecurityGroupEgressRuleResource() resource.Resource {
	return &securityGroupRuleResource{bound: "OUT", pollInterval: time.Second}
}
func (r *securityGroupRuleResource) direction() string {
	if r.bound == "OUT" {
		return "egress"
	}
	return "ingress"
}

type securityGroupRuleModel struct {
	ID          types.String   `tfsdk:"id"`
	RuleID      types.String   `tfsdk:"rule_id"`
	GroupID     types.String   `tfsdk:"security_group_id"`
	Direction   types.String   `tfsdk:"direction"`
	Protocol    types.String   `tfsdk:"ip_protocol"`
	FromPort    types.Int64    `tfsdk:"from_port"`
	ToPort      types.Int64    `tfsdk:"to_port"`
	CIDR        types.String   `tfsdk:"cidr_ipv4"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	Timeouts    timeouts.Value `tfsdk:"timeouts"`
}

func (r *securityGroupRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_group_" + r.direction() + "_rule"
}
func (r *securityGroupRuleResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Manages one " + r.direction() + " TCP/UDP rule in an iwinv security group. Clearing an existing description requires replacement. Rules do not own the parent group or its instance attachments.", Attributes: map[string]schema.Attribute{
		"id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Composite identity security_group_id/rule_id, also used for import.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"rule_id":           schema.StringAttribute{Computed: true, MarkdownDescription: "Exact positive decimal API rule_id.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"security_group_id": schema.StringAttribute{Required: true, MarkdownDescription: "Exact parent firewall_id. Changing the parent replaces the rule.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"direction":         schema.StringAttribute{Computed: true, MarkdownDescription: "Observed direction. A plan restores the direction fixed by the resource type when external drift changes it."},
		"ip_protocol":       schema.StringAttribute{Required: true, MarkdownDescription: "tcp or udp. Updated in place."},
		"from_port":         schema.Int64Attribute{Required: true, MarkdownDescription: "First port, 1–65535 inclusive."},
		"to_port":           schema.Int64Attribute{Required: true, MarkdownDescription: "Last port, 1–65535 and at least from_port. Use equal values for a single port."},
		"cidr_ipv4":         schema.StringAttribute{Required: true, MarkdownDescription: "IPv4 CIDR. Host bits are preserved exactly; IPv6 and bare addresses are unsupported."},
		"name":              schema.StringAttribute{Required: true, MarkdownDescription: "Rule name, 1–25 Unicode code points.", Validators: []validator.String{groupTextLength{min: 1, max: 25}}},
		"description":       schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString(""), MarkdownDescription: "Up to 25 Unicode code points; default empty. Empty API null is represented as an empty string. Clearing a nonempty value requires replacement.", Validators: []validator.String{groupTextLength{min: 0, max: 25}}},
	}, Blocks: map[string]schema.Block{"timeouts": timeouts.BlockAll(ctx)}}
}
func (r *securityGroupRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected the configured iwinv network client.")
		return
	}
	r.network = s.Network
}
func (r *securityGroupRuleResource) configured(d *diag.Diagnostics) bool {
	if r.network == nil {
		d.AddError("Missing provider client", "Configure iwinv credentials before managing rules.")
		return false
	}
	return true
}
func validateRuleConfiguration(m securityGroupRuleModel, d *diag.Diagnostics) {
	validateGroupTimeouts(m.Timeouts, d)
	if !m.GroupID.IsNull() && !m.GroupID.IsUnknown() {
		if err := network.ValidateGroupID(m.GroupID.ValueString()); err != nil {
			d.AddAttributeError(path.Root("security_group_id"), "Invalid security group ID", err.Error())
		}
	}
	if !m.Protocol.IsNull() && !m.Protocol.IsUnknown() && m.Protocol.ValueString() != "tcp" && m.Protocol.ValueString() != "udp" {
		d.AddAttributeError(path.Root("ip_protocol"), "Unsupported rule protocol", "Use tcp or udp; other protocols have no verified rule contract.")
	}
	if !m.CIDR.IsNull() && !m.CIDR.IsUnknown() {
		if err := network.ValidateRuleIPv4(m.CIDR.ValueString()); err != nil {
			d.AddAttributeError(path.Root("cidr_ipv4"), "Unsupported rule address", err.Error())
		}
	}
	for key, value := range map[string]types.Int64{"from_port": m.FromPort, "to_port": m.ToPort} {
		if !value.IsNull() && !value.IsUnknown() && (value.ValueInt64() < 1 || value.ValueInt64() > 65535) {
			d.AddAttributeError(path.Root(key), "Invalid rule port", "Port must be between 1 and 65535.")
		}
	}
	if !m.FromPort.IsNull() && !m.FromPort.IsUnknown() && !m.ToPort.IsNull() && !m.ToPort.IsUnknown() && m.FromPort.ValueInt64() > m.ToPort.ValueInt64() {
		d.AddAttributeError(path.Root("to_port"), "Invalid rule port range", "to_port must be at least from_port.")
	}
}
func (r *securityGroupRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m securityGroupRuleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		validateRuleConfiguration(m, &resp.Diagnostics)
	}
}
func (r *securityGroupRuleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	// Direction is computed but is managed, not merely observed. Expose external
	// direction drift in state, then explicitly plan its restoration.
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("direction"), r.direction())...)
	if req.State.Raw.IsNull() {
		return
	}
	var plan, prior securityGroupRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// An unknown new description might resolve to empty during apply. Plan
	// replacement conservatively now instead of introducing it after approval.
	if prior.Description.ValueString() != "" && (plan.Description.IsUnknown() || (!plan.Description.IsNull() && plan.Description.ValueString() == "")) {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("description"))
	}
}
func knownRulePlan(m securityGroupRuleModel, d *diag.Diagnostics) bool {
	validateRuleConfiguration(m, d)
	if d.HasError() || !knownOperationTimeouts(m.Timeouts, d) {
		return false
	}
	for _, v := range []types.String{m.GroupID, m.Protocol, m.CIDR, m.Name, m.Description} {
		if v.IsNull() || v.IsUnknown() {
			d.AddError("Unknown security rule configuration", "All rule inputs must be known before a write.")
			return false
		}
	}
	if m.FromPort.IsNull() || m.FromPort.IsUnknown() || m.ToPort.IsNull() || m.ToPort.IsUnknown() {
		d.AddError("Unknown security rule ports", "Both rule ports must be known before a write.")
	}
	return !d.HasError()
}
func (r *securityGroupRuleResource) input(m securityGroupRuleModel) network.RuleInput {
	port := fmt.Sprint(m.FromPort.ValueInt64())
	if !m.FromPort.Equal(m.ToPort) {
		port += "-" + fmt.Sprint(m.ToPort.ValueInt64())
	}
	desc := m.Description.ValueString()
	return network.RuleInput{Bound: r.bound, Protocol: strings.ToUpper(m.Protocol.ValueString()), Port: port, IP: m.CIDR.ValueString(), Name: m.Name.ValueString(), Description: &desc}
}
func parseRuleIdentity(id string) (string, string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("rule identity must be security_group_id/rule_id")
	}
	if err := network.ValidateGroupID(parts[0]); err != nil {
		return "", "", err
	}
	if err := network.ValidateRuleID(parts[1]); err != nil {
		return "", "", err
	}
	return parts[0], parts[1], nil
}
func validRuleState(m securityGroupRuleModel, d *diag.Diagnostics) bool {
	group, id, err := parseRuleIdentity(m.ID.ValueString())
	if err != nil {
		d.AddError("Invalid rule state identity", err.Error())
		return false
	}
	if group != m.GroupID.ValueString() || id != m.RuleID.ValueString() {
		d.AddError("Inconsistent rule state identity", "Composite identity does not match the parent and rule ID attributes.")
		return false
	}
	return true
}
func ruleDescription(rule *network.Rule) string {
	if rule.Description == nil {
		return ""
	}
	return *rule.Description
}
func (r *securityGroupRuleResource) matches(rule *network.Rule, m *securityGroupRuleModel) bool {
	if rule == nil {
		return false
	}
	from, to, err := network.ParseRulePorts(rule.Port)
	return err == nil && rule.ID == m.RuleID.ValueString() && rule.Bound == r.bound && strings.ToLower(rule.Protocol) == m.Protocol.ValueString() && from == m.FromPort.ValueInt64() && to == m.ToPort.ValueInt64() && rule.IP == m.CIDR.ValueString() && rule.Name == m.Name.ValueString() && ruleDescription(rule) == m.Description.ValueString()
}
func (r *securityGroupRuleResource) wait(ctx context.Context, group, id string, want *securityGroupRuleModel) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rule, err := r.network.Rule(ctx, group, id)
		if err != nil {
			return err
		}
		if (want == nil && rule == nil) || (want != nil && r.matches(rule, want)) {
			return nil
		}
		timer := time.NewTimer(r.pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (r *securityGroupRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m securityGroupRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || !knownRulePlan(m, &resp.Diagnostics) {
		return
	}
	op, cancel := groupOperationContext(ctx, m.Timeouts, "create", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := r.network.CreateRule(op, m.GroupID.ValueString(), r.input(m))
	if created.ID != "" {
		m.RuleID = types.StringValue(created.ID)
		m.ID = types.StringValue(m.GroupID.ValueString() + "/" + created.ID)
		m.Direction = types.StringValue(r.direction())
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to create iwinv security rule", err.Error()+" No create retry was performed. Reconcile unknown outcomes; a returned ID is retained in state.")
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if err = r.wait(op, m.GroupID.ValueString(), m.RuleID.ValueString(), &m); err != nil {
		resp.Diagnostics.AddError("Security rule creation not verified", err.Error()+" The returned ID remains in state. Inspect Terraform's proposed replacement before another apply.")
	}
}
func (r *securityGroupRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m securityGroupRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || !validRuleState(m, &resp.Diagnostics) {
		return
	}
	op, cancel := groupOperationContext(ctx, m.Timeouts, "read", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.network.Rule(op, m.GroupID.ValueString(), m.RuleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read iwinv security rule", err.Error())
		return
	}
	if rule == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	from, to, err := network.ParseRulePorts(rule.Port)
	if err != nil {
		resp.Diagnostics.AddError("Invalid security rule ports", err.Error())
		return
	}
	direction := "ingress"
	if rule.Bound == "OUT" {
		direction = "egress"
	}
	m.Direction = types.StringValue(direction)
	m.Protocol = types.StringValue(strings.ToLower(rule.Protocol))
	m.FromPort = types.Int64Value(from)
	m.ToPort = types.Int64Value(to)
	m.CIDR = types.StringValue(rule.IP)
	m.Name = types.StringValue(rule.Name)
	m.Description = types.StringValue(ruleDescription(rule))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *securityGroupRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var plan, prior securityGroupRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() || !knownRulePlan(plan, &resp.Diagnostics) || !validRuleState(prior, &resp.Diagnostics) {
		return
	}
	if !plan.GroupID.Equal(prior.GroupID) {
		resp.Diagnostics.AddError("Rule parent change requires replacement", "The existing rule cannot be moved to another group.")
		return
	}
	plan.ID, plan.RuleID = prior.ID, prior.RuleID
	plan.Direction = types.StringValue(r.direction())
	resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
	op, cancel := groupOperationContext(ctx, plan.Timeouts, "update", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	changed := !plan.Direction.Equal(prior.Direction) || !plan.Protocol.Equal(prior.Protocol) || !plan.FromPort.Equal(prior.FromPort) || !plan.ToPort.Equal(prior.ToPort) || !plan.CIDR.Equal(prior.CIDR) || !plan.Name.Equal(prior.Name) || !plan.Description.Equal(prior.Description)
	if changed {
		input := r.input(plan)
		if plan.Description.Equal(prior.Description) {
			input.Description = nil
		}
		if _, err := r.network.UpdateRule(op, plan.GroupID.ValueString(), plan.RuleID.ValueString(), input); err != nil {
			resp.Diagnostics.AddError("Unable to update iwinv security rule", err.Error()+" No write retry was performed; refresh to reconcile the outcome.")
			return
		}
		if err := r.wait(op, plan.GroupID.ValueString(), plan.RuleID.ValueString(), &plan); err != nil {
			resp.Diagnostics.AddError("Security rule update not verified", err.Error())
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
func (r *securityGroupRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.configured(&resp.Diagnostics) {
		return
	}
	var m securityGroupRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || !validRuleState(m, &resp.Diagnostics) {
		return
	}
	op, cancel := groupOperationContext(ctx, m.Timeouts, "delete", &resp.Diagnostics)
	defer cancel()
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.network.Rule(op, m.GroupID.ValueString(), m.RuleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify rule before deletion", err.Error())
		return
	}
	if rule == nil {
		return
	}
	if err = r.network.DeleteRule(op, m.GroupID.ValueString(), m.RuleID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete iwinv security rule", err.Error()+" No delete retry was performed.")
		return
	}
	if err = r.wait(op, m.GroupID.ValueString(), m.RuleID.ValueString(), nil); err != nil {
		resp.Diagnostics.AddError("Security rule deletion not verified", err.Error()+" The rule remains in state.")
	}
}
func (r *securityGroupRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	group, id, err := parseRuleIdentity(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid security rule import identity", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("security_group_id"), group)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("rule_id"), id)...)
}
