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
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type hostingCatalogAPI struct {
	mu    sync.Mutex
	mode  string
	calls int
}

func (a *hostingCatalogAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if a.mode == "error" {
		return client.Envelope{}, errors.New("synthetic catalog failure")
	}
	if a.mode == "empty" {
		return client.Envelope{Status: 200, Result: json.RawMessage(`[]`)}, nil
	}
	var rows []map[string]any
	switch p {
	case "/v1/webhosting/products":
		if len(q) > 1 || len(q) > 0 && q.Get("type") == "" {
			return client.Envelope{}, errors.New("unexpected product query")
		}
		for _, v := range []struct{ id, kind string }{{"synthetic-b", "SINGLE"}, {"synthetic&a", "SHARE"}} {
			if q.Get("type") != "" && q.Get("type") != v.kind {
				continue
			}
			rows = append(rows, map[string]any{"product_id": v.id, "product_name": "합성 &amp; + 상품", "status": "available", "spec": map[string]any{"type": v.kind, "allow_php_version": []string{"PHP 8.4", "PHP 8.2"}, "domain": map[string]any{"allow_custom_domain": true, "enable_domain_folder": false, "max_domain_count": 2, "edit_interval_days": 1}}})
		}
	case "/v1/webhosting/servers":
		if len(q) != 1 || q.Get("product_id") != "synthetic&a" {
			return client.Envelope{}, errors.New("required product query not preserved")
		}
		for _, id := range []int64{9007199254740993, 12} {
			rows = append(rows, map[string]any{"idx": id, "charset": "UTF-8", "php_version": "PHP 8.4", "db": "MariaDB", "program": ""})
		}
	default:
		return client.Envelope{}, errors.New("unexpected catalog endpoint")
	}
	if a.mode == "duplicate" {
		rows = append(rows, rows[0])
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
func hostingCatalogFactories(a *hostingCatalogAPI) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(&IwinvProvider{version: "test", newClient: func(string, string) (compute.API, error) { return a, nil }})}
}

const hostingCatalogConfig = `provider "iwinv" {}
data "iwinv_webhosting_products" "all" {}
data "iwinv_webhosting_products" "shared" { type = "SHARE" }
data "iwinv_webhosting_products" "single" { type = "SINGLE" }
data "iwinv_webhosting_servers" "selected" { product_id = data.iwinv_webhosting_products.shared.ids[0] }
`

func TestProtocolHostingCatalogs(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := &hostingCatalogAPI{}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: hostingCatalogFactories(a), Steps: []resource.TestStep{
		{Config: hostingCatalogConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.all", "ids.#", "2"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.all", "ids.0", "synthetic&a"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.shared", "products.0.type", "SHARE"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.single", "products.0.type", "SINGLE"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.shared", "products.0.name", "합성 &amp; + 상품"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.shared", "products.0.php_versions.0", "PHP 8.2"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.shared", "products.0.allow_custom_domain", "true"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.shared", "products.0.enable_domain_folder", "false"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.shared", "products.0.max_domain_count", "2"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.shared", "products.0.domain_edit_interval_days", "1"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_servers.selected", "ids.0", "12"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_servers.selected", "ids.1", "9007199254740993"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_servers.selected", "servers.1.id", "9007199254740993"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_servers.selected", "servers.0.program", ""),
		)},
		{Config: hostingCatalogConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}
func TestProtocolHostingCatalogFailures(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	for _, kind := range []string{"products", "servers"} {
		for _, mode := range []string{"error", "duplicate", "malformed", "pagination", "empty"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				a := &hostingCatalogAPI{mode: mode}
				input := ""
				if kind == "servers" {
					input = `product_id = "synthetic&a"`
				}
				step := resource.TestStep{Config: `provider "iwinv" {}
data "iwinv_webhosting_` + kind + `" "test" {` + input + `}`}
				if mode == "empty" {
					step.Check = resource.TestCheckResourceAttr("data.iwinv_webhosting_"+kind+".test", "ids.#", "0")
				} else {
					step.ExpectError = regexp.MustCompile("Unable to read hosting")
				}
				resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: hostingCatalogFactories(a), Steps: []resource.TestStep{step}})
			})
		}
	}
	for _, tc := range []struct{ config, message string }{
		{`data "iwinv_webhosting_products" "test" { type = "share" }`, "Invalid hosting product type"},
		{`data "iwinv_webhosting_products" "test" { type = "" }`, "Invalid hosting product type"},
		{`data "iwinv_webhosting_servers" "test" { product_id = "" }`, "Invalid hosting product ID"},
	} {
		a := &hostingCatalogAPI{}
		resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: hostingCatalogFactories(a), Steps: []resource.TestStep{{Config: `provider "iwinv" {}
` + tc.config, ExpectError: regexp.MustCompile(tc.message)}}})
		if a.calls != 0 {
			t.Fatal("invalid input reached API")
		}
	}
}
func TestAccHostingCatalogs(t *testing.T) {
	if os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1")
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
			t.Fatal("environment credentials required")
		}
	}, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
		{Config: hostingCatalogConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("data.iwinv_webhosting_products.all", "products.0.id"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.shared", "products.0.type", "SHARE"),
			resource.TestCheckResourceAttr("data.iwinv_webhosting_products.single", "products.0.type", "SINGLE"),
			resource.TestCheckResourceAttrSet("data.iwinv_webhosting_servers.selected", "servers.0.id"),
			resource.TestCheckResourceAttrSet("data.iwinv_webhosting_servers.selected", "servers.0.php_version"),
		)},
		{Config: hostingCatalogConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}
