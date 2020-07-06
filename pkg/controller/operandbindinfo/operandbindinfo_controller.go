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

package operandbindinfo

import (
	"context"
	"reflect"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	util "github.com/IBM/operand-deployment-lifecycle-manager/pkg/util"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new OperandBindInfo Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileOperandBindInfo{
		client:   mgr.GetClient(),
		recorder: mgr.GetEventRecorderFor("OperandBindInfo"),
		scheme:   mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("operandbindinfo-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	predicateRegistrySpec := predicate.Funcs{
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

	// Watch for changes to primary resource OperandBindInfo
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandBindInfo{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for OperandRegistry status changes and requeue the OperandRequest
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: getRegistryToRquestMapper(mgr),
	}, predicateRegistrySpec)

	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Secret and requeue the owner OperandBindInfo
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		OwnerType: &operatorv1alpha1.OperandBindInfo{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource ConfigMap and requeue the owner OperandBindInfo
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		OwnerType: &operatorv1alpha1.OperandBindInfo{},
	})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileOperandBindInfo implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileOperandBindInfo{}

// ReconcileOperandBindInfo reconciles a OperandBindInfo object
type ReconcileOperandBindInfo struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

// Reconcile reads that state of the cluster for a OperandBindInfo object and makes changes based on the state read
// and what is in the OperandBindInfo.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileOperandBindInfo) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the OperandBindInfo instance
	bindInfoInstance := &operatorv1alpha1.OperandBindInfo{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, bindInfoInstance); err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(1).Infof("Reconciling OperandBindInfo %s", request.NamespacedName)
	// Update labels for the reqistry
	if bindInfoInstance.UpdateLabels() {
		if err := r.client.Update(context.TODO(), bindInfoInstance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Initialize OperandBindInfo status
	if !bindInfoInstance.InitBindInfoStatus() {
		klog.V(3).Infof("Initializing the status of OperandBindInfo %s in the namespace %s", request.Name, request.Namespace)
		if err := r.client.Status().Update(context.TODO(), bindInfoInstance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Fetch the OperandRegistry instance
	registryKey := bindInfoInstance.GetRegistryKey()
	registryInstance := &operatorv1alpha1.OperandRegistry{}
	if err := r.client.Get(context.TODO(), registryKey, registryInstance); err != nil {
		if k8serr.IsNotFound(err) {
			klog.Errorf("NotFound OperandRegistry from the NamespacedName %s", registryKey.String())
			r.recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "NotFound OperandRegistry from the NamespacedName %s", registryKey.String())
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	merr := &util.MultiErr{}
	// Get the OperandRequest namespace
	requestNamespaces := registryInstance.Status.OperatorsStatus[bindInfoInstance.Spec.Operand].ReconcileRequests
	if len(requestNamespaces) == 0 {
		// There is no operand depend on the current bind info, nothing to do.
		return reconcile.Result{}, nil
	}
	// Get the operand namespace
	operandNamespace := registryInstance.GetOperator(bindInfoInstance.Spec.Operand).Namespace
	if operandNamespace == "" {
		klog.Errorf("Not found operator %s in the OperandRegistry %s", bindInfoInstance.Spec.Operand, registryInstance.Name)
		r.recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "NotFound operator %s in the OperandRegistry %s", bindInfoInstance.Spec.Operand, registryInstance.Name)
		return reconcile.Result{}, nil
	}

	// If Secret or ConfigMap not found, reconcile will requeue after 1 min
	var requeue bool

	// Get OperandRequest instance and Copy Secret and/or ConfigMap
	for _, bindRequest := range requestNamespaces {
		if operandNamespace == bindRequest.Namespace {
			// Skip the namespace of OperandBindInfo
			klog.V(3).Infof("Skip to copy secret and/or configmap to themselves namespace %s", bindRequest.Namespace)
			continue
		}
		// Get the OperandRequest of operandBindInfo
		requestInstance := &operatorv1alpha1.OperandRequest{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Name: bindRequest.Name, Namespace: bindRequest.Namespace}, requestInstance); err != nil {
			if k8serr.IsNotFound(err) {
				klog.Errorf("Not found OperandRequest %s in the namespace %s: %s", bindRequest.Name, bindRequest.Namespace, err)
				r.recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "NotFound OperandRequest %s in the namespace %s", bindRequest.Name, bindRequest.Namespace)
			}
			merr.Add(err)
			continue
		}
		// Get binding information from OperandRequest
		secretReq, cmReq := getBindingInfofromRequest(bindInfoInstance, requestInstance)
		// Copy Secret and/or ConfigMap to the OperandRequest namespace
		klog.V(3).Infof("Start to copy secret and/or configmap to the namespace %s", bindRequest.Namespace)
		// Only copy the public bindInfo
		for key, binding := range bindInfoInstance.Spec.Bindings {
			reg, _ := regexp.Compile(`^public(.*)$`)
			if !reg.MatchString(key) {
				continue
			}
			// Copy Secret
			requeueSec, err := r.copySecret(binding.Secret, secretReq[key], operandNamespace, bindRequest.Namespace, bindInfoInstance, requestInstance)
			if err != nil {
				merr.Add(err)
				continue
			}
			requeue = requeue || requeueSec
			// Copy ConfigMap
			requeueCm, err := r.copyConfigmap(binding.Configmap, cmReq[key], operandNamespace, bindRequest.Namespace, bindInfoInstance, requestInstance)
			if err != nil {
				merr.Add(err)
				continue
			}
			requeue = requeue || requeueCm
		}
	}
	if len(merr.Errors) != 0 {
		if err := r.updateBindInfoPhase(bindInfoInstance, operatorv1alpha1.BindInfoFailed, requestNamespaces); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, merr
	}

	if requeue {
		if err := r.updateBindInfoPhase(bindInfoInstance, operatorv1alpha1.BindInfoWaiting, requestNamespaces); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := r.updateBindInfoPhase(bindInfoInstance, operatorv1alpha1.BindInfoCompleted, requestNamespaces); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// Copy secret `sourceName` from source namespace `sourceNs` to target namespace `targetNs`
func (r *ReconcileOperandBindInfo) copySecret(sourceName, targetName, sourceNs, targetNs string,
	bindInfoInstance *operatorv1alpha1.OperandBindInfo, requestInstance *operatorv1alpha1.OperandRequest) (bool, error) {
	if sourceName == "" || sourceNs == "" || targetNs == "" {
		return false, nil
	}

	if targetName == "" {
		targetName = bindInfoInstance.Name + "-" + sourceName
	}

	secret := &corev1.Secret{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: sourceName, Namespace: sourceNs}, secret); err != nil {
		if k8serr.IsNotFound(err) {
			klog.Warningf("Secret %s is not found from the namespace %s", sourceName, sourceNs)
			r.recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "No Secret %s in the namespace %s", sourceName, sourceNs)
			return true, nil
		}
		klog.Errorf("Failed to get Secret %s from the namespace %s: %s", sourceName, sourceNs, err)
		return false, err
	}
	// Create the Secret to the OperandRequest namespace
	secretCopy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName,
			Namespace: targetNs,
			Labels:    secret.Labels,
		},
		Type:       secret.Type,
		Data:       secret.Data,
		StringData: secret.StringData,
	}
	// Set the OperandRequest as the controller of the Secret
	if err := controllerutil.SetControllerReference(requestInstance, secretCopy, r.scheme); err != nil {
		klog.Errorf("Failed to set OperandRequest %s as the owner of Secret %s: %s", requestInstance.Name, targetName, err)
		return false, err
	}
	// Create the Secret in the OperandRequest namespace
	if err := r.client.Create(context.TODO(), secretCopy); err != nil {
		if k8serr.IsAlreadyExists(err) {
			// If already exist, update the Secret
			existingSecret := &corev1.Secret{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{
				Name:      targetName,
				Namespace: targetNs,
			}, existingSecret); err != nil {
				klog.Errorf("Failed to get the existing secret %s in the namespace %s: %s", sourceName, targetNs, err)
				return false, err
			}
			if reflect.DeepEqual(existingSecret.Data, secretCopy.Data) && reflect.DeepEqual(existingSecret.StringData, secretCopy.StringData) {
				klog.V(3).Infof("There is no change in Secret %s in the namespace %s. Skip the update", targetName, targetNs)
				return false, nil
			}
			existingSecret.Data, existingSecret.StringData = secretCopy.Data, secretCopy.StringData
			if err := r.client.Update(context.TODO(), existingSecret); err != nil {
				klog.Errorf("Failed to update secret %s in the namespace %s: %s", targetName, targetNs, err)
				return false, err
			}
			return false, nil
		}
		klog.Errorf("Failed to create secret %s in the namespace %s: %s", targetName, targetNs, err)
		return false, err
	}
	// Set the OperandBindInfo as the controller of the operand Secret
	if err := controllerutil.SetOwnerReference(bindInfoInstance, secret, r.scheme); err != nil {
		klog.Errorf("Failed to set OperandRequest %s as the owner of Secret %s: %s", secret.Name, sourceName, err)
		return false, err
	}
	// Update the operand Secret
	if err := r.client.Update(context.TODO(), secret); err != nil {
		klog.Errorf("Failed to update Secret %s in the namespace %s: %s", secret.Name, secret.Namespace, err)
		return false, err
	}
	klog.V(2).Infof("Copy secret %s from the namespace %s to secret %s in the namespace %s", sourceName, sourceNs, targetName, targetNs)

	return false, nil
}

