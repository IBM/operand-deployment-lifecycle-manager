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

package operandrequest

import (
	"context"

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileOperandRequest) reconcileOperator(opts map[string]operatorv1alpha1.Operator, requestInstance *operatorv1alpha1.OperandRequest, moc *operatorv1alpha1.OperandRegistry, serviceConfigs map[string]operatorv1alpha1.ConfigService, csc *operatorv1alpha1.OperandConfig) error {
	reqLogger := log.WithValues()
	reqLogger.Info("Reconciling Operator")
	for _, o := range opts {
		// Check subscription if exist
		found, err := r.olmClient.OperatorsV1alpha1().Subscriptions(o.Namespace).Get(o.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// Subscription does not exist and state is present, create a new one
				if o.State == Present {
					if err = r.createSubscription(requestInstance, o); err != nil {
						return err
					}
				}
				continue
			}
			return err
		}

		// Subscription existing and managed by Request controller
		if _, ok := found.Labels["operator.ibm.com/opreq-control"]; ok {
			// Check subscription if present
			if o.State == Present {
				// Subscription is present and channel changed, update it.
				if found.Spec.Channel != o.Channel || found.Spec.CatalogSource != o.SourceName || found.Spec.CatalogSourceNamespace != o.SourceNamespace {
					found.Spec.Channel = o.Channel
					found.Spec.CatalogSource = o.SourceName
					found.Spec.CatalogSourceNamespace = o.SourceNamespace
					if err = r.updateSubscription(requestInstance, found); err != nil {
						return err
					}
				}
				// Subscription is absent, delete it.
			} else if err := r.deleteSubscription(requestInstance, found, moc, serviceConfigs, csc); err != nil {
				return err
			}
		} else {
			// Subscription existing and not managed by Set controller
			reqLogger.WithValues("Subscription.Namespace", found.Namespace, "Subscription.Name", found.Name).Info("Subscription has created by other user, ignore update/delete it.")
		}

		if err := r.updateMemberStatus(requestInstance); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileOperandRequest) fetchOperators(moc *operatorv1alpha1.OperandRegistry, cr *operatorv1alpha1.OperandRequest) (map[string]operatorv1alpha1.Operator, error) {

	setMap, err := r.fetchRequests(cr)
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

func (r *ReconcileOperandRequest) createSubscription(cr *operatorv1alpha1.OperandRequest, opt operatorv1alpha1.Operator) error {
	logger := log.WithValues("Subscription.Namespace", opt.Namespace)
	co := generateClusterObjects(opt)

	// Create required namespace
	ns := co.namespace
	logger.Info("Creating the Namespace for Subscription: " + opt.Name)
	if err := r.client.Create(context.TODO(), ns); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create required operatorgroup
	existOG, err := r.olmClient.OperatorsV1().OperatorGroups(co.operatorGroup.Namespace).List(metav1.ListOptions{})

	if err != nil {
		return err
	}
	if len(existOG.Items) == 0 {
		og := co.operatorGroup
		logger.Info("Creating the OperatorGroup for Subscription: " + opt.Name)
		_, err := r.olmClient.OperatorsV1().OperatorGroups(og.Namespace).Create(og)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// Create subscription
	logger.Info("Creating the Subscription: " + opt.Name)
	sub := co.subscription
	cr.Status.SetCreatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	_, err = r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Create(sub)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) updateSubscription(cr *operatorv1alpha1.OperandRequest, sub *olmv1alpha1.Subscription) error {
	logger := log.WithValues("Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name)

	logger.Info("Updating Subscription")
	cr.Status.SetUpdatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	if _, err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Update(sub); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) deleteSubscription(cr *operatorv1alpha1.OperandRequest, sub *olmv1alpha1.Subscription, moc *operatorv1alpha1.OperandRegistry, serviceConfigs map[string]operatorv1alpha1.ConfigService, csc *operatorv1alpha1.OperandConfig) error {
	logger := log.WithValues("Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name)
	installedCsv := sub.Status.InstalledCSV
	logger.Info("Deleting a Custom Resource")
	csv, err := r.getClusterServiceVersion(sub.Name)
	// If can't get CSV, requeue the request
	if err != nil {
		if updateErr := r.updateConditionStatus(cr, sub.Name, DeleteFailed); updateErr != nil {
			return updateErr
		}
		return err
	}

	if csv != nil {
		if err := r.deleteCr(serviceConfigs[sub.Name], csv, csc); err != nil {
			if updateErr := r.updateConditionStatus(cr, sub.Name, DeleteFailed); updateErr != nil {
				return updateErr
			}
			return err
		}
	}
	

	logger.Info("Deleting the Subscription")
	cr.Status.SetDeletingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	if err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{}); err != nil {
		return err
	}
	logger.Info("Deleting the ClusterServiceVersion")
	cr.Status.SetDeletingCondition(installedCsv, operatorv1alpha1.ResourceTypeCsv)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	if err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(sub.Namespace).Delete(installedCsv, &metav1.DeleteOptions{}); err != nil {
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
		"operator.ibm.com/opreq-control": "true",
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
		"operator.ibm.com/opreq-control": "true",
	}
	if targetNamespaces == nil {
		targetNamespaces = append(targetNamespaces, namespace)
	}
	// Operator Group Object
	og := &olmv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operand-deployment-lifecycle-manager-operatorgroup",
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
