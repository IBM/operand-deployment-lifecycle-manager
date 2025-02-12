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

package operandbindinfo

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	ocproute "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
	"github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/util"
)

// Reconciler reconciles a OperandBindInfo object
type Reconciler struct {
	*deploy.ODLMOperator
	Config *rest.Config
}

var (
	publicPrefix, _    = regexp.Compile(`^public(.*)$`)
	privatePrefix, _   = regexp.Compile(`^private(.*)$`)
	protectedPrefix, _ = regexp.Compile(`^protected(.*)$`)
	routeGroupVersion  = "route.openshift.io/v1"
	isRouteAPI         = false
)

// +kubebuilder:rbac:groups=operator.ibm.com,namespace="placeholder",resources=operandbindinfos;operandbindinfos/status;operandbindinfos/finalizers,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a OperandBindInfo object and makes changes based on the state read
// and what is in the OperandBindInfo.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
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

	if isRouteAPI {
		klog.V(2).Info("Route API enabled")
	} else {
		klog.Info("Route API disabled")
	}

	// If the finalizer is added, EnsureFinalizer() will return true. If the finalizer is already there, EnsureFinalizer() will return false
	if bindInfoInstance.EnsureFinalizer() {
		err := r.Patch(ctx, bindInfoInstance, client.MergeFrom(originalInstance))
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
	registryInstance, err := r.GetOperandRegistry(ctx, registryKey)
	if err != nil {
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
	operandOperator, err := r.GetOperandFromRegistry(ctx, registryInstance, bindInfoInstance.Spec.Operand)
	if err != nil || operandOperator == nil {
		klog.Errorf("failed to find operator %s in the OperandRegistry %s: %v", bindInfoInstance.Spec.Operand, registryInstance.Name, err)
		r.Recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "NotFound operator %s in the OperandRegistry %s", bindInfoInstance.Spec.Operand, registryInstance.Name)
		return ctrl.Result{}, err
	}
	operandNamespace := registryInstance.Namespace

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
		} else if !requestInstance.ObjectMeta.DeletionTimestamp.IsZero() {
			klog.Warningf("OperandRequest %s/%s is being deleted, skip the OperandBindInfo %s/%s", requestInstance.Namespace, requestInstance.Name, bindInfoInstance.Namespace, bindInfoInstance.Name)
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

			if isRouteAPI {
				// Copy Route data into configmap and share configmap
				if binding.Route != nil {
					requeueRoute, err := r.copyRoute(ctx, *binding.Route, "", operandNamespace, bindRequest.Namespace, key, bindInfoInstance, requestInstance)
					if err != nil {
						merr.Add(err)
						continue
					}
					requeue = requeue || requeueRoute
				}
			}
			// Copy Service data into configmap and share configmap
			if binding.Service != nil {
				requeueService, err := r.copyService(ctx, *binding.Service, "", operandNamespace, bindRequest.Namespace, key, bindInfoInstance, requestInstance)
				if err != nil {
					merr.Add(err)
					continue
				}
				requeue = requeue || requeueService
			}
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

	klog.V(2).Infof("Finished reconciling OperandBindInfo: %s", req.NamespacedName)
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
	secretLabel[constant.ODLMWatchedLabel] = "true"
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
	// Set the OperandRequest as the owner of the Secret
	if err := controllerutil.SetOwnerReference(requestInstance, secretCopy, r.Scheme); err != nil {
		return false, errors.Wrapf(err, "failed to set OperandRequest %s as the owner of Secret %s", requestInstance.Name, targetName)
	}

	var podRefreshment bool
	// Create the Secret in the OperandRequest namespace
	if err := r.Create(ctx, secretCopy); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If already exist, update the Secret
			existingSecret := &corev1.Secret{}
			if err := r.Client.Get(ctx, types.NamespacedName{Namespace: targetNs, Name: targetName}, existingSecret); err != nil {
				return false, errors.Wrapf(err, "failed to get secret %s/%s", targetNs, targetName)
			}

			// Copy the owner reference from the existing secret to the new secret
			secretCopy.SetOwnerReferences(existingSecret.GetOwnerReferences())
			// Set the OperandRequest as the owner of the new Secret
			if err := controllerutil.SetOwnerReference(requestInstance, secretCopy, r.Scheme); err != nil {
				return false, errors.Wrapf(err, "failed to set OperandRequest %s as the owner of Secret %s", requestInstance.Name, targetName)
			}

			if needUpdate := util.CompareSecret(secretCopy, existingSecret); needUpdate {
				podRefreshment = true
				if err := r.Update(ctx, secretCopy); err != nil {
					return false, errors.Wrapf(err, "failed to update secret %s/%s", targetNs, targetName)
				}
			}
		} else {
			return false, errors.Wrapf(err, "failed to create secret %s/%s", targetNs, targetName)
		}
		if podRefreshment {
			if err := r.refreshPods(targetNs, targetName, "secret"); err != nil {
				return false, errors.Wrapf(err, "failed to refresh pods mounting secret %s/%s", targetNs, targetName)
			}
		}
	}

	util.EnsureLabelsForSecret(secret, map[string]string{
		bindInfoInstance.Namespace + "." + bindInfoInstance.Name + "/bindinfo": "true",
		constant.OpbiTypeLabel:    "original",
		constant.ODLMWatchedLabel: "true",
	})

	// Update the operand Secret
	if err := r.Update(ctx, secret); err != nil {
		klog.Errorf("failed to update Secret %s in the namespace %s: %v", secret.Name, secret.Namespace, err)
		return false, err
	}
	klog.V(1).Infof("Secret %s is copied from the namespace %s to secret %s in the namespace %s", sourceName, sourceNs, targetName, targetNs)

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
	cmLabel[constant.ODLMWatchedLabel] = "true"
	cmCopy := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName,
			Namespace: targetNs,
			Labels:    cmLabel,
		},
		Data:       cm.Data,
		BinaryData: cm.BinaryData,
	}
	// Set OperandRequest as the owners of the configmap
	if err := controllerutil.SetOwnerReference(requestInstance, cmCopy, r.Scheme); err != nil {
		return false, errors.Wrapf(err, "failed to set OperandRequest %s as the owner of ConfigMap %s", requestInstance.Name, sourceName)
	}

	var podRefreshment bool
	// Create the ConfigMap in the OperandRequest namespace
	if err := r.Create(ctx, cmCopy); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If already exist, update the ConfigMap
			existingCm := &corev1.ConfigMap{}
			if err := r.Client.Get(ctx, types.NamespacedName{Namespace: targetNs, Name: targetName}, existingCm); err != nil {
				return false, errors.Wrapf(err, "failed to get ConfigMap %s/%s", targetNs, targetName)
			}

			// Copy the owner reference from the existing secret to the new configmap
			cmCopy.SetOwnerReferences(existingCm.GetOwnerReferences())
			// Set the OperandRequest as the owner of the new configmap
			if err := controllerutil.SetOwnerReference(requestInstance, cmCopy, r.Scheme); err != nil {
				return false, errors.Wrapf(err, "failed to set OperandRequest %s as the owner of ConfigMap %s", requestInstance.Name, targetName)
			}

			if needUpdate := util.CompareConfigMap(cmCopy, existingCm); needUpdate {
				podRefreshment = true
				if err := r.Update(ctx, cmCopy); err != nil {
					return false, errors.Wrapf(err, "failed to update ConfigMap %s/%s", targetNs, sourceName)
				}
			}
		} else {
			return false, errors.Wrapf(err, "failed to create ConfigMap %s/%s", targetNs, sourceName)
		}
	}

	if podRefreshment {
		if err := r.refreshPods(targetNs, targetName, "configmap"); err != nil {
			return false, errors.Wrapf(err, "failed to refresh pods mounting ConfigMap %s/%s", targetNs, targetName)
		}
	}

	// Set the OperandBindInfo label for the ConfigMap
	util.EnsureLabelsForConfigMap(cm, map[string]string{
		bindInfoInstance.Namespace + "." + bindInfoInstance.Name + "/bindinfo": "true",
		constant.OpbiTypeLabel:    "original",
		constant.ODLMWatchedLabel: "true",
	})

	// Update the operand Configmap
	if err := r.Update(ctx, cm); err != nil {
		return false, errors.Wrapf(err, "failed to update ConfigMap %s/%s", cm.Namespace, cm.Name)
	}
	klog.V(1).Infof("Configmap %s is copied from the namespace %s to the namespace %s", sourceName, sourceNs, targetNs)

	return false, nil
}

