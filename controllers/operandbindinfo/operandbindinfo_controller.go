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

package operandbindinfo

import (
	"context"
	"fmt"
	"reflect"
	"regexp"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

// Reconciler reconciles a OperandBindInfo object
type Reconciler struct {
	*deploy.ODLMOperator
}

var (
	publicPrefix, _    = regexp.Compile(`^public(.*)$`)
	privatePrefix, _   = regexp.Compile(`^private(.*)$`)
	protectedPrefix, _ = regexp.Compile(`^protected(.*)$`)
)

// Reconcile reads that state of the cluster for a OperandBindInfo object and makes changes based on the state read
// and what is in the OperandBindInfo.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *Reconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
	// Creat context for the OperandBindInfo reconciler
	ctx := context.Background()

	// Fetch the OperandBindInfo instance
	bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
	if err := r.Client.Get(ctx, req.NamespacedName, bindInfoInstance); err != nil {
		// Error reading the object - requeue the req.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	originalInstance := bindInfoInstance.DeepCopy()

	// Always attempt to patch the status after each reconciliation.
	defer func() {
		if reflect.DeepEqual(originalInstance.Status, bindInfoInstance.Status) {
			return
		}
		if err := r.Client.Status().Patch(ctx, bindInfoInstance, client.MergeFrom(originalInstance)); err != nil {
			reconcileErr = utilerrors.NewAggregate([]error{reconcileErr, fmt.Errorf("error while patching OperandBindInfo.Status: %v", err)})
		}
	}()

	klog.V(1).Infof("Reconciling OperandBindInfo: %s", req.NamespacedName)

	// If the finalizer is added, EnsureFinalizer() will return true. If the finalizer is already there, EnsureFinalizer() will return false
	if bindInfoInstance.EnsureFinalizer() {
		err := r.Update(ctx, bindInfoInstance)
		if err != nil {
			klog.Errorf("failed to update the OperandBindinfo %s: %v", req.NamespacedName.String(), err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Remove finalizer when DeletionTimestamp none zero
	if !bindInfoInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := r.cleanupCopies(ctx, bindInfoInstance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Update labels for the registry
	if bindInfoInstance.UpdateLabels() {
		if err := r.Update(ctx, bindInfoInstance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize OperandBindInfo status
	if !bindInfoInstance.InitBindInfoStatus() {
		klog.V(3).Infof("Initializing the status of OperandBindInfo %s in the namespace %s", req.Name, req.Namespace)
		if err := r.Status().Update(ctx, bindInfoInstance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Fetch the OperandRegistry instance
	registryKey := bindInfoInstance.GetRegistryKey()
	registryInstance := &operatorv1alpha1.OperandRegistry{}
	if err := r.Client.Get(ctx, registryKey, registryInstance); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Errorf("failed to find OperandRegistry from the NamespacedName %s: %v", registryKey.String(), err)
			r.Recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "NotFound OperandRegistry from the NamespacedName %s", registryKey.String())
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	merr := &util.MultiErr{}
	// Get the OperandRequest namespace
	requestNamespaces := registryInstance.Status.OperatorsStatus[bindInfoInstance.Spec.Operand].ReconcileRequests
	if len(requestNamespaces) == 0 {
		// There is no operand depend on the current bind info, nothing to do.
		return ctrl.Result{}, nil
	}
	// Get the operand namespace
	operandOperator := registryInstance.GetOperator(bindInfoInstance.Spec.Operand)
	if operandOperator == nil {
		klog.Errorf("failed to find operator %s in the OperandRegistry %s", bindInfoInstance.Spec.Operand, registryInstance.Name)
		r.Recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "NotFound operator %s in the OperandRegistry %s", bindInfoInstance.Spec.Operand, registryInstance.Name)
		return ctrl.Result{}, nil
	}
	operandNamespace := operandOperator.Namespace

	// If Secret or ConfigMap not found, reconcile will requeue after 1 min
	var requeue bool

	// Get OperandRequest instance and Copy Secret and/or ConfigMap
	for _, bindRequest := range requestNamespaces {
		// Get the OperandRequest of operandBindInfo
		requestInstance := &operatorv1alpha1.OperandRequest{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: bindRequest.Name, Namespace: bindRequest.Namespace}, requestInstance); err != nil {
			if apierrors.IsNotFound(err) {
				klog.Errorf("failed to find OperandRequest %s in the namespace %s: %v", bindRequest.Name, bindRequest.Namespace, err)
				r.Recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "NotFound OperandRequest %s in the namespace %s", bindRequest.Name, bindRequest.Namespace)
			}
			merr.Add(err)
			continue
		}
		// Get binding information from OperandRequest
		secretReq, cmReq := getBindingInfofromRequest(bindInfoInstance, requestInstance)
		// Copy Secret and/or ConfigMap to the OperandRequest namespace
		klog.V(3).Infof("Start to copy secret and/or configmap to the namespace %s", bindRequest.Namespace)
		for key, binding := range bindInfoInstance.Spec.Bindings {
			if !privatePrefix.MatchString(key) && !protectedPrefix.MatchString(key) && !publicPrefix.MatchString(key) {
				klog.Warningf("BindInfo key %s should have one of prefix: private, protected, public", key)
				continue
			}
			if operandNamespace != bindRequest.Namespace {
				// skip the private bindInfo
				if privatePrefix.MatchString(key) {
					continue
				}
			}
			// Copy Secret
			requeueSec, err := r.copySecret(ctx, binding.Secret, secretReq[key], operandNamespace, bindRequest.Namespace, key, bindInfoInstance, requestInstance)
			if err != nil {
				merr.Add(err)
				continue
			}
			requeue = requeue || requeueSec
			// Copy ConfigMap
			requeueCm, err := r.copyConfigmap(ctx, binding.Configmap, cmReq[key], operandNamespace, bindRequest.Namespace, key, bindInfoInstance, requestInstance)
			if err != nil {
				merr.Add(err)
				continue
			}
			requeue = requeue || requeueCm
		}
	}
	if len(merr.Errors) != 0 {
		r.updateBindInfoPhase(bindInfoInstance, operatorv1alpha1.BindInfoFailed, requestNamespaces)
		klog.Errorf("failed to reconcile the OperandBindinfo %s: %v", req.NamespacedName, merr)
		return ctrl.Result{}, merr
	}

	if requeue {
		r.updateBindInfoPhase(bindInfoInstance, operatorv1alpha1.BindInfoWaiting, requestNamespaces)
		return reconcile.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
	}

	r.updateBindInfoPhase(bindInfoInstance, operatorv1alpha1.BindInfoCompleted, requestNamespaces)

	klog.V(1).Infof("Finished reconciling OperandBindInfo: %s", req.NamespacedName)
	return ctrl.Result{}, nil
}

// Copy secret `sourceName` from source namespace `sourceNs` to target namespace `targetNs`
func (r *Reconciler) copySecret(ctx context.Context, sourceName, targetName, sourceNs, targetNs, key string,
	bindInfoInstance *operatorv1alpha1.OperandBindInfo, requestInstance *operatorv1alpha1.OperandRequest) (requeue bool, err error) {
	if sourceName == "" || sourceNs == "" || targetNs == "" {
		return false, nil
	}

	if sourceName == targetName && sourceNs == targetNs {
		return false, nil
	}

	if targetName == "" {
		if publicPrefix.MatchString(key) {
			targetName = bindInfoInstance.Name + "-" + sourceName
		} else {
			return false, nil
		}
	}

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: sourceName, Namespace: sourceNs}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("Secret %s is not found from the namespace %s", sourceName, sourceNs)
			r.Recorder.Eventf(bindInfoInstance, corev1.EventTypeNormal, "NotFound", "No Secret %s in the namespace %s", sourceName, sourceNs)
			return true, nil
		}
		return false, errors.Wrapf(err, "failed to get Secret %s/%s", sourceNs, sourceName)
	}
	// Create the Secret to the OperandRequest namespace
	secretLabel := make(map[string]string)
	// Copy from the original labels to the target labels
	for k, v := range secret.Labels {
		secretLabel[k] = v
	}
	secretLabel[bindInfoInstance.Namespace+"."+bindInfoInstance.Name+"/bindinfo"] = "true"
	secretLabel[constant.OpbiTypeLabel] = "copy"
	secretCopy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName,
			Namespace: targetNs,
			Labels:    secretLabel,
		},
		Type:       secret.Type,
		Data:       secret.Data,
		StringData: secret.StringData,
	}
	// Set the OperandRequest as the controller of the Secret
	if err := controllerutil.SetControllerReference(requestInstance, secretCopy, r.Scheme); err != nil {
		return false, errors.Wrapf(err, "failed to set OperandRequest %s as the owner of Secret %s", requestInstance.Name, targetName)
	}
	// Create the Secret in the OperandRequest namespace
	if err := r.Create(ctx, secretCopy); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If already exist, update the Secret
			if err := r.Update(ctx, secretCopy); err != nil {
				return false, errors.Wrapf(err, "failed to update secret %s/%s", targetNs, targetName)
			}
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to create secret %s/%s", targetNs, targetName)
	}

	ensureLabelsForSecret(secret, map[string]string{
		constant.OpbiNsLabel:   bindInfoInstance.Namespace,
		constant.OpbiNameLabel: bindInfoInstance.Name,
		constant.OpbiTypeLabel: "original",
	})

	// Update the operand Secret
	if err := r.Update(ctx, secret); err != nil {
		klog.Errorf("failed to update Secret %s in the namespace %s: %v", secret.Name, secret.Namespace, err)
		return false, err
	}
	klog.V(2).Infof("Copy secret %s from the namespace %s to secret %s in the namespace %s", sourceName, sourceNs, targetName, targetNs)

	return false, nil
}

