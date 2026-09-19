package provider

import (
	"context"
	"sort"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &availabilityZonesDataSource{}
var _ datasource.DataSourceWithConfigure = &availabilityZonesDataSource{}

type availabilityZonesDataSource struct{ compute *compute.Service }
type availabilityZonesModel struct {
	ZoneIDs types.List `tfsdk:"zone_ids"`
	Names   types.List `tfsdk:"names"`
	Zones   types.List `tfsdk:"zones"`
}
type zoneModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Status types.String `tfsdk:"status"`
}

func NewAvailabilityZonesDataSource() datasource.DataSource { return &availabilityZonesDataSource{} }
func (d *availabilityZonesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_availability_zones"
}
func (d *availabilityZonesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists API-visible availability zones, sorted by zone ID. This does not prove visibility of every existing server or console product.",
		Attributes: map[string]schema.Attribute{
			"zone_ids": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Exact zone IDs for availability_zone inputs, sorted by ID."},
			"names":    schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Display names in the same order as zone_ids."},
			"zones": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Zone catalog with exact server status strings.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Computed: true}, "status": schema.StringAttribute{Computed: true},
			}}},
		},
	}
}
func (d *availabilityZonesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	service, ok := req.ProviderData.(*compute.Service)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected the configured iwinv compute client.")
		return
	}
	d.compute = service
}
func (d *availabilityZonesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.compute == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading zones.")
		return
	}
	zones, err := d.compute.Zones(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read iwinv zones", err.Error())
		return
	}
	sort.Slice(zones, func(i, j int) bool { return zones[i].ID < zones[j].ID })
	ids, names := make([]string, 0, len(zones)), make([]string, 0, len(zones))
	rows := make([]zoneModel, 0, len(zones))
	for _, zone := range zones {
		ids = append(ids, zone.ID)
		names = append(names, zone.Name)
		rows = append(rows, zoneModel{types.StringValue(zone.ID), types.StringValue(zone.Name), types.StringValue(zone.Status)})
	}
	var state availabilityZonesModel
	var diags = resp.Diagnostics
	state.ZoneIDs, diags = types.ListValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	state.Names, diags = types.ListValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	state.Zones, diags = types.ListValueFrom(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "name": types.StringType, "status": types.StringType}}, rows)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
