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

package operandrequestnoolm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	gset "github.com/deckarep/golang-set"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/util"
)

func (r *Reconciler) reconcileOperator(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(1).Infof("Reconciling Operators for OperandRequest: %s/%s", requestInstance.GetNamespace(), requestInstance.GetName())
	// It is important to NOT pass the set directly into defer functions.
	// The arguments to the deferred function are evaluated when the defer executes
	remainingOperands := gset.NewSet()
	for _, m := range requestInstance.Status.Members {
		remainingOperands.Add(m.Name)
	}
	// Update request status
	defer func() {
		requestInstance.FreshMemberStatus(&remainingOperands)
		requestInstance.UpdateClusterPhase()
	}()

	for _, req := range requestInstance.Spec.Requests {
		registryKey := requestInstance.GetRegistryKey(req)
		registryInstance, err := r.GetOperandRegistry(ctx, registryKey)
		if err != nil {
			if apierrors.IsNotFound(err) {
				r.Recorder.Eventf(requestInstance, corev1.EventTypeWarning, "NotFound", "NotFound OperandRegistry NamespacedName %s", registryKey.String())
				requestInstance.SetNotFoundOperatorFromRegistryCondition(registryKey.String(), operatorv1alpha1.ResourceTypeOperandRegistry, corev1.ConditionTrue, &r.Mutex)
			} else {
				requestInstance.SetNoSuitableRegistryCondition(registryKey.String(), err.Error(), operatorv1alpha1.ResourceTypeOperandRegistry, corev1.ConditionTrue, &r.Mutex)
			}
			t := time.Now()
			formatted := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
				t.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), t.Second())
			mergePatch, _ := json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						constant.FindOperandRegistry: formatted,
					},
				},
			})
			if patchErr := r.Patch(ctx, requestInstance, client.RawPatch(types.MergePatchType, mergePatch)); patchErr != nil {
				return utilerrors.NewAggregate([]error{err, patchErr})
			}
			klog.Errorf("Failed to get suitable OperandRegistry %s: %v", registryKey.String(), err)
		}
		merr := &util.MultiErr{}

		// Get the chunk size
		var chunkSize int
		if r.StepSize > 0 {
			chunkSize = r.StepSize
		} else {
			chunkSize = 1
		}

		// reconcile tracking configmaps in batch
		for i := 0; i < len(req.Operands); i += chunkSize {
			j := i + chunkSize
			if j > len(req.Operands) {
				j = len(req.Operands)
			}
			var (
				wg sync.WaitGroup
			)
			for _, operand := range req.Operands[i:j] {
				wg.Add(1)
				go func(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest, registryInstance *operatorv1alpha1.OperandRegistry, operand operatorv1alpha1.Operand, registryKey types.NamespacedName, mu *sync.Mutex) {
					defer wg.Done()
					if err := r.reconcileOpReqCM(ctx, requestInstance, registryInstance, operand, registryKey, mu); err != nil {
						mu.Lock()
						defer mu.Unlock()
						merr.Add(err)
					}
				}(ctx, requestInstance, registryInstance, operand, registryKey, &r.Mutex)
			}
			wg.Wait()
		}

		if len(merr.Errors) != 0 {
			return merr
		}
	}

	// Delete specific operators
	if err := r.absentOperatorsAndOperands(ctx, requestInstance, &remainingOperands); err != nil {
		return err
	}
	klog.V(1).Infof("Finished reconciling Operators for OperandRequest: %s/%s", requestInstance.GetNamespace(), requestInstance.GetName())

	return nil
}

