package provider

import (
	"context"
	"sort"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dbProductsDataSource struct{ service *hosted.DBMSCatalogService }

var _ datasource.DataSourceWithConfigure = &dbProductsDataSource{}
var _ datasource.DataSourceWithValidateConfig = &dbProductsDataSource{}

func NewDBInstanceProductsDataSource() datasource.DataSource { return &dbProductsDataSource{} }
func (d *dbProductsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_db_instance_products"
}
func (d *dbProductsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads DBMS product rows without selecting a creation product. Repeated and empty product IDs are preserved. A catalog version is not a creation-time version selector. Results are sorted by product ID, type and version; ordering is not a recommendation.", Attributes: map[string]schema.Attribute{
		"product_type": schema.StringAttribute{Optional: true, MarkdownDescription: "Exact STD or HM query filter; omit to include other visible tiers."},
		"engine":       schema.StringAttribute{Optional: true, MarkdownDescription: "Exact API query filter: MySQL, MariaDB, mongoDB, MS-SQL, redis or PostgreSQL. The response does not separately echo an engine identifier."},
		"products": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Full validated rows, including empty product IDs and IDs shared by versions. Check the whole unfiltered catalog for ambiguity before choosing a creation ID. Availability does not guarantee provisioning eligibility.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"product_id":     schema.StringAttribute{Computed: true, MarkdownDescription: "Exact catalog value; may be empty or shared by multiple versions."},
			"name":           schema.StringAttribute{Computed: true},
			"status":         schema.StringAttribute{Computed: true},
			"product_type":   schema.StringAttribute{Computed: true, MarkdownDescription: "Observed tier, not engine name."},
			"engine_version": schema.StringAttribute{Computed: true, MarkdownDescription: "Observed version label; POST accepts no version selector."},
		}}},
	}}
}

type dbProductModel struct {
	ID      string `tfsdk:"product_id"`
	Name    string `tfsdk:"name"`
	Status  string `tfsdk:"status"`
	Type    string `tfsdk:"product_type"`
	Version string `tfsdk:"engine_version"`
}
type dbProductsModel struct {
	Type     types.String     `tfsdk:"product_type"`
	Engine   types.String     `tfsdk:"engine"`
	Products []dbProductModel `tfsdk:"products"`
}

func (d *dbProductsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	d.service = s.DBMSCatalogs
}
func validateDBProducts(m dbProductsModel, requireKnown bool, ds *diag.Diagnostics) {
	for _, input := range []struct {
		name    string
		value   types.String
		allowed []string
	}{
		{"product_type", m.Type, []string{"STD", "HM"}},
		{"engine", m.Engine, []string{"MySQL", "MariaDB", "mongoDB", "MS-SQL", "redis", "PostgreSQL"}},
	} {
		if input.value.IsUnknown() {
			if requireKnown {
				ds.AddAttributeError(path.Root(input.name), "Unknown DBMS catalog filter", "Resolve filters before reading; unknown does not mean unfiltered.")
			}
			continue
		}
		if input.value.IsNull() {
			continue
		}
		valid := false
		for _, allowed := range input.allowed {
			valid = valid || input.value.ValueString() == allowed
		}
		if !valid {
			ds.AddAttributeError(path.Root(input.name), "Invalid DBMS catalog filter", "Use a documented case-sensitive value or omit the filter; empty strings are not omission.")
		}
	}
}
func (d *dbProductsDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var m dbProductsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		validateDBProducts(m, false, &resp.Diagnostics)
	}
}
func (d *dbProductsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.service == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading DBMS products.")
		return
	}
	var m dbProductsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateDBProducts(m, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	rows, err := d.service.Products(ctx, m.Type.ValueString(), m.Engine.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read DBMS products", err.Error())
		return
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ID != rows[j].ID {
			return rows[i].ID < rows[j].ID
		}
		if rows[i].Tier != rows[j].Tier {
			return rows[i].Tier < rows[j].Tier
		}
		return rows[i].Version < rows[j].Version
	})
	m.Products = []dbProductModel{}
	for _, row := range rows {
		m.Products = append(m.Products, dbProductModel{row.ID, row.Name, row.Status, row.Tier, row.Version})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
