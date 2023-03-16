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

package operandrequest

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	gset "github.com/deckarep/golang-set"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

func (r *Reconciler) reconcileOperator(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(1).Infof("Reconciling Operators for OperandRequest: %s/%s", requestInstance.GetNamespace(), requestInstance.GetName())
	// It is important to NOT pass the set directly into defer functions.
	// The arguments to the deferred function are evaluated when the defer executes
	failedDeletedOperands := gset.NewSet()
	// Update request status
	defer func() {
		requestInstance.FreshMemberStatus(&failedDeletedOperands)
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
			klog.Errorf("Failed to get suitable OperandRegistry %s: %v", registryKey.String(), err)
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
			return err
		}
		merr := &util.MultiErr{}

		// Get the chunk size
		var chunkSize int
		if r.StepSize > 0 {
			chunkSize = r.StepSize
		} else {
			chunkSize = 1
		}

		// reconcile subscription in batch
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
					if err := r.reconcileSubscription(ctx, requestInstance, registryInstance, operand, registryKey, mu); err != nil {
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

	// TODO: update OperandRequest status before patching. Otherwise, the status will be lost.
	// It is really corner case which would not cause any issue so far since the Status will finally be Running.
	// Need to consider a more elegant way to preserve status instead of calling Client API like below
	// requestInstance.FreshMemberStatus(&failedDeletedOperands)

	mergePatch, _ := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				constant.FindOperandRegistry: "false",
			},
		},
	})
	if err := r.Patch(ctx, requestInstance, client.RawPatch(types.MergePatchType, mergePatch)); err != nil {
		return err
	}

	// Delete specific operators
	if err := r.absentOperatorsAndOperands(ctx, requestInstance, &failedDeletedOperands); err != nil {
		return err
	}
	klog.V(1).Infof("Finished reconciling Operators for OperandRequest: %s/%s", requestInstance.GetNamespace(), requestInstance.GetName())

	return nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest, registryInstance *operatorv1alpha1.OperandRegistry, operand operatorv1alpha1.Operand, registryKey types.NamespacedName, mu sync.Locker) error {
	// Check the requested Operand if exist in specific OperandRegistry
	opt := registryInstance.GetOperator(operand.Name)
	if opt == nil {
		klog.V(1).Infof("Operator %s not found in the OperandRegistry %s/%s", operand.Name, registryInstance.Namespace, registryInstance.Name)
		requestInstance.SetNotFoundOperatorFromRegistryCondition(operand.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, mu)
		requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorNotFound, operatorv1alpha1.ServiceNotFound, mu)
		return nil
	}
	if opt.Scope == operatorv1alpha1.ScopePrivate && requestInstance.Namespace != registryInstance.Namespace {
		klog.Warningf("Operator %s is private. It can't be requested from namespace %s", operand.Name, requestInstance.Namespace)
		requestInstance.SetOutofScopeCondition(operand.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, mu)
		return nil
	}

	// Check subscription if exist
	namespace := r.GetOperatorNamespace(opt.InstallMode, opt.Namespace)
	sub, err := r.GetSubscription(ctx, opt.Name, namespace, opt.PackageName)

	if err != nil {
		if apierrors.IsNotFound(err) {
			if opt.InstallMode == operatorv1alpha1.InstallModeNoop {
				requestInstance.SetNoSuitableRegistryCondition(registryKey.String(), opt.Name+" is in maintenance status", operatorv1alpha1.ResourceTypeOperandRegistry, corev1.ConditionTrue, &r.Mutex)
				requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorRunning, operatorv1alpha1.ServiceRunning, mu)
			} else {
				// Subscription does not exist, create a new one
				if err = r.createSubscription(ctx, requestInstance, opt, registryKey); err != nil {
					requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorFailed, "", mu)
					return err
				}
				requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorInstalling, "", mu)
			}
			return nil
		}
		return err
	}

	// Subscription existing and managed by OperandRequest controller
	if _, ok := sub.Labels[constant.OpreqLabel]; ok {
		originalSub := sub.DeepCopy()

		// add annotations to existing Subscriptions for upgrade case
		if sub.Annotations == nil {
			sub.Annotations = make(map[string]string)
		}
		sub.Annotations[registryKey.Namespace+"."+registryKey.Name+"/registry"] = "true"
		sub.Annotations[registryKey.Namespace+"."+registryKey.Name+"/config"] = "true"
		sub.Annotations[requestInstance.Namespace+"."+requestInstance.Name+"."+operand.Name+"/request"] = opt.Channel

		if opt.InstallMode == operatorv1alpha1.InstallModeNoop {
			requestInstance.SetNoSuitableRegistryCondition(registryKey.String(), opt.Name+" is in maintenance status", operatorv1alpha1.ResourceTypeOperandRegistry, corev1.ConditionTrue, &r.Mutex)
			requestInstance.SetMemberStatus(operand.Name, operatorv1alpha1.OperatorRunning, operatorv1alpha1.ServiceRunning, mu)
		} else {
			//set operator channel back to previous one if it is tombstone service
			sub.Annotations[requestInstance.Namespace+"."+requestInstance.Name+"."+operand.Name+"/request"] = sub.Spec.Channel
			
			sub.Spec.CatalogSource = opt.SourceName
			sub.Spec.CatalogSourceNamespace = opt.SourceNamespace
			sub.Spec.Package = opt.PackageName

			// check request annotation in subscription, get all available channels
			var semverlList []string
			reg, _ := regexp.Compile(`^(.*)\.(.*)\.(.*)\/request`)
			for anno, channel := range sub.Annotations {
				if reg.MatchString(anno) && semver.IsValid(channel) {
					semverlList = append(semverlList, channel)
				}
			}
			if len(semverlList) == 0 {
				// channel is not valid semantic version
				sub.Spec.Channel = opt.Channel
			} else if !util.Contains(semverlList, sub.Spec.Channel) {
				// upgrade channel to minimal version existing in annotation
				sort.Sort(semver.ByVersion(semverlList))
				sub.Spec.Channel = semverlList[0]
			}

			if opt.InstallPlanApproval != "" && sub.Spec.InstallPlanApproval != opt.InstallPlanApproval {
				sub.Spec.InstallPlanApproval = opt.InstallPlanApproval
			}
			if opt.SubscriptionConfig != nil {
				sub.Spec.Config = opt.SubscriptionConfig
			}
		}
		if compareSub(sub, originalSub) {
			if err = r.updateSubscription(ctx, requestInstance, sub); err != nil {
				requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorFailed, "", mu)
				return err
			}
			requestInstance.SetMemberStatus(opt.Name, operatorv1alpha1.OperatorUpdating, "", mu)
		}
	} else {
		// Subscription existing and not managed by OperandRequest controller
		klog.V(1).Infof("Subscription %s in namespace %s isn't created by ODLM. Ignore update/delete it.", sub.Name, sub.Namespace)
	}
	return nil
}