// In No olm installs, we create empty configmaps that house the annotations ODLM looks for when cleaning up operators once an opreq is deleted
func (r *Reconciler) reconcileOpReqCM(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest, registryInstance *operatorv1alpha1.OperandRegistry, operand operatorv1alpha1.Operand, registryKey types.NamespacedName, mu sync.Locker) error {
	// Check the requested Operand if exist in specific OperandRegistry
	var opt *operatorv1alpha1.Operator
	if registryInstance != nil {
		var err error
		opt, err = r.GetOperandFromRegistryNoOLM(ctx, registryInstance, operand.Name)
		if err != nil {
			return err
		}
	}
	if opt == nil {
		if registryInstance != nil {
			klog.V(1).Infof("Operator %s not found in the OperandRegistry %s/%s", operand.Name, registryInstance.Namespace, registryInstance.Name)
		}
		requestInstance.SetNotFoundOperatorFromRegistryCondition(operand.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, mu)
		requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorNotFound, operatorv1alpha1.ServiceNotFound, mu)
		return nil
	}
	if opt.Scope == operatorv1alpha1.ScopePrivate && requestInstance.Namespace != registryInstance.Namespace {
		klog.Warningf("Operator %s is private. It can't be requested from namespace %s", operand.Name, requestInstance.Namespace)
		requestInstance.SetOutofScopeCondition(operand.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, mu)
		return nil
	}

	// Check configmap if exist
	namespace := r.GetOperatorNamespace(opt.InstallMode, opt.Namespace)
	cm, err := r.GetOpReqCM(ctx, opt.Name, namespace, registryInstance.Namespace, opt.PackageName)

	if cm == nil && err == nil {
		if opt.InstallMode == operatorv1alpha1.InstallModeNoop {
			requestInstance.SetNoSuitableRegistryCondition(registryKey.String(), opt.Name+" is in maintenance status", operatorv1alpha1.ResourceTypeOperandRegistry, corev1.ConditionTrue, &r.Mutex)
			requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorRunning, operatorv1alpha1.ServiceRunning, mu)
		} else {
			// CM does not exist, create a new one
			if err = r.createOpReqCM(ctx, requestInstance, opt, registryKey); err != nil {
				requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorFailed, "", mu)
				return err
			}
			requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorInstalling, "", mu)
		}
		return nil
	} else if err != nil {
		return err
	}

	// Operator existing and managed by OperandRequest controller
	if _, ok := cm.Labels[constant.OpreqLabel]; ok {
		originalCM := cm.DeepCopy()
		var isInScope bool

		if cm.Namespace == opt.Namespace {
			isInScope = true
		} else {
			var nsAnnoSlice []string
			namespaceReg, _ := regexp.Compile(`^(.*)\.(.*)\.(.*)\/operatorNamespace`)
			for anno, ns := range cm.Annotations {
				if namespaceReg.MatchString(anno) {
					nsAnnoSlice = append(nsAnnoSlice, ns)
				}
			}
			if len(nsAnnoSlice) != 0 && !util.Contains(nsAnnoSlice, cm.Namespace) {
				if r.checkUninstallLabel(cm) {
					klog.V(1).Infof("Operator %s has label operator.ibm.com/opreq-do-not-uninstall. Skip the uninstall", opt.Name)
					return nil
				}
				if err = r.deleteOpReqCM(ctx, requestInstance, cm); err != nil {
					requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorFailed, "", mu)
					return err
				}
				requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorUpdating, "", mu)
				return nil
			}
		}

		// add annotations to existing tracking configmap for upgrade case
		if cm.Annotations == nil {
			cm.Annotations = make(map[string]string)
		}
		cm.Annotations[registryKey.Namespace+"."+registryKey.Name+"/registry"] = "true"
		cm.Annotations[registryKey.Namespace+"."+registryKey.Name+"/config"] = "true"
		cm.Annotations[requestInstance.Namespace+"."+requestInstance.Name+"."+operand.Name+"/request"] = opt.Channel
		cm.Annotations[requestInstance.Namespace+"."+requestInstance.Name+"."+operand.Name+"/operatorNamespace"] = namespace
		cm.Annotations["packageName"] = opt.PackageName
		cm.Labels["operator.ibm.com/watched-by-odlm"] = "true"

		if opt.InstallMode == operatorv1alpha1.InstallModeNoop {
			requestInstance.SetNoSuitableRegistryCondition(registryKey.String(), opt.Name+" is in maintenance status", operatorv1alpha1.ResourceTypeOperandRegistry, corev1.ConditionTrue, &r.Mutex)
			requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorRunning, operatorv1alpha1.ServiceRunning, mu)
		} else {
			requestInstance.SetNotFoundOperatorFromRegistryCondition(operand.Name, operatorv1alpha1.ResourceTypeConfigmap, corev1.ConditionFalse, mu)
		}
		if compareOpReqCM(cm, originalCM) {
			if err = r.updateOpReqCM(ctx, requestInstance, cm); err != nil {
				requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorFailed, "", mu)
				return err
			}
			requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorUpdating, "", mu)
		}

		if !isInScope {
			requestInstance.SetNoConflictOperatorCondition(operand.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse, mu)
			requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorFailed, "", mu)
		} else {
			requestInstance.SetNoConflictOperatorCondition(operand.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, mu)
		}
	} else {
		// Operator existing and not managed by OperandRequest controller
		klog.V(1).Infof("Configmap %s in namespace %s isn't created by ODLM. Ignore update/delete it.", cm.Name, cm.Namespace)
	}
	return nil
}

