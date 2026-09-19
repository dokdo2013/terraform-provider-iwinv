package provider

import (
	"context"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/billing"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type billingDataSource struct {
	current bool
	service *billing.Service
}

var _ datasource.DataSourceWithConfigure = &billingDataSource{}
var _ datasource.DataSourceWithValidateConfig = &billingDataSource{}

func NewCurrentBillDataSource() datasource.DataSource { return &billingDataSource{current: true} }
func NewBillsDataSource() datasource.DataSource       { return &billingDataSource{} }
func (d *billingDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	suffix := "_bills"
	if d.current {
		suffix = "_current_bill"
	}
	resp.TypeName = req.ProviderTypeName + suffix
}
func (d *billingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	if d.current {
		resp.Schema = schema.Schema{MarkdownDescription: "Reads one current billing estimate, excluding VAT. All returned financial fields are sensitive but remain stored in Terraform state. Amounts are exact API integers without scaling; dates are literal with no inferred timezone. This is not a finalized charge or a deletion/billing-stop guarantee.", Attributes: map[string]schema.Attribute{
			"bill_id":    schema.StringAttribute{Computed: true, Sensitive: true},
			"start_date": schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Literal observed start date; no UTC conversion."},
			"end_date":   schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Literal observed end date; month boundary timezone is unverified."},
			"price":      schema.Int64Attribute{Computed: true, Sensitive: true, MarkdownDescription: "Observed estimated amount excluding VAT in the returned currency, without minor-unit conversion."},
			"currency":   schema.StringAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Observed currency code. KRW was verified; other currencies remain unverified."},
		}}
		return
	}
	resp.Schema = schema.Schema{MarkdownDescription: "Reads a complete filtered bill summary list, sorted by exact bill ID. The entire result is sensitive but stored in state. Payment instruments, invoice/tax links and unverified detail groups are excluded. No cloud resource is owned and no payment is performed.", Attributes: map[string]schema.Attribute{
		"start_date":    schema.StringAttribute{Optional: true, MarkdownDescription: "Inclusive bill-date lower bound, YYYY-MM-DD. Omit for no bound; empty is not omission."},
		"end_date":      schema.StringAttribute{Optional: true, MarkdownDescription: "Inclusive bill-date upper bound, YYYY-MM-DD; not usage end. Omit for no bound."},
		"minimum_price": schema.Int64Attribute{Optional: true, Sensitive: true, MarkdownDescription: "Inclusive VAT-exclusive price lower bound, exact API integer. Zero and negative bounds are sent literally."},
		"maximum_price": schema.Int64Attribute{Optional: true, Sensitive: true, MarkdownDescription: "Inclusive VAT-exclusive price upper bound, exact API integer. Does not filter VAT-inclusive payment_price."},
		"bills": schema.ListNestedAttribute{Computed: true, Sensitive: true, MarkdownDescription: "Complete financial summaries. Sensitive marking does not remove values from state or JSON output. No snapshot token guarantees isolation across pages.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"bill_id":        schema.StringAttribute{Computed: true},
			"usage_start":    schema.StringAttribute{Computed: true},
			"usage_end":      schema.StringAttribute{Computed: true},
			"bill_date":      schema.StringAttribute{Computed: true},
			"type":           schema.StringAttribute{Computed: true},
			"payment_status": schema.StringAttribute{Computed: true},
			"name":           schema.StringAttribute{Computed: true},
			"price":          schema.Int64Attribute{Computed: true, MarkdownDescription: "Observed amount excluding VAT; no currency scaling."},
			"vat":            schema.Int64Attribute{Computed: true, MarkdownDescription: "Observed VAT amount; not computed by the provider."},
			"payment_price":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Observed VAT-inclusive payment amount. No equality with price+vat is enforced."},
			"currency":       schema.StringAttribute{Computed: true},
			"date_paid":      schema.StringAttribute{Computed: true, MarkdownDescription: "Literal observed value, including empty string. No inferred timezone."},
		}}},
	}}
}

