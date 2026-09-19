package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"regexp"
	"unicode/utf8"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// NASAPI is the control plane. It does not authenticate to tenant file APIs or mount storage.
type NASAPI interface {
	HostingCatalogAPI
	PostJSON(context.Context, string, any) (client.Envelope, error)
	PutJSON(context.Context, string, any) (client.Envelope, error)
	Delete(context.Context, string) (client.Envelope, error)
}
type NASService struct{ API NASAPI }
type NASCatalogService struct{ API HostingCatalogAPI }
type NASProduct struct {
	ID, Name, Status             string
	Version                      *string
	MinimumDiskGB, MaximumDiskGB int64
}
type NAS struct {
	ID, ProductID, Name, Status, Domain, MountInfo string
	Description                                    *string
	DiskGB                                         int64
	AllowIPs                                       map[string]string
}
type NASInput struct {
	ProductID, Name, ShareName string
	Description                *string
	DiskGB                     int64
	AllowIPs                   map[string]string
}
type CreatedNAS struct {
	ID      string
	Service *NAS
}

var nasShareName = regexp.MustCompile(`^[A-Za-z0-9]{6,20}$`)

func ValidateNASAllowIPs(ips map[string]string) error {
	if len(ips) == 0 {
		return errors.New("NAS allowed IP map must be nonempty; empty clearing is unsupported")
	}
	for host, mode := range ips {
		ip, err := netip.ParseAddr(host)
		if err != nil || !ip.Is4() || ip.String() != host || (mode != "RO" && mode != "RW") {
			return errors.New("NAS allowed IPs require canonical IPv4 hosts mapped to exact RO or RW; CIDR and IPv6 are unsupported")
		}
	}
	return nil
}
func (s *NASCatalogService) Products(ctx context.Context) ([]NASProduct, error) {
	e, err := s.API.Get(ctx, "/v1/apinas/products", nil)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]NASProduct, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		var row struct {
			ID     *string `json:"product_id"`
			Name   string  `json:"product_name"`
			Status string  `json:"status"`
			Spec   struct {
				Version json.RawMessage `json:"version"`
				Disk    struct {
					Min *int64 `json:"min"`
					Max *int64 `json:"max"`
				} `json:"disk"`
			} `json:"spec"`
		}
		var version *string
		if json.Unmarshal(raw, &row) != nil || row.ID == nil || row.Name == "" || row.Status == "" || json.Unmarshal(row.Spec.Version, &version) != nil || row.Spec.Disk.Min == nil || row.Spec.Disk.Max == nil || *row.Spec.Disk.Min < 0 || *row.Spec.Disk.Max < *row.Spec.Disk.Min {
			return nil, errors.New("NAS catalog fields or disk bounds invalid")
		}
		key := "id:" + *row.ID
		if *row.ID == "" {
			key = "unselectable:" + row.Name
		}
		if seen[key] {
			return nil, errors.New("NAS catalog contains duplicate product identities")
		}
		seen[key] = true
		out = append(out, NASProduct{*row.ID, row.Name, row.Status, version, *row.Spec.Disk.Min, *row.Spec.Disk.Max})
	}
	return out, nil
}
func decodeNAS(raw json.RawMessage, receipt bool) (*NAS, error) {
	var row struct {
		ID          json.RawMessage `json:"service_idx"`
		ProductID   string          `json:"product_id"`
		Name        string          `json:"name"`
		Status      string          `json:"status"`
		Description json.RawMessage `json:"description"`
		Domain      string          `json:"domain"`
		MountInfo   *string         `json:"mount_info"`
		Spec        struct {
			Disk *int64 `json:"disk"`
		} `json:"spec"`
		AllowIPs map[string]string `json:"allowip"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return nil, errors.New("invalid NAS service object")
	}
	id, err := serviceID(row.ID)
	if err != nil {
		return nil, err
	}
	var description *string
	if row.ProductID == "" || row.Name == "" || row.Status == "" || row.Domain == "" || json.Unmarshal(row.Description, &description) != nil || row.Spec.Disk == nil || *row.Spec.Disk < 1 || row.AllowIPs == nil || (!receipt && row.MountInfo == nil) {
		return nil, errors.New("NAS service fields missing or invalid")
	}
	if len(row.AllowIPs) > 0 && ValidateNASAllowIPs(row.AllowIPs) != nil {
		return nil, errors.New("NAS allowed IP read contract is unsupported")
	}
	mount := ""
	if row.MountInfo != nil {
		mount = *row.MountInfo
	}
	// ShareName is intentionally absent: the API does not expose creation history.
	return &NAS{ID: id, ProductID: row.ProductID, Name: row.Name, Status: row.Status, Description: description, Domain: row.Domain, MountInfo: mount, DiskGB: *row.Spec.Disk, AllowIPs: row.AllowIPs}, nil
}
func (s *NASService) List(ctx context.Context) ([]NAS, error) {
	e, err := s.API.Get(ctx, "/v1/apinas", nil)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]NAS, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		row, err := decodeNAS(raw, false)
		if err != nil {
			return nil, err
		}
		if seen[row.ID] {
			return nil, errors.New("NAS list contains duplicate service identities")
		}
		seen[row.ID] = true
		out = append(out, *row)
	}
	return out, nil
}
func (s *NASService) Read(ctx context.Context, id string) (*NAS, error) {
	if err := ValidateServiceID(id); err != nil {
		return nil, err
	}
	rows, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == id {
			return &row, nil
		}
	}
	return nil, nil
}
func (s *NASService) Create(ctx context.Context, in NASInput) (CreatedNAS, error) {
	if in.ProductID == "" || !nasShareName.MatchString(in.ShareName) || utf8.RuneCountInString(in.Name) < 4 || utf8.RuneCountInString(in.Name) > 32 || in.DiskGB < 100 || in.DiskGB > 2000 {
		return CreatedNAS{}, errors.New("NAS creation requires a product ID, 4–32 character name, 6–20 ASCII alphanumeric share name and 100–2000 GB disk")
	}
	if in.Description != nil && (*in.Description == "" || utf8.RuneCountInString(*in.Description) > 50) {
		return CreatedNAS{}, errors.New("NAS description must contain 1–50 characters when supplied; omit empty descriptions")
	}
	if err := ValidateNASAllowIPs(in.AllowIPs); err != nil {
		return CreatedNAS{}, err
	}
	body := map[string]any{"product_id": in.ProductID, "name": in.Name, "sharename": in.ShareName, "hdd": in.DiskGB, "allowip": in.AllowIPs}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	e, err := s.API.PostJSON(ctx, "/v1/apinas", body)
	if err != nil {
		return CreatedNAS{}, err
	}
	id, err := CreateID(e)
	out := CreatedNAS{ID: id}
	if err != nil {
		return out, err
	}
	if dbmsMetadata(e) {
		return out, errors.New("NAS creation metadata changed; preserve returned identity")
	}
	out.Service, err = decodeNAS(e.Result, true)
	return out, err
}
func (s *NASService) ReplaceAllowIPs(ctx context.Context, id string, ips map[string]string) error {
	if err := ValidateServiceID(id); err != nil {
		return err
	}
	if err := ValidateNASAllowIPs(ips); err != nil {
		return err
	}
	e, err := s.API.PutJSON(ctx, "/v1/apinas/"+id+"/allowip", map[string]any{"allowip": ips})
	if err != nil {
		return err
	}
	var rows []struct {
		IP  string `json:"ip"`
		ACL string `json:"acl"`
	}
	if e.Status != 200 || dbmsMetadata(e) || json.Unmarshal(e.Result, &rows) != nil || len(rows) != len(ips) {
		return errors.New("NAS allowlist acknowledgement changed; reconcile exact ID before another write")
	}
	ack := map[string]string{}
	for _, row := range rows {
		if _, exists := ack[row.IP]; exists {
			return errors.New("NAS allowlist acknowledgement contains duplicate IPs")
		}
		ack[row.IP] = row.ACL
	}
	if ValidateNASAllowIPs(ack) != nil || !reflect.DeepEqual(ips, ack) {
		return errors.New("NAS allowlist acknowledgement differs from requested permission map")
	}
	return nil
}
func (s *NASService) Delete(ctx context.Context, id string) error {
	if err := ValidateServiceID(id); err != nil {
		return err
	}
	e, err := s.API.Delete(ctx, "/v1/apinas/"+id)
	if err != nil {
		return err
	}
	var ack string
	if e.Status != 200 || dbmsMetadata(e) || json.Unmarshal(e.Result, &ack) != nil || ack == "" {
		return errors.New("NAS delete acknowledgement changed; reconcile exact ID before another write")
	}
	return nil
}
