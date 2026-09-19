package hosted

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"net/url"
	"regexp"
	"unicode/utf8"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// WebhostingAPI deliberately has no update method: the public hosting API has
// no verified update operation. It must not be inferred from another service.
type WebhostingAPI interface {
	Get(context.Context, string, url.Values) (client.Envelope, error)
	PostJSON(context.Context, string, any) (client.Envelope, error)
	Delete(context.Context, string) (client.Envelope, error)
}

type WebhostingService struct{ API WebhostingAPI }

// Webhosting contains only readable fields. Server selection and passwords
// cannot be reconstructed by Read and are deliberately absent from this model.
// Status is control-plane status, not a successful HTTP/FTP/database probe.
type Webhosting struct {
	ID, ProductID, Name, Account, Status, IP string
	Description                              *string
	WebFirewall                              bool
	Domains                                  map[string]string
}

type WebhostingInput struct {
	ProductID, ServerID, Name, Account string
	Description                        *string
	FTPPassword                        string `json:"-"`
	DatabasePassword                   string `json:"-"`
	WebFirewall                        bool
	// Nil omits domain and lets the service assign its default domain. An empty
	// map is distinct, and is rejected until its meaning is verified.
	Domains map[string]string
}

type CreatedWebhosting struct {
	ID      string
	Service *Webhosting
}

// arrayResult rejects incomplete or newly paginated responses. It never returns
// a partial list that a caller could mistake for confirmed resource absence.
func arrayResult(e client.Envelope) ([]json.RawMessage, error) {
	if e.Status != 200 || len(e.Page) > 0 || len(e.PageNo) > 0 || len(e.PageSize) > 0 || len(e.Total) > 0 {
		return nil, errors.New("hosted list status or pagination contract changed")
	}
	var rows []json.RawMessage
	if json.Unmarshal(e.Result, &rows) != nil || rows == nil {
		return nil, errors.New("hosted result must be a nonnull array")
	}
	if len(e.Count) > 0 {
		var count *int
		if json.Unmarshal(e.Count, &count) != nil || count == nil || *count != len(rows) {
			return nil, errors.New("hosted list count is inconsistent")
		}
	}
	return rows, nil
}