func (r *Reconciler) createOpReqCM(ctx context.Context, cr *operatorv1alpha1.OperandRequest, opt *operatorv1alpha1.Operator, key types.NamespacedName) error {
	namespace := r.GetOperatorNamespace(opt.InstallMode, opt.Namespace)
	klog.V(3).Info("Cofigmap Namespace: ", namespace)

	co := r.generateClusterObjects(opt, key, types.NamespacedName{Namespace: cr.Namespace, Name: cr.Name})

	// Create required namespace
	ns := co.namespace
	klog.V(3).Info("Creating the Namespace for Operator: " + opt.Name)

	// Compare namespace and create namespace
	oprNs := util.GetOperatorNamespace()
	if ns.Name != oprNs && ns.Name != constant.ClusterOperatorNamespace {
		if err := r.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
			klog.Warningf("failed to create the namespace %s, please make sure it exists: %s", ns.Name, err)
		}
	}

	// Create CM
	klog.V(2).Info("Creating the Configmap: " + opt.Name)

	cm := co.configmap
	cr.SetCreatingCondition(cm.Name, operatorv1alpha1.ResourceTypeConfigmap, corev1.ConditionTrue, &r.Mutex)

	if err := r.Create(ctx, cm); err != nil && !apierrors.IsAlreadyExists(err) {
		cr.SetCreatingCondition(cm.Name, operatorv1alpha1.ResourceTypeConfigmap, corev1.ConditionFalse, &r.Mutex)
		return err
	}
	return nil
}

func (r *Reconciler) deleteOpReqCM(ctx context.Context, cr *operatorv1alpha1.OperandRequest, cm *corev1.ConfigMap) error {

	klog.V(2).Infof("Deleting Subscription %s/%s ...", cm.Namespace, cm.Name)

	klog.V(2).Infof("Deleting the Configmap, Namespace: %s, Name: %s", cm.Namespace, cm.Name)
	cr.SetDeletingCondition(cm.Name, operatorv1alpha1.ResourceTypeConfigmap, corev1.ConditionTrue, &r.Mutex)

	if err := r.Delete(ctx, cm); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("Configmap %s was not found in namespace %s", cm.Name, cm.Namespace)
		} else {
			cr.SetDeletingCondition(cm.Name, operatorv1alpha1.ResourceTypeConfigmap, corev1.ConditionFalse, &r.Mutex)
			return err
		}
	}

	klog.V(1).Infof("Configmap %s/%s is deleted", cm.Namespace, cm.Name)
	return nil
}

func (r *Reconciler) updateOpReqCM(ctx context.Context, cr *operatorv1alpha1.OperandRequest, cm *corev1.ConfigMap) error {

	klog.V(2).Infof("Updating Configmap %s/%s ...", cm.Namespace, cm.Name)
	cr.SetUpdatingCondition(cm.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, &r.Mutex)

	if err := r.Update(ctx, cm); err != nil {
		cr.SetUpdatingCondition(cm.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse, &r.Mutex)
		return err
	}
	return nil
}

