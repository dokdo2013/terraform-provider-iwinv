// contract-probe issues one read-only request and prints only allowlisted shape
// metadata. It is not a provider, an inventory export, or an acceptance test.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/dokdo2013/terraform-provider-iwinv/internal/client"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, out io.Writer) error {
	c, err := client.New(getenv("IWINV_ACCESS_KEY"), getenv("IWINV_SECRET_KEY"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	envelope, err := c.Get(ctx, "/v1/zones", nil)
	if err != nil {
		return err
	}
	return report(out, envelope)
}

func report(out io.Writer, envelope client.Envelope) error {
	if envelope.Status != 200 {
		return fmt.Errorf("zones response has unexpected HTTP status %d", envelope.Status)
	}
	var rows []map[string]json.RawMessage
	if len(envelope.Result) == 0 || string(envelope.Result) == "null" || json.Unmarshal(envelope.Result, &rows) != nil {
		return fmt.Errorf("zones result is not an array of objects; contract requires private investigation")
	}
	shapes := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			return fmt.Errorf("zones result contains a null object")
		}
		// Do not output dynamic field names, IDs, names or other remote values.
		shapes = append(shapes, map[string]string{"zone_id": valueType(row["zone_id"]), "zone_name": valueType(row["zone_name"]), "status": valueType(row["status"])})
	}
	return json.NewEncoder(out).Encode(struct {
		Operation   string              `json:"operation"`
		Observation string              `json:"observation"`
		CountType   string              `json:"count_type"`
		PageType    string              `json:"page_type"`
		Rows        []map[string]string `json:"row_shapes"`
	}{"GET /v1/zones", "authenticated_read_only_shape", valueType(envelope.Count), valueType(envelope.Page), shapes})
}

func valueType(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "missing"
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return "invalid"
	}
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "invalid"
	}
}
