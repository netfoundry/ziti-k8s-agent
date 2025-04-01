/*
Copyright 2025 NetFoundry.

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

	kubernetesv1alpha1 "github.com/netfoundry/ziti-k8s-agent/ziti-agent/operator/api/v1alpha1"
)

// ZitiRouterReconciler reconciles a ZitiRouter object
type ZitiRouterReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	ZitiControllerChan chan kubernetesv1alpha1.ZitiController
}

// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitirouters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitirouters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=kubernetes.openziti.io,resources=zitirouters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ZitiRouter object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *ZitiRouterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var err error
	log.Info("ZitiRouter Reconciliation starting")

	zitirouter := &kubernetesv1alpha1.ZitiRouter{}
	if err = r.Get(ctx, req.NamespacedName, zitirouter); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ZitiRouterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubernetesv1alpha1.ZitiRouter{}).
		Complete(r)
}
