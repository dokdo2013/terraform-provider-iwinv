package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type CacheAPI interface {
	HostingCatalogAPI
	PostJSON(context.Context, string, any) (client.Envelope, error)
	PutJSON(context.Context, string, any) (client.Envelope, error)
	Delete(context.Context, string) (client.Envelope, error)
}
type CacheService struct{ API CacheAPI }
type CacheCatalogService struct{ API HostingCatalogAPI }

// Null product IDs are real coming-soon rows. They remain distinguishable from
// empty strings; neither can be used for creation. Unverified units are omitted.
type CacheProduct struct {
	ID                 *string
	Name, Status, Type string
}
type Cache struct {
	ID, ProductID, Name, Account, Status, IP, Type, Domain string
	Description                                            *string
	Referrers                                              []string
}
type CacheInput struct {
	ProductID, Name, Account string
	Description              *string
	FTPPassword              string `json:"-"`
}
type CreatedCache struct {
	ID      string
	Service *Cache
}

func (s *CacheCatalogService) Products(ctx context.Context, kind string) ([]CacheProduct, error) {
	if kind != "" && kind != "SHARE" && kind != "SINGLE" {
		return nil, errors.New("cache product type must be SHARE or SINGLE when supplied")
	}
	q := url.Values{}
	if kind != "" {
		q.Set("type", kind)
	}
	e, err := s.API.Get(ctx, "/v1/cache/products", q)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	result := make([]CacheProduct, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		var r struct {
			ID     json.RawMessage `json:"product_id"`
			Name   string          `json:"product_name"`
			Status string          `json:"status"`
			Spec   struct {
				Type string `json:"type"`
			} `json:"spec"`
		}
		var id *string
		if json.Unmarshal(raw, &r) != nil || json.Unmarshal(r.ID, &id) != nil || r.Name == "" || r.Status == "" || r.Spec.Type == "" || kind != "" && r.Spec.Type != kind {
			return nil, errors.New("cache catalog fields or type filter contract invalid")
		}
		// A null/empty ID cannot identify a product by itself. Preserve distinct
		// named rows; reject duplicate names within that unselectable tier.
		key := "unselectable:" + r.Spec.Type + ":" + r.Name
		if id != nil && *id != "" {
			key = "id:" + *id
		}
		if seen[key] {
			return nil, errors.New("cache catalog has duplicate identities")
		}
		seen[key] = true
		result = append(result, CacheProduct{id, r.Name, r.Status, r.Spec.Type})
	}
	return result, nil
}

