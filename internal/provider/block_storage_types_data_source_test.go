package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/storage"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

type blockTypesAPI struct {
	mu    sync.Mutex
	mode  string
	calls int
}

func (a *blockTypesAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if p != "/v1/block-storages/types" || len(q) > 1 {
		return client.Envelope{}, errors.New("unexpected catalog request")
	}
	rows := []map[string]any{
		{"type": "ssd", "min": int64(10), "max": int64(9007199254740993), "zones": nil},
		{"type": "sata", "min": int64(10), "max": int64(20000), "zones": []string{"zone_z", "zone-a"}},
		{"type": "empty", "min": int64(0), "max": int64(0), "zones": []string{}},
	}
	if a.calls%2 == 0 {
		rows[0], rows[2] = rows[2], rows[0]
		for _, r := range rows {
			if r["type"] == "sata" {
				r["zones"] = []string{"zone-a", "zone_z"}
			}
		}
	}
	if value, ok := q["type"]; ok {
		if len(value) != 1 || value[0] == "" {
			return client.Envelope{}, errors.New("empty filter reached API")
		}
		filtered := make([]map[string]any, 0)
		for _, r := range rows {
			if r["type"] == value[0] {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
		if len(rows) == 0 {
			return client.Envelope{}, &client.Error{Kind: "http_status", Status: 400, Code: "CHECK_PARAM"}
		}
	}
	switch a.mode {
	case "empty":
		rows = []map[string]any{}
	case "missing-zones":
		delete(rows[0], "zones")
	case "mismatch":
		rows = []map[string]any{{"type": "other", "min": int64(0), "max": int64(1), "zones": nil}}
	case "permission":
		return client.Envelope{}, &client.Error{Kind: "http_status", Status: 403, Code: "CHECK_IP"}
	}
	b, _ := json.Marshal(rows)
	return client.Envelope{Status: 200, Result: b, Count: json.RawMessage(strconv.Itoa(len(rows)))}, nil
}

const blockTypesConfig = `provider "iwinv" {}
data "iwinv_block_storage_types" "all" {}
data "iwinv_block_storage_types" "ssd" { type = "ssd" }
data "iwinv_block_storage_types" "sata" { type = "sata" }
`

func TestProtocolBlockStorageTypes(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := &blockTypesAPI{}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{
		{Config: blockTypesConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.all", "types.#", "3"),
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.all", "types.0.type", "empty"),
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.all", "types.1.type", "sata"),
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.sata", "types.0.availability_zones.0", "zone-a"),
		), ConfigStateChecks: []statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.iwinv_block_storage_types.all", tfjsonpath.New("type"), knownvalue.Null()),
			statecheck.ExpectKnownValue("data.iwinv_block_storage_types.all", tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("availability_zones"), knownvalue.ListSizeExact(0)),
			statecheck.ExpectKnownValue("data.iwinv_block_storage_types.ssd", tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("availability_zones"), knownvalue.Null()),
			statecheck.ExpectKnownValue("data.iwinv_block_storage_types.ssd", tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("maximum_size_gb"), knownvalue.Int64Exact(9007199254740993)),
		}},
		{Config: blockTypesConfig, PlanOnly: true},
	}})
}
func TestProtocolBlockStorageTypeFailures(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	for _, tt := range []struct{ mode, filter, want string }{
		{"empty", "", ""}, {"missing-zones", "", "zones must be present"}, {"mismatch", `type="ssd"`, "does not match"},
		{"permission", "", "CHECK_IP"}, {"invalid", `type=""`, "Invalid block storage type filter"}, {"unknown-code", `type="not-a-type"`, "CHECK_PARAM"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			a := &blockTypesAPI{mode: tt.mode}
			step := resource.TestStep{Config: "provider \"iwinv\" {}\ndata \"iwinv_block_storage_types\" \"all\" { " + tt.filter + " }"}
			if tt.want != "" {
				step.ExpectError = regexp.MustCompile(tt.want)
			} else {
				step.Check = resource.TestCheckResourceAttr("data.iwinv_block_storage_types.all", "types.#", "0")
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{step}})
			if tt.mode == "invalid" && a.calls != 0 {
				t.Fatal("empty filter reached API")
			}
		})
	}
}
func TestBlockStorageUnknownRead(t *testing.T) {
	a := &blockTypesAPI{}
	d := &blockStorageTypesDataSource{service: &storage.Service{API: a}}
	var schema datasource.SchemaResponse
	d.Schema(context.Background(), datasource.SchemaRequest{}, &schema)
	config := tfsdk.Config{Schema: schema.Schema}
	state := tfsdk.State{Schema: schema.Schema}
	diags := state.Set(context.Background(), struct {
		Type  types.String `tfsdk:"type"`
		Types types.List   `tfsdk:"types"`
	}{types.StringUnknown(), types.ListUnknown(schema.Schema.Attributes["types"].GetType().(types.ListType).ElemType)})
	if diags.HasError() {
		t.Fatal(diags)
	}
	config.Raw = state.Raw
	var resp datasource.ReadResponse
	d.Read(context.Background(), datasource.ReadRequest{Config: config}, &resp)
	if !resp.Diagnostics.HasError() || a.calls != 0 {
		t.Fatal("unknown filter performed an unfiltered read")
	}
}
func TestProtocolBlockStorageUnknownFilter(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := &blockTypesAPI{}
	config := `provider "iwinv" {}
resource "terraform_data" "filter" { input = "ssd" }
data "iwinv_block_storage_types" "selected" { type = terraform_data.filter.output }
`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(a), Steps: []resource.TestStep{
		{Config: config, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("data.iwinv_block_storage_types.selected", "types.#", "1"), resource.TestCheckResourceAttr("data.iwinv_block_storage_types.selected", "types.0.type", "ssd"))},
		{Config: config, PlanOnly: true},
	}})
}
func TestAccBlockStorageTypes(t *testing.T) {
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
		{Config: blockTypesConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.all", "types.#", "2"),
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.all", "types.0.type", "sata"),
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.all", "types.1.type", "ssd"),
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.ssd", "types.#", "1"),
			resource.TestCheckResourceAttr("data.iwinv_block_storage_types.sata", "types.#", "1"),
			resource.TestCheckResourceAttrPair("data.iwinv_block_storage_types.all", "types.0.maximum_size_gb", "data.iwinv_block_storage_types.sata", "types.0.maximum_size_gb"),
			resource.TestCheckResourceAttrPair("data.iwinv_block_storage_types.all", "types.1.maximum_size_gb", "data.iwinv_block_storage_types.ssd", "types.0.maximum_size_gb"),
		), ConfigStateChecks: []statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.iwinv_block_storage_types.ssd", tfjsonpath.New("types").AtSliceIndex(0).AtMapKey("availability_zones"), knownvalue.Null()),
		}},
		{Config: blockTypesConfig, PlanOnly: true},
	}})
}
