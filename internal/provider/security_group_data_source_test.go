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
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

// This adapter rejects all writes, including accidental writes during destroy.
type readOnlyGroups struct{ network.API }

func (a readOnlyGroups) PostJSON(context.Context, string, any) (client.Envelope, error) {
	return client.Envelope{}, errors.New("data source attempted create")
}
func (a readOnlyGroups) PutJSON(context.Context, string, any) (client.Envelope, error) {
	return client.Envelope{}, errors.New("data source attempted update")
}
func (a readOnlyGroups) Delete(context.Context, string) (client.Envelope, error) {
	return client.Envelope{}, errors.New("data source attempted delete")
}

type groupDataAPI struct {
	network.API
	mu                 sync.Mutex
	mode               string
	calls, secondPages int
	reverse            bool
}

func (a *groupDataAPI) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	rows := make([]map[string]any, 0)
	for i := 0; i < 51; i++ {
		var content any = "한글 &amp;amp; &lt;group&gt;"
		if i == 1 {
			content = nil
		}
		if i == 2 {
			content = ""
		}
		rows = append(rows, map[string]any{"firewall_id": fmt.Sprintf("FIREWALL-%03d", i), "title": "같은 &amp; 이름", "content": content, "icmp": "Y", "rules": []any{map[string]any{"secret": "excluded"}}})
	}
	e := client.Envelope{Status: 200}
	if p == "/v1/security-groups" {
		page, _ := strconv.Atoi(q.Get("page_no"))
		size, _ := strconv.Atoi(q.Get("page_size"))
		if page < 1 || size != 50 {
			return e, errors.New("unexpected pagination query")
		}
		if page > 1 {
			a.secondPages++
			if a.mode == "late-error" {
				return e, errors.New("synthetic late page failure")
			}
		}
		start := (page - 1) * size
		if start >= len(rows) {
			rows = []map[string]any{}
		} else {
			end := start + size
			if end > len(rows) {
				end = len(rows)
			}
			rows = rows[start:end]
		}
		if a.mode == "duplicate" && page == 2 {
			rows[0]["firewall_id"] = "FIREWALL-000"
		}
		if a.mode == "empty" {
			rows = []map[string]any{}
		}
		a.reverse = !a.reverse
		if a.reverse {
			for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
		e.PageNo = json.RawMessage(q.Get("page_no"))
		e.PageSize = json.RawMessage(q.Get("page_size"))
	} else {
		id := strings.TrimPrefix(p, "/v1/security-groups/")
		selected := make([]map[string]any, 0)
		for _, r := range rows {
			if r["firewall_id"] == id {
				selected = append(selected, r)
			}
		}
		rows = selected
		switch a.mode {
		case "missing":
			rows = []map[string]any{}
		case "multiple":
			rows = append(rows, map[string]any{"firewall_id": "FIREWALL-other", "title": "other", "content": nil, "icmp": "N"})
		case "wrong-id":
			rows[0]["firewall_id"] = "FIREWALL-other"
		case "permission":
			return e, &client.Error{Kind: "http_status", Status: 403, Code: "CHECK_IP"}
		}
	}
	e.Result, _ = json.Marshal(rows)
	e.Count = json.RawMessage(strconv.Itoa(len(rows)))
	return e, nil
}

const groupDataConfig = `provider "iwinv" {}
data "iwinv_security_groups" "all" {}
data "iwinv_security_group" "selected" { id = "FIREWALL-000" }
data "iwinv_security_group" "nullable" { id = "FIREWALL-001" }
data "iwinv_security_group" "empty" { id = "FIREWALL-002" }
`

func TestProtocolSecurityGroupDataSources(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := &groupDataAPI{}
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(readOnlyGroups{a}), Steps: []resource.TestStep{
		{Config: groupDataConfig, Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_security_groups.all", "ids.#", "51"),
			resource.TestCheckResourceAttr("data.iwinv_security_groups.all", "groups.#", "51"),
			resource.TestCheckResourceAttr("data.iwinv_security_groups.all", "ids.0", "FIREWALL-000"),
			resource.TestCheckResourceAttr("data.iwinv_security_groups.all", "groups.50.id", "FIREWALL-050"),
			resource.TestCheckResourceAttr("data.iwinv_security_group.selected", "description", "한글 &amp; <group>"),
			resource.TestCheckResourceAttr("data.iwinv_security_group.selected", "name", "같은 &amp; 이름"),
			resource.TestCheckResourceAttr("data.iwinv_security_group.selected", "allow_icmp", "true"),
		), ConfigStateChecks: []statecheck.StateCheck{
			statecheck.ExpectKnownValue("data.iwinv_security_group.nullable", tfjsonpath.New("description"), knownvalue.Null()),
			statecheck.ExpectKnownValue("data.iwinv_security_group.empty", tfjsonpath.New("description"), knownvalue.StringExact("")),
			statecheck.ExpectKnownValue("data.iwinv_security_groups.all", tfjsonpath.New("groups").AtSliceIndex(1).AtMapKey("description"), knownvalue.Null()),
		}},
		{Config: groupDataConfig, PlanOnly: true},
	}})
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.secondPages == 0 {
		t.Fatal("did not traverse second page")
	}
}
func TestProtocolSecurityGroupDataFailures(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	for _, tt := range []struct{ mode, config, want string }{
		{"empty", `data "iwinv_security_groups" "all" {}`, ""},
		{"late-error", `data "iwinv_security_groups" "all" {}`, "synthetic late page failure"},
		{"duplicate", `data "iwinv_security_groups" "all" {}`, "repeats an ID across pages"},
		{"missing", `data "iwinv_security_group" "one" { id="FIREWALL-000" }`, "Security group not found"},
		{"multiple", `data "iwinv_security_group" "one" { id="FIREWALL-000" }`, "exactly the requested ID"},
		{"wrong-id", `data "iwinv_security_group" "one" { id="FIREWALL-000" }`, "exactly the requested ID"},
		{"permission", `data "iwinv_security_group" "one" { id="FIREWALL-000" }`, "CHECK_IP"},
		{"invalid", `data "iwinv_security_group" "one" { id="FIREWALL-000/../other" }`, "Invalid security group ID"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			a := &groupDataAPI{mode: tt.mode}
			step := resource.TestStep{Config: "provider \"iwinv\" {}\n" + tt.config}
			if tt.want != "" {
				step.ExpectError = regexp.MustCompile(tt.want)
			} else {
				step.Check = resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttr("data.iwinv_security_groups.all", "ids.#", "0"), resource.TestCheckResourceAttr("data.iwinv_security_groups.all", "groups.#", "0"))
			}
			resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(readOnlyGroups{a}), Steps: []resource.TestStep{step}})
			if tt.mode == "invalid" && a.calls != 0 {
				t.Fatal("invalid ID reached API")
			}
		})
	}
}
func TestProtocolSecurityGroupDataUnknown(t *testing.T) {
	if os.Getenv("IWINV_PROTOCOL_TEST") != "1" {
		t.Skip("set IWINV_PROTOCOL_TEST=1")
	}
	a := &groupDataAPI{}
	config := `provider "iwinv" {}
resource "terraform_data" "id" { input = "FIREWALL-000" }
data "iwinv_security_group" "one" { id = terraform_data.id.output }
`
	resource.Test(t, resource.TestCase{IsUnitTest: true, ProtoV6ProviderFactories: groupFactories(readOnlyGroups{a}), Steps: []resource.TestStep{
		{Config: config, Check: resource.TestCheckResourceAttr("data.iwinv_security_group.one", "id", "FIREWALL-000")},
		{Config: config, PlanOnly: true},
	}})
}