// copyRoute reads the data map and copies OCP Route data as specified by the
// field path in the data map values
func (r *Reconciler) copyRoute(ctx context.Context, route operatorv1alpha1.Route, targetName, sourceNs, targetNs, key string,
	bindInfoInstance *operatorv1alpha1.OperandBindInfo, requestInstance *operatorv1alpha1.OperandRequest) (requeue bool, err error) {
	if route.Name == "" || sourceNs == "" || targetNs == "" {
		return false, nil
	}

	if route.Name == targetName && sourceNs == targetNs {
		return false, nil
	}

	if targetName == "" {
		if publicPrefix.MatchString(key) {
			targetName = bindInfoInstance.Name + "-" + route.Name
		} else {
			return false, nil
		}
	}

	sourceRoute := &ocproute.Route{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: route.Name, Namespace: sourceNs}, sourceRoute); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("Route %s/%s is not found", sourceNs, route.Name)
			r.Recorder.Eventf(bindInfoInstance, corev1.EventTypeNormal, "NotFound", "No Route %s in the namespace %s", route.Name, sourceNs)
			return true, nil
		}
		return false, errors.Wrapf(err, "failed to get Route %s/%s", sourceNs, route.Name)
	}
	// Create the ConfigMap to the OperandRequest namespace
	labels := make(map[string]string)
	// Copy from the original labels to the target labels
	for k, v := range sourceRoute.Labels {
		labels[k] = v
	}
	labels[bindInfoInstance.Namespace+"."+bindInfoInstance.Name+"/bindinfo"] = "true"
	labels[constant.OpbiTypeLabel] = "copy"

	sanitizedData, err := sanitizeOdlmRouteData(route.Data, sourceRoute.Spec)
	if err != nil {
		return false, err
	}
	cmCopy := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName,
			Namespace: targetNs,
			Labels:    labels,
		},
		Data: sanitizedData,
	}
	// Set the OperandRequest as the controller of the configmap
	if err := controllerutil.SetControllerReference(requestInstance, cmCopy, r.Scheme); err != nil {
		return false, errors.Wrapf(err, "failed to set OperandRequest %s as the owner of ConfigMap %s", requestInstance.Name, route.Name)
	}

	var podRefreshment bool
	// Create the ConfigMap in the OperandRequest namespace
	if err := r.Create(ctx, cmCopy); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If already exist, update the ConfigMap
			existingCm := &corev1.ConfigMap{}
			if err := r.Client.Get(ctx, types.NamespacedName{Namespace: targetNs, Name: targetName}, existingCm); err != nil {
				return false, errors.Wrapf(err, "failed to get ConfigMap %s/%s", targetNs, targetName)
			}
			if needUpdate := util.CompareConfigMap(cmCopy, existingCm); needUpdate {
				podRefreshment = true
				if err := r.Update(ctx, cmCopy); err != nil {
					return false, errors.Wrapf(err, "failed to update ConfigMap %s/%s", targetNs, route.Name)
				}
			}
		} else {
			return false, errors.Wrapf(err, "failed to create ConfigMap %s/%s", targetNs, route.Name)
		}
	}

	if podRefreshment {
		if err := r.refreshPods(targetNs, targetName, "configmap"); err != nil {
			return false, errors.Wrapf(err, "failed to refresh pods mounting ConfigMap %s/%s", targetNs, targetName)
		}
	}

	// Set the OperandBindInfo label for the ConfigMap
	util.EnsureLabelsForRoute(sourceRoute, map[string]string{
		bindInfoInstance.Namespace + "." + bindInfoInstance.Name + "/bindinfo": "true",
		constant.OpbiTypeLabel: "original",
	})

	// Update the operand Configmap
	if err := r.Update(ctx, sourceRoute); err != nil {
		return false, errors.Wrapf(err, "failed to update ConfigMap %s/%s", sourceRoute.Namespace, sourceRoute.Name)
	}
	klog.V(1).Infof("Route %s is copied from the namespace %s to the namespace %s", route.Name, sourceNs, targetNs)

	return false, nil
}