type currentBillModel struct {
	ID        string `tfsdk:"bill_id"`
	StartDate string `tfsdk:"start_date"`
	EndDate   string `tfsdk:"end_date"`
	Price     int64  `tfsdk:"price"`
	Currency  string `tfsdk:"currency"`
}
type billModel struct {
	ID            string `tfsdk:"bill_id"`
	UsageStart    string `tfsdk:"usage_start"`
	UsageEnd      string `tfsdk:"usage_end"`
	BillDate      string `tfsdk:"bill_date"`
	Type          string `tfsdk:"type"`
	PaymentStatus string `tfsdk:"payment_status"`
	Name          string `tfsdk:"name"`
	Price         int64  `tfsdk:"price"`
	VAT           int64  `tfsdk:"vat"`
	PaymentPrice  int64  `tfsdk:"payment_price"`
	Currency      string `tfsdk:"currency"`
	DatePaid      string `tfsdk:"date_paid"`
}
type billsModel struct {
	StartDate    types.String `tfsdk:"start_date"`
	EndDate      types.String `tfsdk:"end_date"`
	MinimumPrice types.Int64  `tfsdk:"minimum_price"`
	MaximumPrice types.Int64  `tfsdk:"maximum_price"`
	Bills        []billModel  `tfsdk:"bills"`
}

func (d *billingDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	s, ok := req.ProviderData.(*providerServices)
	if !ok {
		resp.Diagnostics.AddError("Invalid provider client", "Expected configured iwinv provider services.")
		return
	}
	d.service = s.Billing
}
func billingFilter(m billsModel, known bool, ds *diag.Diagnostics) billing.Filter {
	f := billing.Filter{}
	for _, v := range []struct {
		name   string
		value  types.String
		target *string
	}{{"start_date", m.StartDate, &f.StartDate}, {"end_date", m.EndDate, &f.EndDate}} {
		if v.value.IsUnknown() {
			if known {
				ds.AddAttributeError(path.Root(v.name), "Unknown bill filter", "Resolve every filter before reading; unknown never means an unfiltered query.")
			}
			continue
		}
		if !v.value.IsNull() {
			if v.value.ValueString() == "" {
				ds.AddAttributeError(path.Root(v.name), "Invalid bill date filter", "Omit the bound or supply YYYY-MM-DD; empty strings are not omission.")
			}
			*v.target = v.value.ValueString()
		}
	}
	for _, v := range []struct {
		name   string
		value  types.Int64
		target **int64
	}{{"minimum_price", m.MinimumPrice, &f.MinimumPrice}, {"maximum_price", m.MaximumPrice, &f.MaximumPrice}} {
		if v.value.IsUnknown() {
			if known {
				ds.AddAttributeError(path.Root(v.name), "Unknown bill filter", "Resolve every filter before reading; unknown never means an unfiltered query.")
			}
			continue
		}
		if !v.value.IsNull() {
			n := v.value.ValueInt64()
			*v.target = &n
		}
	}
	if err := f.Validate(); err != nil {
		ds.AddError("Invalid bill filters", err.Error())
	}
	return f
}
func (d *billingDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	if d.current {
		return
	}
	var m billsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		billingFilter(m, false, &resp.Diagnostics)
	}
}
func (d *billingDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.service == nil {
		resp.Diagnostics.AddError("Missing provider client", "Configure iwinv credentials before reading billing records.")
		return
	}
	if d.current {
		rows, err := d.service.Current(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to read current bill", err.Error())
			return
		}
		if len(rows) != 1 {
			resp.Diagnostics.AddError("Unexpected current bill result", "Expected exactly one current estimate; no arbitrary row was selected.")
			return
		}
		r := rows[0]
		resp.Diagnostics.Append(resp.State.Set(ctx, &currentBillModel{r.ID, r.StartDate, r.EndDate, r.Price, r.Currency})...)
		return
	}
	var m billsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	f := billingFilter(m, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	rows, err := d.service.List(ctx, f)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read bills", err.Error())
		return
	}
	m.Bills = []billModel{}
	for _, r := range rows {
		m.Bills = append(m.Bills, billModel{r.ID, r.UsageStart, r.UsageEnd, r.BillDate, r.Type, r.PaymentStatus, r.Name, r.Price, r.VAT, r.PaymentPrice, r.Currency, r.DatePaid})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
