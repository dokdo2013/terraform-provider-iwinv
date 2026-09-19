package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type dbProductsAPI struct {
	mu    sync.Mutex
	mode  string
	calls int
}

func (a *dbProductsAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if p != "/v1/dbms/products" || len(q) > 2 {
		return client.Envelope{}, errors.New("unexpected catalog path/query")
	}
	for k := range q {
		if k != "type" && k != "db" {
			return client.Envelope{}, errors.New("unexpected query key")
		}
	}
	if a.mode == "error" {
		return client.Envelope{}, errors.New("synthetic catalog failure")
	}
	rows := []map[string]any{}
	if a.mode != "empty" {
		for _, v := range []struct{ id, tier, ver string }{{"synthetic&b", "STD", "8"}, {"", "HM", "7"}, {"synthetic&b", "STD", "5"}, {"synthetic-a", "STD", "7"}} {
			if q.Get("type") != "" && q.Get("type") != v.tier {
				continue
			}
			if q.Get("db") != "" && q.Get("db") != "redis" {
				return client.Envelope{}, errors.New("engine query did not retain exact spelling")
			}
			rows = append(rows, map[string]any{"product_id": v.id, "product_name": "합성 &amp; + 상품", "status": "available", "spec": map[string]any{"type": v.tier, "ver": v.ver}})
		}
	}
	if a.mode == "duplicate" {
		rows = append(rows, rows[0])
	}
	if a.mode == "null_id" {
		rows[0]["product_id"] = nil
	}
	raw, _ := json.Marshal(rows)
	if a.mode == "malformed" {
		raw = []byte(`[{"product_id":"partial"}]`)
	}
	e := client.Envelope{Status: 200, Result: raw}
	if a.mode == "pagination" {
		e.PageNo = json.RawMessage(`1`)
	}
	return e, nil
}
func dbProductsFactories(a *dbProductsAPI) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(&IwinvProvider{version: "test", newClient: func(string, string) (compute.API, error) { return a, nil }})}
}

const dbProductsConfig = `provider "iwinv" {}
data "iwinv_db_instance_products" "all" {}
data "iwinv_db_instance_products" "filtered" {
 product_type = "STD"
 engine = "redis"
}
`

func TestProtocolDBProducts(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := &dbProductsAPI{}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: dbProductsFactories(a), Steps: []resource.TestStep{
		{Config: dbProductsConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.all", "products.#", "4"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.all", "products.0.product_id", ""),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.all", "products.0.product_type", "HM"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.all", "products.1.product_id", "synthetic&b"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.all", "products.1.engine_version", "5"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.all", "products.2.product_id", "synthetic&b"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.all", "products.2.engine_version", "8"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.filtered", "products.#", "3"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.filtered", "products.0.product_type", "STD"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.filtered", "products.0.name", "합성 &amp; + 상품"),
		)},
		{Config: dbProductsConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}
func TestProtocolDBProductsFailures(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	for _, mode := range []string{"error", "empty", "duplicate", "null_id", "malformed", "pagination"} {
		t.Run(mode, func(t *testing.T) {
			a := &dbProductsAPI{mode: mode}
			step := resource.TestStep{Config: `provider "iwinv" {}
data "iwinv_db_instance_products" "all" {}`}
			if mode == "empty" {
				step.Check = resource.TestCheckResourceAttr("data.iwinv_db_instance_products.all", "products.#", "0")
			} else {
				step.ExpectError = regexp.MustCompile("Unable to read DBMS products")
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: dbProductsFactories(a), Steps: []resource.TestStep{step}})
		})
	}
	for _, input := range []string{`engine = "Redis"`, `engine = ""`, `product_type = "std"`, `product_type = ""`} {
		a := &dbProductsAPI{}
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: dbProductsFactories(a), Steps: []resource.TestStep{{Config: `provider "iwinv" {}
data "iwinv_db_instance_products" "all" {` + input + `}`, ExpectError: regexp.MustCompile("Invalid DBMS catalog filter")}}})
		if a.calls != 0 {
			t.Fatal("invalid filter reached API")
		}
	}
}
func TestAccDBProducts(t *testing.T) {
	if os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1")
	}
	// Check every documented filter independently, plus the lifecycle-test combination.
	config := dbProductsConfig + "\n"
	for i, engine := range []string{"MySQL", "MariaDB", "mongoDB", "MS-SQL", "redis", "PostgreSQL"} {
		config += fmt.Sprintf("data \"iwinv_db_instance_products\" \"engine%d\" { engine = %q }\n", i, engine)
	}
	for i, tier := range []string{"STD", "HM"} {
		config += fmt.Sprintf("data \"iwinv_db_instance_products\" \"tier%d\" { product_type = %q }\n", i, tier)
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
			t.Fatal("environment credentials required")
		}
	}, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("data.iwinv_db_instance_products.all", "products.0.name"),
			resource.TestCheckResourceAttr("data.iwinv_db_instance_products.filtered", "products.0.product_type", "STD"),
		)},
		{Config: config, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}

func TestDBProductsUnknownReadMakesNoRequest(t *testing.T) {
	ctx := context.Background()
	for _, engineUnknown := range []bool{false, true} {
		a := &dbProductsAPI{}
		d := &dbProductsDataSource{service: &hosted.DBMSCatalogService{API: a}}
		sr := datasource.SchemaResponse{}
		d.Schema(ctx, datasource.SchemaRequest{}, &sr)
		m := dbProductsModel{Type: types.StringUnknown(), Engine: types.StringNull()}
		if engineUnknown {
			m.Type = types.StringNull()
			m.Engine = types.StringUnknown()
		}
		state := tfsdk.State{Schema: sr.Schema}
		if ds := state.Set(ctx, &m); ds.HasError() {
			t.Fatal(ds)
		}
		resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema}}
		d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: state.Raw}}, &resp)
		if !resp.Diagnostics.HasError() || a.calls != 0 {
			t.Fatal("unknown filter became unfiltered API read")
		}
	}
}
