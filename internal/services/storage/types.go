// Package storage models verified block-storage catalog contracts. Catalog
// visibility does not establish permission to create or attach a volume.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type API interface {
	Get(context.Context, string, url.Values) (client.Envelope, error)
}
type Service struct{ API API }
type BlockStorageType struct {
	Type                         string
	MinimumSizeGB, MaximumSizeGB int64
	// nil means the API explicitly returned null, not all or no zones.
	AvailabilityZones []string
}

// Types makes a single unpaginated request. A nil filter omits type. An explicit
// empty filter is invalid; unknown nonempty codes are forwarded without guessing
// an enum. HTTP/API failures (including CHECK_PARAM) are never an empty catalog.
func (s *Service) Types(ctx context.Context, filter *string) ([]BlockStorageType, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	q := url.Values{}
	if filter != nil {
		if *filter == "" {
			return nil, errors.New("block storage type filter must be nonempty when specified")
		}
		q.Set("type", *filter)
	}
	e, err := s.API.Get(ctx, "/v1/block-storages/types", q)
	if err != nil {
		return nil, err
	}
	if e.Status != 200 || len(e.Page) != 0 || len(e.PageNo) != 0 || len(e.PageSize) != 0 || len(e.Total) != 0 {
		return nil, errors.New("block storage type response status or pagination contract changed")
	}
	var rows []struct {
		Type  *string         `json:"type"`
		Min   *int64          `json:"min"`
		Max   *int64          `json:"max"`
		Zones json.RawMessage `json:"zones"`
	}
	var count *int64
	if json.Unmarshal(e.Result, &rows) != nil || rows == nil || json.Unmarshal(e.Count, &count) != nil || count == nil || *count != int64(len(rows)) {
		return nil, errors.New("block storage types must be an array with a matching count")
	}
	out := make([]BlockStorageType, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Type == nil || *row.Type == "" || seen[*row.Type] || row.Min == nil || row.Max == nil || *row.Min < 0 || *row.Max < *row.Min {
			return nil, errors.New("block storage type identity or integer size bounds invalid")
		}
		if filter != nil && *row.Type != *filter {
			return nil, errors.New("block storage type response does not match the requested filter")
		}
		var zones []*string
		if json.Unmarshal(row.Zones, &zones) != nil {
			return nil, errors.New("block storage type zones must be present as null or an array of strings")
		}
		var names []string
		if zones != nil {
			names = make([]string, 0, len(zones))
			seenZone := map[string]bool{}
			for _, zone := range zones {
				if zone == nil || *zone == "" || seenZone[*zone] {
					return nil, errors.New("block storage type zones contain invalid or duplicate values")
				}
				seenZone[*zone] = true
				names = append(names, *zone)
			}
			sort.Strings(names)
		}
		seen[*row.Type] = true
		out = append(out, BlockStorageType{Type: *row.Type, MinimumSizeGB: *row.Min, MaximumSizeGB: *row.Max, AvailabilityZones: names})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out, nil
}
