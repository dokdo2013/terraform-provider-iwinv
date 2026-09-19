package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
)

type WebmailCatalogService struct{ API HostingCatalogAPI }

// WebmailProduct is catalog metadata, not a service or mailbox identity. Empty
// product IDs in coming-soon rows are preserved. Price, disk and traffic are
// excluded until their units and billing semantics have independent evidence.
type WebmailProduct struct{ ID, Name, Status, Type string }

func (s *WebmailCatalogService) Products(ctx context.Context) ([]WebmailProduct, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e, err := s.API.Get(ctx, "/v1/webmail/products", nil)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]WebmailProduct, 0, len(rows))
	ids := map[string]bool{}
	unselectable := map[[2]string]bool{}
	for _, raw := range rows {
		var r struct {
			ID     *string `json:"product_id"`
			Name   string  `json:"product_name"`
			Status string  `json:"status"`
			Spec   struct {
				Type string `json:"type"`
			} `json:"spec"`
		}
		if json.Unmarshal(raw, &r) != nil || r.ID == nil || r.Name == "" || r.Status == "" || r.Spec.Type == "" {
			return nil, errors.New("webmail product fields are missing or invalid")
		}
		if *r.ID != "" {
			if ids[*r.ID] {
				return nil, errors.New("webmail catalog has duplicate product IDs")
			}
			ids[*r.ID] = true
		} else {
			key := [2]string{r.Spec.Type, r.Name}
			if unselectable[key] {
				return nil, errors.New("webmail catalog has ambiguous empty-ID rows")
			}
			unselectable[key] = true
		}
		out = append(out, WebmailProduct{*r.ID, r.Name, r.Status, r.Spec.Type})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Name < b.Name
	})
	return out, nil
}
