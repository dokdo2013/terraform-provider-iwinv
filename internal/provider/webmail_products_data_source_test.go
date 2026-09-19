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
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type webmailProductsAPI struct {
	mu    sync.Mutex
	mode  string
	calls int
}

func (a *webmailProductsAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if p != "/v1/webmail/products" || len(q) != 0 {
		return client.Envelope{}, errors.New("unexpected catalog request")
	}
	rows := []map[string]any{
		{"product_id": "wm-b", "product_name": "한글 &amp; 이름", "status": "available", "spec": map[string]any{"type": "SHARE", "disk": 100}, "price": map[string]any{"private": "excluded"}},
		{"product_id": "", "product_name": "soon B", "status": "coming_soon", "spec": map[string]any{"type": "SHARE"}},
		{"product_id": "wm-a", "product_name": "한글 &amp; 이름", "status": "available", "spec": map[string]any{"type": "SHARE"}},
		{"product_id": "", "product_name": "soon A", "status": "coming_soon", "spec": map[string]any{"type": "SHARE"}},
	}
	if a.calls%2 == 0 {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}
	switch a.mode {
	case "empty":
		rows = []map[string]any{}
	case "null-id":
		rows[0]["product_id"] = nil
	case "duplicate":
		rows = append(rows, rows[0])
	case "error":
		return client.Envelope{}, &client.Error{Kind: "http_status", Status: 403, Code: "CHECK_IP"}
	}
	b, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: b}, nil
}

const webmailProductsConfig = `provider "iwinv" {}
data "iwinv_webmail_products" "all" {}
`

func TestProtocolWebmailProducts(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := &webmailProductsAPI{}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{
		{Config: webmailProductsConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.#", "4"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.0.product_id", ""),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.0.name", "soon A"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.1.product_id", ""),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.1.name", "soon B"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.2.product_id", "wm-a"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.2.name", "한글 &amp; 이름"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.3.product_type", "SHARE"),
		)},
		{Config: webmailProductsConfig, PlanOnly: true},
	}})
	for _, tt := range []struct{ mode, want string }{{"empty", ""}, {"null-id", "missing or invalid"}, {"duplicate", "duplicate|ambiguous"}, {"error", "CHECK_IP"}} {
		t.Run(tt.mode, func(t *testing.T) {
			a := &webmailProductsAPI{mode: tt.mode}
			step := resource.TestStep{Config: webmailProductsConfig}
			if tt.want != "" {
				step.ExpectError = regexp.MustCompile(tt.want)
			} else {
				step.Check = resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.#", "0")
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{step}})
		})
	}
}
func TestAccWebmailProducts(t *testing.T) {
	if os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("requires TF_ACC=1 and IWINV_LIVE_READ=1")
	}
	if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
		t.Fatal("environment credentials required")
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: groupFactories(c), Steps: []resource.TestStep{
		{Config: webmailProductsConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.#", "12"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.0.product_id", ""),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.7.product_id", ""),
			resource.TestCheckResourceAttrSet("data.iwinv_webmail_products.all", "products.8.product_id"),
			resource.TestCheckResourceAttrSet("data.iwinv_webmail_products.all", "products.11.product_id"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.0.status", "coming_soon"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.8.status", "available"),
			resource.TestCheckResourceAttr("data.iwinv_webmail_products.all", "products.8.product_type", "SHARE"),
		)},
		{Config: webmailProductsConfig, PlanOnly: true},
	}})
}