func (r *Reconciler) createSubscription(ctx context.Context, cr *operatorv1alpha1.OperandRequest, opt *operatorv1alpha1.Operator, key types.NamespacedName) error {
	namespace := r.GetOperatorNamespace(opt.InstallMode, opt.Namespace)
	klog.V(3).Info("Subscription Namespace: ", namespace)

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

	if namespace != constant.ClusterOperatorNamespace {
		// Create required operatorgroup
		existOG := &olmv1.OperatorGroupList{}
		if err := r.Client.List(ctx, existOG, &client.ListOptions{Namespace: co.operatorGroup.Namespace}); err != nil {
			return err
		}
		if len(existOG.Items) == 0 {
			og := co.operatorGroup
			klog.V(3).Info("Creating the OperatorGroup for Subscription: " + opt.Name)
			if err := r.Create(ctx, og); err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		}
	}

	// Create subscription
	klog.V(2).Info("Creating the Subscription: " + opt.Name)
	if co.subscription.Spec.CatalogSource == "" || co.subscription.Spec.CatalogSourceNamespace == "" {
		return fmt.Errorf("failed to find catalogsource for subscription %s/%s", co.subscription.Namespace, co.subscription.Name)
	}

	sub := co.subscription
	cr.SetCreatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, &r.Mutex)

	if err := r.Create(ctx, sub); err != nil && !apierrors.IsAlreadyExists(err) {
		cr.SetCreatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse, &r.Mutex)
		return err
	}
	return nil
}

