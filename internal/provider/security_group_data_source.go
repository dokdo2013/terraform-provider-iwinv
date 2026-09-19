package provider

import (
	"context"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type securityGroupDataSource struct {
	singular bool
	network  *network.Service
}

var _ datasource.DataSourceWithConfigure = &securityGroupDataSource{}
var _ datasource.DataSourceWithValidateConfig = &securityGroupDataSource{}

func NewSecurityGroupsDataSource() datasource.DataSource { return &securityGroupDataSource{} }
func NewSecurityGroupDataSource() datasource.DataSource {
	return &securityGroupDataSource{singular: true}
}
func (d *securityGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_groups"
	if d.singular {
		resp.TypeName = req.ProviderTypeName + "_security_group"
	}
}
func groupDataAttributes(requiredID bool) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id":          schema.StringAttribute{Required: requiredID, Computed: !requiredID, MarkdownDescription: "Exact API FIREWALL ID."},
		"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "API title, preserved literally."},
		"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Description decoded once from API HTML escaping; null stays null."},
		"allow_icmp":  schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the API ICMP flag is Y."},
	}
}
func (d *securityGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	if d.singular {
		resp.Schema = schema.Schema{MarkdownDescription: "Reads one exact security group ID. Absence is an error. Does not manage rules or attachments.", Attributes: groupDataAttributes(true)}
		return
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads all API-visible groups in lexical ID order after validating every page. No name-based selection or mutation.", Attributes: map[string]schema.Attribute{
		"ids":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "All exact IDs, sorted lexically."},
		"groups": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Group attributes in the same order as ids. Rules and attachments are excluded.", NestedObject: schema.NestedAttributeObject{Attributes: groupDataAttributes(false)}},
	}}
}
func (d *securityGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected the configured iwinv network client.")
		return
	}
	d.network = s.Network
}
func (d *securityGroupDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	if !d.singular {
		return
	}
	var id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if resp.Diagnostics.HasError() || id.IsUnknown() {
		return
	}
	if id.IsNull() || network.ValidateGroupID(id.ValueString()) != nil {
		resp.Diagnostics.AddAttributeError(path.Root("id"), "Invalid security group ID", "Use one exact FIREWALL ID, without a path or query string.")
	}
}

type securityGroupDataModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	AllowICMP   types.Bool   `tfsdk:"allow_icmp"`
}

func groupDataModel(g network.Group) securityGroupDataModel {
	return securityGroupDataModel{ID: types.StringValue(g.ID), Name: types.StringValue(g.Name), Description: types.StringPointerValue(g.Description), AllowICMP: types.BoolValue(g.AllowICMP)}
}
func (d *securityGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.network == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading security groups.")
		return
	}
	if d.singular {
		var id types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if id.IsNull() || id.IsUnknown() || network.ValidateGroupID(id.ValueString()) != nil {
			resp.Diagnostics.AddError("Invalid security group ID", "A known exact FIREWALL ID is required.")
			return
		}
		g, err := d.network.Group(ctx, id.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read iwinv security group", err.Error())
			return
		}
		if g == nil {
			resp.Diagnostics.AddError("Security group not found", "The detail endpoint returned no group for the exact requested ID.")
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, groupDataModel(*g))...)
		return
	}
	groups, err := d.network.Groups(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read iwinv security groups", err.Error())
		return
	}
	state := struct {
		IDs    []string                 `tfsdk:"ids"`
		Groups []securityGroupDataModel `tfsdk:"groups"`
	}{IDs: make([]string, 0, len(groups)), Groups: make([]securityGroupDataModel, 0, len(groups))}
	for _, g := range groups {
		state.IDs = append(state.IDs, g.ID)
		state.Groups = append(state.Groups, groupDataModel(g))
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