// Copy configmap `sourceName` from namespace `sourceNs` to namespace `targetNs`
// and rename it to `targetName`
func (r *Reconciler) copyConfigmap(ctx context.Context, sourceName, targetName, sourceNs, targetNs, key string,
	bindInfoInstance *operatorv1alpha1.OperandBindInfo, requestInstance *operatorv1alpha1.OperandRequest) (requeue bool, err error) {
	if sourceName == "" || sourceNs == "" || targetNs == "" {
		return false, nil
	}

	if sourceName == targetName && sourceNs == targetNs {
		return false, nil
	}

	if targetName == "" {
		if publicPrefix.MatchString(key) {
			targetName = bindInfoInstance.Name + "-" + sourceName
		} else {
			return false, nil
		}
	}

	cm := &corev1.ConfigMap{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: sourceName, Namespace: sourceNs}, cm); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("Configmap %s/%s is not found", sourceNs, sourceName)
			r.Recorder.Eventf(bindInfoInstance, corev1.EventTypeNormal, "NotFound", "No Configmap %s in the namespace %s", sourceName, sourceNs)
			return true, nil
		}
		return false, errors.Wrapf(err, "failed to get Configmap %s/%s", sourceNs, sourceName)
	}
	// Create the ConfigMap to the OperandRequest namespace
	cmLabel := make(map[string]string)
	// Copy from the original labels to the target labels
	for k, v := range cm.Labels {
		cmLabel[k] = v
	}
	cmLabel[bindInfoInstance.Namespace+"."+bindInfoInstance.Name+"/bindinfo"] = "true"
	cmLabel[constant.OpbiTypeLabel] = "copy"
	cmCopy := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName,
			Namespace: targetNs,
			Labels:    cmLabel,
		},
		Data:       cm.Data,
		BinaryData: cm.BinaryData,
	}
	// Set the OperandRequest as the controller of the configmap
	if err := controllerutil.SetControllerReference(requestInstance, cmCopy, r.Scheme); err != nil {
		return false, errors.Wrapf(err, "failed to set OperandRequest %s as the owner of ConfigMap %s", requestInstance.Name, sourceName)
	}
	// Create the ConfigMap in the OperandRequest namespace
	if err := r.Create(ctx, cmCopy); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If already exist, update the ConfigMap
			if err := r.Update(ctx, cmCopy); err != nil {
				return false, errors.Wrapf(err, "failed to update ConfigMap %s/%s", targetNs, sourceName)
			}
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to create ConfigMap %s/%s", targetNs, sourceName)

	}
	// Set the OperandBindInfo label for the ConfigMap
	ensureLabelsForConfigMap(cm, map[string]string{
		constant.OpbiNsLabel:   bindInfoInstance.Namespace,
		constant.OpbiNameLabel: bindInfoInstance.Name,
		constant.OpbiTypeLabel: "original",
	})

	// Update the operand Configmap
	if err := r.Update(ctx, cm); err != nil {
		return false, errors.Wrapf(err, "failed to update ConfigMap %s/%s", cm.Namespace, cm.Name)
	}
	klog.V(2).Infof("Copy configmap %s from the namespace %s to the namespace %s", sourceName, sourceNs, targetNs)

	return false, nil
}

