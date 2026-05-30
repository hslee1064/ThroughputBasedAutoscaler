package controller

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func NewInferenceEventSource(client client.Client) (*InferenceEventSource, error) {
	return &InferenceEventSource{client: client}, nil
}

type InferenceEventSource struct {
	client client.Client
}

func (s *InferenceEventSource) Start(ctx context.Context, eh handler.EventHandler, _ workqueue.RateLimitingInterface, _ ...predicate.Predicate) error {
	return nil
}

func (s *InferenceEventSource) WaitForSync(_ context.Context) error {
	return nil
}
