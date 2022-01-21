//
// Copyright 2021 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package operatorchecker

import (
	"context"
	"strings"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
)

// Reconciler reconciles a OperatorChecker object
type Reconciler struct {
	*deploy.ODLMOperator
}

// Reconcile watchs on the Subscription of the target namespace and apply the recovery to fixing the Subscription failed error
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
	// Fetch the subscription instance
	subscriptionInstance := &olmv1alpha1.Subscription{}
	if err := r.Client.Get(ctx, req.NamespacedName, subscriptionInstance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(2).Info("Operator Checker is monitoring Subscription...")

	if _, ok := subscriptionInstance.Labels[constant.OpreqLabel]; !ok {
		return
	}
	if subscriptionInstance.Status.State != "" {
		return
	}
	for _, condition := range subscriptionInstance.Status.Conditions {
		if condition.Type == "ResolutionFailed" && condition.Reason == "ConstraintsNotSatisfiable" {
			csvList := &olmv1alpha1.ClusterServiceVersionList{}
			opts := []client.ListOption{
				client.InNamespace(subscriptionInstance.Namespace),
			}
			if err := r.Client.List(ctx, csvList, opts...); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			for _, csv := range csvList.Items {
				if strings.Contains(csv.Name, subscriptionInstance.Name) {
					if csv.Status.Phase == "Succeeded" {
						csvInstance := &olmv1alpha1.ClusterServiceVersion{}
						csvKey := types.NamespacedName{
							Name:      csv.Name,
							Namespace: csv.Namespace,
						}
						if err := r.Client.Get(ctx, csvKey, csvInstance); err != nil {
							return ctrl.Result{}, client.IgnoreNotFound(err)
						}
						r.Client.Delete(ctx, csvInstance)
					}
					break
				}
			}
			break
		}
	}
	return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
}

// SetupWithManager adds subscription to watch to the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.Subscription{}).Complete(r)
}
