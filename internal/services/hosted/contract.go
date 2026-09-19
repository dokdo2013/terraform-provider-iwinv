// Package hosted decodes the observed hosted-service control-plane contracts.
// These helpers do not register Terraform resources or establish readiness.
package hosted

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// Record deliberately excludes credentials, endpoints and product-specific data.
// Name can be missing from webmail reads. Service adapters decide which optional
// fields are required for their own schema. The integer API service_idx is converted without an intermediate float64.
type Record struct {
	ID          string
	ProductID   string
	Name        *string
	Description *string
	Status      string
}

type wireRecord struct {
	ID          json.RawMessage `json:"service_idx"`
	ProductID   string          `json:"product_id"`
	Name        *string         `json:"name"`
	Description *string         `json:"description"`
	Status      string          `json:"status"`
}

func serviceID(raw json.RawMessage) (string, error) {
	var id int64
	if json.Unmarshal(raw, &id) != nil || id <= 0 {
		return "", errors.New("hosted service identity must be a positive integer")
	}
	return strconv.FormatInt(id, 10), nil
}

// CreateID extracts identity independently of remaining response fields. Cache
// create omits product_id while its list includes it. Callers must preserve this
// ID before validating fields, waiting for readiness or applying child settings.
// A newly observed 201/202 returns the ID alongside an error for recovery.
// This is not a decoder for error responses or ambiguous creates.
func CreateID(e client.Envelope) (string, error) {
	if e.Status < 200 || e.Status > 202 {
		return "", errors.New("hosted create returned an unsuccessful HTTP status")
	}
	raw := bytes.TrimSpace(e.Result)
	if len(raw) == 0 || raw[0] != '{' {
		return "", errors.New("hosted create result must be one object")
	}
	var row struct {
		ID json.RawMessage `json:"service_idx"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return "", errors.New("invalid hosted create result")
	}
	id, err := serviceID(row.ID)
	if err != nil {
		return "", err
	}
	if e.Status != 200 {
		return id, errors.New("hosted create returned an unverified success status; preserve the returned ID")
	}
	return id, nil
}

// Records validates a service-list response with the observed absence of count
// and page metadata. It does not invent pagination or assume a page is complete
// if the server starts returning pagination metadata.
func Records(e client.Envelope) ([]Record, error) {
	if e.Status != 200 {
		return nil, errors.New("hosted list returned an unverified HTTP status")
	}
	if len(e.Page) > 0 || len(e.PageNo) > 0 || len(e.PageSize) > 0 || len(e.Total) > 0 {
		return nil, errors.New("hosted list pagination contract changed")
	}
	var wire []wireRecord
	if json.Unmarshal(e.Result, &wire) != nil || wire == nil {
		return nil, errors.New("hosted list must be an array of service objects")
	}
	if len(e.Count) > 0 {
		var count *int
		if json.Unmarshal(e.Count, &count) != nil || count == nil || *count != len(wire) {
			return nil, errors.New("hosted list count is inconsistent")
		}
	}
	result := make([]Record, 0, len(wire))
	seen := map[string]bool{}
	for _, row := range wire {
		id, err := serviceID(row.ID)
		if err != nil {
			return nil, err
		}
		if seen[id] || row.ProductID == "" || row.Status == "" {
			return nil, errors.New("hosted list has duplicate IDs or missing required fields")
		}
		seen[id] = true
		result = append(result, Record{ID: id, ProductID: row.ProductID, Name: row.Name, Description: row.Description, Status: row.Status})
	}
	return result, nil
}

// ValidateServiceID accepts exact positive int64 identities, without floats,
// leading zeros or path/query characters. Import uses the same contract.
func ValidateServiceID(id string) error {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 || strconv.FormatInt(n, 10) != id {
		return errors.New("hosted service ID must be a canonical positive decimal integer")
	}
	return nil
}

// Find selects by exact ID from a validated list, never by name or list position.
// A nil record only means this successful list did not contain the requested ID.
// Account scope, completeness and eventual consistency still belong to each
// service lifecycle; callers must not generalize this into arbitrary absence.
func Find(e client.Envelope, id string) (*Record, error) {
	if err := ValidateServiceID(id); err != nil {
		return nil, err
	}
	rows, err := Records(e)
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
