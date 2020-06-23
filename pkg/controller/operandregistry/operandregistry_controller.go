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
	"time"

	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/pkg/controller/common"
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
	// Add extenal api scheme
	if err := olmv1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Error("Add olm api scheme failed: ", err)
		return nil
	}
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
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, &handler.EnqueueRequestForObject{}, predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}
	// Watch for changes to resource OperandRequest
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRequest{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(
			func(a handler.MapObject) []reconcile.Request {
				or := a.Object.(*operatorv1alpha1.OperandRequest)
				return or.GetAllRegistryReconcileRequest()
			}),
	}, predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObject := e.ObjectOld.(*operatorv1alpha1.OperandRequest)
			newObject := e.ObjectNew.(*operatorv1alpha1.OperandRequest)
			return !reflect.DeepEqual(oldObject.Status.Members, newObject.Status.Members)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	})
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

	// Set Finalizer for the OperandRegistry
	if instance.EnsureFinalizer() {
		if err := r.client.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	if instance.DeletionTimestamp != nil {
		requestList, err := fetch.FetchAllOperandRequests(r.client, map[string]string{instance.Namespace + "." + instance.Name + "/registry": "true"})
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

	// Check if all the catalog sources ready for deployment
	isReady, err := r.checkCatalogSourceStatus(instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	if isReady {
		if err := r.updateRegistryOperatorsStatus(instance); err != nil {
			return reconcile.Result{}, err
		}
		if instance.Status.OperatorsStatus == nil || len(instance.Status.OperatorsStatus) == 0 {
			instance.UpdateRegistryPhase(operatorv1alpha1.RegistryReady)
		} else {
			instance.UpdateRegistryPhase(operatorv1alpha1.RegistryRunning)
		}
	} else {
		instance.UpdateRegistryPhase(operatorv1alpha1.RegistryFailed)
	}

	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileOperandRegistry) updateRegistryOperatorsStatus(instance *operatorv1alpha1.OperandRegistry) error {
	// List the OperandRequests refer the OperatorRegistry by label of the OperandRequests
	requestList, err := fetch.FetchAllOperandRequests(r.client, map[string]string{instance.Namespace + "." + instance.Name + "/registry": "true"})
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

func (r *ReconcileOperandRegistry) checkCatalogSourceStatus(instance *operatorv1alpha1.OperandRegistry) (bool, error) {
	isReady := true
	for _, o := range instance.Spec.Operators {
		catsrcName := o.SourceName
		catsrcNamespace := o.SourceNamespace
		catsrcKey := types.NamespacedName{Namespace: catsrcNamespace, Name: catsrcName}
		catsrc := &olmv1alpha1.CatalogSource{}
		if err := r.client.Get(context.TODO(), catsrcKey, catsrc); err != nil {
			isReady = false
			if !errors.IsNotFound(err) {
				return isReady, err
			}
			r.recorder.Eventf(instance, corev1.EventTypeWarning, "NotFound", "NotFound CatalogSource in NamespacedName %s/%s", catsrcNamespace, catsrcName)
			instance.SetNotFoundCondition(catsrcName, operatorv1alpha1.ResourceTypeCatalogSource, corev1.ConditionTrue)
			continue
		}

		if catsrc.Status.GRPCConnectionState.LastObservedState == "READY" {
			instance.SetReadyCondition(catsrcName, operatorv1alpha1.ResourceTypeCatalogSource, corev1.ConditionTrue)
		} else {
			isReady = false
			r.recorder.Eventf(instance, corev1.EventTypeWarning, "NotReady", "NotReady CatalogSource in NamespacedName %s/%s", catsrcNamespace, catsrcName)
			instance.SetReadyCondition(catsrcName, operatorv1alpha1.ResourceTypeCatalogSource, corev1.ConditionFalse)
		}
	}
	return isReady, nil
}
