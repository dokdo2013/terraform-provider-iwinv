package compute

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type API interface {
	Get(context.Context, string, url.Values) (client.Envelope, error)
}

type Service struct{ API API }

type Zone struct {
	ID     string `json:"zone_id"`
	Name   string `json:"zone_name"`
	Status string `json:"status"`
}

// Zones decodes only the observed, non-paginated zone catalog. No account-wide
// visibility guarantee is inferred from this list.
func (s *Service) Zones(ctx context.Context) ([]Zone, error) {
	envelope, err := s.API.Get(ctx, "/v1/zones", nil)
	if err != nil {
		return nil, err
	}
	if envelope.Status != 200 {
		return nil, errors.New("zone list returned an unexpected HTTP status")
	}
	var zones []Zone
	if json.Unmarshal(envelope.Result, &zones) != nil || zones == nil {
		return nil, errors.New("zone list must be an array")
	}
	var count *int
	if json.Unmarshal(envelope.Count, &count) != nil || count == nil || *count != len(zones) {
		return nil, errors.New("zone list count is missing or inconsistent")
	}
	seen := map[string]bool{}
	for _, zone := range zones {
		if zone.ID == "" || zone.Name == "" || zone.Status == "" || seen[zone.ID] {
			return nil, errors.New("zone list has missing fields or duplicate IDs")
		}
		seen[zone.ID] = true
	}
	return zones, nil
}
