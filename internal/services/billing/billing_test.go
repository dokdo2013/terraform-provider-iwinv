package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

type fake struct {
	call  func(string, url.Values) (client.Envelope, error)
	calls int
}

func (f *fake) Get(_ context.Context, p string, q url.Values) (client.Envelope, error) {
	f.calls++
	return f.call(p, q)
}

const current = `{"bill_id":"BILL-live","start_date":"2026-01-01","end_date":"2026-01-20","price":9007199254740993,"currency":"KRW"}`

func bill(id int) string {
	return fmt.Sprintf(`{"bill_id":"BILL-%03d","usage_start":"2026-01-01","usage_end":"2026-01-31","bill_date":"2026-02-01","type":"regular","payment_status":"paid","name":"합성 &amp; 청구","price":9007199254740993,"vat":10,"payment_price":9007199254741003,"currency":"KRW","date_paid":"2026-02-03","payment_info":"SYNTHETIC-PAYMENT-SECRET","invoice":["https://example.invalid/private-invoice"],"tax":["https://example.invalid/private-tax"]}`, id)
}
func envelope(rows ...string) client.Envelope {
	return client.Envelope{Status: 200, Result: json.RawMessage("[" + strings.Join(rows, ",") + "]"), Count: json.RawMessage(fmt.Sprint(len(rows)))}
}
func paged(page int, rows ...string) client.Envelope {
	e := envelope(rows...)
	e.PageNo = json.RawMessage(fmt.Sprintf("%q", fmt.Sprint(page)))
	e.PageSize = json.RawMessage(`"10"`)
	return e
}
func TestCurrentExactAndPrivateFields(t *testing.T) {
	f := &fake{call: func(p string, q url.Values) (client.Envelope, error) {
		if p != "/v1/bill/live" || len(q) != 0 {
			t.Fatal("wrong live path")
		}
		return envelope(current), nil
	}}
	rows, err := (&Service{API: f}).Current(context.Background())
	if err != nil || len(rows) != 1 || rows[0].Price != 9007199254740993 {
		t.Fatal("current contract changed", err)
	}
	f.call = func(string, url.Values) (client.Envelope, error) { return paged(1, bill(1)), nil }
	bills, err := (&Service{API: f}).List(context.Background(), Filter{})
	if err != nil || bills[0].Price != 9007199254740993 || bills[0].VAT != 10 || bills[0].PaymentPrice != 9007199254741003 {
		t.Fatal("amount precision changed", err)
	}
	raw, _ := json.Marshal(bills)
	for _, s := range []string{"SYNTHETIC-PAYMENT-SECRET", "private-invoice", "private-tax", "payment_info", "invoice", "tax"} {
		if strings.Contains(string(raw), s) {
			t.Fatal("unmodeled private field escaped")
		}
	}
	if bills[0].Name != "합성 &amp; 청구" {
		t.Fatal("literal bill name changed")
	}
}
func TestListPaginationAndFilters(t *testing.T) {
	n := int64(9007199254740993)
	f := &fake{call: func(p string, q url.Values) (client.Envelope, error) {
		if p != "/v1/bill" || q.Get("start_date") != "2026-02-01" || q.Get("end_date") != "2026-02-01" || q.Get("min_price") != fmt.Sprint(n) || q.Get("max_price") != fmt.Sprint(n) || q.Get("page_size") != "10" {
			t.Fatal("filters not forwarded exactly")
		}
		if q.Get("page_no") == "1" {
			rows := []string{}
			for i := 10; i >= 1; i-- {
				rows = append(rows, bill(i))
			}
			return paged(1, rows...), nil
		}
		if q.Get("page_no") == "2" {
			e := paged(2, bill(11))
			e.PageNo = json.RawMessage(`2`)
			e.PageSize = json.RawMessage(`10`)
			return e, nil
		}
		t.Fatal("unexpected extra page")
		return client.Envelope{}, nil
	}}
	rows, err := (&Service{API: f}).List(context.Background(), Filter{StartDate: "2026-02-01", EndDate: "2026-02-01", MinimumPrice: &n, MaximumPrice: &n})
	if err != nil || len(rows) != 11 || rows[0].ID != "BILL-001" || rows[10].ID != "BILL-011" || f.calls != 2 {
		t.Fatal("pagination incomplete", err)
	}
}
func TestBillingEmptyAndErrors(t *testing.T) {
	exact := &client.Error{Kind: "http_status", Status: 400, Code: "EMPTY_SET"}
	f := &fake{call: func(string, url.Values) (client.Envelope, error) { return client.Envelope{}, exact }}
	rows, err := (&Service{API: f}).List(context.Background(), Filter{})
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("verified first-page empty rejection not handled")
	}
	if _, err = (&Service{API: f}).Current(context.Background()); !errors.Is(err, exact) {
		t.Fatal("live error hidden")
	}
	for _, err := range []error{&client.Error{Kind: "http_status", Status: 403, Code: "CHECK_IP"}, &client.Error{Kind: "http_status", Status: 404, Code: "NOT_FOUND"}, &client.Error{Kind: "http_status", Status: 400}, &client.Error{Kind: "business_error", Status: 200, Code: "EMPTY_SET"}, errors.New("synthetic transport failure")} {
		f.call = func(string, url.Values) (client.Envelope, error) { return client.Envelope{}, err }
		if got, e := (&Service{API: f}).List(context.Background(), Filter{}); e == nil || got != nil {
			t.Fatal("unknown failure became empty")
		}
	}
	f.call = func(string, url.Values) (client.Envelope, error) { return paged(1), nil }
	rows, err = (&Service{API: f}).List(context.Background(), Filter{})
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatal("empty successful array rejected")
	}
	for _, mode := range []string{"duplicate", "late_error", "late_empty_error", "limit"} {
		t.Run(mode, func(t *testing.T) {
			f.calls = 0
			f.call = func(_ string, q url.Values) (client.Envelope, error) {
				if q.Get("page_no") != "1" && mode == "late_error" {
					return client.Envelope{}, errors.New("synthetic late error")
				}
				if q.Get("page_no") != "1" && mode == "late_empty_error" {
					return client.Envelope{}, exact
				}
				r := []string{}
				for i := 0; i < 10; i++ {
					id := i
					if mode == "limit" {
						id = f.calls*10 + i
					}
					r = append(r, bill(id))
				}
				e := paged(f.calls, r...)
				return e, nil
			}
			if rows, err := (&Service{API: f}).List(context.Background(), Filter{}); err == nil || rows != nil {
				t.Fatal("partial/repeated/unbounded response accepted")
			}
		})
	}
}
func TestBillingMalformed(t *testing.T) {
	for _, mode := range []string{"null", "null_row", "missing_price", "null_price", "fractional_price", "overflow", "string_price", "missing_name", "bad_id", "duplicate", "count", "missing_count", "page", "total", "page_no", "page_size", "noncanonical_page", "filter_mismatch", "status"} {
		t.Run(mode, func(t *testing.T) {
			row := bill(1)
			e := paged(1, row)
			switch mode {
			case "null":
				e.Result = json.RawMessage(`null`)
			case "null_row":
				e.Result = json.RawMessage(`[null]`)
			case "missing_price":
				e.Result = json.RawMessage("[" + strings.Replace(row, `"price":9007199254740993,`, "", 1) + "]")
			case "null_price", "fractional_price", "overflow", "string_price":
				v := map[string]string{"null_price": "null", "fractional_price": "1.5", "overflow": "9223372036854775808", "string_price": `"10"`}[mode]
				e.Result = json.RawMessage("[" + strings.Replace(row, `"price":9007199254740993`, `"price":`+v, 1) + "]")
			case "missing_name":
				e.Result = json.RawMessage("[" + strings.Replace(row, `"name":"합성 &amp; 청구",`, "", 1) + "]")
			case "bad_id":
				e.Result = json.RawMessage("[" + strings.Replace(row, "BILL-001", "../other", 1) + "]")
			case "duplicate":
				e = paged(1, row, row)
			case "count":
				e.Count = json.RawMessage(`2`)
			case "missing_count":
				e.Count = nil
			case "page":
				e.Page = json.RawMessage(`1`)
			case "total":
				e.Total = json.RawMessage(`1`)
			case "page_no":
				e.PageNo = json.RawMessage(`2`)
			case "page_size":
				e.PageSize = json.RawMessage(`1`)
			case "noncanonical_page":
				e.PageNo = json.RawMessage(`"01"`)
			case "status":
				e.Status = 202
			}
			f := &fake{call: func(string, url.Values) (client.Envelope, error) { return e, nil }}
			filter := Filter{}
			if mode == "filter_mismatch" {
				filter.StartDate = "2026-03-01"
			}
			if rows, err := (&Service{API: f}).List(context.Background(), filter); err == nil || rows != nil {
				t.Fatal("invalid bill accepted")
			}
		})
	}
	for _, raw := range []string{`null`, `[null]`, `[{}]`, "[" + strings.Replace(current, `"price":9007199254740993`, `"price":null`, 1) + "]", "[" + current + "," + current + "]"} {
		f := &fake{call: func(string, url.Values) (client.Envelope, error) {
			e := envelope(current)
			e.Result = json.RawMessage(raw)
			if strings.Contains(raw, "},{") {
				e.Count = json.RawMessage(`2`)
			}
			return e, nil
		}}
		if _, err := (&Service{API: f}).Current(context.Background()); err == nil {
			t.Fatal("malformed current bill accepted")
		}
	}
}
func TestInvalidFiltersAndCancellation(t *testing.T) {
	one, two := int64(1), int64(2)
	f := &fake{call: func(string, url.Values) (client.Envelope, error) {
		t.Fatal("invalid filter reached network")
		return client.Envelope{}, nil
	}}
	for _, filter := range []Filter{{StartDate: "2026-02-30"}, {EndDate: "2026-1-01"}, {StartDate: "2026-02-02", EndDate: "2026-02-01"}, {MinimumPrice: &two, MaximumPrice: &one}} {
		if _, err := (&Service{API: f}).List(context.Background(), filter); err == nil {
			t.Fatal("invalid filter accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&Service{API: f}).List(ctx, Filter{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost")
	}
}
