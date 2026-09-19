package provider

import (
	"context"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type catalogDataSource struct {
	kind     compute.CatalogKind
	singular bool
	compute  *compute.Service
}

var _ datasource.DataSourceWithConfigure = &catalogDataSource{}

func NewImagesDataSource() datasource.DataSource { return &catalogDataSource{kind: compute.Images} }
func NewImageDataSource() datasource.DataSource {
	return &catalogDataSource{kind: compute.Images, singular: true}
}
func NewInstanceTypesDataSource() datasource.DataSource {
	return &catalogDataSource{kind: compute.InstanceTypes}
}
func NewInstanceTypeDataSource() datasource.DataSource {
	return &catalogDataSource{kind: compute.InstanceTypes, singular: true}
}
func (d *catalogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	suffix := "images"
	if d.kind == compute.InstanceTypes {
		suffix = "instance_types"
	}
	if d.singular {
		suffix = suffix[:len(suffix)-1]
	}
	resp.TypeName = req.ProviderTypeName + "_" + suffix
}
func (d *catalogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	if !d.singular {
		resp.Schema = schema.Schema{MarkdownDescription: "Reads all API-visible catalog IDs in lexical order. Catalog membership does not guarantee availability or compatibility in a zone.", Attributes: map[string]schema.Attribute{
			"ids": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Exact API IDs collected across all pages, sorted lexically."},
		}}
		return
	}
	attrs := map[string]schema.Attribute{"id": schema.StringAttribute{Required: true, MarkdownDescription: "Exact API catalog ID. A missing or ambiguous result is an error; no automatic selection is performed."}}
	if d.kind == compute.Images {
		attrs["visibility"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Exact API visibility string. Private-image response variants are not live-verified."}
		attrs["image_type"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Exact API image type string."}
	} else {
		attrs["name"] = schema.StringAttribute{Computed: true, MarkdownDescription: "Product display name. ID corresponds to the API flavor_id."}
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads one exact catalog ID. This data source does not create or own a remote object.", Attributes: attrs}
}
func (d *catalogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *catalogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.compute == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading catalogs.")
		return
	}
	if !d.singular {
		ids, err := d.compute.CatalogIDs(ctx, d.kind)
		if err != nil {
			resp.Diagnostics.AddError("Unable to read iwinv catalog", err.Error())
			return
		}
		state := struct {
			IDs []string `tfsdk:"ids"`
		}{IDs: ids}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}
	var config struct {
		ID types.String `tfsdk:"id"`
	}
	// Read only the required attribute; computed fields are intentionally absent.
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &config.ID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.ID.IsNull() || config.ID.IsUnknown() || config.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Invalid catalog ID", "A known, non-empty exact ID is required.")
		return
	}
	row, err := d.compute.CatalogItem(ctx, d.kind, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read iwinv catalog item", err.Error())
		return
	}
	if d.kind == compute.Images {
		resp.Diagnostics.Append(resp.State.Set(ctx, &struct {
			ID         string `tfsdk:"id"`
			Visibility string `tfsdk:"visibility"`
			ImageType  string `tfsdk:"image_type"`
		}{row.ImageID, row.Visibility, row.ImageType})...)
	} else {
		resp.Diagnostics.Append(resp.State.Set(ctx, &struct {
			ID   string `tfsdk:"id"`
			Name string `tfsdk:"name"`
		}{row.FlavorID, row.Name})...)
	}
}
