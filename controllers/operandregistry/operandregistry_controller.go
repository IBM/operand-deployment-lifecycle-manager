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

package operandregistry

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/v4/api/v1alpha1"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/v4/controllers/operator"
)

// Reconciler reconciles a OperandRegistry object
type Reconciler struct {
	*deploy.ODLMOperator
}

// +kubebuilder:rbac:groups=operator.ibm.com,namespace="placeholder",resources=operandregistries;operandregistries/status;operandregistries/finalizers,verbs=get;list;watch;create;update;patch;delete

// Reconcile reads that state of the cluster for a OperandRegistry object and makes changes based on the state read
// and what is in the OperandRegistry.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
	// Fetch the OperandRegistry instance
	instance := &operatorv1alpha1.OperandRegistry{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	originalInstance := instance.DeepCopy()

	// Always attempt to patch the status after each reconciliation.
	defer func() {
		if reflect.DeepEqual(originalInstance.Status, instance.Status) {
			return
		}
		if err := r.Client.Status().Patch(ctx, instance, client.MergeFrom(originalInstance)); err != nil {
			reconcileErr = utilerrors.NewAggregate([]error{reconcileErr, fmt.Errorf("error while patching OperandRegistry.Status: %v", err)})
		}
	}()

	klog.V(2).Infof("Reconciling OperandRegistry: %s", req.NamespacedName)

	// Update all the operator status
	if err := r.updateStatus(ctx, instance); err != nil {
		klog.Errorf("failed to update the status for OperandRegistry %s : %v", req.NamespacedName.String(), err)
		return ctrl.Result{}, err
	}

	// Summarize instance status
	if len(instance.Status.OperatorsStatus) == 0 {
		instance.UpdateRegistryPhase(operatorv1alpha1.RegistryReady)
	} else {
		instance.UpdateRegistryPhase(operatorv1alpha1.RegistryRunning)
	}

	klog.V(2).Infof("Finished reconciling OperandRegistry: %s", req.NamespacedName)
	return ctrl.Result{}, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, instance *operatorv1alpha1.OperandRegistry) error {
	// List the OperandRequests refer the OperatorRegistry by label of the OperandRequests
	requestList, err := r.ListOperandRequestsByRegistry(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name})
	if err != nil {
		instance.Status.OperatorsStatus = nil
		return err
	}

	// Create an empty OperatorsStatus map
	instance.Status.OperatorsStatus = make(map[string]operatorv1alpha1.OperatorStatus)
	// Update OperandRegistry status from the OperandRequest list
	for _, item := range requestList {
		requestKey := types.NamespacedName{Name: item.Name, Namespace: item.Namespace}
		for _, req := range item.Spec.Requests {
			registryKey := item.GetRegistryKey(req)
			// Skip the status updating if the OperandRegistry doesn't match
			if registryKey.Name != instance.Name || registryKey.Namespace != instance.Namespace {
				continue
			}
			for _, operand := range req.Operands {
				instance.SetOperatorStatus(operand.Name, "", reconcile.Request{NamespacedName: requestKey})
			}
		}
	}
	return nil
}

// SetupWithManager adds OperandRegistry controller to the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	options := controller.Options{
		MaxConcurrentReconciles: r.MaxConcurrentReconciles, // Set the desired value for max concurrent reconciles.
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&operatorv1alpha1.OperandRegistry{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandRequest{}}, handler.EnqueueRequestsFromMapFunc(func(a client.Object) []reconcile.Request {
			or := a.(*operatorv1alpha1.OperandRequest)
			return or.GetAllRegistryReconcileRequest()
		}), builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*operatorv1alpha1.OperandRequest)
				newObject := e.ObjectNew.(*operatorv1alpha1.OperandRequest)
				return !reflect.DeepEqual(oldObject.Status.Members, newObject.Status.Members)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return !e.DeleteStateUnknown
			},
		})).Complete(r)
}
