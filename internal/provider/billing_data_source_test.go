package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/billing"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

type billingAPI struct {
	mu    sync.Mutex
	mode  string
	calls int
}

func (a *billingAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if p != "/v1/bill" && p != "/v1/bill/live" {
		return client.Envelope{}, errors.New("unexpected billing path")
	}
	if a.mode == "error" {
		return client.Envelope{}, &client.Error{Kind: "http_status", Status: 403, Code: "CHECK_IP"}
	}
	var rows []map[string]any
	if p == "/v1/bill/live" {
		if len(q) != 0 {
			return client.Envelope{}, errors.New("unexpected live filter")
		}
		rows = []map[string]any{{"bill_id": "BILL-live", "start_date": "2026-01-01", "end_date": "2026-01-20", "price": int64(9007199254740993), "currency": "KRW", "payment_info": "SYNTHETIC-PAYMENT-SECRET", "invoice": []string{"https://example.invalid/private-invoice"}}}
		if a.mode == "current_empty" {
			rows = []map[string]any{}
		}
		if a.mode == "current_multi" {
			rows = append(rows, map[string]any{"bill_id": "BILL-second", "start_date": "2026-01-01", "end_date": "2026-01-20", "price": int64(10), "currency": "KRW"})
		}
		raw, _ := json.Marshal(rows)
		return client.Envelope{Status: 200, Result: raw, Count: json.RawMessage(fmt.Sprint(len(rows)))}, nil
	}
	for k := range q {
		switch k {
		case "page_no", "page_size", "start_date", "end_date", "min_price", "max_price":
		default:
			return client.Envelope{}, errors.New("unexpected bill filter")
		}
	}
	page, err := strconv.Atoi(q.Get("page_no"))
	if err != nil || q.Get("page_size") != "10" {
		return client.Envelope{}, errors.New("wrong pagination request")
	}
	if a.mode == "late_error" && page > 1 {
		return client.Envelope{}, errors.New("synthetic late billing failure")
	}
	if a.mode == "empty_set" {
		return client.Envelope{}, &client.Error{Kind: "http_status", Status: 400, Code: "EMPTY_SET"}
	}
	all := []map[string]any{}
	if a.mode != "empty" {
		for i := 1; i <= 12; i++ {
			date := "2026-02-01"
			price := int64(9007199254740993)
			if i > 6 {
				date = "2026-03-01"
				price = 1000
			}
			if a.mode != "wrong_filter" {
				if q.Get("start_date") != "" && date < q.Get("start_date") {
					continue
				}
				if q.Get("end_date") != "" && date > q.Get("end_date") {
					continue
				}
				if q.Get("min_price") != "" {
					n, e := strconv.ParseInt(q.Get("min_price"), 10, 64)
					if e != nil {
						return client.Envelope{}, e
					}
					if price < n {
						continue
					}
				}
				if q.Get("max_price") != "" {
					n, e := strconv.ParseInt(q.Get("max_price"), 10, 64)
					if e != nil {
						return client.Envelope{}, e
					}
					if price > n {
						continue
					}
				}
			}
			all = append(all, map[string]any{"bill_id": fmt.Sprintf("BILL-%03d", i), "usage_start": "2026-01-01", "usage_end": "2026-01-31", "bill_date": date, "type": "regular", "payment_status": "paid", "name": "합성 &amp; 청구", "price": price, "vat": 10, "payment_price": price + 10, "currency": "KRW", "date_paid": "", "payment_info": "SYNTHETIC-PAYMENT-SECRET", "invoice": []string{"https://example.invalid/private-invoice"}, "tax": []string{"https://example.invalid/private-tax"}})
		}
	}
	start := (page - 1) * 10
	if start > len(all) {
		start = len(all)
	}
	end := start + 10
	if end > len(all) {
		end = len(all)
	}
	rows = all[start:end]
	if a.calls%2 == 0 {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}
	raw, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: raw, Count: json.RawMessage(fmt.Sprint(len(rows))), PageNo: json.RawMessage(fmt.Sprintf("%q", fmt.Sprint(page))), PageSize: json.RawMessage(`"10"`)}, nil
}

const billingConfig = `provider "iwinv" {}
data "iwinv_current_bill" "current" {}
data "iwinv_bills" "all" {}
data "iwinv_bills" "filtered" {
 start_date = "2026-02-01"
 end_date = "2026-02-01"
 minimum_price = 9007199254740993
 maximum_price = 9007199254740993
}
output "current_price" {
 value = data.iwinv_current_bill.current.price
 sensitive = true
}
output "bill_summaries" {
 value = data.iwinv_bills.all.bills
 sensitive = true
}
`

type billingPrivacyStateCheck struct{}

