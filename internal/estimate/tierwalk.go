package estimate

import (
	"math"
	"sort"
)

// TierEntry represents one pricing tier with lower/upper bounds and a unit price.
type TierEntry struct {
	Lower  float64 // inclusive lower bound (from ParseCountTier or ParseBytesTier)
	Upper  float64 // exclusive upper bound; math.MaxFloat64 means unbounded
	Amount float64 // price per unit in this tier
}

// WalkTiers returns the total cost for the given volume by walking tiers
// ascending by Lower. Each tier contributes min(volume_in_tier, tier_width) * amount.
// Caller pre-converts prices[] to []TierEntry using the correct parser
// (ParseCountTier or ParseBytesTier), setting Upper = math.MaxFloat64
// when tier_upper == "".
func WalkTiers(entries []TierEntry, volume float64) float64 {
	if volume <= 0 || len(entries) == 0 {
		return 0
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Lower < entries[j].Lower
	})
	var total float64
	remaining := volume
	for _, e := range entries {
		if remaining <= 0 {
			break
		}
		if e.Upper == math.MaxFloat64 {
			// Unbounded tier — consume all remaining volume.
			total += remaining * e.Amount
			break
		}
		width := e.Upper - e.Lower
		used := width
		if remaining < width {
			used = remaining
		}
		total += used * e.Amount
		remaining -= used
	}
	return total
}
