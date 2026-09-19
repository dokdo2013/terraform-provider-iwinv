package billing

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// Read only. Run with private logs; no payment instrument or receipt URL enters
// the typed models. The fixture account needs an existing bill for filters.
func TestAccBillingReads(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1")
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("private credentials required")
	}
	s := Service{API: c}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	current, err := s.Current(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != 1 || current[0].ID != "BILL-live" || current[0].Currency != "KRW" {
		t.Fatal("current billing contract changed")
	}
	rows, err := s.List(ctx, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) <= pageSize {
		t.Fatal("existing multi-page billing fixture required")
	}
	for i, row := range rows {
		if row.Currency != "KRW" || (i > 0 && rows[i-1].ID >= row.ID) {
			t.Fatal("bill currency or ordering contract changed")
		}
	}
	selected := rows[0]
	negative := int64(-1)
	filters := []Filter{
		{StartDate: selected.BillDate, EndDate: selected.BillDate, MinimumPrice: &selected.Price, MaximumPrice: &selected.Price},
		{StartDate: selected.BillDate}, {EndDate: selected.BillDate},
		{MinimumPrice: &selected.Price}, {MaximumPrice: &selected.Price},
		{MinimumPrice: &negative}, {MaximumPrice: &negative},
	}
	for _, filter := range filters {
		filtered, err := s.List(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		expected := map[string]bool{}
		for _, r := range rows {
			if (filter.StartDate == "" || r.BillDate >= filter.StartDate) && (filter.EndDate == "" || r.BillDate <= filter.EndDate) && (filter.MinimumPrice == nil || r.Price >= *filter.MinimumPrice) && (filter.MaximumPrice == nil || r.Price <= *filter.MaximumPrice) {
				expected[r.ID] = true
			}
		}
		if len(filtered) != len(expected) {
			t.Fatal("filtered results do not match complete baseline")
		}
		for _, r := range filtered {
			if !expected[r.ID] {
				t.Fatal("unexpected bill in filter")
			}
		}
	}
	empty, err := s.List(ctx, Filter{StartDate: "1900-01-01", EndDate: "1900-01-01"})
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("verified empty filter contract changed")
	}
}
