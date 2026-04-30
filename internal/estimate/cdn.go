package estimate

import (
	"context"
	"errors"
)

// NetworkCDNTier0Estimator handles network.cdn kind estimation (tier-0 only).
type NetworkCDNTier0Estimator struct{}

// Kind returns the kind string this estimator handles.
func (NetworkCDNTier0Estimator) Kind() string { return "network.cdn" }

// Estimate is a stub; full implementation is deferred to M-δ Phase 4.
func (NetworkCDNTier0Estimator) Estimate(_ context.Context, _ Item) (LineItem, error) {
	return LineItem{}, errors.New("estimator not yet implemented; M-δ Phase 4")
}

func init() {
	Register(NetworkCDNTier0Estimator{})
}
