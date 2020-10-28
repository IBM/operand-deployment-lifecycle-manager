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

package controllers

import (
	"context"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/controllers/common"
)

// OperandRegistryReconciler reconciles a OperandRegistry object
type OperandRegistryReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

// +kubebuilder:rbac:groups=*,resources=*,verbs=*

// Reconcile reads that state of the cluster for a OperandRegistry object and makes changes based on the state read
// and what is in the OperandRegistry.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *OperandRegistryReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {

	// Fetch the OperandRegistry instance
	instance := &operatorv1alpha1.OperandRegistry{}
	if err := r.Get(context.TODO(), req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(1).Infof("Reconciling OperandRegistry: %s", req.NamespacedName)

	// Update all the operator status
	if err := r.updateRegistryOperatorsStatus(instance); err != nil {
		klog.Errorf("failed to update the status for OperandRegistry %s/%s : %v", req.NamespacedName.Namespace, req.NamespacedName.Name, err)

		return ctrl.Result{}, err
	}

	if instance.Status.OperatorsStatus == nil || len(instance.Status.OperatorsStatus) == 0 {
		instance.UpdateRegistryPhase(operatorv1alpha1.RegistryReady)
	} else {
		instance.UpdateRegistryPhase(operatorv1alpha1.RegistryRunning)
	}
	if err := r.updateOperandRegistryStatus(instance); err != nil {
		klog.Errorf("failed to update the status for OperandRegistry %s/%s : %v", req.NamespacedName.Namespace, req.NamespacedName.Name, err)
		return ctrl.Result{}, err
	}

	klog.V(1).Infof("Finished reconciling OperandRegistry: %s", req.NamespacedName)
	return ctrl.Result{}, nil
}

func (r *OperandRegistryReconciler) updateRegistryOperatorsStatus(instance *operatorv1alpha1.OperandRegistry) error {
	// List the OperandRequests refer the OperatorRegistry by label of the OperandRequests
	requestList, err := fetch.FetchAllOperandRequests(r.Client, map[string]string{instance.Namespace + "." + instance.Name + "/registry": "true"})
	if err != nil {
		instance.Status.OperatorsStatus = nil
		return err
	}

	// Create an empty OperatorsStatus map
	instance.Status.OperatorsStatus = make(map[string]operatorv1alpha1.OperatorStatus)
	// Update OperandRegistry status from the OperandRequest list
	for _, item := range requestList.Items {
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
func (r *OperandRegistryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.OperandRegistry{}).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandRequest{}}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(
				func(a handler.MapObject) []reconcile.Request {
					or := a.Object.(*operatorv1alpha1.OperandRequest)
					return or.GetAllRegistryReconcileRequest()
				}),
		}, builder.WithPredicates(predicate.Funcs{
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

func (r *OperandRegistryReconciler) updateOperandRegistryStatus(newRegistryInstance *operatorv1alpha1.OperandRegistry) error {
	err := wait.PollImmediate(time.Millisecond*250, time.Second*5, func() (bool, error) {
		existingRegistryInstance, err := fetch.FetchOperandRegistry(r.Client, types.NamespacedName{Name: newRegistryInstance.Name, Namespace: newRegistryInstance.Namespace})
		if err != nil {
			klog.Error("failed to fetch the existing OperandRegistry: ", err)
			return false, err
		}

		existingStatus := existingRegistryInstance.Status.DeepCopy()
		newStatus := newRegistryInstance.Status.DeepCopy()
		if reflect.DeepEqual(existingStatus, newStatus) {
			return true, nil
		}
		existingRegistryInstance.Status = *newStatus
		if err := r.Status().Update(context.TODO(), existingRegistryInstance); err != nil {
			return false, err
		}
		return true, nil
	})

	if err != nil {
		klog.Error("update OperandRegistry status failed: ", err)
		return err
	}
	return nil
}
