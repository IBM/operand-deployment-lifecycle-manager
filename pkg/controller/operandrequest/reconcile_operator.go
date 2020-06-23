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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileOperandRequest) reconcileOperator(requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(1).Info("Reconciling Operators")
	// Update request phase status
	defer func() {
		requestInstance.UpdateClusterPhase()
		if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
			klog.Error("Update request phase failed", err)
		}
	}()
	for _, req := range requestInstance.Spec.Requests {
		registryInstance, err := r.getRegistryInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			if errors.IsNotFound(err) {
				r.recorder.Eventf(requestInstance, corev1.EventTypeWarning, "NotFound", "NotFound OperandRegistry %s from the namespace %s", req.Registry, req.RegistryNamespace)
			}
			return err
		}
		for _, operand := range req.Operands {
			// Check the requested Operand if exist in specific OperandRegistry
			opt := registryInstance.GetOperator(operand.Name)
			if opt != nil {
				if opt.Scope == operatorv1alpha1.ScopePrivate && requestInstance.Namespace != registryInstance.Namespace {
					klog.Warningf("Operator %s is private. It can't be requested from namespace %s", operand.Name, requestInstance.Namespace)
					requestInstance.SetOutofScopeCondition(operand.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue)
					if updateErr := r.client.Status().Update(context.TODO(), requestInstance); updateErr != nil {
						return updateErr
					}
					continue
				}
				// Check subscription if exist
				found, err := r.olmClient.OperatorsV1alpha1().Subscriptions(opt.Namespace).Get(opt.Name, metav1.GetOptions{})
				if err != nil {
					if errors.IsNotFound(err) {
						// Subscription does not exist, create a new one
						if err = r.createSubscription(requestInstance, opt); err != nil {
							requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorFailed, "")
							return err
						}
						requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorInstalling, "")
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
							requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorFailed, "")
							return err
						}
						requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorUpdating, "")
					}
				} else {
					// Subscription existing and not managed by OperandRequest controller
					klog.V(2).Infof("Subscription %s in namespace %s isn't created by ODLM. Ignore update/delete it.", found.Name, found.Namespace)
				}
			} else {
				klog.V(2).Infof("Operator %s not found in the registry %s/%s", operand.Name, registryInstance.Namespace, registryInstance.Name)
				requestInstance.SetNotFoundOperatorFromRegistryCondition(operand.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue)
				if updateErr := r.client.Status().Update(context.TODO(), requestInstance); updateErr != nil {
					return updateErr
				}
			}

		}
		// SetCondition for notfind registry
	}

	// Delete specific operators
	needDeletedOperands, err := r.getNeedDeletedOperands(requestInstance)
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
			fmt.Println(o)
			if err := r.deleteSubscription(fmt.Sprintf("%v", o), requestInstance, registryInstance, configInstance); err != nil {
				return err
			}
		}
	}
	klog.V(1).Info("Finished reconciling Operators")

	return nil
}

