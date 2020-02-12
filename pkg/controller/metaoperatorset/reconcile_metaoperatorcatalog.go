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

package metaoperatorset

import (
	"context"

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1alpha1 "github.com/IBM/meta-operator/pkg/apis/operator/v1alpha1"
	"github.com/IBM/meta-operator/pkg/util"
)

func (r *ReconcileMetaOperatorSet) reconcileMetaOperator(opts map[string]operatorv1alpha1.Operator, setInstance *operatorv1alpha1.MetaOperatorSet, moc *operatorv1alpha1.MetaOperatorCatalog) error {
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
		if _, ok := found.Labels["operator.ibm.com/mos-control"]; ok {
			// Check subscription if present
			if o.State == Present {
				// Subscription is present and channel changed, update it.
				if found.Spec.Channel != o.Channel || found.Spec.CatalogSource != o.SourceName || found.Spec.CatalogSourceNamespace != o.SourceNamespace {
					found.Spec.Channel = o.Channel
					found.Spec.CatalogSource = o.SourceName
					found.Spec.CatalogSourceNamespace = o.SourceNamespace
					if err = r.updateSubscription(setInstance, found); err != nil {
						return err
					}
				}
				if err := r.checkOperatorGroup(o.TargetNamespaces, o.Namespace); err != nil {
					return err
				}
				// Subscription is absent, delete it.
			} else if err := r.deleteSubscription(setInstance, found, moc); err != nil {
				return err
			}
		} else {
			// Subscription existing and not managed by Set controller
			reqLogger.WithValues("Subscription.Namespace", found.Namespace, "Subscription.Name", found.Name).Info("Subscription has created by other user, ignore create it.")
		}
	}
	return nil
}

func (r *ReconcileMetaOperatorSet) fetchOperators(moc *operatorv1alpha1.MetaOperatorCatalog, cr *operatorv1alpha1.MetaOperatorSet) (map[string]operatorv1alpha1.Operator, error) {

	setMap, err := r.fetchSets(cr)
	if err != nil {
		return nil, err
	}

	optMap := make(map[string]operatorv1alpha1.Operator)
	for _, v := range moc.Spec.Operators {
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

func (r *ReconcileMetaOperatorSet) createSubscription(cr *operatorv1alpha1.MetaOperatorSet, opt operatorv1alpha1.Operator) error {
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

func (r *ReconcileMetaOperatorSet) updateSubscription(cr *operatorv1alpha1.MetaOperatorSet, sub *olmv1alpha1.Subscription) error {
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

func (r *ReconcileMetaOperatorSet) deleteSubscription(cr *operatorv1alpha1.MetaOperatorSet, sub *olmv1alpha1.Subscription, moc *operatorv1alpha1.MetaOperatorCatalog) error {
	logger := log.WithValues("Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name)
	installedCsv := sub.Status.InstalledCSV
	logger.Info("Deleting a Subscription")
	if err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{}); err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, DeleteFailed); updateErr != nil {
			return updateErr
		}
		return err
	}
	logger.Info("Deleting CSV related with Subscription")
	if err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(sub.Namespace).Delete(installedCsv, &metav1.DeleteOptions{}); err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, DeleteFailed); updateErr != nil {
			return updateErr
		}
		return err
	}
	if err := r.updateConditionStatus(cr, sub.Name, DeleteSuccessed); err != nil {
		return err
	}
	if err := r.deleteOperatorStatus(moc, sub.Name); err != nil {
		return err
	}
	return nil
}

func generateClusterObjects(o operatorv1alpha1.Operator) *clusterObjects {
	co := &clusterObjects{}
	labels := map[string]string{
		"operator.ibm.com/mos-control": "true",
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
	og := generateOperatorGroup(o.Namespace, o.TargetNamespaces)
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

func generateOperatorGroup(namespace string, targetNamespaces []string) *olmv1.OperatorGroup {
	labels := map[string]string{
		"operator.ibm.com/mos-control": "true",
	}

	// Operator Group Object
	og := &olmv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "meta-operator-operatorgroup",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: olmv1.OperatorGroupSpec{
			TargetNamespaces: targetNamespaces,
		},
	}
	og.SetGroupVersionKind(schema.GroupVersionKind{Group: olmv1.SchemeGroupVersion.Group, Kind: "OperatorGroup", Version: olmv1.SchemeGroupVersion.Version})

	return og
}

func (r *ReconcileMetaOperatorSet) checkOperatorGroup(targetNamespaces []string, namespace string) error {
	existOG, err := r.olmClient.OperatorsV1().OperatorGroups(namespace).Get("meta-operator-operatorgroup", metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if util.Equal(existOG.Spec.TargetNamespaces, targetNamespaces) {
		return nil
	}

	if errors.IsNotFound(err) {
		og := generateOperatorGroup(namespace, targetNamespaces)
		_, err := r.olmClient.OperatorsV1().OperatorGroups(namespace).Create(og)
		if err != nil {
			return err
		}
	} else {
		existOG.Spec.TargetNamespaces = targetNamespaces
		_, err := r.olmClient.OperatorsV1().OperatorGroups(namespace).Update(existOG)
		if err != nil {
			return err
		}
	}
	return nil
}
