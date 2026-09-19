package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/hosted"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type cacheProductsAPI struct {
	mu    sync.Mutex
	mode  string
	calls int
}

func (a *cacheProductsAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if p != "/v1/cache/products" || len(q) > 1 {
		return client.Envelope{}, errors.New("unexpected catalog request")
	}
	for k := range q {
		if k != "type" {
			return client.Envelope{}, errors.New("unexpected filter")
		}
	}
	if a.mode == "error" {
		return client.Envelope{}, errors.New("synthetic catalog failure")
	}
	rows := []map[string]any{}
	if a.mode != "empty" {
		for _, v := range []struct {
			id                 any
			name, kind, status string
		}{{"synthetic-b", "합성 &amp; + 상품", "SHARE", "available"}, {nil, "Coming soon", "SINGLE", "comingsoon"}, {"synthetic-a", "A", "SHARE", "available"}, {"", "Empty ID", "SINGLE", "available"}} {
			if q.Get("type") != "" && q.Get("type") != v.kind && a.mode != "wrong_filter" {
				continue
			}
			rows = append(rows, map[string]any{"product_id": v.id, "product_name": v.name, "status": v.status, "spec": map[string]any{"type": v.kind}})
		}
	}
	switch a.mode {
	case "duplicate":
		rows = append(rows, rows[0])
	case "missing_id":
		delete(rows[0], "product_id")
	case "number_id":
		rows[0]["product_id"] = 123
	case "null_result":
		return client.Envelope{Status: 200, Result: json.RawMessage(`null`)}, nil
	}
	// Reverse every other response to exercise stable state without relying on API order.
	if a.calls%2 == 0 {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}
	raw, _ := json.Marshal(rows)
	e := client.Envelope{Status: 200, Result: raw}
	if a.mode == "pagination" {
		e.Count = json.RawMessage(`1`)
	}
	return e, nil
}

const cacheProductsConfig = `provider "iwinv" {}
data "iwinv_content_cache_products" "all" {}
data "iwinv_content_cache_products" "shared" { product_type = "SHARE" }
data "iwinv_content_cache_products" "single" { product_type = "SINGLE" }
`

func cacheCatalogProtocol(t *testing.T) {
	t.Helper()
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
}
func TestProtocolCacheProducts(t *testing.T) {
	cacheCatalogProtocol(t)
	a := &cacheProductsAPI{}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{
		{Config: cacheProductsConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_content_cache_products.all", "products.#", "4"),
			resource.TestCheckNoResourceAttr("data.iwinv_content_cache_products.all", "products.0.product_id"),
			resource.TestCheckResourceAttr("data.iwinv_content_cache_products.all", "products.0.name", "Coming soon"),
			resource.TestCheckResourceAttr("data.iwinv_content_cache_products.all", "products.1.product_id", ""),
			resource.TestCheckResourceAttr("data.iwinv_content_cache_products.all", "products.2.product_id", "synthetic-a"),
			resource.TestCheckResourceAttr("data.iwinv_content_cache_products.all", "products.3.name", "합성 &amp; + 상품"),
			resource.TestCheckResourceAttr("data.iwinv_content_cache_products.shared", "products.#", "2"),
			resource.TestCheckResourceAttr("data.iwinv_content_cache_products.shared", "products.0.product_type", "SHARE"),
			resource.TestCheckResourceAttr("data.iwinv_content_cache_products.single", "products.#", "2"),
			resource.TestCheckNoResourceAttr("data.iwinv_content_cache_products.single", "products.0.product_id"),
		)},
		{Config: cacheProductsConfig, PlanOnly: true},
	}})
}
func TestProtocolCacheProductsFailures(t *testing.T) {
	cacheCatalogProtocol(t)
	for _, mode := range []string{"error", "empty", "duplicate", "missing_id", "number_id", "null_result", "wrong_filter", "pagination"} {
		t.Run(mode, func(t *testing.T) {
			a := &cacheProductsAPI{mode: mode}
			step := resource.TestStep{Config: cacheProductsConfig}
			if mode == "empty" {
				step.Check = resource.TestCheckResourceAttr("data.iwinv_content_cache_products.all", "products.#", "0")
			} else {
				step.ExpectError = regexp.MustCompile("Unable to read cache products")
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{step}})
		})
	}
	for _, filter := range []string{`""`, `"share"`, `"DEDICATED"`} {
		a := &cacheProductsAPI{}
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{{Config: `provider "iwinv" {}
data "iwinv_content_cache_products" "all" { product_type = ` + filter + ` }`, ExpectError: regexp.MustCompile("Invalid cache catalog filter")}}})
		if a.calls != 0 {
			t.Fatal("invalid filter reached API")
		}
	}
}
func TestCacheProductsUnknownReadMakesNoRequest(t *testing.T) {
	ctx := context.Background()
	a := &cacheProductsAPI{}
	d := &cacheProductsDataSource{service: &hosted.CacheCatalogService{API: a}}
	sr := datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, &sr)
	state := tfsdk.State{Schema: sr.Schema}
	if ds := state.Set(ctx, &cacheProductsModel{Type: types.StringUnknown()}); ds.HasError() {
		t.Fatal(ds)
	}
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema}}
	d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: state.Raw}}, &resp)
	if !resp.Diagnostics.HasError() || a.calls != 0 {
		t.Fatal("unknown filter became unfiltered request")
	}
}
func TestAccCacheProducts(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1")
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
			t.Fatal("environment credentials required")
		}
	}, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
		{Config: cacheProductsConfig, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("data.iwinv_content_cache_products.all", "products.0.name"), resource.TestCheckNoResourceAttr("data.iwinv_content_cache_products.all", "products.0.product_id"), resource.TestCheckResourceAttr("data.iwinv_content_cache_products.shared", "products.0.product_type", "SHARE"), resource.TestCheckResourceAttr("data.iwinv_content_cache_products.single", "products.0.product_type", "SINGLE"))},
		{Config: cacheProductsConfig, PlanOnly: true},
	}})
}