func (r *ReconcileOperandRequest) createSubscription(cr *operatorv1alpha1.OperandRequest, opt *operatorv1alpha1.Operator) error {
	klog.V(3).Info("Subscription Namespace: ", opt.Namespace)
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
	klog.V(2).Info("Creating the Subscription: " + opt.Name)
	sub := co.subscription
	cr.SetCreatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	_, err = r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Create(sub)
	if err != nil && !errors.IsAlreadyExists(err) {
		cr.SetCreatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse)
		if updateErr := r.client.Status().Update(context.TODO(), cr); updateErr != nil {
			return updateErr
		}
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) updateSubscription(cr *operatorv1alpha1.OperandRequest, sub *olmv1alpha1.Subscription) error {

	klog.V(2).Info("Updating Subscription...", " Subscription Namespace: ", sub.Namespace, " Subscription Name: ", sub.Name)
	cr.SetUpdatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	if _, err := r.olmClient.OperatorsV1alpha1().Subscriptions(sub.Namespace).Update(sub); err != nil {
		cr.SetUpdatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse)
		if updateErr := r.client.Status().Update(context.TODO(), cr); updateErr != nil {
			return updateErr
		}
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) deleteSubscription(operandName string, requestInstance *operatorv1alpha1.OperandRequest, registryInstance *operatorv1alpha1.OperandRegistry, configInstance *operatorv1alpha1.OperandConfig) error {
	op := registryInstance.GetOperator(operandName)
	csv, err := r.getClusterServiceVersion(operandName, op.Namespace)
	// If can't get CSV, requeue the request
	if err != nil {
		return err
	}

	if csv != nil {
		klog.V(2).Infof("Deleting all the Custom Resources for CSV, Namespace: %s, Name: %s", csv.Namespace, csv.Name)
		if err := r.deleteAllCustomResource(csv, configInstance, operandName); err != nil {
			klog.Error("Failed to Delete a Custom Resource: ", err)
			return err
		}

		klog.V(3).Info("Set Deleting Condition in the operandRequest")
		requestInstance.SetDeletingCondition(csv.Name, operatorv1alpha1.ResourceTypeCsv, corev1.ConditionTrue)
		if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
			klog.Error("Failed to update operandRequest status:", err)
			return err
		}

		klog.V(2).Infof("Deleting the ClusterServiceVersion, Namespace: %s, Name: %s", csv.Namespace, csv.Name)
		if err := r.olmClient.OperatorsV1alpha1().ClusterServiceVersions(csv.Namespace).Delete(csv.Name, &metav1.DeleteOptions{}); err != nil {
			klog.Error("Failed to delete the ClusterServiceVersion: ", err)
			requestInstance.SetDeletingCondition(csv.Name, operatorv1alpha1.ResourceTypeCsv, corev1.ConditionFalse)
			if updateErr := r.client.Status().Update(context.TODO(), requestInstance); updateErr != nil {
				return updateErr
			}
			return err
		}
	}

	opt := registryInstance.GetOperator(operandName)
	if opt != nil {
		klog.V(2).Infof("Deleting the Subscription, Namespace: %s, Name: %s", opt.Namespace, opt.Name)
		requestInstance.SetDeletingCondition(opt.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue)
		if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
			klog.Error("Failed to update delete condition for operandRequest: ", err)
			return err
		}
		if err := r.olmClient.OperatorsV1alpha1().Subscriptions(opt.Namespace).Delete(opt.Name, &metav1.DeleteOptions{}); err != nil {
			if errors.IsNotFound(err) {
				klog.Warningf("Subscription %s does not found", opt.Name)
			} else {
				klog.Error("Failed to delete subscription: ", err)
				requestInstance.SetDeletingCondition(opt.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse)
				if updateErr := r.client.Status().Update(context.TODO(), requestInstance); updateErr != nil {
					klog.Error("Failed to update delete condition for operandRequest: ", updateErr)
					return updateErr
				}
				return err
			}
		}

		requestInstance.CleanMemberStatus(opt.Name)
		if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
			klog.Error("Failed to delete member in the operandRequest status: ", err)
			return err
		}
	}
	return nil
}

func (r *ReconcileOperandRequest) getNeedDeletedOperands(requestInstance *operatorv1alpha1.OperandRequest) (gset.Set, error) {
	klog.V(3).Info("Getting the operater need to be delete")
	deployedOperands := gset.NewSet()
	for _, req := range requestInstance.Status.Members {
		deployedOperands.Add(req.Name)
	}

	currentOperands, err := r.getCurrentOperands(requestInstance)
	if err != nil {
		return nil, err
	}
	fmt.Println(deployedOperands)
	fmt.Println(currentOperands)
	needDeleteOperands := deployedOperands.Difference(currentOperands)
	fmt.Println(needDeleteOperands)
	return needDeleteOperands, nil
}

func (r *ReconcileOperandRequest) getCurrentOperands(requestInstance *operatorv1alpha1.OperandRequest) (gset.Set, error) {
	klog.V(3).Info("Getting the operaters have been deployed")
	deployedOperands := gset.NewSet()
	for _, req := range requestInstance.Spec.Requests {

		requestList := &operatorv1alpha1.OperandRequestList{}

		opts := []client.ListOption{
			client.MatchingLabels(map[string]string{req.RegistryNamespace + "." + req.Registry + "/registry": "true"}),
		}

		if err := r.client.List(context.TODO(), requestList, opts...); err != nil {
			return nil, err
		}

		for _, item := range requestList.Items {
			for _, existingReq := range item.Spec.Requests {
				if req.Registry != existingReq.Registry || req.RegistryNamespace != existingReq.RegistryNamespace {
					continue
				}
				for _, operand := range existingReq.Operands {
					deployedOperands.Add(operand.Name)
				}
			}
		}
	}

	return deployedOperands, nil
}

func generateClusterObjects(o *operatorv1alpha1.Operator) *clusterObjects {
	klog.V(3).Info("Generating Cluster Objects")
	co := &clusterObjects{}
	labels := map[string]string{
		"operator.ibm.com/opreq-control": "true",
	}

	klog.V(3).Info("Generating Namespace: ", o.Namespace)
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
	klog.V(3).Info("Generating Operator Group in the Namespace: ", o.Namespace, " with target namespace: ", o.TargetNamespaces)
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
	klog.V(3).Info("Generating Subscription:  ", o.Name, " in the Namespace: ", o.Namespace)
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
	klog.V(3).Info("Get the OperandRegistry instance from the name: ", name, " in the namespace: ", namespace)
	reg := &operatorv1alpha1.OperandRegistry{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, reg); err != nil {
		return nil, err
	}
	return reg, nil
}
