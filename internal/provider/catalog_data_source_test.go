package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type catalogAPI struct{}

func (catalogAPI) Get(_ context.Context, path string, q url.Values) (client.Envelope, error) {
	flavor := strings.HasPrefix(path, "/v1/flavors")
	field := "image_id"
	if flavor {
		field = "flavor_id"
	}
	rows := []map[string]string{}
	if !strings.HasSuffix(path, "/missing") {
		ids := []string{"synthetic-b.2", "synthetic-a.1"}
		if strings.Count(path, "/") == 3 {
			ids = []string{path[strings.LastIndex(path, "/")+1:]}
		}
		for _, id := range ids {
			rows = append(rows, map[string]string{field: id, "name": "합성 상품", "visibility": "public", "image_type": "os_linux"})
		}
	}
	b, _ := json.Marshal(rows)
	e := client.Envelope{Status: 200, Result: b, Count: json.RawMessage(fmt.Sprint(len(rows)))}
	if strings.Count(path, "/") == 2 {
		e.PageNo = json.RawMessage(q.Get("page_no"))
		e.PageSize = json.RawMessage(q.Get("page_size"))
	}
	if flavor {
		e.Total = json.RawMessage(fmt.Sprint(len(rows)))
	}
	return e, nil
}

const catalogConfig = `
provider "iwinv" {}
data "iwinv_images" "all" {}
data "iwinv_instance_types" "all" {}
data "iwinv_image" "selected" { id = data.iwinv_images.all.ids[0] }
data "iwinv_instance_type" "selected" { id = data.iwinv_instance_types.all.ids[0] }
`

func TestProtocolCatalogs(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	factory := providerserver.NewProtocol6WithError(&IwinvProvider{version: "test", newClient: func(string, string) (compute.API, error) { return catalogAPI{}, nil }})
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": factory}, Steps: []resource.TestStep{
		{Config: catalogConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_images.all", "ids.0", "synthetic-a.1"),
			resource.TestCheckResourceAttr("data.iwinv_instance_types.all", "ids.#", "2"),
			resource.TestCheckResourceAttr("data.iwinv_image.selected", "visibility", "public"),
			resource.TestCheckResourceAttr("data.iwinv_image.selected", "image_type", "os_linux"),
			resource.TestCheckResourceAttr("data.iwinv_instance_type.selected", "name", "합성 상품"),
		)},
		{Config: catalogConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: `provider "iwinv" {}
data "iwinv_instance_type" "missing" { id = "missing" }`, ExpectError: regexp.MustCompile("exactly one matching ID")},
		{Config: catalogConfig},
	}})
}
func TestAccCatalogs(t *testing.T) {
	if os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1 for authenticated read-only acceptance")
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
			t.Fatal("environment credentials required")
		}
	}, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
		{Config: catalogConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrSet("data.iwinv_image.selected", "image_type"),
			resource.TestCheckResourceAttrSet("data.iwinv_instance_type.selected", "name"),
		)},
		{Config: catalogConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}
