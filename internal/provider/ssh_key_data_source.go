package provider

import (
	"context"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type sshKeyDataSource struct {
	singular bool
	compute  *compute.Service
}

var _ datasource.DataSourceWithConfigure = &sshKeyDataSource{}

func NewSSHKeysDataSource() datasource.DataSource { return &sshKeyDataSource{} }
func NewSSHKeyDataSource() datasource.DataSource  { return &sshKeyDataSource{singular: true} }

func (d *sshKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_keys"
	if d.singular {
		resp.TypeName = req.ProviderTypeName + "_ssh_key"
	}
}
func (d *sshKeyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	if d.singular {
		resp.Schema = schema.Schema{MarkdownDescription: "Looks up one exact existing SSH key ID after validating all list pages. Read-only; no key material is exposed.", Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Required: true, MarkdownDescription: "Exact API ssh_key_id. Missing IDs are errors; names are not used for selection."},
			"name": schema.StringAttribute{Computed: true, MarkdownDescription: "Key display name returned by the API."},
		}}
		return
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Lists all API-visible SSH key references, sorted by exact ID. This does not create, upload, delete or retrieve key material.", Attributes: map[string]schema.Attribute{
		"ids": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Exact SSH key IDs, sorted lexically."},
		"keys": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Key reference metadata in the same order as ids.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true},
			"name": schema.StringAttribute{Computed: true},
		}}},
	}}
}
func (d *sshKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	service, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected the configured iwinv compute client.")
		return
	}
	d.compute = service.Compute
}

type sshKeyModel struct {
	ID   string `tfsdk:"id"`
	Name string `tfsdk:"name"`
}

func (d *sshKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.compute == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading SSH keys.")
		return
	}
	if d.singular {
		var id types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if id.IsNull() || id.IsUnknown() || id.ValueString() == "" {
			resp.Diagnostics.AddError("Invalid SSH key ID", "A known, non-empty exact SSH key ID is required.")
			return
		}
		key, err := d.compute.SSHKey(ctx, id.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read iwinv SSH key", err.Error())
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, sshKeyModel{ID: key.ID, Name: key.Name})...)
		return
	}
	keys, err := d.compute.SSHKeys(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read iwinv SSH keys", err.Error())
		return
	}
	state := struct {
		IDs  []string      `tfsdk:"ids"`
		Keys []sshKeyModel `tfsdk:"keys"`
	}{IDs: make([]string, 0, len(keys)), Keys: make([]sshKeyModel, 0, len(keys))}
	for _, key := range keys {
		state.IDs = append(state.IDs, key.ID)
		state.Keys = append(state.Keys, sshKeyModel{ID: key.ID, Name: key.Name})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