func (r *Reconciler) updateSubscription(ctx context.Context, cr *operatorv1alpha1.OperandRequest, sub *olmv1alpha1.Subscription) error {

	klog.V(2).Infof("Updating Subscription %s/%s ...", sub.Namespace, sub.Name)
	cr.SetUpdatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, &r.Mutex)

	if err := r.Update(ctx, sub); err != nil {
		cr.SetUpdatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse, &r.Mutex)
		return err
	}
	return nil
}

func (r *Reconciler) deleteSubscription(ctx context.Context, operandName string, requestInstance *operatorv1alpha1.OperandRequest, registryInstance *operatorv1alpha1.OperandRegistry, configInstance *operatorv1alpha1.OperandConfig) error {
	op := registryInstance.GetOperator(operandName)
	if op == nil {
		klog.Warningf("Operand %s not found", operandName)
		return nil
	}

	namespace := r.GetOperatorNamespace(op.InstallMode, op.Namespace)
	sub, err := r.GetSubscription(ctx, operandName, namespace, op.PackageName)
	originalsub := sub.DeepCopy()
	if apierrors.IsNotFound(err) {
		klog.V(3).Infof("There is no Subscription %s or %s in the namespace %s", operandName, op.PackageName, namespace)
		return nil
	} else if err != nil {
		klog.Errorf("Failed to get Subscription %s or %s in the namespace %s", operandName, op.PackageName, namespace)
		return err
	}

	if sub.Labels == nil {
		// Subscription existing and not managed by OperandRequest controller
		klog.V(2).Infof("Subscription %s in the namespace %s isn't created by ODLM", sub.Name, sub.Namespace)
		return nil
	}

	if _, ok := sub.Labels[constant.OpreqLabel]; !ok {
		// Subscription existing and not managed by OperandRequest controller
		klog.V(2).Infof("Subscription %s in the namespace %s isn't created by ODLM", sub.Name, sub.Namespace)
		return nil
	}

	// remove request in annotation of subscription
	reqName := requestInstance.ObjectMeta.Name
	reqNs := requestInstance.ObjectMeta.Namespace
	delete(sub.Annotations, reqNs+"."+reqName+"."+op.Name+"/request")

	var semverlList []string
	var annoSlice []string
	reg, _ := regexp.Compile(`^(.*)\.(.*)\.(.*)\/request`)
	for anno, channel := range sub.Annotations {
		if reg.MatchString(anno) {
			annoSlice = append(annoSlice, anno)
			if semver.IsValid(channel) {
				semverlList = append(semverlList, channel)
			}
		}
	}
	if len(annoSlice) != 0 {
		// update channel to minmial version remaining in annotations
		if len(semverlList) != 0 && !util.Contains(semverlList, sub.Spec.Channel) {
			sort.Sort(semver.ByVersion(semverlList))
			sub.Spec.Channel = semverlList[0]
		}
		// remove the associated registry from annotation of subscription
		if err := r.Patch(ctx, sub, client.MergeFrom(originalsub)); err != nil {
			requestInstance.SetUpdatingCondition(sub.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse, &r.Mutex)
			return err
		}
		klog.V(1).Infof("Did not delete Subscription %s/%s which is requested by other OperandRequests", sub.Namespace, sub.Name)
		return nil
	}

	csv, err := r.GetClusterServiceVersion(ctx, sub)
	// If can't get CSV, requeue the request
	if err != nil {
		return err
	}

	if csv != nil {
		klog.V(2).Infof("Deleting all the Custom Resources for CSV, Namespace: %s, Name: %s", csv.Namespace, csv.Name)
		if err := r.deleteAllCustomResource(ctx, csv, requestInstance, configInstance, operandName, configInstance.Namespace); err != nil {
			return err
		}
		klog.V(2).Infof("Deleting all the k8s Resources for CSV, Namespace: %s, Name: %s", csv.Namespace, csv.Name)
		if err := r.deleteAllK8sResource(ctx, configInstance, operandName, configInstance.Namespace); err != nil {
			return err
		}
		if r.checkUninstallLabel(sub) {
			klog.V(1).Infof("Operator %s has label operator.ibm.com/opreq-do-not-uninstall. Skip the uninstall", op.Name)
			return nil
		}

		klog.V(3).Info("Set Deleting Condition in the operandRequest")
		requestInstance.SetDeletingCondition(csv.Name, operatorv1alpha1.ResourceTypeCsv, corev1.ConditionTrue, &r.Mutex)

		klog.V(1).Infof("Deleting the ClusterServiceVersion, Namespace: %s, Name: %s", csv.Namespace, csv.Name)
		if err := r.Delete(ctx, csv); err != nil {
			requestInstance.SetDeletingCondition(csv.Name, operatorv1alpha1.ResourceTypeCsv, corev1.ConditionFalse, &r.Mutex)
			return errors.Wrap(err, "failed to delete the ClusterServiceVersion")
		}
	}

	klog.V(2).Infof("Deleting the Subscription, Namespace: %s, Name: %s", namespace, op.Name)
	requestInstance.SetDeletingCondition(op.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionTrue, &r.Mutex)

	if err := r.Delete(ctx, sub); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("Subscription %s was not found in namespace %s", op.Name, namespace)
		} else {
			requestInstance.SetDeletingCondition(op.Name, operatorv1alpha1.ResourceTypeSub, corev1.ConditionFalse, &r.Mutex)
			return errors.Wrap(err, "failed to delete subscription")
		}
	}

	klog.V(1).Infof("Subscription %s/%s is deleted", namespace, op.Name)
	return nil
}

