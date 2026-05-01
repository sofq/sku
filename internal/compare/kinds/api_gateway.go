package kinds

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// WarningWriter is the destination for compare-kind warnings (e.g. the
// api.gateway mixed-unit notice). Defaults to os.Stderr; tests swap it via
// SetWarningWriter so messages don't bleed into stdout JSON output.
var warningWriter io.Writer = os.Stderr

// SetWarningWriter overrides the default stderr destination. Returns the
// previous writer so callers can restore it (typical use: t.Cleanup).
func SetWarningWriter(w io.Writer) io.Writer {
	prev := warningWriter
	warningWriter = w
	return prev
}

// APIGatewaySpec captures the api.gateway equivalence shape.
//
// Mode filters by extra.mode
// ("rest" | "http" | "websocket" | "consumption" | "provisioned").
// Leave empty to return all modes; in that case, if the result set mixes
// per-call (1M-req) and per-unit-hour (hr) pricing rows, a warning is emitted
// to stderr advising the caller to pass --mode.
type APIGatewaySpec struct {
	Mode     string
	MaxPrice float64
	Regions  []string
}

// QueryAPIGateway runs the api.gateway equivalence query against a single
// shard and returns rows with prices populated.
// Term pin: commitment='on_demand'. Mode is optionally filtered via
// json_extract(extra, '$.mode').
//
// When Mode is empty and the result mixes rows with unit "1M-req" and rows
// with unit "hr", a warning is logged to stderr.
func QueryAPIGateway(ctx context.Context, c *catalog.Catalog, spec APIGatewaySpec) ([]catalog.Row, error) {
	where := []string{
		"s.kind = 'api.gateway'",
		"t.commitment = 'on_demand'",
	}
	var args []any

	if spec.Mode != "" {
		where = append(where, "json_extract(ra.extra, '$.mode') = ?")
		args = append(args, spec.Mode)
	}
	if spec.MaxPrice > 0 {
		where = append(where, "mp.min_price IS NOT NULL AND mp.min_price <= ?")
		args = append(args, spec.MaxPrice)
	}
	if len(spec.Regions) > 0 {
		placeholders := strings.Repeat("?,", len(spec.Regions))
		placeholders = placeholders[:len(placeholders)-1]
		where = append(where, "s.region IN ("+placeholders+")")
		for _, r := range spec.Regions {
			args = append(args, r)
		}
	}

	query := `
SELECT s.sku_id, s.provider, s.service, s.kind, s.resource_name, s.region,
       s.region_normalized, s.terms_hash,
       t.commitment, t.tenancy, t.os, t.support_tier, t.upfront, t.payment_option,
       ra.vcpu, ra.memory_gb, ra.storage_gb, ra.gpu_count, ra.gpu_model,
       ra.architecture, ra.extra,
       COALESCE(mp.min_price, 0) AS min_price
FROM skus s
JOIN terms t ON t.sku_id = s.sku_id
LEFT JOIN resource_attrs ra ON ra.sku_id = s.sku_id
LEFT JOIN (
  SELECT sku_id, MIN(amount) AS min_price FROM prices GROUP BY sku_id
) mp ON mp.sku_id = s.sku_id
WHERE ` + strings.Join(where, " AND ") + `
ORDER BY COALESCE(mp.min_price, 1e308) ASC, s.provider, s.resource_name, s.sku_id` //nolint:gosec // G202: WHERE composed from literals + placeholders only

	rs, err := c.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("compare: api_gateway query: %w", err)
	}
	defer func() { _ = rs.Close() }()
	var out []catalog.Row
	for rs.Next() {
		r, err := scanVMRow(rs)
		if err != nil {
			return nil, err
		}
		if err := c.FillPrices(ctx, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}

	// Warn when no mode filter is set and the result mixes per-call and
	// per-unit-hour pricing rows.
	if spec.Mode == "" {
		hasPerCall := false
		hasPerHour := false
		// Match both the canonical compare unit ("1M-req"/"hr") and the
		// raw ingest unit ("request"/"hour") so the warning fires regardless
		// of which normalisation stage was applied to the shard.
		for _, row := range out {
			for _, p := range row.Prices {
				switch p.Unit {
				case "1M-req", "request":
					hasPerCall = true
				case "hr", "hour":
					hasPerHour = true
				}
			}
		}
		if hasPerCall && hasPerHour {
			_, _ = fmt.Fprintln(warningWriter,
				"warning: api.gateway result mixes per-call and per-unit-hour pricing; pass --mode to narrow")
		}
	}

	return out, nil
}
