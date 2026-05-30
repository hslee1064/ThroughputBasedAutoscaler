/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	autoscalerv1 "github.com/hlee118/custom-autoscaler/api/v1"
	v1 "github.com/hlee118/custom-autoscaler/api/v1"
)

// AutoscalerReconciler reconciles a Autoscaler object
type AutoscalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=autoscaler.github.com,resources=autoscalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=autoscaler.github.com,resources=autoscalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=autoscaler.github.com,resources=autoscalers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Autoscaler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *AutoscalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	var autoscaler v1.Autoscaler
	if err := r.Get(ctx, req.NamespacedName, &autoscaler); err != nil {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AutoscalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	eventSource, err := NewAutoscalerEventSource(r.Client)
	if err != nil {
		return err
	}

	eventHandler, err := NewAutoscalerEventHandler()
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&autoscalerv1.Autoscaler{}).
		WatchesRawSource(eventSource, eventHandler).
		Complete(r)
}
