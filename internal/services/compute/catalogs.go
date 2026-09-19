package compute

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// CatalogKind is deliberately closed: callers cannot choose arbitrary API paths.
type CatalogKind string

const (
	Images          CatalogKind = "images"
	InstanceTypes   CatalogKind = "flavors"
	catalogPageSize             = 10
	catalogMaxPages             = 1000
)

// CatalogItem contains only fields shared by the observed public catalog reads.
// Rich image metadata and product specifications need separate schema contracts.
type CatalogItem struct {
	ImageID    string `json:"image_id"`
	FlavorID   string `json:"flavor_id"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	ImageType  string `json:"image_type"`
}

func (r CatalogItem) ID(kind CatalogKind) string {
	if kind == Images {
		return r.ImageID
	}
	return r.FlavorID
}
func catalogPath(kind CatalogKind) (string, error) {
	if kind != Images && kind != InstanceTypes {
		return "", errors.New("unsupported catalog")
	}
	return "/v1/" + string(kind), nil
}

var catalogID = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.-]*$`)

func integer(raw json.RawMessage) (int, error) {
	var n *int
	if json.Unmarshal(raw, &n) != nil || n == nil || *n < 0 {
		return 0, errors.New("catalog metadata must be a nonnegative integer")
	}
	return *n, nil
}
func catalogRows(e client.Envelope, kind CatalogKind) ([]CatalogItem, error) {
	if e.Status != 200 {
		return nil, errors.New("catalog returned an unexpected HTTP status")
	}
	var rows []CatalogItem
	if json.Unmarshal(e.Result, &rows) != nil || rows == nil {
		return nil, errors.New("catalog result must be an array")
	}
	count, err := integer(e.Count)
	if err != nil || count != len(rows) {
		return nil, errors.New("catalog count is missing or inconsistent")
	}
	for _, r := range rows {
		if !catalogID.MatchString(r.ID(kind)) {
			return nil, errors.New("catalog contains a missing or unsupported ID")
		}
	}
	return rows, nil
}

// CatalogIDs traverses the complete observed pagination contract. A changed
// total, duplicate ID, malformed page or late failure never yields partial state.
// These APIs offer no snapshot token, so concurrent catalog changes remain a risk.
func (s *Service) CatalogIDs(ctx context.Context, kind CatalogKind) ([]string, error) {
	path, err := catalogPath(kind)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	seen := map[string]bool{}
	total := -1
	for page := 1; page <= catalogMaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e, err := s.API.Get(ctx, path, url.Values{"page_no": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(catalogPageSize)}})
		if err != nil {
			return nil, err
		}
		rows, err := catalogRows(e, kind)
		if err != nil {
			return nil, err
		}
		n, nerr := integer(e.PageNo)
		size, serr := integer(e.PageSize)
		if nerr != nil || serr != nil || n != page || size != catalogPageSize || len(rows) > size {
			return nil, errors.New("catalog pagination metadata is inconsistent")
		}
		if kind == InstanceTypes {
			t, err := integer(e.Total)
			if err != nil || (total >= 0 && total != t) {
				return nil, errors.New("catalog total is missing or changed during pagination")
			}
			total = t
		}
		for _, r := range rows {
			id := r.ID(kind)
			if seen[id] {
				return nil, errors.New("catalog has duplicate IDs across pages")
			}
			seen[id] = true
			ids = append(ids, id)
		}
		if total >= 0 && (len(ids) > total || (len(rows) < size && len(ids) != total)) {
			return nil, errors.New("catalog total does not match collected rows")
		}
		if len(rows) < size || (total >= 0 && len(ids) == total) {
			sort.Strings(ids)
			return ids, nil
		}
	}
	return nil, errors.New("catalog exceeded the bounded pagination limit")
}

// CatalogItem requires exactly one matching result. In particular, a successful
// empty flavor lookup and an image ID_INVALID response are both lookup failures.
func (s *Service) CatalogItem(ctx context.Context, kind CatalogKind, id string) (CatalogItem, error) {
	path, err := catalogPath(kind)
	if err != nil {
		return CatalogItem{}, err
	}
	if !catalogID.MatchString(id) {
		return CatalogItem{}, errors.New("catalog ID must be a single supported path segment")
	}
	e, err := s.API.Get(ctx, path+"/"+id, nil)
	if err != nil {
		return CatalogItem{}, err
	}
	rows, err := catalogRows(e, kind)
	if err != nil {
		return CatalogItem{}, err
	}
	if len(rows) != 1 || rows[0].ID(kind) != id {
		return CatalogItem{}, errors.New("catalog lookup did not return exactly one matching ID")
	}
	r := rows[0]
	if (kind == Images && (r.Visibility == "" || r.ImageType == "")) || (kind == InstanceTypes && r.Name == "") {
		return CatalogItem{}, errors.New("catalog detail is missing required fields")
	}
	return r, nil
}