// copyRoute reads the data map and copies K8s Service data as specified by the
// field path in the data map values
func (r *Reconciler) copyService(ctx context.Context, service operatorv1alpha1.ServiceData, targetName, sourceNs, targetNs, key string,
	bindInfoInstance *operatorv1alpha1.OperandBindInfo, requestInstance *operatorv1alpha1.OperandRequest) (requeue bool, err error) {
	if service.Name == "" || sourceNs == "" || targetNs == "" {
		return false, nil
	}

	if service.Name == targetName && sourceNs == targetNs {
		return false, nil
	}

	if targetName == "" {
		if publicPrefix.MatchString(key) {
			targetName = bindInfoInstance.Name + "-" + service.Name
		} else {
			return false, nil
		}
	}

	sourceService := &corev1.Service{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: sourceNs}, sourceService); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(3).Infof("Route %s/%s is not found", sourceNs, service.Name)
			r.Recorder.Eventf(bindInfoInstance, corev1.EventTypeNormal, "NotFound", "No Service %s in the namespace %s", service.Name, sourceNs)
			return true, nil
		}
		return false, errors.Wrapf(err, "failed to get Service %s/%s", sourceNs, service.Name)
	}
	// Create the ConfigMap to the OperandRequest namespace
	labels := make(map[string]string)
	// Copy from the original labels to the target labels
	for k, v := range sourceService.Labels {
		labels[k] = v
	}
	labels[bindInfoInstance.Namespace+"."+bindInfoInstance.Name+"/bindinfo"] = "true"
	labels[constant.OpbiTypeLabel] = "copy"

	sanitizedData, err := sanitizeServiceData(service.Data, *sourceService)
	if err != nil {
		return false, err
	}
	cmCopy := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName,
			Namespace: targetNs,
			Labels:    labels,
		},
		Data: sanitizedData,
	}
	// Set the OperandRequest as the controller of the configmap
	if err := controllerutil.SetControllerReference(requestInstance, cmCopy, r.Scheme); err != nil {
		return false, errors.Wrapf(err, "failed to set OperandRequest %s as the owner of ConfigMap %s", requestInstance.Name, service.Name)
	}

	var podRefreshment bool
	// Create the ConfigMap in the OperandRequest namespace
	if err := r.Create(ctx, cmCopy); err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If already exist, update the ConfigMap
			existingCm := &corev1.ConfigMap{}
			if err := r.Client.Get(ctx, types.NamespacedName{Namespace: targetNs, Name: targetName}, existingCm); err != nil {
				return false, errors.Wrapf(err, "failed to get ConfigMap %s/%s", targetNs, targetName)
			}
			if needUpdate := util.CompareConfigMap(cmCopy, existingCm); needUpdate {
				podRefreshment = true
				if err := r.Update(ctx, cmCopy); err != nil {
					return false, errors.Wrapf(err, "failed to update ConfigMap %s/%s", targetNs, service.Name)
				}
			}
		} else {
			return false, errors.Wrapf(err, "failed to create ConfigMap %s/%s", targetNs, service.Name)
		}
	}

	if podRefreshment {
		if err := r.refreshPods(targetNs, targetName, "configmap"); err != nil {
			return false, errors.Wrapf(err, "failed to refresh pods mounting ConfigMap %s/%s", targetNs, targetName)
		}
	}

	// Set the OperandBindInfo label for the ConfigMap
	util.EnsureLabelsForService(sourceService, map[string]string{
		bindInfoInstance.Namespace + "." + bindInfoInstance.Name + "/bindinfo": "true",
		constant.OpbiTypeLabel: "original",
	})

	// Update the operand Configmap
	if err := r.Update(ctx, sourceService); err != nil {
		return false, errors.Wrapf(err, "failed to update ConfigMap %s/%s", sourceService.Namespace, sourceService.Name)
	}
	klog.V(1).Infof("Service %s is copied from the namespace %s to the namespace %s", service.Name, sourceNs, targetNs)

	return false, nil
}

