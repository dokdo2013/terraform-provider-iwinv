package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
)

type ruleCaptureClient struct {
	*client.Client
	capture func(string, client.Envelope) error
}

func (c *ruleCaptureClient) PostJSON(ctx context.Context, path string, body any) (client.Envelope, error) {
	e, err := c.Client.PostJSON(ctx, path, body)
	if captureErr := c.capture(path, e); captureErr != nil {
		return client.Envelope{}, captureErr
	}
	return e, err
}

// TestAccRuleControlPlaneWrites is an opt-in contract test, not Terraform
// resource acceptance. The parent and every known rule ID are journaled before
// further writes; cleanup is installed before the first create request.
func TestAccRuleControlPlaneWrites(t *testing.T) {
	if os.Getenv("IWINV_LIVE_RULE_WRITE") != "1" {
		t.Skip("requires IWINV_LIVE_RULE_WRITE=1 and private IWINV_TEST_JOURNAL_DIR")
	}
	dir := os.Getenv("IWINV_TEST_JOURNAL_DIR")
	if !filepath.IsAbs(dir) {
		t.Fatal("absolute private journal directory required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal("cannot create journal directory")
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatal("real mode-0700 journal directory required")
	}
	f, err := os.CreateTemp(dir, "rule-write-*.json")
	if err != nil {
		t.Fatal("cannot create journal")
	}
	journal := f.Name()
	if err = f.Close(); err != nil {
		t.Fatal("cannot close journal")
	}
	type ruleRecord struct {
		ID      string `json:"created_id"`
		Deleted bool   `json:"deleted_verified"`
	}
	record := struct {
		ParentID      string            `json:"parent_id,omitempty"`
		ParentDeleted bool              `json:"parent_deleted_verified"`
		Rules         []*ruleRecord     `json:"rules"`
		Receipts      []json.RawMessage `json:"create_receipts"`
	}{Rules: []*ruleRecord{}, Receipts: []json.RawMessage{}}
	save := func() error {
		b, err := json.Marshal(record)
		if err != nil {
			return errors.New("cannot encode private journal")
		}
		f, err := os.CreateTemp(dir, ".rule-stage-*")
		if err != nil {
			return errors.New("cannot stage private journal")
		}
		defer os.Remove(f.Name())
		defer f.Close()
		if _, err = f.Write(b); err == nil {
			err = f.Sync()
		}
		if err != nil {
			return errors.New("cannot persist private journal")
		}
		if err = f.Close(); err != nil {
			return errors.New("cannot close private journal")
		}
		if err = os.Rename(f.Name(), journal); err != nil {
			return errors.New("cannot replace private journal")
		}
		return nil
	}
	if err = save(); err != nil {
		t.Fatal(err)
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("environment credentials required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	plain := network.Service{API: c}
	baseline, err := plain.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	existing := map[string]bool{}
	for _, g := range baseline {
		existing[g.ID] = true
	}
	captured := &ruleCaptureClient{Client: c, capture: func(path string, e client.Envelope) error {
		record.Receipts = append(record.Receipts, e.Result)
		if e.Status >= 200 && e.Status <= 202 {
			if path == "/v1/security-groups" {
				var ids []struct {
					ID string `json:"firewall_id"`
				}
				if json.Unmarshal(e.Result, &ids) == nil && len(ids) == 1 && network.ValidateGroupID(ids[0].ID) == nil && !existing[ids[0].ID] {
					record.ParentID = ids[0].ID
				}
			} else if record.ParentID != "" && path == "/v1/security-groups/"+record.ParentID+"/rules" {
				var ids []struct {
					ID *int64 `json:"rule_id"`
				}
				if json.Unmarshal(e.Result, &ids) == nil && len(ids) == 1 && ids[0].ID != nil && *ids[0].ID > 0 {
					id := strconv.FormatInt(*ids[0].ID, 10)
					duplicate := false
					for _, r := range record.Rules {
						if r.ID == id {
							duplicate = true
						}
					}
					if !duplicate {
						record.Rules = append(record.Rules, &ruleRecord{ID: id})
					}
				}
			}
		}
		return save()
	}}
	service := network.Service{API: captured}
	defer func() {
		if record.ParentID == "" {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		// Rule absence is checked while the parent still exists, before removing it.
		for _, entry := range record.Rules {
			r, err := service.Rule(cleanupCtx, record.ParentID, entry.ID)
			if err != nil {
				t.Error("rule cleanup read failed; inspect private journal")
				continue
			}
			if r != nil {
				if err = service.DeleteRule(cleanupCtx, record.ParentID, entry.ID); err != nil {
					t.Error("rule delete failed; inspect private journal")
				}
			}
			for attempt := 0; attempt < 10; attempt++ {
				remaining, err := service.Rule(cleanupCtx, record.ParentID, entry.ID)
				if err != nil {
					t.Error("rule cleanup verification failed")
					break
				}
				if remaining == nil {
					entry.Deleted = true
					break
				}
			}
			if !entry.Deleted {
				t.Error("rule absence remains unverified")
			}
		}
		if err = save(); err != nil {
			t.Error(err)
		}
		if err = service.DeleteGroup(cleanupCtx, record.ParentID); err != nil {
			t.Error("parent delete failed; inspect private journal")
		}
		remaining, err := service.Group(cleanupCtx, record.ParentID)
		record.ParentDeleted = err == nil && remaining == nil
		if !record.ParentDeleted {
			t.Error("parent absence remains unverified")
		}
		if err = save(); err != nil {
			t.Error(err)
		}
	}()
	desc := "Rule adapter contract"
	group, err := service.CreateGroup(ctx, network.GroupInput{Name: "tf-rule-go-" + time.Now().UTC().Format("150405.000000000"), Description: &desc})
	if err != nil || group.ID == "" || group.ID != record.ParentID {
		t.Fatal("new parent identity unresolved; inspect private journal")
	}
	initial, err := service.Rules(ctx, group.ID)
	if err != nil || len(initial) != 0 {
		t.Fatal("new parent rules are not empty")
	}
	description := "한글 &amp; <rule>"
	in := network.RuleInput{Bound: "IN", Protocol: "TCP", Port: "8443", IP: "192.0.2.1/32", Name: "tf-rule", Description: &description}
	first, err := service.CreateRule(ctx, group.ID, in)
	if err != nil || first.ID == "" || first.Rule == nil || first.Rule.Description == nil || *first.Rule.Description != description {
		t.Fatal("first rule create or verbatim description failed")
	}
	empty := ""
	secondInput := in
	secondInput.Port = "8444"
	secondInput.Description = &empty
	second, err := service.CreateRule(ctx, group.ID, secondInput)
	if err != nil || second.ID == "" || second.ID == first.ID || second.Rule == nil || second.Rule.Description != nil {
		t.Fatal("empty-create/null-description contract failed")
	}
	all, err := service.Rules(ctx, group.ID)
	if err != nil || len(all) != 2 {
		t.Fatal("complete rule listing failed")
	}
	read, err := service.Rule(ctx, group.ID, first.ID)
	if err != nil || read == nil || read.Description == nil || *read.Description != description {
		t.Fatal("exact rule read failed")
	}
	description = "수정 &amp; <x>"
	in = network.RuleInput{Bound: "OUT", Protocol: "UDP", Port: "8445-8447", IP: "192.0.2.1/24", Name: "tf 한글 &amp; <x>", Description: &description}
	updated, err := service.UpdateRule(ctx, group.ID, first.ID, in)
	if err != nil || updated == nil || updated.ID != first.ID {
		t.Fatal("full update failed")
	}
	read, err = service.Rule(ctx, group.ID, first.ID)
	if err != nil || read == nil || read.Bound != in.Bound || read.Protocol != in.Protocol || read.Port != in.Port || read.IP != in.IP || read.Name != in.Name || read.Description == nil || *read.Description != description {
		t.Fatal("updated rule fields did not round-trip")
	}
	in.Description = nil
	in.Name = "tf-renamed"
	if _, err = service.UpdateRule(ctx, group.ID, first.ID, in); err != nil {
		t.Fatal(err)
	}
	read, err = service.Rule(ctx, group.ID, first.ID)
	if err != nil || read == nil || read.Description == nil || *read.Description != description {
		t.Fatal("omitted description changed")
	}
	in.Description = &empty
	if _, err = service.UpdateRule(ctx, group.ID, first.ID, in); err == nil || !strings.Contains(err.Error(), "cannot be cleared") {
		t.Fatal("unsupported clear accepted")
	}
	if err = service.DeleteRule(ctx, group.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	read, err = service.Rule(ctx, group.ID, first.ID)
	if err != nil || read != nil {
		t.Fatal("deleted rule remained visible")
	}
	read, err = service.Rule(ctx, group.ID, second.ID)
	if err != nil || read == nil {
		t.Fatal("independent rule was removed")
	}
}