func (r *Reconciler) absentOperatorsAndOperands(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest, failedDeletedOperands *gset.Set) error {
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
				if err := r.deleteSubscription(ctx, fmt.Sprintf("%v", o), requestInstance, registryInstance, configInstance); err != nil {
					r.Mutex.Lock()
					defer r.Mutex.Unlock()
					merr.Add(err)
					(*failedDeletedOperands).Add(o)
				}
				remainingOp.Remove(o)
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
	klog.V(3).Info("Getting the operater need to be delete")
	deployedOperands := gset.NewSet()
	for _, req := range requestInstance.Status.Members {
		deployedOperands.Add(req.Name)
	}

	currentOperands := gset.NewSet()
	if requestInstance.DeletionTimestamp.IsZero() {
		for _, req := range requestInstance.Spec.Requests {
			for _, op := range req.Operands {
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
		constant.OpreqLabel: "true",
	}
	annotations := map[string]string{
		registryKey.Namespace + "." + registryKey.Name + "/registry":             "true",
		registryKey.Namespace + "." + registryKey.Name + "/config":               "true",
		requestKey.Namespace + "." + requestKey.Name + "." + o.Name + "/request": o.Channel,
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

	// The namespace is 'openshift-operators' when installMode is cluster
	namespace := r.GetOperatorNamespace(o.InstallMode, o.Namespace)

	// Subscription Object
	sub := &olmv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:        o.Name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: &olmv1alpha1.SubscriptionSpec{
			Channel:                o.Channel,
			Package:                o.PackageName,
			CatalogSource:          o.SourceName,
			CatalogSourceNamespace: o.SourceNamespace,
			InstallPlanApproval:    o.InstallPlanApproval,
			StartingCSV:            o.StartingCSV,
			Config:                 o.SubscriptionConfig,
		},
	}
	sub.SetGroupVersionKind(schema.GroupVersionKind{Group: olmv1alpha1.SchemeGroupVersion.Group, Kind: "Subscription", Version: olmv1alpha1.SchemeGroupVersion.Version})
	klog.V(3).Info("Generating Subscription:  ", o.Name, " in the Namespace: ", namespace)
	co.subscription = sub
	return co
}

func generateOperatorGroup(namespace string, targetNamespaces []string) *olmv1.OperatorGroup {
	labels := map[string]string{
		constant.OpreqLabel: "true",
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

func (r *Reconciler) checkUninstallLabel(sub *olmv1alpha1.Subscription) bool {
	subLabels := sub.GetLabels()
	return subLabels[constant.NotUninstallLabel] == "true"
}

func compareSub(sub *olmv1alpha1.Subscription, originalSub *olmv1alpha1.Subscription) (needUpdate bool) {
	return !equality.Semantic.DeepEqual(sub.Spec, originalSub.Spec) || !equality.Semantic.DeepEqual(sub.Annotations, originalSub.Annotations)
}

func CheckSingletonServices(operator string) bool {
	singletonServices := []string{"ibm-cert-manager-operator", "ibm-licensing-operator"}
	return util.Contains(singletonServices, operator)
}