// Copy configmap `sourceName` from namespace `sourceNs` to namespace `targetNs`
// and rename it to `targetName`
func (r *ReconcileOperandBindInfo) copyConfigmap(sourceName, targetName, sourceNs, targetNs string,
	bindInfoInstance *operatorv1alpha1.OperandBindInfo, requestInstance *operatorv1alpha1.OperandRequest) (bool, error) {
	if sourceName == "" || sourceNs == "" || targetNs == "" {
		return false, nil
	}

	if targetName == "" {
		targetName = bindInfoInstance.Name + "-" + sourceName
	}

	cm := &corev1.ConfigMap{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: sourceName, Namespace: sourceNs}, cm); err != nil {
		if k8serr.IsNotFound(err) {
			klog.Warningf("Configmap %s is not found from the namespace %s", sourceName, sourceNs)
			r.recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "No Configmap %s in the namespace %s", sourceName, sourceNs)
			return true, nil
		}
		klog.Errorf("Failed to get Configmap %s from the namespace %s: %s", sourceName, sourceNs, err)
		return false, err
	}
	// Create the ConfigMap to the OperandRequest namespace
	cmCopy := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName,
			Namespace: targetNs,
			Labels:    cm.Labels,
		},
		Data:       cm.Data,
		BinaryData: cm.BinaryData,
	}
	// Set the OperandRequest as the controller of the configmap
	if err := controllerutil.SetControllerReference(requestInstance, cmCopy, r.scheme); err != nil {
		klog.Errorf("Failed to set OperandRequest %s as the owner of ConfigMap %s: %s", requestInstance.Name, sourceName, err)
		return false, err
	}
	// Create the ConfigMap in the OperandRequest namespace
	if err := r.client.Create(context.TODO(), cmCopy); err != nil {
		if k8serr.IsAlreadyExists(err) {
			// If already exist, update the ConfigMap
			existingCm := &corev1.ConfigMap{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{
				Name:      targetName,
				Namespace: targetNs,
			}, existingCm); err != nil {
				klog.Errorf("Failed to get the existing ConfigMap %s in the namespace %s: %s", sourceName, targetNs, err)
				return false, err
			}
			if reflect.DeepEqual(existingCm.Data, cmCopy.Data) && reflect.DeepEqual(existingCm.BinaryData, cmCopy.BinaryData) {
				klog.V(3).Infof("There is no change in ConfigMap %s in the namespace %s. Skip the update", targetName, targetNs)
				return false, nil
			}
			existingCm.Data, existingCm.BinaryData = cmCopy.Data, cmCopy.BinaryData
			if err := r.client.Update(context.TODO(), existingCm); err != nil {
				klog.Errorf("Failed to update ConfigMap %s in the namespace %s: %s", sourceName, targetNs, err)
				return false, err
			}
			return false, nil
		}
		klog.Errorf("Failed to create ConfigMap %s in the namespace %s: %s", sourceName, targetNs, err)
		return false, err

	}
	// Set the OperandBindInfo as the controller of the operand Configmap
	if err := controllerutil.SetOwnerReference(bindInfoInstance, cm, r.scheme); err != nil {
		klog.Errorf("Failed to set OperandRequest %s as the owner of Configmap %s: %s", bindInfoInstance.Name, sourceName, err)
		return false, err
	}
	// Update the operand Configmap
	if err := r.client.Update(context.TODO(), cm); err != nil {
		klog.Errorf("Failed to update Configmap %s in the namespace %s: %s", cm.Name, cm.Namespace, err)
		return false, err
	}
	klog.V(2).Infof("Copy configmap %s from the namespace %s to the namespace %s", sourceName, sourceNs, targetNs)

	return false, nil
}

