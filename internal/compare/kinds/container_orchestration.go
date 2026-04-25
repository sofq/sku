package kinds

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

// ContainerOrchestrationSpec captures the container.orchestration equivalence shape.
//
// Mode defaults to "control-plane" so the most common compare query returns flat
// cluster-hour rows without confusing per-pod / per-vCPU-second pricing.
// Pass an explicit Mode = "autopilot" / "fargate" / "virtual-nodes" to query serverless rows.
//
// Tier filters terms.os exact-match. Unknown tier returns empty rows.
type ContainerOrchestrationSpec struct {
	Mode     string   // default "control-plane"; also "fargate" | "autopilot" | "virtual-nodes"
	Tier     string   // optional terms.os filter
	MaxPrice float64
	Regions  []string
}

const defaultContainerMode = "control-plane"

// QueryContainerOrchestration runs the container.orchestration equivalence query.
func QueryContainerOrchestration(ctx context.Context, c *catalog.Catalog, spec ContainerOrchestrationSpec) ([]catalog.Row, error) {
	where := []string{
		"s.kind = 'container.orchestration'",
		"t.commitment = 'on_demand'",
	}
	var args []any

	mode := spec.Mode
	if mode == "" {
		mode = defaultContainerMode
	}
	// SQLite (modernc.org/sqlite) ships json1: json_extract on a string-valued
	// JSON key returns the string directly, so equality compares cleanly.
	where = append(where, "json_extract(ra.extra, '$.mode') = ?")
	args = append(args, mode)

	if spec.Tier != "" {
		where = append(where, "t.os = ?")
		args = append(args, spec.Tier)
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
		return nil, fmt.Errorf("compare: container_orchestration query: %w", err)
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
	return out, rs.Err()
}
