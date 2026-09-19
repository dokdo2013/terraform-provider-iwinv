// Package network implements verified control-plane contracts independently of
// Terraform schemas. Groups, rules and attachments have separate ownership.
package network

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strconv"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type API interface {
	Get(context.Context, string, url.Values) (client.Envelope, error)
	PostJSON(context.Context, string, any) (client.Envelope, error)
	PutJSON(context.Context, string, any) (client.Envelope, error)
	Delete(context.Context, string) (client.Envelope, error)
}

type Service struct{ API API }

// Group deliberately excludes inline rules and attachments. Description nil
// represents an explicit API null; a missing response field is an error.
// Description is decoded once from the observed HTML-escaped response. Name is
// returned verbatim: the API does not apply the same escaping to it.
type Group struct {
	ID          string
	Name        string
	Description *string
	AllowICMP   bool
}

// GroupInput always supplies name and ICMP explicitly. A nil description omits
// content; it does not clear the remote value. Update omission is live-verified;
// create omission remains unresolved. Empty updates are rejected because
// the observed API does not reliably clear or preserve the previous description.
type GroupInput struct {
	Name        string
	Description *string
	AllowICMP   bool
}

// CreatedGroup retains a known identity even if the remaining response contract
// fails validation. Callers MUST persist ID before handling an accompanying error.
// A missing ID after a failed/ambiguous create never authorizes a create retry.
type CreatedGroup struct {
	ID    string
	Group *Group
}

var groupID = regexp.MustCompile(`^FIREWALL-[A-Za-z0-9_-]+$`)

// ValidateGroupID validates an import/reference without making a request.
func ValidateGroupID(id string) error {
	_, err := groupPath(id)
	return err
}

func groupPath(id string) (string, error) {
	if !groupID.MatchString(id) {
		return "", errors.New("security group ID must be one supported FIREWALL path segment")
	}
	return "/v1/security-groups/" + id, nil
}

func groupBody(in GroupInput, updating bool) (map[string]string, error) {
	if in.Name == "" {
		return nil, errors.New("security group name must not be empty")
	}
	if updating && in.Description != nil && *in.Description == "" {
		return nil, errors.New("security group description cannot be cleared with an empty update; the API does not reliably clear or preserve it")
	}
	b := map[string]string{"title": in.Name, "icmp": "N"}
	if in.AllowICMP {
		b["icmp"] = "Y"
	}
	if in.Description != nil {
		b["content"] = *in.Description
	}
	return b, nil
}

func groupRows(e client.Envelope, paginated bool) ([]Group, error) {
	if e.Status != 200 || len(e.Page) != 0 || len(e.Total) != 0 || (!paginated && (len(e.PageNo) != 0 || len(e.PageSize) != 0)) {
		return nil, errors.New("security group response status or pagination contract changed")
	}
	var rows []struct {
		ID          string          `json:"firewall_id"`
		Name        string          `json:"title"`
		Description json.RawMessage `json:"content"`
		ICMP        string          `json:"icmp"`
	}
	if json.Unmarshal(e.Result, &rows) != nil || rows == nil {
		return nil, errors.New("security group result must be an array")
	}
	var count *int
	if json.Unmarshal(e.Count, &count) != nil || count == nil || *count != len(rows) {
		return nil, errors.New("security group count is missing or inconsistent")
	}
	groups := make([]Group, 0, len(rows))
	seen := map[string]bool{}
	for _, r := range rows {
		var description *string
		if !groupID.MatchString(r.ID) || seen[r.ID] || r.Name == "" || (r.ICMP != "Y" && r.ICMP != "N") || json.Unmarshal(r.Description, &description) != nil {
			return nil, errors.New("security group has missing, invalid or duplicate fields")
		}
		seen[r.ID] = true
		if description != nil {
			decoded := html.UnescapeString(*description)
			description = &decoded
		}
		groups = append(groups, Group{ID: r.ID, Name: r.Name, Description: description, AllowICMP: r.ICMP == "Y"})
	}
	return groups, nil
}