func (r *Reconciler) uninstallOperatorsAndOperands(ctx context.Context, operandName string, requestInstance *operatorv1alpha1.OperandRequest, registryInstance *operatorv1alpha1.OperandRegistry, configInstance *operatorv1alpha1.OperandConfig) error {
	// No error handling for un-installation step in case Catalog has been deleted
	op, _ := r.GetOperandFromRegistryNoOLM(ctx, registryInstance, operandName)
	// klog.V(1).Info("op to check in uninstallOperatorsAndOperands: ", op.Name, " o: ", fmt.Sprintf("%v", operandName))
	if op == nil {
		klog.Warningf("Operand %s not found", operandName)
		return nil
	}

	namespace := r.GetOperatorNamespace(op.InstallMode, op.Namespace)

	//Assuing we can still use op as a parameter, we should be able to get the deployment with ease
	deploy, err := r.GetDeployment(ctx, operandName, namespace, registryInstance.Namespace, op.PackageName)
	// klog.V(1).Info("deployment in uninstallOperatorsAndOperands: ", deploy.Name)
	if deploy == nil && err == nil {
		klog.V(3).Infof("There is no Deployment called %s or using package name %s in the namespace %s and %s", operandName, op.PackageName, namespace, registryInstance.Namespace)
		return nil
	} else if err != nil {
		klog.Errorf("Failed to get Deployment called %s or using package name %s in the namespace %s and %s", operandName, op.PackageName, namespace, registryInstance.Namespace)
		return err
	}

	if deploy.Labels == nil {
		// Subscription existing and not managed by OperandRequest controller
		klog.V(2).Infof("Deployment %s in the namespace %s isn't created by ODLM", deploy.Name, deploy.Namespace)
		return nil
	}

	// if _, ok := deploy.Labels[constant.OpreqLabel]; !ok {
	// 	if !op.UserManaged {
	// 		klog.V(2).Infof("Deployment %s in the namespace %s isn't created by ODLM and isn't user managed", deploy.Name, deploy.Namespace)
	// 		return nil
	// 	}
	// }

	cm, err := r.GetOpReqCM(ctx, op.Name, deploy.Namespace, registryInstance.Namespace, op.PackageName)
	// klog.V(1).Info("Configmap tracking operand:  ", cm.Name)
	if cm != nil && err == nil {
		uninstallOperator, uninstallOperand := checkOpReqCMAnnotationsForUninstall(requestInstance.ObjectMeta.Name, requestInstance.ObjectMeta.Namespace, op.Name, op.InstallMode, cm)
		// klog.V(1).Info("uinstall operator:  ", uninstallOperator, " uninstall operand: ", uninstallOperand)
		if !uninstallOperand && !uninstallOperator {
			if err = r.updateOpReqCM(ctx, requestInstance, cm); err != nil {
				requestInstance.SetMemberStatus(op.Name, operatorv1alpha1.OperatorFailed, "", &r.Mutex)
				return err
			}
			requestInstance.SetMemberStatus(op.Name, operatorv1alpha1.OperatorUpdating, "", &r.Mutex)

			klog.V(1).Infof("No deletion, operator %s/%s and its operands are still requested by other OperandRequests", cm.Namespace, cm.Name)
			return nil
		}
		if deploymentList, err := r.GetDeploymentListFromPackage(ctx, op.PackageName, op.Namespace); err != nil {
			// If can't get deployment, requeue the request
			return err
		} else if deploymentList != nil {
			klog.Infof("Found %d Deployment for package %s/%s", len(deploymentList), op.Name, namespace)
			if uninstallOperand {
				klog.V(1).Infof("Deleting all the Custom Resources for Deployment, Namespace: %s, Name: %s", deploymentList[0].Namespace, deploymentList[0].Name)
				if err := r.deleteAllCustomResource(ctx, deploymentList[0], requestInstance, configInstance, operandName, configInstance.Namespace); err != nil {
					return err
				}
				klog.V(1).Infof("Deleting all the k8s Resources for Deployment, Namespace: %s, Name: %s", deploymentList[0].Namespace, deploymentList[0].Name)
				if err := r.deleteAllK8sResource(ctx, configInstance, operandName, configInstance.Namespace); err != nil {
					return err
				}
			}
			//TODO should odlm delete deployments or should that be handled by helm?
			// if uninstallOperator {
			// 	if r.checkUninstallLabel(cm) {
			// 		klog.V(1).Infof("Operator %s has label operator.ibm.com/opreq-do-not-uninstall. Skip the uninstall", op.Name)
			// 		return nil
			// 	}

			// 	klog.V(3).Info("Set Deleting Condition in the operandRequest")
			// 	//TODO replace the resource types set in these setdeletingcondition functions
			// 	requestInstance.SetDeletingCondition(deploymentList[0].Name, operatorv1alpha1.ResourceTypeDeployment, corev1.ConditionTrue, &r.Mutex)

			// 	for _, deployment := range deploymentList {
			// 		klog.V(1).Infof("Deleting the deployment, Namespace: %s, Name: %s", deployment.Namespace, deployment.Name)
			// 		if err := r.Delete(ctx, deployment); err != nil {
			// 			requestInstance.SetDeletingCondition(deployment.Name, operatorv1alpha1.ResourceTypeDeployment, corev1.ConditionFalse, &r.Mutex)
			// 			return errors.Wrapf(err, "failed to delete the deployment %s/%s", deployment.Namespace, deployment.Name)
			// 		}
			// 	}
			// }
		}
	} else if err != nil {
		klog.Errorf("Failed to get Configmap called %s or using package name %s in the namespace %s and %s", operandName, op.PackageName, namespace, registryInstance.Namespace)
		return err
	}

	return nil
}