func (r *Reconciler) cleanupCopies(ctx context.Context, bindInfoInstance *operatorv1alpha1.OperandBindInfo) error {
	secretList := &corev1.SecretList{}
	cmList := &corev1.ConfigMapList{}

	opts := []client.ListOption{
		client.MatchingLabels(map[string]string{bindInfoInstance.Namespace + "." + bindInfoInstance.Name + "/bindinfo": "true"}),
	}
	if err := r.Client.List(ctx, secretList, opts...); err != nil {
		return err
	}
	if err := r.Client.List(ctx, cmList, opts...); err != nil {
		return err
	}

	for i := range secretList.Items {
		if err := r.Delete(ctx, &secretList.Items[i]); err != nil {
			return err
		}
	}

	for i := range cmList.Items {
		if err := r.Delete(ctx, &cmList.Items[i]); err != nil {
			return err
		}
	}
	// Update finalizer to allow delete CR
	removed := bindInfoInstance.RemoveFinalizer()
	if removed {
		err := r.Update(ctx, bindInfoInstance)
		if err != nil {
			return err
		}
	}
	return nil
}

func getBindingInfofromRequest(bindInfoInstance *operatorv1alpha1.OperandBindInfo, requestInstance *operatorv1alpha1.OperandRequest) (map[string]string, map[string]string) {
	secretReq, cmReq := make(map[string]string), make(map[string]string)
	for _, req := range requestInstance.Spec.Requests {
		if req.Registry != bindInfoInstance.Spec.Registry {
			continue
		}
		for _, operand := range req.Operands {
			if operand.Name != bindInfoInstance.Spec.Operand {
				continue
			}
			if len(operand.Bindings) == 0 {
				continue
			}
			for key, binding := range operand.Bindings {
				secretReq[key] = binding.Secret
				cmReq[key] = binding.Configmap
			}
		}
	}
	return secretReq, cmReq
}