const groupPageSize = 50
const groupMaxPages = 1000

func (s *Service) Groups(ctx context.Context) ([]Group, error) {
	groups := make([]Group, 0)
	seen := map[string]bool{}
	for page := 1; page <= groupMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e, err := s.API.Get(ctx, "/v1/security-groups", url.Values{"page_no": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(groupPageSize)}})
		if err != nil {
			return nil, err
		}
		rows, err := groupRows(e, true)
		if err != nil {
			return nil, err
		}
		var number, size *int
		if json.Unmarshal(e.PageNo, &number) != nil || json.Unmarshal(e.PageSize, &size) != nil || number == nil || size == nil || *number != page || *size != groupPageSize || len(rows) > *size {
			return nil, errors.New("security group list page metadata is missing or inconsistent")
		}
		for _, row := range rows {
			if seen[row.ID] {
				return nil, errors.New("security group list repeats an ID across pages")
			}
			seen[row.ID] = true
			groups = append(groups, row)
		}
		if len(rows) < *size {
			sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
			return groups, nil
		}
	}
	return nil, errors.New("security group list exceeded the bounded pagination limit")
}

// Group returns nil only for the observed endpoint-specific HTTP 200, empty
// array, count=0 contract. HTTP/API errors never mean absence. A create waiter
// must still account for visibility delay before deciding to discard identity.
func (s *Service) Group(ctx context.Context, id string) (*Group, error) {
	path, err := groupPath(id)
	if err != nil {
		return nil, err
	}
	e, err := s.API.Get(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	rows, err := groupRows(e, false)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 || rows[0].ID != id {
		return nil, errors.New("security group detail did not return exactly the requested ID")
	}
	return &rows[0], nil
}

func (s *Service) CreateGroup(ctx context.Context, in GroupInput) (CreatedGroup, error) {
	body, err := groupBody(in, false)
	if err != nil {
		return CreatedGroup{}, err
	}
	e, err := s.API.PostJSON(ctx, "/v1/security-groups", body)
	if err != nil {
		return CreatedGroup{}, err
	}
	if e.Status < 200 || e.Status > 202 {
		return CreatedGroup{}, errors.New("security group create returned an unsuccessful status")
	}
	var identities []struct {
		ID string `json:"firewall_id"`
	}
	if json.Unmarshal(e.Result, &identities) != nil || len(identities) != 1 || !groupID.MatchString(identities[0].ID) {
		return CreatedGroup{}, errors.New("security group create identity is unresolved; reconcile without retrying create")
	}
	created := CreatedGroup{ID: identities[0].ID}
	rows, err := groupRows(e, false)
	if err != nil {
		return created, err
	}
	created.Group = &rows[0]
	return created, nil
}

func (s *Service) UpdateGroup(ctx context.Context, id string, in GroupInput) (*Group, error) {
	path, err := groupPath(id)
	if err != nil {
		return nil, err
	}
	body, err := groupBody(in, true)
	if err != nil {
		return nil, err
	}
	e, err := s.API.PutJSON(ctx, path, body)
	if err != nil {
		return nil, err
	}
	rows, err := groupRows(e, false)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 || rows[0].ID != id {
		return nil, errors.New("security group update did not return exactly the requested ID")
	}
	return &rows[0], nil
}

// DeleteGroup makes one delete attempt and validates its acknowledgement. It
// does not establish completion: callers must subsequently verify Group absence.
func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	path, err := groupPath(id)
	if err != nil {
		return err
	}
	e, err := s.API.Delete(ctx, path)
	if err != nil {
		return err
	}
	var acknowledgement *string
	if e.Status != 200 || json.Unmarshal(e.Result, &acknowledgement) != nil || acknowledgement == nil {
		return errors.New("security group delete acknowledgement contract changed; verify remote state before another write")
	}
	return nil
}
