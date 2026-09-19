package provider

import (
	"context"
	"sort"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type storageProductsDataSource struct{ service *hosted.NASCatalogService }

var _ datasource.DataSourceWithConfigure = &storageProductsDataSource{}

func NewSharedStorageProductsDataSource() datasource.DataSource { return &storageProductsDataSource{} }
func (d *storageProductsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shared_storage_products"
}
func (d *storageProductsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads the complete API NAS product catalog. Empty creation IDs, nullable versions and zero disk bounds are preserved. Rows sort by ID then name; ordering is not a recommendation. No provisioning, pricing or billing guarantee.", Attributes: map[string]schema.Attribute{
		"products": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Complete validated catalog rows, including coming-soon products. Review a nonempty ID and capacity bounds before creation.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"product_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "Exact creation ID. Empty IDs are preserved and cannot create a service."},
			"name":            schema.StringAttribute{Computed: true},
			"status":          schema.StringAttribute{Computed: true},
			"version":         schema.StringAttribute{Computed: true, MarkdownDescription: "Observed nullable catalog version; no independent version selector is implied."},
			"minimum_size_gb": schema.Int64Attribute{Computed: true, MarkdownDescription: "Observed minimum capacity in documented GB. Zero is preserved for coming-soon rows."},
			"maximum_size_gb": schema.Int64Attribute{Computed: true, MarkdownDescription: "Observed maximum capacity in documented GB; not a resize capability."},
		}}},
	}}
}

type storageProductModel struct {
	ID            string       `tfsdk:"product_id"`
	Name          string       `tfsdk:"name"`
	Status        string       `tfsdk:"status"`
	Version       types.String `tfsdk:"version"`
	MinimumSizeGB int64        `tfsdk:"minimum_size_gb"`
	MaximumSizeGB int64        `tfsdk:"maximum_size_gb"`
}
type storageProductsModel struct {
	Products []storageProductModel `tfsdk:"products"`
}

func (d *storageProductsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	d.service = s.NASCatalogs
}
func (d *storageProductsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.service == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading NAS products.")
		return
	}
	rows, err := d.service.Products(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read NAS products", err.Error())
		return
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ID != rows[j].ID {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].Name < rows[j].Name
	})
	m := storageProductsModel{Products: []storageProductModel{}}
	for _, row := range rows {
		m.Products = append(m.Products, storageProductModel{row.ID, row.Name, row.Status, types.StringPointerValue(row.Version), row.MinimumDiskGB, row.MaximumDiskGB})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
