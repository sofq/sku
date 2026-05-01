package kinds

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// MessagingQueueSpec captures the messaging.queue equivalence shape.
//
// Mode filters by extra.mode ("standard" | "fifo" | "premium" | "throughput").
// Leave empty to return all modes.
type MessagingQueueSpec struct {
	Mode     string
	MaxPrice float64
	Regions  []string
}

// QueryMessagingQueue runs the messaging.queue equivalence query against a
// single shard and returns rows with prices populated.
// Term pin: commitment='on_demand'. Mode is optionally filtered via
// json_extract(extra, '$.mode').
//
// When Mode is empty and the result mixes price units across the count-,
// hour-, and byte-families (e.g. SQS per-million-requests vs Azure Service
// Bus mu/hour vs Pub/Sub per-GiB-month), a warning is logged to stderr
// because aggregating across these axes is not meaningful.
func QueryMessagingQueue(ctx context.Context, c *catalog.Catalog, spec MessagingQueueSpec) ([]catalog.Row, error) {
	where := []string{
		"s.kind = 'messaging.queue'",
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
		return nil, fmt.Errorf("compare: messaging_queue query: %w", err)
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

	if spec.Mode == "" {
		warnIfMixedMessagingUnits(out, "messaging.queue")
	}
	return out, nil
}
