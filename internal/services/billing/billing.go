// Package billing exposes read-only, typed billing records. Payment instruments,
// invoice/tax URLs and unverified detail fields never enter these models.
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type API interface {
	Get(context.Context, string, url.Values) (client.Envelope, error)
}
type Service struct{ API API }

const pageSize = 10
const maxPages = 1000

var billID = regexp.MustCompile(`^BILL-[A-Za-z0-9_-]+$`)

// Amounts are exact API integers, without float conversion, VAT calculation,
// currency conversion or assumed minor-unit scaling. Dates remain literal;
// these fields carry no authoritative timezone or timestamp semantics.
type CurrentBill struct {
	ID        string `json:"bill_id"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Price     int64  `json:"price"`
	Currency  string `json:"currency"`
}
type Bill struct {
	ID            string `json:"bill_id"`
	UsageStart    string `json:"usage_start"`
	UsageEnd      string `json:"usage_end"`
	BillDate      string `json:"bill_date"`
	Type          string `json:"type"`
	PaymentStatus string `json:"payment_status"`
	Name          string `json:"name"`
	Price         int64  `json:"price"`
	VAT           int64  `json:"vat"`
	PaymentPrice  int64  `json:"payment_price"`
	Currency      string `json:"currency"`
	DatePaid      string `json:"date_paid"`
}
type Filter struct {
	StartDate, EndDate         string
	MinimumPrice, MaximumPrice *int64
}

func (f Filter) Validate() error {
	for _, s := range []string{f.StartDate, f.EndDate} {
		if s != "" {
			t, e := time.Parse("2006-01-02", s)
			if e != nil || t.Format("2006-01-02") != s {
				return errors.New("billing date filters require YYYY-MM-DD calendar dates")
			}
		}
	}
	if f.StartDate != "" && f.EndDate != "" && f.StartDate > f.EndDate {
		return errors.New("billing start date must not follow end date")
	}
	if f.MinimumPrice != nil && f.MaximumPrice != nil && *f.MinimumPrice > *f.MaximumPrice {
		return errors.New("billing minimum price must not exceed maximum price")
	}
	return nil
}
func (f Filter) query(page int) url.Values {
	q := url.Values{"page_no": {strconv.Itoa(page)}, "page_size": {strconv.Itoa(pageSize)}}
	if f.StartDate != "" {
		q.Set("start_date", f.StartDate)
	}
	if f.EndDate != "" {
		q.Set("end_date", f.EndDate)
	}
	if f.MinimumPrice != nil {
		q.Set("min_price", strconv.FormatInt(*f.MinimumPrice, 10))
	}
	if f.MaximumPrice != nil {
		q.Set("max_price", strconv.FormatInt(*f.MaximumPrice, 10))
	}
	return q
}
func number(raw json.RawMessage, allowString bool) (int, error) {
	var n *int
	if json.Unmarshal(raw, &n) == nil && n != nil && *n >= 0 {
		return *n, nil
	}
	if allowString {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			v, e := strconv.Atoi(s)
			if e == nil && v >= 0 && strconv.Itoa(v) == s {
				return v, nil
			}
		}
	}
	return 0, errors.New("billing metadata must be a canonical nonnegative integer")
}
func rows(e client.Envelope) ([]json.RawMessage, error) {
	if e.Status != 200 || len(e.Page) != 0 || len(e.Total) != 0 {
		return nil, errors.New("billing envelope contract is unsupported")
	}
	var out []json.RawMessage
	if json.Unmarshal(e.Result, &out) != nil || out == nil {
		return nil, errors.New("billing result must be an array")
	}
	n, err := number(e.Count, false)
	if err != nil || n != len(out) {
		return nil, errors.New("billing count must match this page's rows")
	}
	return out, nil
}
func required(raw json.RawMessage, fields []string) error {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return errors.New("billing row must be an object")
	}
	for _, k := range fields {
		v, ok := m[k]
		if !ok || string(v) == "null" {
			return errors.New("billing row has missing or null fields")
		}
	}
	return nil
}
func (s *Service) Current(ctx context.Context) ([]CurrentBill, error) {
	e, err := s.API.Get(ctx, "/v1/bill/live", nil)
	if err != nil {
		return nil, err
	}
	raw, err := rows(e)
	if err != nil {
		return nil, err
	}
	if len(e.PageNo) != 0 || len(e.PageSize) != 0 {
		return nil, errors.New("current billing unexpectedly contains pagination")
	}
	out := make([]CurrentBill, 0, len(raw))
	seen := map[string]bool{}
	for _, v := range raw {
		var r CurrentBill
		if required(v, []string{"bill_id", "start_date", "end_date", "price", "currency"}) != nil || json.Unmarshal(v, &r) != nil || !billID.MatchString(r.ID) || r.StartDate == "" || r.EndDate == "" || r.Currency == "" || seen[r.ID] {
			return nil, errors.New("current billing fields or identity invalid")
		}
		seen[r.ID] = true
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (s *Service) List(ctx context.Context, f Filter) ([]Bill, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	out := []Bill{}
	seen := map[string]bool{}
	for page := 1; page <= maxPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		e, err := s.API.Get(ctx, "/v1/bill", f.query(page))
		if err != nil {
			var apiErr *client.Error
			// Only this endpoint's exact first-page empty-filter rejection means an
			// empty result. A late error never publishes partial traversal state.
			if page == 1 && errors.As(err, &apiErr) && apiErr.Kind == "http_status" && apiErr.Status == 400 && apiErr.Code == "EMPTY_SET" {
				sortBills(out)
				return out, nil
			}
			return nil, err
		}
		raw, err := rows(e)
		if err != nil {
			return nil, err
		}
		n, ne := number(e.PageNo, true)
		size, se := number(e.PageSize, true)
		if ne != nil || se != nil || n != page || size != pageSize || len(raw) > pageSize {
			return nil, errors.New("billing pagination metadata is inconsistent")
		}
		for _, v := range raw {
			var r Bill
			if required(v, []string{"bill_id", "usage_start", "usage_end", "bill_date", "type", "payment_status", "name", "price", "vat", "payment_price", "currency", "date_paid"}) != nil || json.Unmarshal(v, &r) != nil || !billID.MatchString(r.ID) || r.BillDate == "" || r.Type == "" || r.PaymentStatus == "" || r.Currency == "" || seen[r.ID] {
				return nil, errors.New("billing fields or duplicate identity invalid")
			}
			if (f.StartDate != "" && r.BillDate < f.StartDate) || (f.EndDate != "" && r.BillDate > f.EndDate) || (f.MinimumPrice != nil && r.Price < *f.MinimumPrice) || (f.MaximumPrice != nil && r.Price > *f.MaximumPrice) {
				return nil, errors.New("billing response does not match requested filters")
			}
			seen[r.ID] = true
			out = append(out, r)
		}
		if len(raw) < pageSize {
			sortBills(out)
			return out, nil
		}
	}
	return nil, errors.New("billing list exceeded bounded pagination limit")
}
func sortBills(rows []Bill) { sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID }) }
