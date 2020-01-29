//
// Copyright 2020 IBM Corporation
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

package commonserviceset

import (
	"context"

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1alpha1 "github.com/IBM/common-service-operator/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileCommonServiceSet) reconcileMetaOperator(opts map[string]operatorv1alpha1.Operator, setInstance *operatorv1alpha1.CommonServiceSet, mo *operatorv1alpha1.MetaOperator) error {
	reqLogger := log.WithValues()
	reqLogger.Info("Reconciling MetaOperator")
	for _, o := range opts {
		// Check subscription if exist
		found, err := r.olmClient.OperatorsV1alpha1().Subscriptions(o.Namespace).Get(o.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// Subscription does not exist and state is present, create a new one
				if o.State == Present {
					if err = r.createSubscription(setInstance, o); err != nil {
						return err
					}
				}
				continue
			}
			return err
		}

		// Subscription existing and managed by Set controller
		if _, ok := found.Labels["operator.ibm.com/css-control"]; ok {
			// Check subscription if present
			if o.State == Present {
				// Subscription is present and channel changed, update it.
				if found.Spec.Channel != o.Channel {
					found.Spec.Channel = o.Channel
					if err = r.updateSubscription(setInstance, found); err != nil {
						return err
					}
				}
			} else {
				// // Subscription is absent, delete it.
				if err := r.deleteSubscription(setInstance, found, mo); err != nil {
					return err
				}
			}
		} else {
			// Subscription existing and not managed by Set controller
			reqLogger.WithValues("Subscription.Namespace", found.Namespace, "Subscription.Name", found.Name).Info("Subscription has created by other user, ignore create it.")
		}
	}
	return nil
}

func (r *ReconcileCommonServiceSet) fetchOperators(mo *operatorv1alpha1.MetaOperator, cr *operatorv1alpha1.CommonServiceSet) (map[string]operatorv1alpha1.Operator, error) {

	setMap, err := r.fetchSets(cr)
	if err != nil {
		return nil, err
	}

	optMap := make(map[string]operatorv1alpha1.Operator)
	for _, v := range mo.Spec.Operators {
		if _, ok := setMap[v.Name]; ok {
			if setMap[v.Name].Channel != "" && setMap[v.Name].Channel != v.Channel {
				v.Channel = setMap[v.Name].Channel
			}
			v.State = setMap[v.Name].State
		} else {
			v.State = Absent
		}
		optMap[v.Name] = v
	}
	return optMap, nil
}

func (r *ReconcileCommonServiceSet) createSubscription(cr *operatorv1alpha1.CommonServiceSet, opt operatorv1alpha1.Operator) error {
	logger := log.WithValues("Subscription.Namespace", opt.Namespace, "Subscription.Name", opt.Name)
	co := generateClusterObjects(opt)

	// Create required namespace
	ns := co.namespace
	if err := r.client.Create(context.TODO(), ns); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create required operatorgroup
	og := co.operatorGroup
	_, err := r.olmClient.OperatorsV1().OperatorGroups(og.Namespace).Create(og)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create subscription
	logger.Info("Creating a new Subscription")
	sub := co.subscription
	_, err = r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Create(sub)
	if err != nil && !errors.IsAlreadyExists(err) {
		if updateErr := r.updateConditionStatus(cr, sub.Name, InstallFailed); updateErr != nil {
			return updateErr
		}
		return err
	}

	if err = r.updateConditionStatus(cr, sub.Name, InstallSuccessed); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileCommonServiceSet) updateSubscription(cr *operatorv1alpha1.CommonServiceSet, sub *olmv1alpha1.Subscription) error {
	logger := log.WithValues("Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name)

	logger.Info("Updating Subscription")
	if _, err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Update(sub); err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, UpdateFailed); updateErr != nil {
			return updateErr
		}
		return err
	}

	if err := r.updateConditionStatus(cr, sub.Name, UpdateSuccessed); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileCommonServiceSet) deleteSubscription(cr *operatorv1alpha1.CommonServiceSet, sub *olmv1alpha1.Subscription, mo *operatorv1alpha1.MetaOperator) error {
	logger := log.WithValues("Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name)
	logger.Info("Deleting CSV related with Subscription")
	installedCsv := sub.Status.InstalledCSV
	if err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(sub.Namespace).Delete(installedCsv, &metav1.DeleteOptions{}); err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, DeleteFailed); updateErr != nil {
			return updateErr
		}
		return err
	}
	logger.Info("Deleting a Subscription")
	if err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{}); err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, DeleteFailed); updateErr != nil {
			return updateErr
		}
		return err
	}
	if err := r.updateConditionStatus(cr, sub.Name, DeleteSuccessed); err != nil {
		return err
	}
	if err := r.deleteOperatorStatus(mo, sub.Name); err != nil {
		return err
	}
	return nil
}

func generateClusterObjects(o operatorv1alpha1.Operator) *clusterObjects {
	co := &clusterObjects{}
	labels := map[string]string{
		"operator.ibm.com/css-control": "true",
	}
	// Namespace Object
	co.namespace = &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   o.Namespace,
			Labels: labels,
		},
	}

	// Operator Group Object
	og := &olmv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "common-service-operatorgroup",
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: o.TargetNamespaces,
		},
	}
	og.SetGroupVersionKind(schema.GroupVersionKind{Group: olmv1.SchemeGroupVersion.Group, Kind: "OperatorGroup", Version: olmv1.SchemeGroupVersion.Version})
	co.operatorGroup = og

	// Subscription Object
	installPlanApproval := olmv1alpha1.ApprovalAutomatic
	if o.InstallPlanApproval == "Manual" {
		installPlanApproval = olmv1alpha1.ApprovalManual
	}
	sub := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.Name,
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                o.Channel,
			Package:                o.PackageName,
			CatalogSource:          o.SourceName,
			CatalogSourceNamespace: o.SourceNamespace,
			InstallPlanApproval:    installPlanApproval,
		},
	}
	sub.SetGroupVersionKind(schema.GroupVersionKind{Group: olmv1alpha1.SchemeGroupVersion.Group, Kind: "Subscription", Version: olmv1alpha1.SchemeGroupVersion.Version})
	co.subscription = sub
	return co
}
