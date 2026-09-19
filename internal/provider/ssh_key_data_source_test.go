package provider

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/compute"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

type sshAPI struct{ empty bool }

func (a sshAPI) Get(_ context.Context, path string, q url.Values) (client.Envelope, error) {
	if path != "/v1/auth/ssh_key" {
		panic("unexpected synthetic SSH operation")
	}
	body, count := `[{"ssh_key_id":"synthetic-b","name":"동일 이름"},{"ssh_key_id":"synthetic-a","name":"동일 이름"}]`, `2`
	if a.empty {
		body, count = `[]`, `0`
	}
	return client.Envelope{Status: 200, Result: json.RawMessage(body), Count: json.RawMessage(count), PageNo: json.RawMessage(q.Get("page_no")), PageSize: json.RawMessage(q.Get("page_size"))}, nil
}

const sshConfig = `
provider "iwinv" {}
data "iwinv_ssh_keys" "all" {}
data "iwinv_ssh_key" "selected" { id = data.iwinv_ssh_keys.all.ids[0] }
`

func TestProtocolSSHKeys(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	factory := providerserver.NewProtocol6WithError(&IwinvProvider{version: "test", newClient: func(string, string) (compute.API, error) { return sshAPI{}, nil }})
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": factory}, Steps: []resource.TestStep{
		{Config: sshConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_ssh_keys.all", "ids.#", "2"),
			resource.TestCheckResourceAttr("data.iwinv_ssh_keys.all", "ids.0", "synthetic-a"),
			resource.TestCheckResourceAttr("data.iwinv_ssh_keys.all", "keys.0.id", "synthetic-a"),
			resource.TestCheckResourceAttr("data.iwinv_ssh_key.selected", "name", "동일 이름"),
		)},
		{Config: sshConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
		{Config: `provider "iwinv" {}
data "iwinv_ssh_key" "missing" { id = "missing" }`, ExpectError: regexp.MustCompile("did not find the exact requested ID")},
		{Config: sshConfig},
	}})
	view := providerserver.NewProtocol6WithError(&IwinvProvider{version: "test", newClient: func(string, string) (compute.API, error) { return sshAPI{empty: true}, nil }})
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": view}, Steps: []resource.TestStep{{
		Config: `provider "iwinv" {}
data "iwinv_ssh_keys" "all" {}`,
		Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_ssh_keys.all", "ids.#", "0"),
			resource.TestCheckResourceAttr("data.iwinv_ssh_keys.all", "keys.#", "0"),
		),
	}}})
}

func TestAccSSHKeys(t *testing.T) {
	if os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1 for authenticated read-only acceptance")
	}
	resource.Test(t, resource.TestCase{PreCheck: func() {
		if os.Getenv("IWINV_ACCESS_KEY") == "" || os.Getenv("IWINV_SECRET_KEY") == "" {
			t.Fatal("environment credentials required")
		}
	}, ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){"iwinv": providerserver.NewProtocol6WithError(New("test")())}, Steps: []resource.TestStep{
		{Config: sshConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttrPair("data.iwinv_ssh_key.selected", "id", "data.iwinv_ssh_keys.all", "ids.0"),
			resource.TestCheckResourceAttrPair("data.iwinv_ssh_key.selected", "name", "data.iwinv_ssh_keys.all", "keys.0.name"),
		)},
		{Config: sshConfig, PlanOnly: true, ExpectNonEmptyPlan: false},
	}})
}