// Get the OperandBindInfo instance with the name and namespace
func (r *ReconcileOperandBindInfo) getBindInfoInstance(name, namespace string) (*operatorv1alpha1.OperandBindInfo, error) {
	klog.V(3).Infof("Get the OperandBindInfo %s from the namespace %s", name, namespace)
	// Fetch the OperandBindInfo instance
	bindInfo := &operatorv1alpha1.OperandBindInfo{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, bindInfo); err != nil {
		// Error reading the object - requeue the request.
		return nil, err
	}
	return bindInfo, nil
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
				reg, _ := regexp.Compile(`^public(.*)$`)
				if !reg.MatchString(key) {
					continue
				}
				secretReq[key] = binding.Secret
				cmReq[key] = binding.Configmap
			}
		}
	}
	return secretReq, cmReq
}

func getRegistryToRquestMapper(mgr manager.Manager) handler.ToRequestsFunc {
	return func(object handler.MapObject) []reconcile.Request {
		mgrClient := mgr.GetClient()
		bindInfoList := &operatorv1alpha1.OperandBindInfoList{}
		opts := []client.ListOption{
			client.MatchingLabels(map[string]string{object.Meta.GetNamespace() + "." + object.Meta.GetName() + "/registry": "true"}),
		}

		_ = mgrClient.List(context.TODO(), bindInfoList, opts...)

		bindinfos := []reconcile.Request{}
		for _, bindinfo := range bindInfoList.Items {
			namespaceName := types.NamespacedName{Name: bindinfo.Name, Namespace: bindinfo.Namespace}
			req := reconcile.Request{NamespacedName: namespaceName}
			bindinfos = append(bindinfos, req)
		}
		return bindinfos
	}
}
