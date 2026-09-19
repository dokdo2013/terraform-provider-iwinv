package provider

import (
	"context"
	"sort"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type hostingCatalogDataSource struct {
	servers bool
	service *hosted.HostingCatalogService
}

var _ datasource.DataSourceWithConfigure = &hostingCatalogDataSource{}
var _ datasource.DataSourceWithValidateConfig = &hostingCatalogDataSource{}

func NewWebhostingProductsDataSource() datasource.DataSource { return &hostingCatalogDataSource{} }
func NewWebhostingServersDataSource() datasource.DataSource {
	return &hostingCatalogDataSource{servers: true}
}
func (d *hostingCatalogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	suffix := "webhosting_products"
	if d.servers {
		suffix = "webhosting_servers"
	}
	resp.TypeName = req.ProviderTypeName + "_" + suffix
}
func (d *hostingCatalogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"ids": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Exact catalog IDs sorted lexically. The order is stable, not a recommendation or availability ranking."},
	}
	if d.servers {
		attrs["product_id"] = schema.StringAttribute{Required: true, MarkdownDescription: "Exact product ID sent as the required product_id query. A catalog choice does not guarantee successful provisioning."}
		attrs["servers"] = schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Choices for the selected product, in the same order as ids. Server IDs are creation selectors, not service IDs.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true}, "charset": schema.StringAttribute{Computed: true}, "php_version": schema.StringAttribute{Computed: true}, "database": schema.StringAttribute{Computed: true}, "program": schema.StringAttribute{Computed: true},
		}}}
	} else {
		attrs["type"] = schema.StringAttribute{Optional: true, MarkdownDescription: "Optional exact SHARE or SINGLE API filter. Omit for all visible products."}
		attrs["products"] = schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Validated product metadata in the same order as ids. Prices, VAT, disk and traffic units are deliberately excluded until verified.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true}, "name": schema.StringAttribute{Computed: true}, "status": schema.StringAttribute{Computed: true}, "type": schema.StringAttribute{Computed: true},
			"php_versions":        schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Exact supported PHP labels, sorted lexically."},
			"allow_custom_domain": schema.BoolAttribute{Computed: true}, "enable_domain_folder": schema.BoolAttribute{Computed: true},
			"max_domain_count": schema.Int64Attribute{Computed: true}, "domain_edit_interval_days": schema.Int64Attribute{Computed: true},
		}}}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads the public webhosting catalog with full response validation. Empty lists are valid; malformed/partial lists and API errors are errors. No remote resource is created or selected automatically.", Attributes: attrs}
}
func (d *hostingCatalogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	services, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	d.service = services.HostingCatalogs
}
func (d *hostingCatalogDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	name := "type"
	if d.servers {
		name = "product_id"
	}
	var value types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
	if resp.Diagnostics.HasError() || value.IsUnknown() || value.IsNull() {
		return
	}
	if d.servers && value.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid hosting product ID", "Use a nonempty exact product ID.")
	}
	if !d.servers && value.ValueString() != "SHARE" && value.ValueString() != "SINGLE" {
		resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid hosting product type", "Use SHARE or SINGLE, or omit the filter.")
	}
}

type hostingProductModel struct {
	ID                     string   `tfsdk:"id"`
	Name                   string   `tfsdk:"name"`
	Status                 string   `tfsdk:"status"`
	Type                   string   `tfsdk:"type"`
	PHPVersions            []string `tfsdk:"php_versions"`
	AllowCustomDomain      bool     `tfsdk:"allow_custom_domain"`
	EnableDomainFolder     bool     `tfsdk:"enable_domain_folder"`
	MaxDomainCount         int64    `tfsdk:"max_domain_count"`
	DomainEditIntervalDays int64    `tfsdk:"domain_edit_interval_days"`
}
type hostingServerModel struct {
	ID         string `tfsdk:"id"`
	Charset    string `tfsdk:"charset"`
	PHPVersion string `tfsdk:"php_version"`
	Database   string `tfsdk:"database"`
	Program    string `tfsdk:"program"`
}

func (d *hostingCatalogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.service == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading hosting catalogs.")
		return
	}
	name := "type"
	if d.servers {
		name = "product_id"
	}
	var value types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if value.IsUnknown() || d.servers && (value.IsNull() || value.ValueString() == "") || !d.servers && !value.IsNull() && value.ValueString() != "SHARE" && value.ValueString() != "SINGLE" {
		resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid hosting catalog input", "Resolve a valid known input before reading the catalog.")
		return
	}
	if d.servers {
		rows, err := d.service.Servers(ctx, value.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read hosting servers", err.Error())
			return
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
		state := struct {
			ProductID types.String         `tfsdk:"product_id"`
			IDs       []string             `tfsdk:"ids"`
			Servers   []hostingServerModel `tfsdk:"servers"`
		}{ProductID: value, IDs: []string{}, Servers: []hostingServerModel{}}
		for _, r := range rows {
			state.IDs = append(state.IDs, r.ID)
			state.Servers = append(state.Servers, hostingServerModel{r.ID, r.Charset, r.PHPVersion, r.Database, r.Program})
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}
	rows, err := d.service.Products(ctx, value.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read hosting products", err.Error())
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	state := struct {
		Type     types.String          `tfsdk:"type"`
		IDs      []string              `tfsdk:"ids"`
		Products []hostingProductModel `tfsdk:"products"`
	}{Type: value, IDs: []string{}, Products: []hostingProductModel{}}
	for _, r := range rows {
		sort.Strings(r.PHPVersions)
		state.IDs = append(state.IDs, r.ID)
		state.Products = append(state.Products, hostingProductModel{r.ID, r.Name, r.Status, r.Type, r.PHPVersions, r.AllowCustomDomain, r.EnableDomainFolder, r.MaxDomainCount, r.DomainEditIntervalDays})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
