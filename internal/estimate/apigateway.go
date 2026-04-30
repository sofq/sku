package estimate

import (
	"context"
	"errors"
)

// APIGatewayEstimator handles api.gateway kind estimation.
type APIGatewayEstimator struct{}

// Kind returns the kind string this estimator handles.
func (APIGatewayEstimator) Kind() string { return "api.gateway" }

// Estimate is a stub; full implementation is deferred to M-δ Phase 4.
func (APIGatewayEstimator) Estimate(_ context.Context, _ Item) (LineItem, error) {
	return LineItem{}, errors.New("estimator not yet implemented; M-δ Phase 4")
}

func init() {
	Register(APIGatewayEstimator{})
}