func decodeWebhosting(raw json.RawMessage) (*Webhosting, error) {
	var row struct {
		ID          json.RawMessage   `json:"service_idx"`
		ProductID   string            `json:"product_id"`
		Name        string            `json:"name"`
		Account     string            `json:"id"`
		Status      string            `json:"status"`
		Description json.RawMessage   `json:"description"`
		Security    string            `json:"security"`
		IP          string            `json:"ip"`
		Domains     map[string]string `json:"domain"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return nil, errors.New("invalid webhosting service object")
	}
	id, err := serviceID(row.ID)
	if err != nil {
		return nil, err
	}
	var description *string
	if row.ProductID == "" || row.Name == "" || row.Account == "" || row.Status == "" || json.Unmarshal(row.Description, &description) != nil || (row.Security != "Y" && row.Security != "N") || len(row.Domains) == 0 {
		return nil, errors.New("webhosting service has missing or invalid required fields")
	}
	if _, err = netip.ParseAddr(row.IP); err != nil {
		return nil, errors.New("webhosting service has an invalid IP address")
	}
	for domain, folder := range row.Domains {
		if domain == "" || folder == "" {
			return nil, errors.New("webhosting domain mapping is invalid")
		}
	}
	return &Webhosting{ID: id, ProductID: row.ProductID, Name: row.Name, Account: row.Account, Status: row.Status, Description: description, WebFirewall: row.Security == "Y", IP: row.IP, Domains: row.Domains}, nil
}

func (s *WebhostingService) List(ctx context.Context) ([]Webhosting, error) {
	e, err := s.API.Get(ctx, "/v1/webhosting", nil)
	if err != nil {
		return nil, err
	}
	rows, err := arrayResult(e)
	if err != nil {
		return nil, err
	}
	out := make([]Webhosting, 0, len(rows))
	seen := map[string]bool{}
	for _, raw := range rows {
		row, err := decodeWebhosting(raw)
		if err != nil {
			return nil, err
		}
		if seen[row.ID] {
			return nil, errors.New("webhosting list contains duplicate identities")
		}
		seen[row.ID] = true
		out = append(out, *row)
	}
	return out, nil
}

// Read returns nil only after a validated full successful list. A create waiter
// must tolerate successful but empty reads, retaining the known create ID.
func (s *WebhostingService) Read(ctx context.Context, id string) (*Webhosting, error) {
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

var hostingAccount = regexp.MustCompile(`^[A-Za-z]{6,12}$`)

func validHostingPassword(password string) bool {
	if len(password) < 7 || len(password) > 20 {
		return false
	}
	letters, digits, special := false, false, false
	for _, c := range password {
		switch {
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			letters = true
		case c >= '0' && c <= '9':
			digits = true
		case c >= 33 && c <= 126:
			special = true
		default:
			return false
		}
	}
	return letters && digits || letters && special || digits && special
}

func webhostingBody(in WebhostingInput) (map[string]any, error) {
	if in.ProductID == "" || ValidateServiceID(in.ServerID) != nil || utf8.RuneCountInString(in.Name) < 4 || utf8.RuneCountInString(in.Name) > 32 || !hostingAccount.MatchString(in.Account) {
		return nil, errors.New("webhosting requires a product, canonical server ID, 4–32 character name and 6–12 letter account")
	}
	if in.Description != nil && (*in.Description == "" || utf8.RuneCountInString(*in.Description) > 50) {
		return nil, errors.New("webhosting description must contain 1–50 characters when supplied; omit it for an empty description")
	}
	if !validHostingPassword(in.FTPPassword) || !validHostingPassword(in.DatabasePassword) || in.FTPPassword == in.DatabasePassword {
		return nil, errors.New("webhosting passwords must differ and each contain 7–20 printable ASCII characters from at least two of letters, digits and symbols")
	}
	b := map[string]any{"product_id": in.ProductID, "server_idx": in.ServerID, "name": in.Name, "id": in.Account, "ftppw": in.FTPPassword, "dbpw": in.DatabasePassword, "security": "N"}
	if in.WebFirewall {
		b["security"] = "Y"
	}
	if in.Description != nil {
		b["description"] = *in.Description
	}
	if in.Domains != nil {
		if len(in.Domains) == 0 {
			return nil, errors.New("empty webhosting domain mapping is not verified; omit it to use the default domain")
		}
		for domain, folder := range in.Domains {
			if domain == "" || folder == "" {
				return nil, errors.New("webhosting domain and folder must not be empty")
			}
		}
		b["domain"] = in.Domains
	}
	return b, nil
}

// Create makes one request. The caller MUST persist ID before handling err; an
// error after identity extraction never authorizes another create request.
func (s *WebhostingService) Create(ctx context.Context, in WebhostingInput) (CreatedWebhosting, error) {
	b, err := webhostingBody(in)
	if err != nil {
		return CreatedWebhosting{}, err
	}
	e, err := s.API.PostJSON(ctx, "/v1/webhosting", b)
	if err != nil {
		return CreatedWebhosting{}, err
	}
	id, err := CreateID(e)
	out := CreatedWebhosting{ID: id}
	if err != nil {
		return out, err
	}
	if len(e.Page) > 0 || len(e.PageNo) > 0 || len(e.PageSize) > 0 || len(e.Total) > 0 || len(e.Count) > 0 {
		return out, errors.New("webhosting create metadata contract changed")
	}
	out.Service, err = decodeWebhosting(e.Result)
	return out, err
}

// Delete only verifies an acknowledgement. Callers must subsequently verify
// absence. The vendor documents a 24-hour ban on reusing the deleted account.
func (s *WebhostingService) Delete(ctx context.Context, id string) error {
	if err := ValidateServiceID(id); err != nil {
		return err
	}
	e, err := s.API.Delete(ctx, "/v1/webhosting/"+id)
	if err != nil {
		return err
	}
	var ack string
	if e.Status != 200 || json.Unmarshal(e.Result, &ack) != nil || ack == "" || len(e.Count) > 0 || len(e.Page) > 0 || len(e.PageNo) > 0 || len(e.PageSize) > 0 || len(e.Total) > 0 {
		return errors.New("webhosting delete acknowledgement contract changed; verify the exact service before retrying")
	}
	return nil
}