// sanitizeOdlmRouteData takes a map, i.e. ODLM's Route.Data, and an OCP Route.Spec,
// and returns a map ready to be included into a ConfigMap's data. The ODLM's
// Route.Data is sanitized because the values are YAML path references
// in map because they correspond to YAML fields in a OCP Route. Ensures that:
//  1. the field actually exists, otherwise returns an error
//  2. extracts the value from the OCP Route's field, the value must be a basic
//     type, which includes: int, float, bool, and strings. Anything else and
//     an error is returned
func sanitizeOdlmRouteData(m map[string]string, route ocproute.RouteSpec) (map[string]string, error) {
	sanitized := make(map[string]string, len(m))
	for k, v := range m {
		fields := strings.Split(v, ".")
		fields = fields[1:]
		if fields[0] != "spec" {
			return nil, errors.Errorf("Bindable Route.Data must only reference values from OCP Route.Spec")
		}
		fields = fields[1:]

		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&route)
		if err != nil {
			return nil, err
		}
		newUnstr := &unstructured.Unstructured{}
		newUnstr.SetUnstructuredContent(content)

		trueValue, isExists, err := nestedBasicType(newUnstr.Object, fields...)
		if err != nil {
			return nil, err
		}
		if !isExists {
			return nil, errors.Errorf("Bindable Route.Data references a field that does not exist: %s", v)
		}

		sanitized[k] = trueValue
	}
	return sanitized, nil
}

