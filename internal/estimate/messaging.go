package estimate

import (
	"context"
	"errors"
)

// MessagingQueueEstimator handles messaging.queue kind estimation.
type MessagingQueueEstimator struct{}

// Kind returns the kind string this estimator handles.
func (MessagingQueueEstimator) Kind() string { return "messaging.queue" }

// Estimate is a stub; full implementation is deferred to M-δ Phase 4.
func (MessagingQueueEstimator) Estimate(_ context.Context, _ Item) (LineItem, error) {
	return LineItem{}, errors.New("estimator not yet implemented; M-δ Phase 4")
}

// MessagingTopicEstimator handles messaging.topic kind estimation.
// Full implementation deferred to M-ε.
type MessagingTopicEstimator struct{}

// Kind returns the kind string this estimator handles.
func (MessagingTopicEstimator) Kind() string { return "messaging.topic" }

// Estimate is a stub; full implementation is deferred to M-ε.
func (MessagingTopicEstimator) Estimate(_ context.Context, _ Item) (LineItem, error) {
	return LineItem{}, errors.New("estimator for messaging.topic deferred to M-ε")
}

func init() {
	Register(MessagingQueueEstimator{})
	Register(MessagingTopicEstimator{})
}
