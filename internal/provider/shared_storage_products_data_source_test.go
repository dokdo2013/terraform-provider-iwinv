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
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type storageProductsAPI struct {
	mu    sync.Mutex
	mode  string
	calls int
}

func (a *storageProductsAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if p != "/v1/apinas/products" || len(q) != 0 {
		return client.Envelope{}, errors.New("unexpected catalog request")
	}
	if a.mode == "error" {
		return client.Envelope{}, errors.New("synthetic catalog failure")
	}
	rows := []map[string]any{}
	if a.mode != "empty" {
		for _, v := range []struct {
			id, name, status string
			version          any
			min, max         int64
		}{
			{"synthetic-b", "합성 &amp; + 상품", "available", "v2", 100, 2000},
			{"", "Coming soon B", "comingsoon", nil, 0, 0},
			{"synthetic-a", "A", "available", "", 200, 1000},
			{"", "Coming soon A", "comingsoon", nil, 0, 0},
		} {
			rows = append(rows, map[string]any{"product_id": v.id, "product_name": v.name, "status": v.status, "spec": map[string]any{"version": v.version, "disk": map[string]any{"min": v.min, "max": v.max}}})
		}
	}
	switch a.mode {
	case "duplicate":
		rows = append(rows, rows[0])
	case "duplicate_empty":
		rows = append(rows, rows[1])
	case "missing_id":
		delete(rows[0], "product_id")
	case "null_id":
		rows[0]["product_id"] = nil
	case "number_id":
		rows[0]["product_id"] = 123
	case "missing_version":
		delete(rows[0]["spec"].(map[string]any), "version")
	case "number_version":
		rows[0]["spec"].(map[string]any)["version"] = 2
	case "missing_min":
		delete(rows[0]["spec"].(map[string]any)["disk"].(map[string]any), "min")
	case "negative_min":
		rows[0]["spec"].(map[string]any)["disk"].(map[string]any)["min"] = -1
	case "inverted_bounds":
		rows[0]["spec"].(map[string]any)["disk"].(map[string]any)["max"] = 99
	case "fractional_max":
		rows[0]["spec"].(map[string]any)["disk"].(map[string]any)["max"] = 2000.5
	case "null_result":
		return client.Envelope{Status: 200, Result: json.RawMessage(`null`)}, nil
	}
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

const storageProductsConfig = `provider "iwinv" {}
data "iwinv_shared_storage_products" "all" {}
`

func TestProtocolStorageProducts(t *testing.T) {
	cacheCatalogProtocol(t)
	const address = "data.iwinv_shared_storage_products.all"
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(&storageProductsAPI{}), Steps: []resource.TestStep{
		{Config: storageProductsConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(address, "products.#", "4"),
			resource.TestCheckResourceAttr(address, "products.0.product_id", ""),
			resource.TestCheckResourceAttr(address, "products.0.name", "Coming soon A"),
			resource.TestCheckNoResourceAttr(address, "products.0.version"),
			resource.TestCheckResourceAttr(address, "products.0.minimum_size_gb", "0"),
			resource.TestCheckResourceAttr(address, "products.0.maximum_size_gb", "0"),
			resource.TestCheckResourceAttr(address, "products.1.name", "Coming soon B"),
			resource.TestCheckResourceAttr(address, "products.2.product_id", "synthetic-a"),
			resource.TestCheckResourceAttr(address, "products.2.version", ""),
			resource.TestCheckResourceAttr(address, "products.2.minimum_size_gb", "200"),
			resource.TestCheckResourceAttr(address, "products.2.maximum_size_gb", "1000"),
			resource.TestCheckResourceAttr(address, "products.3.name", "합성 &amp; + 상품"),
			resource.TestCheckResourceAttr(address, "products.3.version", "v2"),
			resource.TestCheckResourceAttr(address, "products.3.minimum_size_gb", "100"),
			resource.TestCheckResourceAttr(address, "products.3.maximum_size_gb", "2000"),
		)},
		{Config: storageProductsConfig, PlanOnly: true},
	}})
}
func TestProtocolStorageProductsFailures(t *testing.T) {
	cacheCatalogProtocol(t)
	for _, mode := range []string{"error", "empty", "duplicate", "duplicate_empty", "missing_id", "null_id", "number_id", "missing_version", "number_version", "missing_min", "negative_min", "inverted_bounds", "fractional_max", "null_result", "pagination"} {
		t.Run(mode, func(t *testing.T) {
			step := resource.TestStep{Config: storageProductsConfig}
			if mode == "empty" {
				step.Check = resource.TestCheckResourceAttr("data.iwinv_shared_storage_products.all", "products.#", "0")
			} else {
				step.ExpectError = regexp.MustCompile("Unable to read NAS products")
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(&storageProductsAPI{mode: mode}), Steps: []resource.TestStep{step}})
		})
	}
}
func TestAccStorageProducts(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1")
	}
	config := storageProductsConfig + `
output "available_api_nas" {
 value = one([for p in data.iwinv_shared_storage_products.all.products : p if p.product_id == "api_nas" && p.status == "available"])
 precondition {
  condition = length([for p in data.iwinv_shared_storage_products.all.products : p if p.product_id == "api_nas" && p.status == "available" && p.minimum_size_gb == 100 && p.maximum_size_gb == 2000]) == 1
  error_message = "Observed NAS product bounds changed; review the contract."
 }
 precondition {
  condition = length([for p in data.iwinv_shared_storage_products.all.products : p if p.product_id == "" && p.version == null && p.minimum_size_gb == 0 && p.maximum_size_gb == 0]) > 0
  error_message = "Expected unselectable NAS rows changed; review the contract."
 }
}
`
	resource.Test(t, resource.TestCase{PreCheck: func() {
		if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
			t.Fatal("environment credentials required")
		}
	}, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
		{Config: config, Check: resource.TestCheckResourceAttrSet("data.iwinv_shared_storage_products.all", "products.0.name")},
		{Config: config, PlanOnly: true},
	}})
}