// sanitizeServiceData takes a map, i.e. ODLM's Service.Data, and a K8s Service object
// and returns a map ready to be included into a ConfigMap's data. The ODLM's
// Service.Data is sanitized because the values are YAML fields in a K8s Service.
// Ensures that:
//  1. the field actually exists, otherwise returns an error
//  2. extracts the value from the K8s Service's field, the value will be
//     stringified
func sanitizeServiceData(m map[string]string, service corev1.Service) (map[string]string, error) {
	sanitized := make(map[string]string, len(m))
	for k, v := range m {
		trueValue, err := util.SanitizeObjectString(v, service)
		if err != nil {
			return nil, err
		}
		sanitized[k] = trueValue
	}
	return sanitized, nil
}

// nestedBasicType is a wrapper around the various unstructured.Nested methods
// used to extract values from nested fields. It takes the same arguments
// as the Nested methods and returns a string if some basic type value is found.
// Otherwise the bool and error values from the Nested functions will be
// returned.
// Basic types include: string, int64, float64, and bool
func nestedBasicType(obj map[string]interface{}, fields ...string) (string, bool, error) {
	typeTest, isExists, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil {
		return "", isExists, err
	}
	if !isExists {
		return "", isExists, err
	}

	switch fieldType := reflect.TypeOf(typeTest); fieldType.String() {
	case "string":
		return unstructured.NestedString(obj, fields...)
	case "int", "int32", "int64":
		var value int64
		value, isExists, err = unstructured.NestedInt64(obj, fields...)
		return fmt.Sprintf("%d", value), isExists, err
	case "float32", "float64":
		var value float64
		value, isExists, err = unstructured.NestedFloat64(obj, fields...)
		return fmt.Sprintf("%f", value), isExists, err
	case "bool":
		var value bool
		value, isExists, err = unstructured.NestedBool(obj, fields...)
		return fmt.Sprintf("%t", value), isExists, err
	default:
		return "", false, errors.Errorf("Path reference does not lead to an int, float, bool, or string: .spec.%s", strings.Join(fields, "."))
	}
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
	originalBind := bindInfoInstance.DeepCopy()
	removed := bindInfoInstance.RemoveFinalizer()
	if removed {
		err := r.Patch(ctx, bindInfoInstance, client.MergeFrom(originalBind))
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

func (r *Reconciler) getOperandRegistryToRequestMapper(mgr manager.Manager) handler.MapFunc {
	ctx := context.Background()

	return func(object client.Object) []reconcile.Request {
		mgrClient := mgr.GetClient()
		bindInfoList := &operatorv1alpha1.OperandBindInfoList{}
		opts := []client.ListOption{
			client.MatchingLabels(map[string]string{object.GetNamespace() + "." + object.GetName() + "/registry": "true"}),
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

func (r *Reconciler) getOperandRequestToRequestMapper(mgr manager.Manager) handler.MapFunc {
	ctx := context.Background()
	return func(a client.Object) []reconcile.Request {
		or := a.(*operatorv1alpha1.OperandRequest)
		reglist := or.GetAllRegistryReconcileRequest()
		mgrClient := mgr.GetClient()
		bindInfoList := &operatorv1alpha1.OperandBindInfoList{}

		for _, registry := range reglist {
			opts := []client.ListOption{
				client.MatchingLabels(map[string]string{registry.Namespace + "." + registry.Name + "/registry": "true"}),
			}

			_ = mgrClient.List(ctx, bindInfoList, opts...)
		}

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
	bindInfoInstance.Status.RequestNamespaces = requestNsList
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

func toOpbiRequest() handler.MapFunc {
	return func(object client.Object) []reconcile.Request {
		opbiInstance := []reconcile.Request{}
		labels := object.GetLabels()
		reg, _ := regexp.Compile(`^(.*)\.(.*)\/bindinfo`)
		for annotation := range labels {
			if reg.MatchString(annotation) {
				annotationSlices := strings.Split(annotation, ".")
				bindinfoNamespace := annotationSlices[0]
				bindinfoName := strings.Split(annotationSlices[1], "/")[0]
				opbiInstance = append(opbiInstance, reconcile.Request{NamespacedName: types.NamespacedName{Name: bindinfoName, Namespace: bindinfoNamespace}})
			}
		}
		return opbiInstance
	}
}

func (r *Reconciler) refreshPods(ns, name, resourceType string) error {
	merr := &util.MultiErr{}
	if err := r.refreshPodsFromDeploy(ns, name, resourceType); err != nil {
		merr.Add(err)
	}
	if err := r.refreshPodsFromSts(ns, name, resourceType); err != nil {
		merr.Add(err)
	}
	if err := r.refreshPodsFromDaemonSet(ns, name, resourceType); err != nil {
		merr.Add(err)
	}

	if len(merr.Errors) != 0 {
		return merr
	}

	return nil
}

func (r *Reconciler) refreshPodsFromDeploy(ns, name, resourceType string) error {
	timeNow := time.Now().Format("2006-1-2.1504")
	deploymentCandidates := []appsv1.Deployment{}
	deployments := &appsv1.DeploymentList{}
	opts := []client.ListOption{
		client.MatchingLabels{constant.BindInfoRefreshLabel: "enabled"},
		client.InNamespace(ns),
	}
	if err := r.Client.List(context.TODO(), deployments, opts...); err != nil {
		return fmt.Errorf("error getting deployments: %v", err)
	}
	for _, deployment := range deployments.Items {
		if resourceList, ok := deployment.Annotations["bindinfoRefresh/"+resourceType]; ok {
			resources := strings.Split(resourceList, ",")
			for _, r := range resources {
				if r == name {
					deploymentCandidates = append(deploymentCandidates, deployment)
					break
				}
			}

		}
	}
	for _, deployment := range deploymentCandidates {
		deployment := deployment
		//in case of deployments not having labels section, create the label section
		if deployment.ObjectMeta.Annotations == nil {
			deployment.ObjectMeta.Annotations = make(map[string]string)
		}
		if deployment.Spec.Template.ObjectMeta.Annotations == nil {
			deployment.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		deployment.ObjectMeta.Annotations["bindinfo/restartTime"] = timeNow
		deployment.Spec.Template.ObjectMeta.Annotations["bindinfo/restartTime"] = timeNow
		err := r.Client.Update(context.TODO(), &deployment)
		if err != nil {
			return fmt.Errorf("error updating deployment: %v", err)
		}
		klog.V(2).Infof("BindInfo controller refreshing deployment %s/%s to pick up updated bindinfos", deployment.Namespace, deployment.Name)
	}
	return nil
}

func (r *Reconciler) refreshPodsFromSts(ns, name, resourceType string) error {
	timeNow := time.Now().Format("2006-1-2.1504")
	statefulSetCandidates := []appsv1.StatefulSet{}
	statefulSets := &appsv1.StatefulSetList{}
	opts := []client.ListOption{
		client.MatchingLabels{constant.BindInfoRefreshLabel: "enabled"},
		client.InNamespace(ns),
	}
	if err := r.Client.List(context.TODO(), statefulSets, opts...); err != nil {
		return fmt.Errorf("error getting statefulSets: %v", err)
	}
	for _, statefulSet := range statefulSets.Items {
		if resourceList, ok := statefulSet.Annotations["bindinfoRefresh/"+resourceType]; ok {
			resources := strings.Split(resourceList, ",")
			for _, r := range resources {
				if r == name {
					statefulSetCandidates = append(statefulSetCandidates, statefulSet)
					break
				}
			}

		}
	}
	for _, statefulSet := range statefulSetCandidates {
		statefulSet := statefulSet
		//in case of statefulSets not having labels section, create the label section
		if statefulSet.ObjectMeta.Annotations == nil {
			statefulSet.ObjectMeta.Annotations = make(map[string]string)
		}
		if statefulSet.Spec.Template.ObjectMeta.Annotations == nil {
			statefulSet.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		statefulSet.ObjectMeta.Annotations["bindinfo/restartTime"] = timeNow
		statefulSet.Spec.Template.ObjectMeta.Annotations["bindinfo/restartTime"] = timeNow
		err := r.Client.Update(context.TODO(), &statefulSet)
		if err != nil {
			return fmt.Errorf("error updating StatefulSet: %v", err)
		}
		klog.V(2).Infof("BindInfo controller refreshing StatefulSet %s/%s to pick up updated bindinfos", statefulSet.Namespace, statefulSet.Name)
	}
	return nil
}

func (r *Reconciler) refreshPodsFromDaemonSet(ns, name, resourceType string) error {
	timeNow := time.Now().Format("2006-1-2.1504")
	daemonSetCandidates := []appsv1.DaemonSet{}
	daemonSets := &appsv1.DaemonSetList{}
	opts := []client.ListOption{
		client.MatchingLabels{constant.BindInfoRefreshLabel: "enabled"},
		client.InNamespace(ns),
	}
	if err := r.Client.List(context.TODO(), daemonSets, opts...); err != nil {
		return fmt.Errorf("error getting daemonSets: %v", err)
	}
	for _, daemonSet := range daemonSets.Items {
		if resourceList, ok := daemonSet.Annotations["bindinfoRefresh/"+resourceType]; ok {
			resources := strings.Split(resourceList, ",")
			for _, r := range resources {
				if r == name {
					daemonSetCandidates = append(daemonSetCandidates, daemonSet)
					break
				}
			}

		}
	}
	for _, daemonSet := range daemonSetCandidates {
		daemonSet := daemonSet
		//in case of daemonSets not having labels section, create the label section
		if daemonSet.ObjectMeta.Annotations == nil {
			daemonSet.ObjectMeta.Annotations = make(map[string]string)
		}
		if daemonSet.Spec.Template.ObjectMeta.Annotations == nil {
			daemonSet.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}
		daemonSet.ObjectMeta.Annotations["bindinfo/restartTime"] = timeNow
		daemonSet.Spec.Template.ObjectMeta.Annotations["bindinfo/restartTime"] = timeNow
		err := r.Client.Update(context.TODO(), &daemonSet)
		if err != nil {
			return fmt.Errorf("error updating daemonSet: %v", err)
		}
		klog.V(2).Infof("BindInfo controller refreshing daemonSet %s/%s to pick up updated bindinfos", daemonSet.Namespace, daemonSet.Name)
	}
	return nil
}

// SetupWithManager adds OperandBindInfo controller to the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	bindablePredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			labels := e.Object.GetLabels()
			for labelKey, labelValue := range labels {
				if labelKey == constant.OpbiTypeLabel {
					return labelValue == "original"
				}
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			labels := e.ObjectNew.GetLabels()
			for labelKey, labelValue := range labels {
				if labelKey == constant.OpbiTypeLabel {
					return labelValue == "original"
				}
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
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

	options := controller.Options{
		MaxConcurrentReconciles: r.MaxConcurrentReconciles, // Set the desired value for max concurrent reconciles.
	}

	var err error
	if r.Config == nil {
		r.Config, err = config.GetConfig()
		if err != nil {
			klog.Errorf("Failed to get config: %v", err)
			return err
		}
	}
	dc := discovery.NewDiscoveryClientForConfigOrDie(r.Config)
	_, apiLists, err := dc.ServerGroupsAndResources()
	if err != nil {
		return err
	}
	for _, apiList := range apiLists {
		if apiList.GroupVersion == routeGroupVersion {
			isRouteAPI = true
		}
	}

	controller := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&operatorv1alpha1.OperandBindInfo{}).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(toOpbiRequest()),
			builder.WithPredicates(bindablePredicates),
		).
		Watches(
			&source.Kind{Type: &corev1.Secret{}},
			handler.EnqueueRequestsFromMapFunc(toOpbiRequest()),
			builder.WithPredicates(bindablePredicates),
		).
		Watches(
			&source.Kind{Type: &operatorv1alpha1.OperandRequest{}},
			handler.EnqueueRequestsFromMapFunc(r.getOperandRequestToRequestMapper(mgr)),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}},
			handler.EnqueueRequestsFromMapFunc(r.getOperandRegistryToRequestMapper(mgr)),
			builder.WithPredicates(opregPredicates),
		)
	if isRouteAPI {
		controller.Watches(
			&source.Kind{Type: &ocproute.Route{}},
			handler.EnqueueRequestsFromMapFunc(toOpbiRequest()),
			builder.WithPredicates(bindablePredicates),
		)
	}
	return controller.Complete(r)
}
