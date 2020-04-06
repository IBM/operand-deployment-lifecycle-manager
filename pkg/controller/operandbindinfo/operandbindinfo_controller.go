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
	"errors"

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
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

	// Watch for changes to primary resource OperandBindInfo
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandBindInfo{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner OperandBindInfo
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.OperandBindInfo{},
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
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
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

	klog.V(1).Infof("Reconciling OperandBindInfo %s in the namespace %s", bindInfoInstance.Name, bindInfoInstance.Namespace)

	// Initialize OperandBindInfo status
	bindInfoInstance.InitBindInfoStatus()
	klog.V(3).Info("Initializing OperandBindInfo instance status: ", request)
	if err := r.client.Status().Update(context.TODO(), bindInfoInstance); err != nil {
		return reconcile.Result{}, err
	}

	// Fetch the OperandRegistry instance
	registryInstance := &operatorv1alpha1.OperandRegistry{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: bindInfoInstance.Spec.Registry, Namespace: request.Namespace}, registryInstance); err != nil {
		if k8serr.IsNotFound(err) {
			r.recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "NotFound OperandRegistry %s from the namespace %s", bindInfoInstance.Spec.Registry, request.Namespace)
		}
		return reconcile.Result{}, err
	}

	merr := &util.MultiErr{}
	// Get the OperandRequest namespace
	if _, ok := registryInstance.Status.OperatorsStatus[bindInfoInstance.Spec.Operand]; !ok {
		return reconcile.Result{}, nil
	}
	requestNamespaces := registryInstance.Status.OperatorsStatus[bindInfoInstance.Spec.Operand].ReconcileRequests
	if len(requestNamespaces) == 0 {
		return reconcile.Result{}, nil
	}
	// Get the operand namespace
	var operandNamespace string
	for _, op := range registryInstance.Spec.Operators {
		if op.Name == bindInfoInstance.Spec.Operand {
			operandNamespace = op.Namespace
		}
	}

	if operandNamespace == "" {
		klog.Errorf("Not found operator %s in the OperandRegistry %s", bindInfoInstance.Spec.Operand, registryInstance.Name)
		return reconcile.Result{}, errors.New("not found operator in the OperandRegistry")
	}

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
			// Error reading the object - requeue the request.
			klog.Errorf("Not found OperandRequest %s in the namespace %s : %s", bindRequest.Name, bindRequest.Namespace, err)
			return reconcile.Result{}, client.IgnoreNotFound(err)
		}
		// Copy Secret and/or ConfigMap to the OperandRequest namespace
		klog.V(2).Infof("Copy secret and/or configmap to namespace %s", bindRequest.Namespace)
		for _, secretcm := range bindInfoInstance.Spec.Bindings {
			// Only copy the public bindInfo
			if secretcm.Scope == operatorv1alpha1.ScopePublic {
				// Copy Secret
				if secretcm.Secret != "" {
					klog.V(3).Infof("Copy secret %s to namespace %s", secretcm.Secret, bindRequest.Namespace)
					secret := &corev1.Secret{}
					if err := r.client.Get(context.TODO(), types.NamespacedName{Name: secretcm.Secret, Namespace: operandNamespace}, secret); err != nil {
						if k8serr.IsNotFound(err) {
							klog.Errorf("Failed to get Secret %s from the namespace %s", secretcm.Secret, request.Namespace)
							r.recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "No Secret %s in the namespace %s", secretcm.Secret, request.Namespace)
							continue
						} else {
							klog.Errorf("Failed to get Secret %s from the namespace %s : %s", secretcm.Secret, request.Namespace, err)
							merr.Add(err)
							continue
						}
					}
					// Create the Secret to the OperandRequest namespace
					secretCopy := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretcm.Secret,
							Namespace: bindRequest.Namespace,
						},
						Type:       secret.Type,
						Data:       secret.Data,
						StringData: secret.StringData,
					}
					// Set the OperandRequest as the controller of the Secret
					if err := controllerutil.SetControllerReference(requestInstance, secretCopy, r.scheme); err != nil {
						klog.Errorf("Failed to set OperandRequest %s as thr Owner of Secret %s : %s", requestInstance.Name, secretcm.Secret, err)
						merr.Add(err)
						continue
					}
					// Create the Secret in the OperandRequest namespace
					if err := r.client.Create(context.TODO(), secretCopy); err != nil {
						if k8serr.IsAlreadyExists(err) {
							// If already exist, update the Secret
							if err := r.client.Update(context.TODO(), secretCopy); err != nil {
								klog.Errorf("Failed to update Secret %s in the namespace %s : %s", secretcm.Secret, bindRequest.Namespace, err)
								merr.Add(err)
								continue
							}
						} else {
							klog.Errorf("Failed to create Secret %s in the namespace %s : %s", secretcm.Secret, bindRequest.Namespace, err)
							merr.Add(err)
							continue
						}
					}
				}
				// Copy ConfigMap
				if secretcm.Configmap != "" {
					klog.V(3).Infof("Copy config map %s to namespace %s", secretcm.Configmap, bindRequest.Namespace)
					cm := &corev1.ConfigMap{}
					if err := r.client.Get(context.TODO(), types.NamespacedName{Name: secretcm.Configmap, Namespace: operandNamespace}, cm); err != nil {
						if k8serr.IsNotFound(err) {
							klog.Errorf("Failed tp get Configmap %s from the namespace %s", secretcm.Configmap, request.Namespace)
							r.recorder.Eventf(bindInfoInstance, corev1.EventTypeWarning, "NotFound", "No Configmap %s in the namespace %s", secretcm.Configmap, request.Namespace)
							continue
						} else {
							klog.Errorf("Failed tp get Configmap %s from the namespace %s : %s", secretcm.Configmap, request.Namespace, err)
							merr.Add(err)
							continue
						}
					}
					// Create the ConfigMap to the OperandRequest namespace
					cmCopy := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      secretcm.Configmap,
							Namespace: bindRequest.Namespace,
						},
						Data: cm.Data,
					}
					// Set the OperandRequest as the controller of the configmap
					if err := controllerutil.SetControllerReference(requestInstance, cmCopy, r.scheme); err != nil {
						klog.Errorf("Failed to set OperandRequest %s as thr Owner of ComfigMap %s : %s", requestInstance.Name, secretcm.Configmap, err)
						merr.Add(err)
						continue
					}
					// Create the ConfigMap in the OperandRequest namespace
					if err := r.client.Create(context.TODO(), cmCopy); err != nil {
						if k8serr.IsAlreadyExists(err) {
							// If already exist, update the ConfigMap
							if err := r.client.Update(context.TODO(), cmCopy); err != nil {
								klog.Errorf("Failed to update ComfigMap %s in the namespace %s : %s", secretcm.Configmap, bindRequest.Namespace, err)
								merr.Add(err)
								continue
							}
						} else {
							klog.Errorf("Failed to create ComfigMap %s in the namespace %s : %s", secretcm.Configmap, bindRequest.Namespace, err)
							merr.Add(err)
							continue
						}
					}
				}
			}
		}
	}
	if len(merr.Errors) != 0 {
		if err := r.updateBindInfoPhase(bindInfoInstance, operatorv1alpha1.BindInfoFailed, requestNamespaces); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, merr
	}
	if err := r.updateBindInfoPhase(bindInfoInstance, operatorv1alpha1.BindInfoCompleted, requestNamespaces); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// Get the OperandBindInfo instance with the name and namespace
func (r *ReconcileOperandBindInfo) getBindInfoInstance(name, namespace string) (*operatorv1alpha1.OperandBindInfo, error) {
	klog.V(3).Info("Get the OperandBindInfo instance from the name: ", name, " namespace: ", namespace)
	// Fetch the OperandBindInfo instance
	bindInfo := &operatorv1alpha1.OperandBindInfo{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, bindInfo); err != nil {
		// Error reading the object - requeue the request.
		return nil, err
	}
	return bindInfo, nil
}
