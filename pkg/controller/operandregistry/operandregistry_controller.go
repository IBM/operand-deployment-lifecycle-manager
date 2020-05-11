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

package operandregistry

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new OperandRegistry Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileOperandRegistry{
		client:   mgr.GetClient(),
		recorder: mgr.GetEventRecorderFor("operandregistry"),
		scheme:   mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("operandregistry-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource OperandRegistry
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileOperandRegistry implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileOperandRegistry{}

// ReconcileOperandRegistry reconciles a OperandRegistry object
type ReconcileOperandRegistry struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

// Reconcile reads that state of the cluster for a OperandRegistry object and makes changes based on the state read
// and what is in the OperandRegistry.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileOperandRegistry) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the OperandRegistry instance
	instance := &operatorv1alpha1.OperandRegistry{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(1).Infof("Reconciling OperandRegistry %s", request.NamespacedName)

	// Set the default scope for OperandRegistry instance
	instance.SetDefaultsRegistry()
	if err := r.client.Update(context.TODO(), instance); err != nil {
		return reconcile.Result{}, err
	}
	// Set the default status for OperandRegistry instance
	instance.InitRegistryStatus()
	klog.V(3).Info("Initializing the status of OperandRegistry %s in the namespace %s", request.Name, request.Namespace)
	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateOperandRequestStatus(request); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateOperandBindInfoStatus(request); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// updateOperandRequestStatus update the cluster phase of OperandRequest
// When the registry changed, query all the OperandRequests that use it, and then
// update these OperandRequest's Phase to Updating, to trigger reconcile of these OperandRequests.
func (r *ReconcileOperandRegistry) updateOperandRequestStatus(request reconcile.Request) error {
	// Get the requests related with current registry
	requestList := &operatorv1alpha1.OperandRequestList{}

	opts := []client.ListOption{
		client.MatchingLabels(map[string]string{request.Namespace + "." + request.Name + "/registry": "true"}),
	}
	if err := r.client.List(context.TODO(), requestList, opts...); err != nil {
		return err
	}

	for _, req := range requestList.Items {
		req.SetUpdatingClusterPhase()
		if err := r.client.Status().Update(context.TODO(), &req); err != nil {
			return err
		}
	}
	return nil
}

// updateOperandBindInfoStatus update the phase of OperandBindInfo
// When the registry changed, query all the OperandBindInfo that use it, and then
// update these OperandBindInfo's Phase to Updating, to trigger reconcile of these OperandBindInfo.
func (r *ReconcileOperandRegistry) updateOperandBindInfoStatus(request reconcile.Request) error {
	// Get the bindinfos related with current registry
	bindInfoList := &operatorv1alpha1.OperandBindInfoList{}

	opts := []client.ListOption{
		client.MatchingLabels(map[string]string{request.Namespace + "." + request.Name + "/registry": "true"}),
	}
	if err := r.client.List(context.TODO(), bindInfoList, opts...); err != nil {
		return err
	}

	for _, binding := range bindInfoList.Items {
		binding.SetUpdatingBindInfoPhase()
		if err := r.client.Status().Update(context.TODO(), &binding); err != nil {
			return err
		}
	}
	return nil
}
