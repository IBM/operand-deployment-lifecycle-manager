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
	"fmt"

	gset "github.com/deckarep/golang-set"
	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileOperandRequest) reconcileOperator(requestInstance *operatorv1alpha1.OperandRequest, reconcileReq reconcile.Request) error {
	klog.V(1).Info("Reconciling Operator")
	for _, req := range requestInstance.Spec.Requests {
		registryInstance, err := r.getRegistryInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return err
		}
		for _, operand := range req.Operands {
			// Check the requested Operand if exist in specific OperandRegistry
			opt := r.getOperatorFromRegistryInstance(operand.Name, registryInstance)
			if opt != nil {
				// Check subscription if exist
				found, err := r.olmClient.OperatorsV1alpha1().Subscriptions(opt.Namespace).Get(opt.Name, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						// Subscription does not exist, create a new one
						if err = r.createSubscription(requestInstance, opt); err != nil {
							return err
						}
						continue
					}
					return err
				}
				// Subscription existing and managed by OperandRequest controller
				if _, ok := found.Labels["operator.ibm.com/opreq-control"]; ok {
					// Subscription channel changed, update it.
					if found.Spec.Channel != opt.Channel {
						found.Spec.Channel = opt.Channel
						if err = r.updateSubscription(requestInstance, found); err != nil {
							return err
						}
					}
				} else {
					// Subscription existing and not managed by OperandRequest controller
					klog.V(3).Info("Subscription has created by other user, ignore update/delete it.","Subscription.Namespace", found.Namespace, "Subscription.Name", found.Name)
				}
			}
		}
		// SetCondition for notfind registry
	}

	// Delete specific operators
	needDeletedOperands, err := r.getNeedDeletedOperands(requestInstance, reconcileReq)
	if err != nil {
		return err
	}
	for _, req := range requestInstance.Spec.Requests {
		registryInstance, err := r.getRegistryInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return err
		}
		configInstance, err := r.getConfigInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return err
		}
		for o := range needDeletedOperands.Iter() {
			operandN := fmt.Sprintf("%v", o)
			fmt.Println(operandN)
			if err := r.deleteSubscription(fmt.Sprintf("%v", o), requestInstance, registryInstance, configInstance, reconcileReq); err != nil {
				return err
			}
		}
	}

	if err := r.updateMemberStatus(requestInstance); err != nil {
		return err
	}
	if err := r.updateClusterPhase(requestInstance); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) createSubscription(cr *operatorv1alpha1.OperandRequest, opt *operatorv1alpha1.Operator) error {
	klog.V(2).Info("Subscription.Namespace", opt.Namespace)
	co := generateClusterObjects(opt)

	// Create required namespace
	ns := co.namespace
	klog.V(3).Info("Creating the Namespace for Subscription: " + opt.Name)
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
		klog.V(3).Info("Creating the OperatorGroup for Subscription: " + opt.Name)
		_, err := r.olmClient.OperatorsV1().OperatorGroups(og.Namespace).Create(og)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// Create subscription
	klog.V(3).Info("Creating the Subscription: " + opt.Name)
	sub := co.subscription
	cr.SetCreatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub)
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

	klog.Info("Updating Subscription", "Subscription.Namespace", sub.Namespace, "Subscription.Name", sub.Name)
	cr.SetUpdatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	if _, err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Update(sub); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) deleteSubscription(operandName string, requestInstance *operatorv1alpha1.OperandRequest, registryInstance *operatorv1alpha1.OperandRegistry, configInstance *operatorv1alpha1.OperandConfig, reconcileReq reconcile.Request) error {
	klog.V(2).Info("Subscription.Name", operandName)
	config := r.getServiceFromConfigInstance(operandName, configInstance)
	opt := r.getOperatorFromRegistryInstance(operandName, registryInstance)

	csv, err := r.getClusterServiceVersion(operandName)
	// If can't get CSV, requeue the request
	if err != nil {
		return err
	}

	if csv != nil {
		klog.V(3).Info("Deleting a Custom Resource")
		if err := r.deleteCr(config, csv, configInstance); err != nil {
			return err
		}
		klog.V(3).Info("Deleting the ClusterServiceVersion")
		requestInstance.SetDeletingCondition(csv.Name, operatorv1alpha1.ResourceTypeCsv)
		if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
			return err
		}
		if err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(csv.Namespace).Delete(csv.Name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	if opt != nil {
		klog.V(3).Info("Deleting the Subscription")
		requestInstance.SetDeletingCondition(opt.Name, operatorv1alpha1.ResourceTypeSub)
		if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
			return err
		}
		if err := r.olmClient.OperatorsV1alpha1().Subscriptions(opt.Namespace).Delete(opt.Name, &metav1.DeleteOptions{}); err != nil {
			return client.IgnoreNotFound(err)
		}

		requestInstance.CleanMemberStatus(opt.Name)
		if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
			return err
		}
		if err := r.deleteRegistryStatus(registryInstance, reconcileReq, opt.Name); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileOperandRequest) getNeedDeletedOperands(requestInstance *operatorv1alpha1.OperandRequest, reconcileReq reconcile.Request) (gset.Set, error) {
	requestOperands := gset.NewSet()
	for _, req := range requestInstance.Spec.Requests {
		for _, o := range req.Operands {
			requestOperands.Add(o.Name)
		}
	}

	deployedOperands, err := r.getDeployedOperands(requestInstance, reconcileReq)
	if err != nil {
		return nil, err
	}
	needDeleteOperands := deployedOperands.Difference(requestOperands)
	return needDeleteOperands, nil
}

func (r *ReconcileOperandRequest) getDeployedOperands(requestInstance *operatorv1alpha1.OperandRequest, reconcileReq reconcile.Request) (gset.Set, error) {
	deployedOperands := gset.NewSet()
	for _, req := range requestInstance.Spec.Requests {
		registryInstance, err := r.getRegistryInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return nil, err
		}

		for name, status := range registryInstance.Status.OperatorsStatus {
			if status.Phase != operatorv1alpha1.OperatorReady {
				if pos := registryInstance.GetReconcileRequest(name, reconcileReq); pos != -1 {
					deployedOperands.Add(name)
				}
			}
		}
	}

	return deployedOperands, nil
}

func generateClusterObjects(o *operatorv1alpha1.Operator) *clusterObjects {
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

// Get the OperandRegistry instance with the name and namespace
func (r *ReconcileOperandRequest) getRegistryInstance(name, namespace string) (*operatorv1alpha1.OperandRegistry, error) {
	reg := &operatorv1alpha1.OperandRegistry{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, reg); err != nil {
		return nil, err
	}
	return reg, nil
}

func (r *ReconcileOperandRequest) getOperatorFromRegistryInstance(operandName string, registryInstance *operatorv1alpha1.OperandRegistry) *operatorv1alpha1.Operator {
	for _, o := range registryInstance.Spec.Operators {
		if o.Name == operandName {
			return &o
		}
	}
	return nil
}
