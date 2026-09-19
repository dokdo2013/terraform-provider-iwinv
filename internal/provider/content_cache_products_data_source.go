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

type cacheProductsDataSource struct{ service *hosted.CacheCatalogService }

var _ datasource.DataSourceWithConfigure = &cacheProductsDataSource{}
var _ datasource.DataSourceWithValidateConfig = &cacheProductsDataSource{}

func NewContentCacheProductsDataSource() datasource.DataSource { return &cacheProductsDataSource{} }
func (d *cacheProductsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_content_cache_products"
}
func (d *cacheProductsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads the complete cache product catalog, preserving null and empty IDs as unavailable creation choices. Optional exact type filter. Results sort null IDs first, then ID, type and name; order is not a recommendation. Pricing, capacity and traffic units are excluded until verified.", Attributes: map[string]schema.Attribute{
		"product_type": schema.StringAttribute{Optional: true, MarkdownDescription: "Exact SHARE or SINGLE filter. Omit for all visible types; empty string is not omission."},
		"products": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Full validated catalog rows, including coming-soon entries with null product IDs. Visibility and available status do not guarantee creation eligibility.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"product_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "Exact nullable catalog ID. Null and empty IDs are preserved and cannot create a service."},
			"name":         schema.StringAttribute{Computed: true},
			"status":       schema.StringAttribute{Computed: true},
			"product_type": schema.StringAttribute{Computed: true, MarkdownDescription: "Catalog spec.type. It may disagree with a created service's product_type; neither is an isolation guarantee."},
		}}},
	}}
}

type cacheProductModel struct {
	ID     types.String `tfsdk:"product_id"`
	Name   string       `tfsdk:"name"`
	Status string       `tfsdk:"status"`
	Type   string       `tfsdk:"product_type"`
}
type cacheProductsModel struct {
	Type     types.String        `tfsdk:"product_type"`
	Products []cacheProductModel `tfsdk:"products"`
}

func (d *cacheProductsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	d.service = s.CacheCatalogs
}
func validateCacheProducts(m cacheProductsModel, known bool, ds *diag.Diagnostics) {
	if m.Type.IsUnknown() {
		if known {
			ds.AddAttributeError(path.Root("product_type"), "Unknown cache catalog filter", "Resolve the filter before reading; unknown does not mean unfiltered.")
		}
		return
	}
	if !m.Type.IsNull() && m.Type.ValueString() != "SHARE" && m.Type.ValueString() != "SINGLE" {
		ds.AddAttributeError(path.Root("product_type"), "Invalid cache catalog filter", "Use exact SHARE or SINGLE, or omit the filter; empty strings are not omission.")
	}
}
func (d *cacheProductsDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var m cacheProductsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		validateCacheProducts(m, false, &resp.Diagnostics)
	}
}
func (d *cacheProductsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.service == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading cache products.")
		return
	}
	var m cacheProductsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateCacheProducts(m, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	rows, err := d.service.Products(ctx, m.Type.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read cache products", err.Error())
		return
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if (a.ID == nil) != (b.ID == nil) {
			return a.ID == nil
		}
		if a.ID != nil && *a.ID != *b.ID {
			return *a.ID < *b.ID
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Name < b.Name
	})
	m.Products = []cacheProductModel{}
	for _, row := range rows {
		m.Products = append(m.Products, cacheProductModel{types.StringPointerValue(row.ID), row.Name, row.Status, row.Type})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
