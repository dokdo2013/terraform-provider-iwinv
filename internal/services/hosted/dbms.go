package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"unicode/utf8"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// DBMSAPI supports whole-allowlist replacement, not account/engine updates.
type DBMSAPI interface {
	HostingCatalogAPI
	PostJSON(context.Context, string, any) (client.Envelope, error)
	PutJSON(context.Context, string, any) (client.Envelope, error)
	Delete(context.Context, string) (client.Envelope, error)
}
type DBMSService struct{ API DBMSAPI }
type DBMSCatalogService struct{ API HostingCatalogAPI }

// Product IDs are not unique across versions. Never collapse the catalog into
// a product_id-keyed map or imply that POST accepts an engine/version selector.
type DBMSProduct struct{ ID, Name, Status, Tier, Version string }
type DBMS struct {
	ID, ProductID, Name, Status, Tier, Version string
	Description                                *string
	Domains                                    map[string]string
	AllowIPs                                   []string
}
type DBMSInput struct {
	ProductID, Name, Account string
	Description              *string
	AllowIPs                 []string
}
type DBMSReceipt struct {
	ID, Name, Status, Domain string
	Description              *string
	AllowIPs                 []string
}
type CreatedDBMS struct {
	ID      string
	Receipt *DBMSReceipt
}

var dbmsAccount = regexp.MustCompile(`^[A-Za-z]{6,12}$`)

// ValidateDBMSAllowIPs supports a nonempty authoritative set of canonical IPv4
// hosts. CIDR/IPv6 and empty clearing are not verified. Values never enter errors.
func ValidateDBMSAllowIPs(ips []string) error {
	if len(ips) == 0 {
		return errors.New("DBMS allowed IPs must be a nonempty set; empty clearing is unsupported")
	}
	seen := map[string]bool{}
	for _, s := range ips {
		ip, err := netip.ParseAddr(s)
		if err != nil || !ip.Is4() || ip.String() != s || seen[s] {
			return errors.New("DBMS allowed IPs must be distinct canonical IPv4 host addresses, without CIDR prefixes")
		}
		seen[s] = true
	}
	return nil
}
func dbmsMetadata(e client.Envelope) bool {
	return len(e.Count) > 0 || len(e.Page) > 0 || len(e.PageNo) > 0 || len(e.PageSize) > 0 || len(e.Total) > 0
}
func (s *DBMSCatalogService) Products(ctx context.Context, tier, engine string) ([]DBMSProduct, error) {
	if tier != "" && tier != "STD" && tier != "HM" {
		return nil, errors.New("DBMS product type must be STD or HM when supplied")
	}
	switch engine {
	case "", "MySQL", "MariaDB", "mongoDB", "MS-SQL", "redis", "PostgreSQL":
	default:
		return nil, errors.New("DBMS engine filter is not a documented value")
	}
	q := url.Values{}
	if tier != "" {
		q.Set("type", tier)
	}
	if engine != "" {
		q.Set("db", engine)
	}
	e, err := s.API.Get(ctx, "/v1/dbms/products", q)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]DBMSProduct, 0, len(rows))
	seen := map[[3]string]bool{}
	for _, raw := range rows {
		var r struct {
			ID     *string `json:"product_id"`
			Name   string  `json:"product_name"`
			Status string  `json:"status"`
			Spec   struct {
				Tier    string `json:"type"`
				Version string `json:"ver"`
			} `json:"spec"`
		}
		if json.Unmarshal(raw, &r) != nil {
			return nil, errors.New("invalid DBMS product object")
		}
		if r.ID == nil {
			return nil, errors.New("DBMS product ID must be present as a string")
		}
		// Available rows with an empty creation ID are real catalog entries.
		// Preserve them for review; Create still rejects an empty product ID.
		key := [3]string{*r.ID, r.Spec.Tier, r.Spec.Version}
		if r.Name == "" || r.Status == "" || r.Spec.Tier == "" || r.Spec.Version == "" || seen[key] || tier != "" && r.Spec.Tier != tier {
			return nil, errors.New("DBMS product fields, exact duplicate or type filter contract invalid")
		}
		seen[key] = true
		out = append(out, DBMSProduct{*r.ID, r.Name, r.Status, r.Spec.Tier, r.Spec.Version})
	}
	return out, nil
}
func decodeDBMS(raw json.RawMessage) (*DBMS, error) {
	var r struct {
		ID          json.RawMessage `json:"service_idx"`
		ProductID   string          `json:"product_id"`
		Name        string          `json:"name"`
		Status      string          `json:"status"`
		Description json.RawMessage `json:"description"`
		Spec        struct {
			Tier    string `json:"type"`
			Version string `json:"ver"`
		} `json:"spec"`
		Domains  map[string]string `json:"domain"`
		AllowIPs []string          `json:"allowip"`
	}
	if json.Unmarshal(raw, &r) != nil {
		return nil, errors.New("invalid DBMS service object")
	}
	id, err := serviceID(r.ID)
	if err != nil {
		return nil, err
	}
	var description *string
	if r.ProductID == "" || r.Name == "" || r.Status == "" || r.Spec.Tier == "" || r.Spec.Version == "" || json.Unmarshal(r.Description, &description) != nil || len(r.Domains) == 0 || r.AllowIPs == nil {
		return nil, errors.New("DBMS service has missing or invalid required fields")
	}
	for label, domain := range r.Domains {
		if label == "" || domain == "" {
			return nil, errors.New("DBMS domain mapping is invalid")
		}
	}
	// A readable empty list is preserved even though empty PUTs are unsupported.
	// This permits importing/detecting externally created states without inventing
	// a successful clearing operation.
	if len(r.AllowIPs) > 0 {
		if err = ValidateDBMSAllowIPs(r.AllowIPs); err != nil {
			return nil, errors.New("DBMS allowed IP read contract is invalid")
		}
	}
	sort.Strings(r.AllowIPs)
	return &DBMS{ID: id, ProductID: r.ProductID, Name: r.Name, Status: r.Status, Tier: r.Spec.Tier, Version: r.Spec.Version, Description: description, Domains: r.Domains, AllowIPs: r.AllowIPs}, nil
}
func (s *DBMSService) List(ctx context.Context) ([]DBMS, error) {
	e, err := s.API.Get(ctx, "/v1/dbms", nil)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]DBMS, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		r, err := decodeDBMS(raw)
		if err != nil {
			return nil, err
		}
		if seen[r.ID] {
			return nil, errors.New("DBMS list contains duplicate service identities")
		}
		seen[r.ID] = true
		out = append(out, *r)
	}
	return out, nil
}

