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

package operandconfig

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/pkg/controller/common"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new OperandConfig Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileOperandConfig{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("operandconfig-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource OperandConfig
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandConfig{}}, &handler.EnqueueRequestForObject{}, predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileOperandConfig implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileOperandConfig{}

// ReconcileOperandConfig reconciles a OperandConfig object
type ReconcileOperandConfig struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a OperandConfig object and makes changes based on the state read
// and what is in the OperandConfig.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileOperandConfig) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the OperandConfig instance
	instance := &operatorv1alpha1.OperandConfig{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(1).Infof("Reconciling OperandConfig %s", request.NamespacedName)

	// Set the init status for OperandConfig instance
	if !instance.InitConfigStatus() {
		klog.V(3).Infof("Initializing the status of OperandConfig %s in the namespace %s", request.Name, request.Namespace)
		if err := r.client.Status().Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Set Finalizer for the OperandConfig
	added := instance.EnsureFinalizer()
	if added {
		if err := r.client.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	if instance.DeletionTimestamp != nil {
		requestList, err := fetch.FetchAllOperandRequests(r.client, map[string]string{instance.Namespace + "." + instance.Name + "/config": "true"})
		if err != nil {
			return reconcile.Result{}, err
		}
		if len(requestList.Items) == 0 {
			removed := instance.RemoveFinalizer()
			if removed {
				if err := r.client.Update(context.TODO(), instance); err != nil {
					return reconcile.Result{}, err
				}
			}
		} else {
			return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	return reconcile.Result{}, nil
}
