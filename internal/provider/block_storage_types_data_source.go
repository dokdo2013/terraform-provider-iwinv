package provider

import (
	"context"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/storage"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type blockStorageTypesDataSource struct{ service *storage.Service }

var _ datasource.DataSourceWithConfigure = &blockStorageTypesDataSource{}
var _ datasource.DataSourceWithValidateConfig = &blockStorageTypesDataSource{}

func NewBlockStorageTypesDataSource() datasource.DataSource { return &blockStorageTypesDataSource{} }
func (d *blockStorageTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_block_storage_types"
}
func (d *blockStorageTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads block-storage type bounds and nullable zone restrictions. Catalog visibility is not provisioning eligibility. No volumes are created or attached.", Attributes: map[string]schema.Attribute{
		"type": schema.StringAttribute{Optional: true, MarkdownDescription: "Exact optional disk-type filter. Null omits the query; empty is invalid. API errors for unsupported values remain errors."},
		"types": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Complete catalog sorted by exact disk type. There is no implicit default selection.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"type":               schema.StringAttribute{Computed: true, MarkdownDescription: "Exact API disk type."},
			"minimum_size_gb":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum size in documented GB."},
			"maximum_size_gb":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum size in documented GB; not a resize guarantee."},
			"availability_zones": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Exact zone strings in lexical order, or null if the API supplies null. Null does not prove all zones are usable."},
		}}},
	}}
}
func (d *blockStorageTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	d.service = s.BlockStorage
}
func (d *blockStorageTypesDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var filter types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &filter)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !filter.IsNull() && !filter.IsUnknown() && filter.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root("type"), "Invalid block storage type filter", "Omit type or provide a nonempty exact API disk type.")
	}
}

type blockStorageTypeModel struct {
	Type              string     `tfsdk:"type"`
	MinimumSizeGB     int64      `tfsdk:"minimum_size_gb"`
	MaximumSizeGB     int64      `tfsdk:"maximum_size_gb"`
	AvailabilityZones types.List `tfsdk:"availability_zones"`
}

func (d *blockStorageTypesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.service == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading block storage types.")
		return
	}
	var filter types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &filter)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if filter.IsUnknown() {
		resp.Diagnostics.AddError("Unknown block storage type filter", "The exact filter must be known before this read; an unknown value cannot trigger an unfiltered request.")
		return
	}
	var selected *string
	if !filter.IsNull() {
		v := filter.ValueString()
		selected = &v
	}
	rows, err := d.service.Types(ctx, selected)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read block storage types", err.Error())
		return
	}
	state := struct {
		Type  types.String            `tfsdk:"type"`
		Types []blockStorageTypeModel `tfsdk:"types"`
	}{Type: filter, Types: make([]blockStorageTypeModel, 0, len(rows))}
	for _, row := range rows {
		zones := types.ListNull(types.StringType)
		if row.AvailabilityZones != nil {
			value, diags := types.ListValueFrom(ctx, types.StringType, row.AvailabilityZones)
			zones = value
			resp.Diagnostics.Append(diags...)
		}
		state.Types = append(state.Types, blockStorageTypeModel{row.Type, row.MinimumSizeGB, row.MaximumSizeGB, zones})
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
