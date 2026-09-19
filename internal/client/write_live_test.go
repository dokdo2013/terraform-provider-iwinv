package client_test

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
	"github.com/dokdo2013/terraform-provider-iwinv/internal/services/network"
)

// journalWriteClient captures the raw create receipt before adapter validation.
// The test registers ID-based cleanup before persisting it, so journal I/O
// failure after creation still runs cleanup. Recovery files stay outside Git.
type journalWriteClient struct {
	*client.Client
	capture     func(client.Envelope)
	captureList func(client.Envelope)
}

func (c *journalWriteClient) Get(ctx context.Context, path string, q url.Values) (client.Envelope, error) {
	e, err := c.Client.Get(ctx, path, q)
	if err == nil && path == "/v1/security-groups" {
		c.captureList(e)
	}
	return e, err
}

func (c *journalWriteClient) PostJSON(ctx context.Context, path string, body any) (client.Envelope, error) {
	e, err := c.Client.PostJSON(ctx, path, body)
	if err == nil {
		c.capture(e)
	}
	return e, err
}

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
		ListResponse   json.RawMessage `json:"list_response,omitempty"`
		CreateError    string          `json:"create_error,omitempty"`
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
	service := network.Service{API: &journalWriteClient{Client: c, capture: func(e client.Envelope) { record.CreateResponse = e.Result }, captureList: func(e client.Envelope) { record.ListResponse = e.Result }}}
	baseline, err := service.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	name := "tf-net-" + time.Now().UTC().Format("150405.000000000")
	record.Name = name
	save()
	// Use the previously verified explicit-description create contract.
	initialDescription := "Go &amp; <test> +"
	created, createErr := service.CreateGroup(ctx, network.GroupInput{Name: name, Description: &initialDescription, AllowICMP: false})
	if createErr != nil {
		record.CreateError = createErr.Error()
	}
	if created.ID == "" {
		save()
		t.Fatalf("create identity unresolved; inspect private journal without retrying create: %v", createErr)
	}
	id := created.ID
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
		if err := service.DeleteGroup(cleanupCtx, id); err != nil {
			t.Error("delete failed; reconcile private cleanup journal")
		}
		for attempt := 0; attempt < 10; attempt++ {
			remaining, readErr := service.Group(cleanupCtx, id)
			if readErr == nil && remaining == nil {
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
	if createErr != nil {
		t.Fatal(createErr)
	}
	if created.Group == nil || created.Group.Description == nil || *created.Group.Description != initialDescription {
		t.Fatal("create description escaping contract changed")
	}
	detail, err := service.Group(ctx, id)
	if err != nil || detail == nil || detail.Name != name || detail.AllowICMP {
		t.Fatal("created fields did not round-trip")
	}
	inventory, err := service.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range inventory {
		if g.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("created group missing from validated inventory")
	}
	description := "Updated Go 계약 & + %"
	if _, err := service.UpdateGroup(ctx, id, network.GroupInput{Name: name, Description: &description, AllowICMP: true}); err != nil {
		t.Fatal(err)
	}
	detail, err = service.Group(ctx, id)
	if err != nil || detail == nil || detail.Name != name || detail.Description == nil || *detail.Description != description || !detail.AllowICMP {
		t.Fatal("updated fields did not round-trip")
	}

	// The adapter must reject an unsupported clear without changing the remote
	// description. Raw empty updates have also shown non-preserving behavior for
	// HTML-escaped text; do not use them as a harmless no-op.
	emptyDescription := ""
	if _, err := service.UpdateGroup(ctx, id, network.GroupInput{Name: name, Description: &emptyDescription, AllowICMP: true}); err == nil {
		t.Fatal("unsupported clear was accepted")
	}
	detail, err = service.Group(ctx, id)
	if err != nil || detail == nil || detail.Description == nil || *detail.Description != description {
		t.Fatal("rejected clear changed remote description")
	}

	// Omission is distinct from clearing: name/ICMP can change while preserving
	// description. Verify the adapter's response and a separate authoritative Read.
	updated, err := service.UpdateGroup(ctx, id, network.GroupInput{Name: name + "-u", AllowICMP: false})
	if err != nil || updated == nil || updated.Description == nil || *updated.Description != description {
		t.Fatal("omitted-description update contract changed")
	}
	detail, err = service.Group(ctx, id)
	if err != nil || detail == nil || detail.Name != name+"-u" || detail.AllowICMP || detail.Description == nil || *detail.Description != description {
		t.Fatal("omitted fields or rename failed read-back")
	}
}
