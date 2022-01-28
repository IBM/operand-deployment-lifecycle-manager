//
// Copyright 2022 IBM Corporation
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
	"fmt"
	"time"
	"errors"
	"context"
	"strings"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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

	if subscriptionInstance.Status.CurrentCSV == "" && subscriptionInstance.Status.State == "" {
		// cover fresh install case
		csv, err := r.getCSVBySubscription(ctx, subscriptionInstance)
		if err != nil {
			// not found csv
			klog.Error(err)
			return
		}
		err = wait.PollImmediate(time.Second*3, time.Minute*1, func() (bool, error) {
			if subscriptionInstance.Status.CurrentCSV == "" && subscriptionInstance.Status.State == "" {
				return false, nil
			}
			return true, nil
		})
		if subscriptionInstance.Status.CurrentCSV == "" && subscriptionInstance.Status.State == "" {
			r.deleteCSV(ctx, csv.Name, csv.Namespace)
		}
	}

	if subscriptionInstance.Status.CurrentCSV != "" && subscriptionInstance.Status.State == "UpgradePending" {
		// cover upgrade case
		csv, err := r.getCSVBySubscription(ctx, subscriptionInstance)
		if err != nil {
			// get multi versions of CSV
			klog.Error(err)
			return
		}
		if subscriptionInstance.Status.CurrentCSV != fmt.Sprintf("%v", csv.Spec.Version) {
			err = wait.PollImmediate(time.Second*3, time.Minute*1, func() (bool, error) {
				if subscriptionInstance.Status.CurrentCSV != "" && subscriptionInstance.Status.State == "UpgradePending" {
					return false, nil
				}
				return true, nil
			})
			if subscriptionInstance.Status.CurrentCSV != "" && subscriptionInstance.Status.State == "UpgradePending" {
				r.deleteCSV(ctx, csv.Name, csv.Namespace)
			}
		}
	}

	return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
}

func (r *Reconciler) getCSVBySubscription(ctx context.Context, subscriptionInstance *olmv1alpha1.Subscription) (*olmv1alpha1.ClusterServiceVersion, error) {
	csvList := &olmv1alpha1.ClusterServiceVersionList{}
	opts := []client.ListOption{
		client.InNamespace(subscriptionInstance.Namespace),
	}
	if err := r.Client.List(ctx, csvList, opts...); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	var matchCSVList = []olmv1alpha1.ClusterServiceVersion{}
	for _, csv := range csvList.Items {
		if strings.Contains(csv.Name, subscriptionInstance.Name) {
			matchCSVList = append(matchCSVList, csv)
		}
	}

	if len(matchCSVList) == 1 {
		return &matchCSVList[0], nil
	}
	return nil, errors.New("Fail to find matched CSV")
}

func (r *Reconciler) deleteCSV(ctx context.Context, name, namespace string) error {
	csvInstance := &olmv1alpha1.ClusterServiceVersion{}
	csvKey := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	if err := r.Client.Get(ctx, csvKey, csvInstance); err != nil {
		return client.IgnoreNotFound(err)
	}
	if err := r.Client.Delete(ctx, csvInstance); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

// SetupWithManager adds subscription to watch to the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&olmv1alpha1.Subscription{}).Complete(r)
}
