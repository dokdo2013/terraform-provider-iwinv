package provider

import (
	"context"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

type webmailProductsDataSource struct{ service *hosted.WebmailCatalogService }

var _ datasource.DataSourceWithConfigure = &webmailProductsDataSource{}

func NewWebmailProductsDataSource() datasource.DataSource { return &webmailProductsDataSource{} }
func (d *webmailProductsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webmail_products"
}
func (d *webmailProductsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{MarkdownDescription: "Reads all webmail product metadata. Empty IDs are preserved, not selectable creation IDs. Catalog availability does not establish reliable service/mailbox provisioning or deletion.", Attributes: map[string]schema.Attribute{
		"products": schema.ListNestedAttribute{Computed: true, MarkdownDescription: "Complete catalog sorted by exact ID, product type and name. No filters or automatic product selection.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"product_id":   schema.StringAttribute{Computed: true, MarkdownDescription: "Exact product ID, including empty IDs of coming-soon rows."},
			"name":         schema.StringAttribute{Computed: true, MarkdownDescription: "Literal catalog product name."},
			"status":       schema.StringAttribute{Computed: true, MarkdownDescription: "Observed catalog status; not service readiness."},
			"product_type": schema.StringAttribute{Computed: true, MarkdownDescription: "Literal spec.type value; no isolation guarantee."},
		}}},
	}}
}
func (d *webmailProductsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	d.service = s.WebmailCatalogs
}

type webmailProductModel struct {
	ID     string `tfsdk:"product_id"`
	Name   string `tfsdk:"name"`
	Status string `tfsdk:"status"`
	Type   string `tfsdk:"product_type"`
}

func (d *webmailProductsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.service == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading webmail products.")
		return
	}
	rows, err := d.service.Products(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read webmail products", err.Error())
		return
	}
	state := struct {
		Products []webmailProductModel `tfsdk:"products"`
	}{Products: make([]webmailProductModel, 0, len(rows))}
	for _, r := range rows {
		state.Products = append(state.Products, webmailProductModel{r.ID, r.Name, r.Status, r.Type})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
