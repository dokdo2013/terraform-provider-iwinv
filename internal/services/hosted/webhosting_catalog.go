package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
)

// HostingProduct exposes the fields needed for an explicit creation choice.
// Prices, VAT, disk and traffic are excluded until their units are verified.
type HostingProduct struct {
	ID, Name, Status, Type                 string
	PHPVersions                            []string
	AllowCustomDomain, EnableDomainFolder  bool
	MaxDomainCount, DomainEditIntervalDays int64
}

type HostingServer struct {
	ID, Charset, PHPVersion, Database, Program string
}

func (s *WebhostingService) Products(ctx context.Context, kind string) ([]HostingProduct, error) {
	if kind != "" && kind != "SHARE" && kind != "SINGLE" {
		return nil, errors.New("hosting product type must be SHARE or SINGLE when supplied")
	}
	var q url.Values
	if kind != "" {
		q = url.Values{"type": {kind}}
	}
	e, err := s.API.Get(ctx, "/v1/webhosting/products", q)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]HostingProduct, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		var row struct {
			ID     string `json:"product_id"`
			Name   string `json:"product_name"`
			Status string `json:"status"`
			Spec   struct {
				Type        string   `json:"type"`
				PHPVersions []string `json:"allow_php_version"`
				Domain      struct {
					AllowCustom  *bool  `json:"allow_custom_domain"`
					EnableFolder *bool  `json:"enable_domain_folder"`
					MaxCount     *int64 `json:"max_domain_count"`
					IntervalDays *int64 `json:"edit_interval_days"`
				} `json:"domain"`
			} `json:"spec"`
		}
		if json.Unmarshal(raw, &row) != nil {
			return nil, errors.New("invalid hosting product object")
		}
		d := row.Spec.Domain
		if row.ID == "" || row.Name == "" || row.Status == "" || (row.Spec.Type != "SHARE" && row.Spec.Type != "SINGLE") || (kind != "" && row.Spec.Type != kind) || row.Spec.PHPVersions == nil || d.AllowCustom == nil || d.EnableFolder == nil || d.MaxCount == nil || *d.MaxCount < 0 || d.IntervalDays == nil || *d.IntervalDays < 0 || seen[row.ID] {
			return nil, errors.New("hosting product has missing, invalid or duplicate fields")
		}
		versions := map[string]bool{}
		for _, version := range row.Spec.PHPVersions {
			if version == "" || versions[version] {
				return nil, errors.New("hosting product PHP versions are invalid or duplicated")
			}
			versions[version] = true
		}
		seen[row.ID] = true
		out = append(out, HostingProduct{ID: row.ID, Name: row.Name, Status: row.Status, Type: row.Spec.Type, PHPVersions: row.Spec.PHPVersions, AllowCustomDomain: *d.AllowCustom, EnableDomainFolder: *d.EnableFolder, MaxDomainCount: *d.MaxCount, DomainEditIntervalDays: *d.IntervalDays})
	}
	return out, nil
}

// Servers always sends the required product_id query. The numeric idx is an
// exact string selector; it is not the service identity and Read cannot recover
// the server selector from a provisioned hosting service.
func (s *WebhostingService) Servers(ctx context.Context, productID string) ([]HostingServer, error) {
	if productID == "" {
		return nil, errors.New("hosting server lookup requires product_id")
	}
	e, err := s.API.Get(ctx, "/v1/webhosting/servers", url.Values{"product_id": {productID}})
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]HostingServer, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		var row struct {
			ID         json.RawMessage `json:"idx"`
			Charset    *string         `json:"charset"`
			PHPVersion *string         `json:"php_version"`
			Database   *string         `json:"db"`
			Program    *string         `json:"program"`
		}
		if json.Unmarshal(raw, &row) != nil {
			return nil, errors.New("invalid hosting server object")
		}
		id, err := serviceID(row.ID)
		if err != nil {
			return nil, err
		}
		if seen[id] || row.Charset == nil || *row.Charset == "" || row.PHPVersion == nil || *row.PHPVersion == "" || row.Database == nil || row.Program == nil {
			return nil, errors.New("hosting server has missing, invalid or duplicate fields")
		}
		seen[id] = true
		out = append(out, HostingServer{ID: id, Charset: *row.Charset, PHPVersion: *row.PHPVersion, Database: *row.Database, Program: *row.Program})
	}
	return out, nil
}
