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
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// AddBindInfoController creates a new OperandRegistry Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddBindInfoController(mgr manager.Manager) error {
	return addBindinfoController(mgr, newReconcilerBindInfoController(mgr))
}

// newReconcilerBindInfoController returns a new reconcile.Reconciler
func newReconcilerBindInfoController(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileOperandRegistryBindinfo{
		client:   mgr.GetClient(),
		recorder: mgr.GetEventRecorderFor("operandregistry"),
		scheme:   mgr.GetScheme()}
}

// addBindinfoController adds a new Controller to mgr with r as the reconcile.Reconciler
func addBindinfoController(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("operandregistry-bindinfo-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	predicatebindinfo := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObject := e.ObjectOld.(*operatorv1alpha1.OperandRegistry)
			newObject := e.ObjectNew.(*operatorv1alpha1.OperandRegistry)
			return !reflect.DeepEqual(oldObject.Status, newObject.Status)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}

	// Watch for changes to primary resource OperandRegistry
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, &handler.EnqueueRequestForObject{}, predicatebindinfo)
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileOperandRegistryBindinfo implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileOperandRegistryBindinfo{}

// ReconcileOperandRegistryBindinfo reconciles a OperandRegistry object
type ReconcileOperandRegistryBindinfo struct {
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
func (r *ReconcileOperandRegistryBindinfo) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the OperandRegistry instance
	instance := &operatorv1alpha1.OperandRegistry{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(1).Infof("Reconciling OperandRegistry for OperandBindInfo %s", request.NamespacedName)

	if err := r.updateOperandBindInfoStatus(request); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// updateOperandBindInfoStatus update the phase of OperandBindInfo
// When the registry changed, query all the OperandBindInfo that use it, and then
// update these OperandBindInfo's Phase to Updating, to trigger reconcile of these OperandBindInfo.
func (r *ReconcileOperandRegistryBindinfo) updateOperandBindInfoStatus(request reconcile.Request) error {
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
