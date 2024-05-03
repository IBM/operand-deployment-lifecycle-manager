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
	"context"
	"strings"
	"time"

	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
)

// Reconciler reconciles a OperatorChecker object
type Reconciler struct {
	*deploy.ODLMOperator
}

// Reconcile watchs on the Subscription of the target namespace and apply the recovery to fixing the Subscription failed error
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
	subscriptionInstance, err := r.getSubscription(ctx, req)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(2).Info("Operator Checker is monitoring Subscription...")

	if _, ok := subscriptionInstance.Labels[constant.OpreqLabel]; !ok {
		return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
	}

	if subscriptionInstance.Status.CurrentCSV == "" && subscriptionInstance.Status.State == "" {
		// cover fresh install case
		csvList, err := r.getCSVBySubscription(ctx, subscriptionInstance)
		if err != nil {
			klog.Error(err)
			return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
		}
		if len(csvList) != 1 {
			klog.Warning("Not found matched CSV, CSVList length: ", len(csvList))
			return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
		}
		csv := csvList[0]

		time.Sleep(constant.DefaultCSVWaitPeriod)
		subscriptionInstance, err := r.getSubscription(ctx, req)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		if subscriptionInstance.Status.CurrentCSV == "" && subscriptionInstance.Status.State == "" {
			if err = r.deleteCSV(ctx, csv.Name, csv.Namespace); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}
	}

	if subscriptionInstance.Status.State == "UpgradePending" && subscriptionInstance.Status.CurrentCSV != "" && subscriptionInstance.Status.InstalledCSV != "" {
		// cover upgrade case
		csvList, err := r.getCSVBySubscription(ctx, subscriptionInstance)
		if err != nil {
			klog.Error(err)
			return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
		}
		if len(csvList) != 1 {
			klog.Warning("Not found matched CSV, CSVList length: ", len(csvList))
			return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
		}
		csv := csvList[0]

		currentCSVVersion := ""
		if len(strings.SplitN(subscriptionInstance.Status.CurrentCSV, ".", 2)) == 2 {
			currentCSVVersion = strings.SplitN(subscriptionInstance.Status.CurrentCSV, ".", 2)[1]
		}

		if currentCSVVersion != csv.Spec.Version.String() {
			time.Sleep(constant.DefaultCSVWaitPeriod)
			subscriptionInstance, err := r.getSubscription(ctx, req)
			if err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			if subscriptionInstance.Status.State == "UpgradePending" && subscriptionInstance.Status.CurrentCSV != "" && subscriptionInstance.Status.InstalledCSV != "" {
				if err = r.deleteCSV(ctx, csv.Name, csv.Namespace); err != nil {
					return ctrl.Result{}, client.IgnoreNotFound(err)
				}
			}
		}
	}

	return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
}

func (r *Reconciler) getCSVBySubscription(ctx context.Context, subscriptionInstance *olmv1alpha1.Subscription) ([]olmv1alpha1.ClusterServiceVersion, error) {
	csvList := &olmv1alpha1.ClusterServiceVersionList{}
	opts := []client.ListOption{
		client.InNamespace(subscriptionInstance.Namespace),
	}
	if err := r.Client.List(ctx, csvList, opts...); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	var matchCSVList = []olmv1alpha1.ClusterServiceVersion{}
	for _, csv := range csvList.Items {
		csvPackageName := csv.GetAnnotations()["operatorframework.io/properties"]
		if strings.Contains(csvPackageName, subscriptionInstance.Spec.Package) {
			matchCSVList = append(matchCSVList, csv)
		}
	}
	return matchCSVList, nil
}

func (r *Reconciler) getSubscription(ctx context.Context, req ctrl.Request) (*olmv1alpha1.Subscription, error) {
	// Fetch the subscription instance
	subscriptionInstance := &olmv1alpha1.Subscription{}
	if err := r.Client.Get(ctx, req.NamespacedName, subscriptionInstance); err != nil {
		return nil, err
	}
	return subscriptionInstance, nil
}

func (r *Reconciler) deleteCSV(ctx context.Context, name, namespace string) error {
	csvInstance := &olmv1alpha1.ClusterServiceVersion{}
	csvInstance.Name = name
	csvInstance.Namespace = namespace
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