func (r *Reconciler) getRegistryToRequestMapper(mgr manager.Manager) handler.ToRequestsFunc {
	ctx := context.Background()

	return func(object handler.MapObject) []reconcile.Request {
		mgrClient := mgr.GetClient()
		bindInfoList := &operatorv1alpha1.OperandBindInfoList{}
		opts := []client.ListOption{
			client.MatchingLabels(map[string]string{object.Meta.GetNamespace() + "." + object.Meta.GetName() + "/registry": "true"}),
		}

		_ = mgrClient.List(ctx, bindInfoList, opts...)

		bindinfos := []reconcile.Request{}
		for _, bindinfo := range bindInfoList.Items {
			namespaceName := types.NamespacedName{Name: bindinfo.Name, Namespace: bindinfo.Namespace}
			req := reconcile.Request{NamespacedName: namespaceName}
			bindinfos = append(bindinfos, req)
		}
		return bindinfos
	}
}

func (r *Reconciler) updateBindInfoPhase(bindInfoInstance *operatorv1alpha1.OperandBindInfo, phase operatorv1alpha1.BindInfoPhase, requestNamespaces []operatorv1alpha1.ReconcileRequest) {
	var requestNsList []string
	for _, ns := range requestNamespaces {
		if ns.Namespace == bindInfoInstance.Namespace {
			continue
		}
		requestNsList = append(requestNsList, ns.Namespace)
	}
	requestNsList = unique(requestNsList)
	if bindInfoInstance.Status.Phase == phase && reflect.DeepEqual(requestNsList, bindInfoInstance.Status.RequestNamespaces) {
		return
	}
	if len(requestNsList) != 0 {
		bindInfoInstance.Status.RequestNamespaces = requestNsList
	}
	bindInfoInstance.Status.Phase = phase
}

func unique(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func toOpbiRequest() handler.ToRequestsFunc {
	return func(object handler.MapObject) []reconcile.Request {
		opbiInstance := []reconcile.Request{}
		lables := object.Meta.GetLabels()
		name, nameOk := lables[constant.OpbiNameLabel]
		ns, namespaceOK := lables[constant.OpbiNsLabel]
		if nameOk && namespaceOK {
			opbiInstance = append(opbiInstance, reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: ns}})
		}
		return opbiInstance
	}
}

// SetupWithManager adds OperandBindInfo controller to the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	cmSecretPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			labels := e.Meta.GetLabels()
			for labelKey, labelValue := range labels {
				if labelKey == constant.OpbiTypeLabel {
					return labelValue == "original"
				}
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			labels := e.MetaNew.GetLabels()
			for labelKey, labelValue := range labels {
				if labelKey == constant.OpbiTypeLabel {
					return labelValue == "original"
				}
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			labels := e.Meta.GetLabels()
			for labelKey, labelValue := range labels {
				if labelKey == constant.OpbiTypeLabel {
					return labelValue == "original"
				}
			}
			return false
		},
	}

	opregPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObject := e.ObjectOld.(*operatorv1alpha1.OperandRegistry)
			newObject := e.ObjectNew.(*operatorv1alpha1.OperandRegistry)
			return !reflect.DeepEqual(oldObject.Status.OperatorsStatus, newObject.Status.OperatorsStatus)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.OperandBindInfo{}).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: toOpbiRequest()},
			builder.WithPredicates(cmSecretPredicates),
		).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: toOpbiRequest()},
			builder.WithPredicates(cmSecretPredicates),
		).
		Watches(
			&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}},
			&handler.EnqueueRequestsFromMapFunc{ToRequests: r.getRegistryToRequestMapper(mgr)},
			builder.WithPredicates(opregPredicates),
		).Complete(r)
}

func ensureLabelsForSecret(secret *corev1.Secret, labels map[string]string) {
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	for k, v := range labels {
		secret.Labels[k] = v
	}
}

func ensureLabelsForConfigMap(cm *corev1.ConfigMap, labels map[string]string) {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	for k, v := range labels {
		cm.Labels[k] = v
	}
}
