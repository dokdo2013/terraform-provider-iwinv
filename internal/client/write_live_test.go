package client_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// TestAccControlPlaneWrites is deliberately opt-in and is never enabled in CI.
// Only a group returned by this run's create is mutated. Persist the response
// and identity before further writes, including malformed recovery data.
func TestAccControlPlaneWrites(t *testing.T) {
	if os.Getenv("IWINV_LIVE_WRITE") != "1" {
		t.Skip("set IWINV_LIVE_WRITE=1 and a private IWINV_TEST_JOURNAL_DIR for a scoped live write test")
	}
	dir := os.Getenv("IWINV_TEST_JOURNAL_DIR")
	if !filepath.IsAbs(dir) {
		t.Fatal("an absolute private journal directory is required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal("cannot create private journal directory")
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("journal directory must have mode 0700")
	}
	f, err := os.CreateTemp(dir, "iwinv-write-*.json")
	if err != nil {
		t.Fatal("cannot create private cleanup journal")
	}
	journalPath := f.Name()
	if err := f.Close(); err != nil {
		t.Fatal("cannot close initial journal")
	}
	record := struct {
		CreateResponse json.RawMessage `json:"create_response,omitempty"`
		Name           string          `json:"requested_name,omitempty"`
		ID             string          `json:"created_id,omitempty"`
		Deleted        bool            `json:"deleted_verified"`
	}{}
	save := func() {
		t.Helper()
		b, e := json.Marshal(record)
		if e != nil {
			t.Fatal("cannot encode cleanup journal")
		}
		stage, e := os.CreateTemp(dir, ".iwinv-journal-stage-*")
		if e != nil {
			t.Fatal("cannot create journal staging file")
		}
		defer os.Remove(stage.Name())
		defer stage.Close()
		if _, e = stage.Write(b); e != nil {
			t.Fatal("cannot write cleanup journal")
		}
		if e = stage.Sync(); e != nil {
			t.Fatal("cannot sync cleanup journal")
		}
		if e = stage.Close(); e != nil {
			t.Fatal("cannot close cleanup journal")
		}
		if e = os.Rename(stage.Name(), journalPath); e != nil {
			t.Fatal("cannot atomically replace cleanup journal")
		}
	}
	save()
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("environment credentials required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	type group struct {
		ID          string `json:"firewall_id"`
		Name        string `json:"title"`
		Description string `json:"content"`
		ICMP        string `json:"icmp"`
	}
	before, err := c.Get(ctx, "/v1/security-groups", nil)
	if err != nil {
		t.Fatal(err)
	}
	var baseline []group
	if before.Status != 200 || json.Unmarshal(before.Result, &baseline) != nil || baseline == nil {
		t.Fatal("invalid baseline group inventory")
	}
	name := "tf-go-" + time.Now().UTC().Format("20060102T150405.000000000")
	record.Name = name
	save()
	created, err := c.PostJSON(ctx, "/v1/security-groups", map[string]string{"title": name, "content": "Go client contract test", "icmp": "N"})
	if err != nil {
		t.Fatal(err)
	}
	record.CreateResponse = created.Result
	var rows []group
	if json.Unmarshal(created.Result, &rows) != nil || len(rows) != 1 || !regexp.MustCompile(`^FIREWALL-[A-Za-z0-9_-]+$`).MatchString(rows[0].ID) {
		save()
		t.Fatal("create identity unresolved; inspect private journal without retrying create")
	}
	id := rows[0].ID
	for _, existing := range baseline {
		if existing.ID == id {
			save()
			t.Fatal("create returned a pre-existing ID; refusing to mutate it")
		}
	}
	record.ID = id
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		_, deleteErr := c.Delete(cleanupCtx, "/v1/security-groups/"+id)
		if deleteErr != nil {
			t.Error("delete failed; reconcile private cleanup journal")
		}
		for attempt := 0; attempt < 10; attempt++ {
			result, readErr := c.Get(cleanupCtx, "/v1/security-groups/"+id, nil)
			var remaining []group
			if readErr == nil && result.Status == 200 && json.Unmarshal(result.Result, &remaining) == nil && remaining != nil && len(remaining) == 0 && string(result.Count) == "0" {
				record.Deleted = true
				save()
				return
			}
			if cleanupCtx.Err() != nil {
				break
			}
		}
		t.Error("created group absence not verified; inspect private cleanup journal")
	}()
	save()
	updated, err := c.PutJSON(ctx, "/v1/security-groups/"+id, map[string]string{"title": name, "content": "Updated Go contract", "icmp": "Y"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != 200 {
		t.Fatal("unexpected update HTTP status")
	}
	detail, err := c.Get(ctx, "/v1/security-groups/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(detail.Result, &rows) != nil || len(rows) != 1 || rows[0].ID != id || rows[0].Name != name || rows[0].Description != "Updated Go contract" || rows[0].ICMP != "Y" {
		t.Fatal("updated fields did not round-trip")
	}

	// Observed endpoint behavior: an explicitly empty content is ignored, not
	// a clear operation. A future resource must not claim that it cleared it.
	if _, err := c.PutJSON(ctx, "/v1/security-groups/"+id, map[string]string{"title": name, "content": "", "icmp": "Y"}); err != nil {
		t.Fatal(err)
	}
	detail, err = c.Get(ctx, "/v1/security-groups/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(detail.Result, &rows) != nil || len(rows) != 1 || rows[0].ID != id || rows[0].Description != "Updated Go contract" {
		t.Fatal("empty-description update contract changed")
	}
}