var cacheAccount = regexp.MustCompile(`^[A-Za-z0-9]{6,12}$`)
var cacheDNSLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidateCacheReferrers supports exact lowercase ASCII DNS hostnames only.
// It does not infer wildcard, URL, IP, port, IDN or empty-clearing behavior.
func ValidateCacheReferrers(refs []string) error {
	if len(refs) == 0 {
		return errors.New("cache referrers must be a nonempty set; empty clearing is unsupported")
	}
	seen := map[string]bool{}
	for _, ref := range refs {
		labels := strings.Split(ref, ".")
		_, ipErr := netip.ParseAddr(ref)
		if len(ref) > 253 || len(labels) < 2 || ipErr == nil || seen[ref] {
			return errors.New("cache referrers must be distinct lowercase DNS hostnames without URLs, wildcards, IPs or ports")
		}
		for _, label := range labels {
			if !cacheDNSLabel.MatchString(label) {
				return errors.New("cache referrer hostname syntax is unsupported")
			}
		}
		seen[ref] = true
	}
	return nil
}
func ValidateCachePassword(password string) error {
	if !validHostingPassword(password) {
		return errors.New("cache FTP password must contain 7–20 non-space printable ASCII characters from at least two of letters, digits and symbols")
	}
	return nil
}
func decodeCache(raw json.RawMessage, receipt bool) (*Cache, error) {
	var r struct {
		ID          json.RawMessage `json:"service_idx"`
		ProductID   string          `json:"product_id"`
		Name        string          `json:"name"`
		Account     string          `json:"id"`
		Status      string          `json:"status"`
		Description json.RawMessage `json:"description"`
		IP          string          `json:"ip"`
		Domain      string          `json:"domain"`
		Spec        struct {
			Type string `json:"type"`
		} `json:"spec"`
		Referrers []string `json:"allow_referer"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return nil, errors.New("invalid cache service object")
	}
	id, err := serviceID(r.ID)
	if err != nil {
		return nil, err
	}
	var description *string
	if !receipt && r.ProductID == "" || r.Name == "" || r.Account == "" || r.Status == "" || r.Domain == "" || r.Spec.Type == "" || json.Unmarshal(r.Description, &description) != nil || r.Referrers == nil {
		return nil, errors.New("cache service has missing or invalid required fields")
	}
	if _, err = netip.ParseAddr(r.IP); err != nil {
		return nil, errors.New("cache service IP is invalid")
	}
	if len(r.Referrers) > 0 && ValidateCacheReferrers(r.Referrers) != nil {
		return nil, errors.New("cache referrer read contract is unsupported")
	}
	sort.Strings(r.Referrers)
	return &Cache{ID: id, ProductID: r.ProductID, Name: r.Name, Account: r.Account, Status: r.Status, IP: r.IP, Type: r.Spec.Type, Domain: r.Domain, Description: description, Referrers: r.Referrers}, nil
}
func (s *CacheService) List(ctx context.Context) ([]Cache, error) {
	e, err := s.API.Get(ctx, "/v1/cache", nil)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]Cache, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		r, err := decodeCache(raw, false)
		if err != nil {
			return nil, err
		}
		if seen[r.ID] {
			return nil, errors.New("cache list contains duplicate identities")
		}
		seen[r.ID] = true
		out = append(out, *r)
	}
	return out, nil
}
func (s *CacheService) Read(ctx context.Context, id string) (*Cache, error) {
	if err := ValidateServiceID(id); err != nil {
		return nil, err
	}
	rows, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, nil
}
func (s *CacheService) Create(ctx context.Context, in CacheInput) (CreatedCache, error) {
	if in.ProductID == "" || !cacheAccount.MatchString(in.Account) || utf8.RuneCountInString(in.Name) < 4 || utf8.RuneCountInString(in.Name) > 32 {
		return CreatedCache{}, errors.New("cache create requires a product ID, 4–32 character name and 6–12 ASCII alphanumeric account")
	}
	if in.Description != nil && (*in.Description == "" || utf8.RuneCountInString(*in.Description) > 50) {
		return CreatedCache{}, errors.New("cache description must contain 1–50 characters when supplied; omit empty description")
	}
	if err := ValidateCachePassword(in.FTPPassword); err != nil {
		return CreatedCache{}, err
	}
	// ftppw is contradicted by the live validation contract. Referrers are not
	// included: create-time allow_referer was silently ignored in live tests.
	body := map[string]any{"product_id": in.ProductID, "name": in.Name, "id": in.Account, "pw": map[string]string{"FTP": in.FTPPassword}}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	e, err := s.API.PostJSON(ctx, "/v1/cache", body)
	if err != nil {
		return CreatedCache{}, err
	}
	id, err := CreateID(e)
	out := CreatedCache{ID: id}
	if err != nil {
		return out, err
	}
	if dbmsMetadata(e) {
		return out, errors.New("cache create metadata contract changed")
	}
	out.Service, err = decodeCache(e.Result, true)
	return out, err
}

// ReplaceReferrers performs one attempt; even the narrowly classified busy
// rejection is returned to the caller. Accepted writes are never replayed.
func (s *CacheService) ReplaceReferrers(ctx context.Context, id string, refs []string) error {
	if err := ValidateServiceID(id); err != nil {
		return err
	}
	if err := ValidateCacheReferrers(refs); err != nil {
		return err
	}
	e, err := s.API.PutJSON(ctx, "/v1/cache/"+id+"/allow_referer", map[string]any{"allow_referer": refs})
	if err != nil {
		return err
	}
	var ack []string
	if e.Status != 200 || dbmsMetadata(e) || json.Unmarshal(e.Result, &ack) != nil || ValidateCacheReferrers(ack) != nil || len(ack) != len(refs) {
		return errors.New("cache referrer acknowledgement contract changed; reconcile before another write")
	}
	set := map[string]bool{}
	for _, r := range refs {
		set[r] = true
	}
	for _, r := range ack {
		if !set[r] {
			return errors.New("cache referrer acknowledgement differs from requested set")
		}
	}
	return nil
}
func (s *CacheService) Delete(ctx context.Context, id string) error {
	if err := ValidateServiceID(id); err != nil {
		return err
	}
	e, err := s.API.Delete(ctx, "/v1/cache/"+id)
	if err != nil {
		return err
	}
	var ack string
	if e.Status != 200 || dbmsMetadata(e) || json.Unmarshal(e.Result, &ack) != nil || ack == "" {
		return errors.New("cache delete acknowledgement contract changed; reconcile exact ID before another attempt")
	}
	return nil
}
