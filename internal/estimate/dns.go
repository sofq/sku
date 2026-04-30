package estimate

import (
	"context"
	"errors"
)

// DNSZoneEstimator handles dns.zone kind estimation.
type DNSZoneEstimator struct{}

// Kind returns the kind string this estimator handles.
func (DNSZoneEstimator) Kind() string { return "dns.zone" }

// Estimate is a stub; full implementation is deferred to M-δ Phase 4.
func (DNSZoneEstimator) Estimate(_ context.Context, _ Item) (LineItem, error) {
	return LineItem{}, errors.New("estimator not yet implemented; M-δ Phase 4")
}

func init() {
	Register(DNSZoneEstimator{})
}
