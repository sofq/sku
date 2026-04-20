package catalog

import "time"

// Age returns the integer number of days between the shard's
// metadata.generated_at and now. Missing or unparseable values return 0
// so callers degrade gracefully (no false stale-error exit).
func (c *Catalog) Age(now time.Time) int {
	if c.generatedAt == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, c.generatedAt)
	if err != nil {
		return 0
	}
	diff := now.Sub(t).Hours() / 24
	if diff < 0 {
		return 0
	}
	return int(diff)
}