func (r *Reconciler) uninstallOperands(ctx context.Context, operandName string, requestInstance *operatorv1alpha1.OperandRequest, registryInstance *operatorv1alpha1.OperandRegistry, configInstance *operatorv1alpha1.OperandConfig) error {
	// No error handling for un-installation step in case Catalog has been deleted
	op, _ := r.GetOperandFromRegistryNoOLM(ctx, registryInstance, operandName)
	if op == nil {
		klog.Warningf("Operand %s not found", operandName)
		return nil
	}

	namespace := r.GetOperatorNamespace(op.InstallMode, op.Namespace)
	uninstallOperand := false
	operatorStatus, ok := registryInstance.Status.OperatorsStatus[op.Name]
	if !ok {
		return nil
	}
	if operatorStatus.ReconcileRequests == nil {
		return nil
	}
	if len(operatorStatus.ReconcileRequests) > 1 {
		return nil
	}
	if operatorStatus.ReconcileRequests[0].Name == requestInstance.Name {
		uninstallOperand = true
	}

	// get list reconcileRequests
	// ignore the name which triggered reconcile
	// if list is empty then uninstallOperand = true

	if deploymentList, err := r.GetDeploymentListFromPackage(ctx, op.PackageName, op.Namespace); err != nil {
		// If can't get deployment, requeue the request
		return err
	} else if deploymentList != nil {
		klog.Infof("Found %d Deployment for package %s/%s", len(deploymentList), op.Name, namespace)
		if uninstallOperand {
			klog.V(2).Infof("Deleting all the Custom Resources for Deployment, Namespace: %s, Name: %s", deploymentList[0].Namespace, deploymentList[0].Name)
			if err := r.deleteAllCustomResource(ctx, deploymentList[0], requestInstance, configInstance, operandName, configInstance.Namespace); err != nil {
				return err
			}
			klog.V(2).Infof("Deleting all the k8s Resources for Deployment, Namespace: %s, Name: %s", deploymentList[0].Namespace, deploymentList[0].Name)
			if err := r.deleteAllK8sResource(ctx, configInstance, operandName, configInstance.Namespace); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Reconciler) absentOperatorsAndOperands(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest, remainingOperands *gset.Set) error {
	needDeletedOperands := r.getNeedDeletedOperands(requestInstance)

	var (
		wg sync.WaitGroup
	)

	for _, req := range requestInstance.Spec.Requests {
		registryKey := requestInstance.GetRegistryKey(req)
		registryInstance, err := r.GetOperandRegistry(ctx, registryKey)
		if err != nil {
			return err
		}
		configInstance, err := r.GetOperandConfig(ctx, registryKey)
		if err != nil {
			if apierrors.IsNotFound(err) {
				configInstance = &operatorv1alpha1.OperandConfig{}
			} else {
				return err
			}
		}
		merr := &util.MultiErr{}
		remainingOp := needDeletedOperands.Clone()
		for o := range needDeletedOperands.Iter() {
			var (
				o = o
			)
			wg.Add(1)
			go func() {
				defer wg.Done()
				op, _ := r.GetOperandFromRegistryNoOLM(ctx, registryInstance, fmt.Sprintf("%v", o))
				// klog.V(1).Info("op to check in absentOperatorsandOperands: ", op.Name, " o: ", fmt.Sprintf("%v", o))
				if op == nil {
					klog.Warningf("Operand %s not found", fmt.Sprintf("%v", o))
				}
				if op != nil && !op.UserManaged {
					//TODO do we need to uninstall operators and operands? Should the user uninstall operators with helm uninstall going forward?
					//The below function currently does not delete operators
					if err := r.uninstallOperatorsAndOperands(ctx, fmt.Sprintf("%v", o), requestInstance, registryInstance, configInstance); err != nil {
						r.Mutex.Lock()
						defer r.Mutex.Unlock()
						merr.Add(err)
						return // return here to avoid removing the operand from remainingOperands
					}
				} else {
					if err := r.uninstallOperands(ctx, fmt.Sprintf("%v", o), requestInstance, registryInstance, configInstance); err != nil {
						r.Mutex.Lock()
						defer r.Mutex.Unlock()
						merr.Add(err)
						return // return here to avoid removing the operand from remainingOperands
					}
				}
				requestInstance.RemoveServiceStatus(fmt.Sprintf("%v", o), &r.Mutex)
				(*remainingOperands).Remove(o)
				remainingOp.Remove(o)
				// klog.V(1).Info("op removed: ", op.Name, " o: ", fmt.Sprintf("%v", o))
			}()
		}
		timeout := util.WaitTimeout(&wg, constant.DefaultSubDeleteTimeout)
		if timeout {
			merr.Add(fmt.Errorf("timeout for cleaning up subscription %v", strings.Trim(fmt.Sprint(remainingOp.ToSlice()), "[]")))
		}
		if len(merr.Errors) != 0 {
			return merr
		}

	}
	return nil
}

func (r *Reconciler) getNeedDeletedOperands(requestInstance *operatorv1alpha1.OperandRequest) gset.Set {
	klog.V(3).Info("Getting the operator need to be delete")
	deployedOperands := gset.NewSet()
	for _, req := range requestInstance.Status.Members {
		deployedOperands.Add(req.Name)
	}

	currentOperands := gset.NewSet()
	if requestInstance.DeletionTimestamp.IsZero() {
		for _, req := range requestInstance.Spec.Requests {
			for _, op := range req.Operands {
				// klog.V(1).Info("Add current operand in getNeedDeletedOperands ", op.Name)
				currentOperands.Add(op.Name)
			}
		}
	}

	needDeleteOperands := deployedOperands.Difference(currentOperands)
	return needDeleteOperands
}

func (r *Reconciler) generateClusterObjects(o *operatorv1alpha1.Operator, registryKey, requestKey types.NamespacedName) *clusterObjects {
	klog.V(3).Info("Generating Cluster Objects")
	co := &clusterObjects{}
	labels := map[string]string{
		constant.OpreqLabel:                "true",
		"operator.ibm.com/watched-by-odlm": "true",
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

	// The namespace is 'openshift-operators' when installMode is cluster
	namespace := r.GetOperatorNamespace(o.InstallMode, o.Namespace)

	annotations := map[string]string{
		registryKey.Namespace + "." + registryKey.Name + "/registry":                       "true",
		registryKey.Namespace + "." + registryKey.Name + "/config":                         "true",
		requestKey.Namespace + "." + requestKey.Name + "." + o.Name + "/request":           o.Channel,
		requestKey.Namespace + "." + requestKey.Name + "." + o.Name + "/operatorNamespace": namespace,
		"packageName": o.PackageName,
	}

	//CM object
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        o.PackageName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: map[string]string{},
	}

	cm.SetGroupVersionKind(schema.GroupVersionKind{Group: corev1.SchemeGroupVersion.Group, Kind: "Configmap", Version: corev1.SchemeGroupVersion.Version})
	klog.V(2).Info("Generating tracking Configmap:  ", o.PackageName, " in the Namespace: ", namespace)
	co.configmap = cm
	return co
}

func (r *Reconciler) checkUninstallLabel(cm *corev1.ConfigMap) bool {
	cmLabels := cm.GetLabels()
	return cmLabels[constant.NotUninstallLabel] == "true"
}

func compareOpReqCM(cm *corev1.ConfigMap, originalCM *corev1.ConfigMap) (needUpdate bool) {
	return !equality.Semantic.DeepEqual(cm.Annotations, originalCM.Annotations)
}

func CheckSingletonServices(operator string) bool {
	singletonServices := []string{"ibm-cert-manager-operator", "ibm-licensing-operator"}
	return util.Contains(singletonServices, operator)
}

// checkOpReqCMAnnotationsForUninstall checks the annotations of a tracking configmap object
// to determine whether the operator and operand should be uninstalled.
// It takes the name of the OperandRequest, the namespace of the OperandRequest,
// the name of the operator, and a pointer to the configmap object as input.
// It returns two boolean values: uninstallOperator and uninstallOperand.
// If uninstallOperator is true, it means the operator should be uninstalled.
// If uninstallOperand is true, it means the operand should be uninstalled.
func checkOpReqCMAnnotationsForUninstall(reqName, reqNs, opName, installMode string, cm *corev1.ConfigMap) (bool, bool) {
	uninstallOperator := true
	uninstallOperand := true

	klog.V(2).Info("Checking tracking configmap for uninstall for operand: ", opName)
	delete(cm.Annotations, reqNs+"."+reqName+"."+opName+"/request")
	delete(cm.Annotations, reqNs+"."+reqName+"."+opName+"/operatorNamespace")

	var opreqNsSlice []string
	var operatorNameSlice []string
	namespaceReg, _ := regexp.Compile(`^(.*)\.(.*)\.(.*)\/operatorNamespace`)
	channelReg, _ := regexp.Compile(`^(.*)\.(.*)\.(.*)\/request`)

	for key, value := range cm.Annotations {
		if namespaceReg.MatchString(key) {
			opreqNsSlice = append(opreqNsSlice, value)
		}

		if channelReg.MatchString(key) {
			// Extract the operator name from the key
			keyParts := strings.Split(key, "/")
			annoPrefix := strings.Split(keyParts[0], ".")
			operatorNameSlice = append(operatorNameSlice, annoPrefix[len(annoPrefix)-1])
		}
	}

	// If one of remaining <prefix>/operatorNamespace annotations' values is the same as subscription's namespace,
	// the operator should NOT be uninstalled.
	if util.Contains(opreqNsSlice, cm.Namespace) {
		uninstallOperator = false
	}

	if value, ok := cm.Labels[constant.OpreqLabel]; !ok || value != "true" {
		uninstallOperator = false
	}

	// When one of following conditions are met, the operand will NOT be uninstalled:
	// 1. operator is not uninstalled AND intallMode is no-op.
	// 2. operator is uninstalled AND  at least one other <prefix>/operatorNamespace annotation exists.
	// 2. remaining <prefix>/request annotation's values contain the same operator name
	if (!uninstallOperator && installMode == operatorv1alpha1.InstallModeNoop) || (uninstallOperator && len(opreqNsSlice) != 0) || util.Contains(operatorNameSlice, opName) {
		uninstallOperand = false
	}

	return uninstallOperator, uninstallOperand
}
