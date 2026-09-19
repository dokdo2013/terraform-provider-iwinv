package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestProviderSchema(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil || len(resp.Diagnostics) != 0 {
		t.Fatalf("schema diagnostics: %v %v", err, resp.Diagnostics)
	}
	if len(resp.ResourceSchemas) != 6 || resp.ResourceSchemas["iwinv_security_group"] == nil || len(resp.DataSourceSchemas) != 11 || resp.DataSourceSchemas["iwinv_availability_zones"] == nil {
		t.Fatal("unexpected public schema")
	}
}

type zoneAPI struct{ key string }

func (a zoneAPI) Get(_ context.Context, path string, _ url.Values) (client.Envelope, error) {
	if path != "/v1/zones" {
		return client.Envelope{}, fmt.Errorf("unexpected API operation")
	}
	rows := []compute.Zone{{ID: a.key + "-b", Name: "두 번째", Status: "on"}, {ID: a.key + "-a", Name: "첫 번째", Status: "on"}}
	b, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: b, Count: json.RawMessage(`2`)}, nil
}

func mockFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	factory := providerserver.NewProtocol6WithError(&IwinvProvider{version: "test", newClient: func(access, secret string) (compute.API, error) {
		if access == "" || secret != access+"-secret" {
			return nil, fmt.Errorf("synthetic credential validation failed")
		}
		return zoneAPI{key: access}, nil
	}})
	// plugin-testing does not isolate aliases on its reattached server. Use
	// distinct factories as its TestCase documentation prescribes. Native
	// alias behavior requires a separately launched binary test.
	return map[string]func() (tfprotov6.ProviderServer, error){"iwinv": factory, "iwinvsecondary": factory}
}

func TestProtocolAvailabilityZones(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1 for synthetic Terraform CLI tests")
	}
	config := `
provider "iwinv" {
  access_key = "synthetic-primary"
  secret_key = "synthetic-primary-secret"
}
provider "iwinvsecondary" {
  access_key = "synthetic-secondary"
  secret_key = "synthetic-secondary-secret"
}
data "iwinv_availability_zones" "primary" {}
data "iwinv_availability_zones" "secondary" { provider = iwinvsecondary }
`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: mockFactories(), Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_availability_zones.primary", "zone_ids.0", "synthetic-primary-a"),
			resource.TestCheckResourceAttr("data.iwinv_availability_zones.primary", "names.0", "첫 번째"),
			resource.TestCheckResourceAttr("data.iwinv_availability_zones.primary", "zones.0.id", "synthetic-primary-a"),
			resource.TestCheckResourceAttr("data.iwinv_availability_zones.secondary", "zone_ids.0", "synthetic-secondary-a"),
		)},
		{Config: config, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}

func TestProtocolCredentialPrecedence(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1 for synthetic Terraform CLI tests")
	}
	t.Setenv("IWINV_ACCESS_KEY", "synthetic-env")
	t.Setenv("IWINV_SECRET_KEY", "synthetic-env-secret")
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: mockFactories(), Steps: []resource.TestStep{
		{Config: `provider "iwinv" {}
data "iwinv_availability_zones" "test" {}`, Check: resource.TestCheckResourceAttr("data.iwinv_availability_zones.test", "zone_ids.0", "synthetic-env-a")},
		{Config: `provider "iwinv" { access_key = "" }
data "iwinv_availability_zones" "test" {}`, ExpectError: regexp.MustCompile("Invalid iwinv configuration")},
		{Config: `provider "iwinv" {}
data "iwinv_availability_zones" "test" {}`, Check: resource.TestCheckResourceAttr("data.iwinv_availability_zones.test", "zone_ids.0", "synthetic-env-a")},
	}})
}

func TestAccAvailabilityZones(t *testing.T) {
	if os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1 for explicit authenticated read-only acceptance")
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
			t.Fatal("iwinv environment credentials required")
		}
	}, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
		{Config: `provider "iwinv" {}
data "iwinv_availability_zones" "test" {}`, Check: resource.TestCheckResourceAttrWith("data.iwinv_availability_zones.test", "zone_ids.#", func(value string) error {
			count, err := strconv.Atoi(value)
			if err != nil || count < 1 {
				return fmt.Errorf("expected at least one API-visible zone")
			}
			return nil
		})},
		{Config: `provider "iwinv" {}
data "iwinv_availability_zones" "test" {}`, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}
