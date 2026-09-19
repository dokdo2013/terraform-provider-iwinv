package provider

import (
	"context"
	"os"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &IwinvProvider{}

type clientFactory func(string, string) (compute.API, error)

type IwinvProvider struct {
	version   string
	newClient clientFactory
}

type resourceServices struct {
	Network *network.Service
	Hosting *hosted.WebhostingService
}

type providerModel struct {
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &IwinvProvider{version: version, newClient: func(access, secret string) (compute.API, error) { return client.New(access, secret) }}
	}
}

func (p *IwinvProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "iwinv"
	resp.Version = p.version
}

func (p *IwinvProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Independent community iwinv provider. Development build: zone, image, instance-type and SSH-key data sources plus security-group attributes, independent ingress/egress rules and webhosting accounts. Instance attachments are not implemented.",
		Attributes: map[string]schema.Attribute{
			"access_key": schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Control-plane access key. Defaults to IWINV_ACCESS_KEY when omitted."},
			"secret_key": schema.StringAttribute{Optional: true, Sensitive: true, MarkdownDescription: "Control-plane secret key. Defaults to IWINV_SECRET_KEY when omitted."},
		},
	}
}

func (p *IwinvProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.AccessKey.IsUnknown() || config.SecretKey.IsUnknown() {
		resp.Diagnostics.AddError("Unknown iwinv credentials", "Credentials must be known before accessing iwinv. Unknown configuration never falls back to another account's environment credentials.")
		return
	}
	access, secret := config.AccessKey.ValueString(), config.SecretKey.ValueString()
	if config.AccessKey.IsNull() {
		access = os.Getenv("IWINV_ACCESS_KEY")
	}
	if config.SecretKey.IsNull() {
		secret = os.Getenv("IWINV_SECRET_KEY")
	}
	api, err := p.newClient(access, secret)
	if err != nil {
		resp.Diagnostics.AddError("Invalid iwinv configuration", err.Error())
		return
	}
	resp.DataSourceData = &compute.Service{API: api}
	services := &resourceServices{}
	if writes, ok := api.(network.API); ok {
		services.Network = &network.Service{API: writes}
	}
	if hosting, ok := api.(hosted.WebhostingAPI); ok {
		services.Hosting = &hosted.WebhostingService{API: hosting}
	}
	resp.ResourceData = services
}

func (p *IwinvProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewAvailabilityZonesDataSource, NewImagesDataSource, NewImageDataSource, NewInstanceTypesDataSource, NewInstanceTypeDataSource, NewSSHKeysDataSource, NewSSHKeyDataSource}
}

func (p *IwinvProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewSecurityGroupResource, NewSecurityGroupIngressRuleResource, NewSecurityGroupEgressRuleResource, NewWebhostingResource}
}
