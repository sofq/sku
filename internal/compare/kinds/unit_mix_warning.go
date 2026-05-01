package kinds

import (
	"fmt"

	"github.com/sofq/sku/internal/catalog"
)

// classifyUnitFamily groups a price unit into one of four families used by
// the unit-mix warnings. Returns "" for units that are not relevant to the
// detection (e.g. tokens, queries) so they do not trigger a warning.
func classifyUnitFamily(unit string) string {
	switch unit {
	case "1M-req", "request", "requests":
		return "count"
	case "hr", "hour":
		return "hour"
	case "gb", "gb-mo", "gib-mo":
		return "byte"
	case "month":
		return "month"
	}
	return ""
}

// warnIfMixedMessagingUnits emits a warning when a messaging.{queue,topic}
// compare result mixes pricing axes (count, hour, byte, month). These axes
// are not commensurable and the MIN(amount) sort would otherwise rank
// (e.g.) Pub/Sub's per-GiB price below SQS's per-million-request price.
// Today's messaging shards only emit count/hour/byte; month is included
// for symmetry with the CDN warning so a future base-fee dimension would
// be caught automatically.
func warnIfMixedMessagingUnits(rows []catalog.Row, kind string) {
	families := map[string]bool{}
	for _, row := range rows {
		for _, p := range row.Prices {
			if fam := classifyUnitFamily(p.Unit); fam != "" {
				families[fam] = true
			}
		}
	}
	if len(families) > 1 {
		_, _ = fmt.Fprintf(warningWriter,
			"warning: %s result mixes per-call, per-unit-hour, and per-byte pricing; pass --mode to narrow\n",
			kind)
	}
}

// warnIfMixedCDNUnits emits a warning when a network.cdn compare result
// mixes byte-, count-, and month-domain pricing rows (egress vs requests
// vs Front Door base fee). Same rationale as the messaging warning.
func warnIfMixedCDNUnits(rows []catalog.Row) {
	families := map[string]bool{}
	for _, row := range rows {
		for _, p := range row.Prices {
			if fam := classifyUnitFamily(p.Unit); fam != "" {
				families[fam] = true
			}
		}
	}
	if len(families) > 1 {
		_, _ = fmt.Fprintln(warningWriter,
			"warning: network.cdn result mixes per-byte, per-call, and per-month pricing; pass --mode to narrow")
	}
}
