package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccSecurityGroupDataSources(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_TERRAFORM_WRITE") != "1" {
		t.Skip("requires explicitly authorized live fixture writes and private journal")
	}
	dir := os.Getenv("IWINV_TEST_JOURNAL_DIR")
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute private journal directory required")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatal("private journal directory must have mode 0700")
	}
	f, err := os.CreateTemp(dir, "terraform-group-data-*.json")
	if err != nil {
		t.Fatal("cannot create private journal")
	}
	journal := f.Name()
	if f.Close() != nil {
		t.Fatal("cannot close private journal")
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("environment credentials required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	reads := network.Service{API: c}
	baseline, err := reads.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a := &ownedGroupAPI{Client: c, baseline: map[string]bool{}, journal: journal, records: []*groupLiveRecord{}}
	for _, g := range baseline {
		a.baseline[g.ID] = true
	}
	if a.save() != nil {
		t.Fatal("cannot initialize private journal")
	}
	writes := network.Service{API: a}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		for _, id := range a.identities() {
			g, err := writes.Group(ctx, id)
			if err != nil {
				t.Error("cleanup read failed")
				continue
			}
			if g != nil {
				if err := writes.DeleteGroup(ctx, id); err != nil {
					t.Error("cleanup delete failed")
				}
			}
			if a.verifyDeleted(ctx, id) != nil {
				t.Error("cleanup absence unverified")
			}
		}
		after, err := reads.Groups(ctx)
		if err != nil {
			t.Error("cleanup list failed")
			return
		}
		visible := map[string]bool{}
		for _, g := range after {
			visible[g.ID] = true
		}
		for id := range a.baseline {
			if !visible[id] {
				t.Error("baseline identity no longer visible")
			}
		}
		for _, id := range a.identities() {
			if visible[id] {
				t.Error("owned identity remains in list")
			}
		}
	})
	name := "tf-data-" + time.Now().UTC().Format("150405.000000000")
	description := "한글 &amp; <read> + %"
	created, err := writes.CreateGroup(ctx, network.GroupInput{Name: name, Description: &description, AllowICMP: true})
	if err != nil {
		t.Fatal("fixture creation failed; inspect private journal")
	}
	if created.ID == "" {
		t.Fatal("fixture identity unresolved")
	}
	config := fmt.Sprintf("provider \"iwinv\" {}\ndata \"iwinv_security_groups\" \"all\" {}\ndata \"iwinv_security_group\" \"one\" { id = %q }\n", created.ID)
	check := func(wantName string) resource.TestCheckFunc {
		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("data.iwinv_security_group.one", "id", created.ID),
			resource.TestCheckResourceAttr("data.iwinv_security_group.one", "name", wantName),
			resource.TestCheckResourceAttr("data.iwinv_security_group.one", "description", description),
			resource.TestCheckResourceAttr("data.iwinv_security_group.one", "allow_icmp", "true"),
			func(state *terraform.State) error {
				s := state.RootModule().Resources["data.iwinv_security_groups.all"]
				if s == nil || s.Primary == nil {
					return errors.New("list state missing")
				}
				attrs := s.Primary.Attributes
				for i := 0; ; i++ {
					id, ok := attrs[fmt.Sprintf("groups.%d.id", i)]
					if !ok {
						break
					}
					if id == created.ID {
						if attrs[fmt.Sprintf("ids.%d", i)] != id || attrs[fmt.Sprintf("groups.%d.name", i)] != wantName || attrs[fmt.Sprintf("groups.%d.description", i)] != description {
							return errors.New("list and detail disagree")
						}
						return nil
					}
				}
				return errors.New("owned fixture not found in complete list")
			},
		)
	}
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: groupFactories(readOnlyGroups{c}), Steps: []resource.TestStep{
		{Config: config, Check: check(name)},
		{Config: config, PlanOnly: true},
		{PreConfig: func() {
			if _, err := writes.UpdateGroup(ctx, created.ID, network.GroupInput{Name: name + "-u", AllowICMP: true}); err != nil {
				t.Fatal("fixture update failed")
			}
		}, Config: config, Check: check(name + "-u")},
		{Config: config, PlanOnly: true},
	}})
	if len(a.identities()) != 1 {
		t.Fatal("unexpected fixture count")
	}
}