func (billingPrivacyStateCheck) CheckState(_ context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	raw, err := json.Marshal(req.State)
	if err != nil {
		resp.Error = errors.New("cannot inspect synthetic billing state")
		return
	}
	for _, v := range []string{"SYNTHETIC-PAYMENT-SECRET", "private-invoice", "private-tax", "payment_info", "invoice", "tax"} {
		if strings.Contains(string(raw), v) {
			resp.Error = errors.New("excluded payment or document field reached state")
			return
		}
	}
	// Sensitive is redaction, not removal: real financial values still persist.
	if !strings.Contains(string(raw), "합성 &amp; 청구") && !strings.Contains(string(raw), `합성 \u0026amp; 청구`) {
		resp.Error = errors.New("financial value was not retained in state")
	}
}

type billingSensitivePlanCheck struct{}

func (billingSensitivePlanCheck) CheckPlan(_ context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	if req.Plan.PlannedValues == nil || req.Plan.PlannedValues.RootModule == nil {
		resp.Error = errors.New("planned billing values absent")
		return
	}
	expected := map[string][]string{
		"data.iwinv_current_bill.current": {"bill_id", "start_date", "end_date", "price", "currency"},
		"data.iwinv_bills.all":            {"bills"},
		"data.iwinv_bills.filtered":       {"bills", "minimum_price", "maximum_price"},
	}
	// Known data sources may be evaluated during planning and placed only in
	// the plan's prior state, while output changes carry the sensitivity forward.
	resources := req.Plan.PlannedValues.RootModule.Resources
	if req.Plan.PriorState != nil && req.Plan.PriorState.Values != nil && req.Plan.PriorState.Values.RootModule != nil {
		resources = append(resources, req.Plan.PriorState.Values.RootModule.Resources...)
	}
	for _, r := range resources {
		fields, ok := expected[r.Address]
		if !ok {
			continue
		}
		var sensitive map[string]any
		if json.Unmarshal(r.SensitiveValues, &sensitive) != nil {
			resp.Error = errors.New("billing sensitivity unavailable")
			return
		}
		for _, field := range fields {
			if sensitive[field] != true {
				resp.Error = errors.New("financial plan value lacks sensitivity")
				return
			}
		}
		delete(expected, r.Address)
	}
	if len(expected) != 0 {
		resp.Error = errors.New("billing values missing from saved plan")
		return
	}
	for _, name := range []string{"current_price", "bill_summaries"} {
		change := req.Plan.OutputChanges[name]
		if change == nil || change.AfterSensitive != true {
			resp.Error = errors.New("billing output change is not sensitive")
			return
		}
	}

}
func TestProtocolBilling(t *testing.T) {
	cacheCatalogProtocol(t)
	states := []statecheck.StateCheck{billingPrivacyStateCheck{}, statecheck.ExpectSensitiveValue("data.iwinv_bills.all", tfjsonpath.New("bills")), statecheck.ExpectSensitiveValue("data.iwinv_bills.filtered", tfjsonpath.New("minimum_price")), statecheck.ExpectSensitiveValue("data.iwinv_bills.filtered", tfjsonpath.New("maximum_price")), statecheck.ExpectKnownValue("data.iwinv_current_bill.current", tfjsonpath.New("price"), knownvalue.Int64Exact(9007199254740993)), statecheck.ExpectKnownValue("data.iwinv_bills.all", tfjsonpath.New("bills").AtSliceIndex(0).AtMapKey("price"), knownvalue.Int64Exact(9007199254740993))}
	work := t.TempDir()
	plans := []plancheck.PlanCheck{billingSensitivePlanCheck{}, hostingSecretPlanCheck{secrets: []string{"SYNTHETIC-PAYMENT-SECRET", "private-invoice", "private-tax"}, directory: work}}
	for _, field := range []string{"bill_id", "start_date", "end_date", "price", "currency"} {
		states = append(states, statecheck.ExpectSensitiveValue("data.iwinv_current_bill.current", tfjsonpath.New(field)))
	}
	resource.Test(t, resource.TestCase{IsUnitTest: true, WorkingDir: work, ProtoV6ProviderFactories: groupFactories(&billingAPI{}), Steps: []resource.TestStep{
		{Config: billingConfig, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: plans}, ConfigStateChecks: states, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("data.iwinv_bills.all", "bills.#", "12"), resource.TestCheckResourceAttr("data.iwinv_bills.all", "bills.0.bill_id", "BILL-001"), resource.TestCheckResourceAttr("data.iwinv_bills.all", "bills.0.date_paid", ""), resource.TestCheckResourceAttr("data.iwinv_bills.filtered", "bills.#", "6"))},
		{Config: billingConfig, PlanOnly: true},
	}})
}
func TestProtocolBillingFailures(t *testing.T) {
	cacheCatalogProtocol(t)
	for _, mode := range []string{"error", "late_error", "wrong_filter", "current_empty", "current_multi", "empty", "empty_set"} {
		t.Run(mode, func(t *testing.T) {
			step := resource.TestStep{Config: billingConfig}
			if mode == "empty" || mode == "empty_set" {
				step.Check = resource.TestCheckResourceAttr("data.iwinv_bills.all", "bills.#", "0")
			} else {
				step.ExpectError = regexp.MustCompile("Unable to read bills|Unable to read current bill|Unexpected current bill result")
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(&billingAPI{mode: mode}), Steps: []resource.TestStep{step}})
		})
	}
	for _, config := range []string{`start_date = ""`, `end_date = "2026-02-30"`, `start_date = "2026-02-02"
end_date = "2026-02-01"`, `minimum_price = 2
maximum_price = 1`} {
		a := &billingAPI{}
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: `provider "iwinv" {}
data "iwinv_bills" "test" {
` + config + `
}`, ExpectError: regexp.MustCompile("Invalid bill")}}})
		if a.calls != 0 {
			t.Fatal("invalid filters reached API")
		}
	}
	for _, value := range []string{"data.iwinv_current_bill.current.price", "data.iwinv_bills.all.bills"} {
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(&billingAPI{}), Steps: []resource.TestStep{{Config: billingConfig + `output "unmarked" { value = ` + value + ` }`, ExpectError: regexp.MustCompile("Output refers to sensitive values")}}})
	}
}
func TestBillingUnknownReadMakesNoRequest(t *testing.T) {
	ctx := context.Background()
	for _, field := range []string{"start", "end", "min", "max"} {
		a := &billingAPI{}
		d := &billingDataSource{service: &billing.Service{API: a}}
		sr := datasource.SchemaResponse{}
		d.Schema(ctx, datasource.SchemaRequest{}, &sr)
		m := billsModel{StartDate: types.StringNull(), EndDate: types.StringNull(), MinimumPrice: types.Int64Null(), MaximumPrice: types.Int64Null()}
		switch field {
		case "start":
			m.StartDate = types.StringUnknown()
		case "end":
			m.EndDate = types.StringUnknown()
		case "min":
			m.MinimumPrice = types.Int64Unknown()
		case "max":
			m.MaximumPrice = types.Int64Unknown()
		}
		state := tfsdk.State{Schema: sr.Schema}
		if ds := state.Set(ctx, &m); ds.HasError() {
			t.Fatal(ds)
		}
		resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema}}
		d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Raw: state.Raw, Schema: sr.Schema}}, &resp)
		if !resp.Diagnostics.HasError() || a.calls != 0 {
			t.Fatal("unknown filter became account-wide read")
		}
	}
}
func TestProtocolBillingUnknownAndZero(t *testing.T) {
	cacheCatalogProtocol(t)
	config := `provider "iwinv" {}
resource "terraform_data" "date" { input = "2026-02-01" }
resource "terraform_data" "price" { input = 0 }
data "iwinv_bills" "test" {
 start_date = terraform_data.date.output
 end_date = "2026-02-01"
 minimum_price = terraform_data.price.output
}
data "iwinv_bills" "zero" { maximum_price = 0 }
data "iwinv_bills" "negative" { maximum_price = -1 }
`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(&billingAPI{}), Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("data.iwinv_bills.test", "bills.#", "6"), resource.TestCheckResourceAttr("data.iwinv_bills.zero", "bills.#", "0"), resource.TestCheckResourceAttr("data.iwinv_bills.negative", "bills.#", "0"))},
		{Config: config, PlanOnly: true},
	}})
}
func TestAccBillingDataSources(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1")
	}
	config := `provider "iwinv" {}
data "iwinv_current_bill" "current" {}
data "iwinv_bills" "all" {}
data "iwinv_bills" "empty" {
 start_date = "1900-01-01"
 end_date = "1900-01-01"
}
output "current_price" {
 value = data.iwinv_current_bill.current.price
 sensitive = true
}
output "bills" {
 value = data.iwinv_bills.all.bills
 sensitive = true
}
`
	resource.Test(t, resource.TestCase{PreCheck: func() {
		if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
			t.Fatal("private environment credentials required")
		}
	}, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
		{Config: config, ConfigStateChecks: []statecheck.StateCheck{statecheck.ExpectSensitiveValue("data.iwinv_bills.all", tfjsonpath.New("bills")), statecheck.ExpectSensitiveValue("data.iwinv_current_bill.current", tfjsonpath.New("price"))}, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("data.iwinv_bills.all", "bills.0.bill_id"), resource.TestCheckResourceAttr("data.iwinv_current_bill.current", "bill_id", "BILL-live"), resource.TestCheckResourceAttr("data.iwinv_current_bill.current", "currency", "KRW"), resource.TestCheckResourceAttr("data.iwinv_bills.empty", "bills.#", "0"))},
		{Config: config, PlanOnly: true},
	}})
}
