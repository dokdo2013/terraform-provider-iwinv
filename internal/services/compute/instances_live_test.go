package compute

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

// Read-only and intentionally separate from populated detail/lifecycle acceptance.
// Run with private output; the only observation logged here is the row count.
func TestAccInstanceListRead(t *testing.T) {
	if os.Getenv("TF_ACC") != "1" || os.Getenv("IWINV_LIVE_READ") != "1" {
		t.Skip("set TF_ACC=1 and IWINV_LIVE_READ=1")
	}
	c, err := client.New(os.Getenv("IWINV_ACCESS_KEY"), os.Getenv("IWINV_SECRET_KEY"))
	if err != nil {
		t.Fatal("private credentials required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s := Service{API: c}
	rows, err := s.Instances(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rows == nil {
		t.Fatal("instance list returned a nil result")
	}
	t.Logf("API-visible instance count: %d; this is not detail or lifecycle acceptance", len(rows))
}
