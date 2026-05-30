package controller

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type AutoscalerEventHandler struct {
}

func NewAutoscalerEventHandler() (*AutoscalerEventHandler, error) {
	return &AutoscalerEventHandler{}, nil
}

func (h *AutoscalerEventHandler) Create(c context.Context, e event.CreateEvent, _ workqueue.RateLimitingInterface) {
}

func (h *AutoscalerEventHandler) Update(c context.Context, e event.UpdateEvent, _ workqueue.RateLimitingInterface) {
}

func (h *AutoscalerEventHandler) Delete(c context.Context, e event.DeleteEvent, _ workqueue.RateLimitingInterface) {
}

func (h *AutoscalerEventHandler) Generic(c context.Context, _ event.GenericEvent, _ workqueue.RateLimitingInterface) {
}