// Read describes a validated full list. Resource-specific readiness/absence
// handling must retain an unverified create identity across delayed visibility.
func (s *DBMSService) Read(ctx context.Context, id string) (*DBMS, error) {
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
func (s *DBMSService) Create(ctx context.Context, in DBMSInput) (CreatedDBMS, error) {
	if in.ProductID == "" || !dbmsAccount.MatchString(in.Account) || utf8.RuneCountInString(in.Name) < 4 || utf8.RuneCountInString(in.Name) > 32 {
		return CreatedDBMS{}, errors.New("DBMS creation requires a product ID, 4–32 character name and 6–12 ASCII-letter account")
	}
	if in.Description != nil && (*in.Description == "" || utf8.RuneCountInString(*in.Description) > 50) {
		return CreatedDBMS{}, errors.New("DBMS description must contain 1–50 characters when supplied; omit an empty description")
	}
	if err := ValidateDBMSAllowIPs(in.AllowIPs); err != nil {
		return CreatedDBMS{}, err
	}
	body := map[string]any{"product_id": in.ProductID, "name": in.Name, "id": in.Account, "allowip": in.AllowIPs}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	e, err := s.API.PostJSON(ctx, "/v1/dbms", body)
	if err != nil {
		return CreatedDBMS{}, err
	}
	id, err := CreateID(e)
	out := CreatedDBMS{ID: id}
	if err != nil {
		return out, err
	}
	if dbmsMetadata(e) {
		return out, errors.New("DBMS create metadata contract changed")
	}
	var r struct {
		Name        string          `json:"name"`
		Status      string          `json:"status"`
		Domain      string          `json:"domain"`
		Description json.RawMessage `json:"description"`
		AllowIPs    []string        `json:"allowip"`
	}
	var description *string
	if json.Unmarshal(e.Result, &r) != nil || r.Name == "" || r.Status == "" || r.Domain == "" || json.Unmarshal(r.Description, &description) != nil || ValidateDBMSAllowIPs(r.AllowIPs) != nil {
		return out, errors.New("DBMS creation receipt fields are invalid; preserve the returned ID")
	}
	out.Receipt = &DBMSReceipt{ID: id, Name: r.Name, Status: r.Status, Domain: r.Domain, Description: description, AllowIPs: r.AllowIPs}
	return out, nil
}

// ReplaceAllowIPs owns the complete set. The endpoint's name does not imply
// append semantics. The acknowledgement must contain exactly the requested set;
// a later Read is still required to verify convergence.
func (s *DBMSService) ReplaceAllowIPs(ctx context.Context, id string, ips []string) error {
	if err := ValidateServiceID(id); err != nil {
		return err
	}
	if err := ValidateDBMSAllowIPs(ips); err != nil {
		return err
	}
	e, err := s.API.PutJSON(ctx, "/v1/dbms/"+id+"/allowip", map[string]any{"allowip": ips})
	if err != nil {
		return err
	}
	var ack []string
	if e.Status != 200 || dbmsMetadata(e) || json.Unmarshal(e.Result, &ack) != nil || ValidateDBMSAllowIPs(ack) != nil || len(ack) != len(ips) {
		return errors.New("DBMS allowlist acknowledgement contract changed; verify by exact service ID before another write")
	}
	expected := map[string]bool{}
	for _, v := range ips {
		expected[v] = true
	}
	for _, v := range ack {
		if !expected[v] {
			return errors.New("DBMS allowlist acknowledgement differs from requested set")
		}
	}
	return nil
}
func (s *DBMSService) Delete(ctx context.Context, id string) error {
	if err := ValidateServiceID(id); err != nil {
		return err
	}
	e, err := s.API.Delete(ctx, "/v1/dbms/"+id)
	if err != nil {
		return err
	}
	var ack string
	if e.Status != 200 || dbmsMetadata(e) || json.Unmarshal(e.Result, &ack) != nil || ack == "" {
		return errors.New("DBMS delete acknowledgement contract changed; verify by exact service ID before retrying")
	}
	return nil
}
