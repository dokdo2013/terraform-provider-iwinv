package compute

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// Documentation-derived projection, pending populated live-instance validation.
// https://api-kr.iwinv.kr/fields/v1/instances assigns these seven bits.
// default_account (128) and vnc (16384) must never be requested here.
const instanceReadFields = 1 | 2 | 4 | 8 | 512 | 1024 | 2048
const instancePageSize = 10
const instanceMaxPages = 1000

// Instance contains only the selected control-plane attributes. It deliberately
// has no raw response, default account, console URL, or inferred primary NIC.
// Description preserves API null versus an explicitly empty string.
type Instance struct {
	ID          string  `json:"instance_id"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Status      string  `json:"status"`
	ZoneID      string  `json:"zone_id"`
	FlavorID    string  `json:"flavor_id"`
	ImageID     string  `json:"image_id"`
}

func instanceRows(e client.Envelope) ([]Instance, error) {
	if e.Status != 200 {
		return nil, errors.New("instance read returned an unexpected HTTP status")
	}
	var wire []struct {
		ID          string          `json:"instance_id"`
		Name        *string         `json:"name"`
		Description json.RawMessage `json:"description"`
		Status      string          `json:"status"`
		Zone        struct {
			ID string `json:"zone_id"`
		} `json:"zone"`
		Flavor struct {
			ID string `json:"flavor_id"`
		} `json:"flavor"`
		Image struct {
			ID string `json:"image_id"`
		} `json:"image"`
	}
	if json.Unmarshal(e.Result, &wire) != nil || wire == nil {
		return nil, errors.New("instance result must be a valid array")
	}
	count, err := integer(e.Count)
	if err != nil || count != len(wire) {
		return nil, errors.New("instance count is missing or inconsistent")
	}
	rows := make([]Instance, 0, len(wire))
	for _, w := range wire {
		if !catalogID.MatchString(w.ID) || w.Name == nil || w.Status == "" ||
			!catalogID.MatchString(w.Zone.ID) || !catalogID.MatchString(w.Flavor.ID) || !catalogID.MatchString(w.Image.ID) {
			return nil, errors.New("instance is missing selected attributes or has unsupported identifiers")
		}
		var description *string
		if len(w.Description) == 0 || json.Unmarshal(w.Description, &description) != nil {
			return nil, errors.New("instance description must be a present string or null")
		}
		rows = append(rows, Instance{ID: w.ID, Name: *w.Name, Description: description,
			Status: w.Status, ZoneID: w.Zone.ID, FlavorID: w.Flavor.ID, ImageID: w.Image.ID})
	}
	return rows, nil
}

// Instances traverses the API-visible list only. The vendor explicitly excludes
// instances outside API-supported zones, so absence here is NOT proof of remote
// deletion. No filters or caller-supplied fields can widen the secret-free mask.
// There is no documented snapshot token or total; a short page ends traversal.
func (s *Service) Instances(ctx context.Context) ([]Instance, error) {
	rows := make([]Instance, 0)
	seen := map[string]bool{}
	for page := 1; page <= instanceMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e, err := s.API.Get(ctx, "/v1/instances", url.Values{
			"fields":  {strconv.Itoa(instanceReadFields)},
			"page_no": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(instancePageSize)},
		})
		if err != nil {
			return nil, err
		}
		batch, err := instanceRows(e)
		if err != nil {
			return nil, err
		}
		n, nerr := integer(e.PageNo)
		size, serr := integer(e.PageSize)
		if nerr != nil || serr != nil || n != page || size != instancePageSize || len(batch) > size {
			return nil, errors.New("instance pagination metadata is inconsistent")
		}
		for _, row := range batch {
			if seen[row.ID] {
				return nil, errors.New("instance list contains duplicate identifiers")
			}
			seen[row.ID] = true
			rows = append(rows, row)
		}
		if len(batch) < size {
			sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
			return rows, nil
		}
	}
	return nil, errors.New("instance list exceeded the bounded pagination limit")
}

// Instance requires exactly one matching ID. Until absence semantics are live
// verified, empty results and all API errors stay errors; no not-found sentinel
// is exposed for a future resource Read to mistakenly remove Terraform state.
func (s *Service) Instance(ctx context.Context, id string) (Instance, error) {
	if !catalogID.MatchString(id) {
		return Instance{}, errors.New("instance ID must be a single supported path segment")
	}
	if err := ctx.Err(); err != nil {
		return Instance{}, err
	}
	e, err := s.API.Get(ctx, "/v1/instances/"+id, url.Values{"fields": {strconv.Itoa(instanceReadFields)}})
	if err != nil {
		return Instance{}, err
	}
	rows, err := instanceRows(e)
	if err != nil {
		return Instance{}, err
	}
	if len(rows) != 1 || rows[0].ID != id {
		return Instance{}, errors.New("instance lookup did not return exactly one matching identifier")
	}
	return rows[0], nil
}
