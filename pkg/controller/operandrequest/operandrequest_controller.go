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

package operandrequest

import (
	"context"
	"reflect"
	"time"

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
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

// Add creates a new OperandRequest Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	olmClientset, err := olmclient.NewForConfig(mgr.GetConfig())
	if err != nil {
		klog.Error("Initialize the OLM client failed: ", err)
		return nil
	}
	return &ReconcileOperandRequest{
		client:    mgr.GetClient(),
		recorder:  mgr.GetEventRecorderFor("OperandRequest"),
		scheme:    mgr.GetScheme(),
		olmClient: olmClientset}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("OperandRequest-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource OperandRequest
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRequest{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to resource OperandRegistry
	if err := c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(
			func(a handler.MapObject) []reconcile.Request {
				or := a.Object.(*operatorv1alpha1.OperandRegistry)
				return or.GetAllReconcileRequest()
			}),
	}, predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObject := e.ObjectOld.(*operatorv1alpha1.OperandRegistry)
			newObject := e.ObjectNew.(*operatorv1alpha1.OperandRegistry)
			return !reflect.DeepEqual(oldObject.Spec, newObject.Spec)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	}); err != nil {
		return err
	}

	// Watch for OperandConfig spec changes and requeue the OperandRequest
	if err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandConfig{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: getConfigToRquestMapper(mgr),
	}, predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObject := e.ObjectOld.(*operatorv1alpha1.OperandConfig)
			newObject := e.ObjectNew.(*operatorv1alpha1.OperandConfig)
			return !reflect.DeepEqual(oldObject.Spec, newObject.Spec)
		},
	}); err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner OperandRequest
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.Subscription{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.OperandRequest{},
	})

	return nil
}

// blank assignment to verify that ReconcileOperandRequest implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileOperandRequest{}

// ReconcileOperandRequest reconciles a OperandRequest object
type ReconcileOperandRequest struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client    client.Client
	recorder  record.EventRecorder
	scheme    *runtime.Scheme
	olmClient olmclient.Interface
}

type clusterObjects struct {
	namespace     *corev1.Namespace
	operatorGroup *olmv1.OperatorGroup
	subscription  *olmv1alpha1.Subscription
}

// Reconcile reads that state of the cluster for a OperandRequest object and makes changes based on the state read
// and what is in the OperandRequest.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileOperandRequest) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// Fetch the OperandRequest instance
	requestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, requestInstance); err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	klog.V(1).Infof("Reconciling OperandRequest %s in the namespace %s", requestInstance.Name, requestInstance.Namespace)

	// Set default for OperandRequest instance
	requestInstance.SetDefaultsRequestSpec()
	// Add labels for the request
	requestInstance.AddLabels()
	if err := r.client.Update(context.TODO(), requestInstance); err != nil {
		return reconcile.Result{}, err
	}
	// Set the default status for OperandRequest instance
	requestInstance.SetDefaultRequestStatus()
	if err := r.client.Status().Update(context.TODO(), requestInstance); err != nil {
		return reconcile.Result{}, err
	}

	// Add finalizer
	if requestInstance.GetFinalizers() == nil {
		if err := r.addFinalizer(requestInstance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Remove finalizer when DeletionTimestamp none zero
	if !requestInstance.ObjectMeta.DeletionTimestamp.IsZero() {

		// Check and clean up the subscriptions
		err := r.checkFinalizer(requestInstance, request)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Update finalizer to allow delete CR
		requestInstance.SetFinalizers(nil)
		err = r.client.Update(context.TODO(), requestInstance)
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	if err := r.reconcileOperator(requestInstance, request); err != nil {
		return reconcile.Result{}, err
	}

	// Update request status after subscription ready
	if err := r.updateMemberStatus(requestInstance); err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile the Operand
	merr := r.reconcileOperand(requestInstance, request)

	if len(merr.Errors) != 0 {
		return reconcile.Result{}, merr
	}

	// Update request status after subscription ready
	if err := r.updateMemberStatus(requestInstance); err != nil {
		return reconcile.Result{}, err
	}

	// Check if all csv deploy successed
	if requestInstance.Status.Phase != operatorv1alpha1.ClusterPhaseRunning {
		klog.V(2).Info("Waiting for all operands to be deployed successfully ...")
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileOperandRequest) addFinalizer(cr *operatorv1alpha1.OperandRequest) error {
	klog.V(2).Infof("Add finializer to the OperandRequest %s in the namespace %s", cr.Name, cr.Namespace)
	if len(cr.GetFinalizers()) < 1 && cr.GetDeletionTimestamp() == nil {
		cr.SetFinalizers([]string{"finalizer.request.ibm.com"})
		// Update CR
		err := r.client.Update(context.TODO(), cr)
		if err != nil {
			klog.Errorf("Failed to update the OperandRequest %s in the namespace %s: %s", cr.Name, cr.Namespace, err)
			return err
		}
	}
	return nil
}

func (r *ReconcileOperandRequest) checkFinalizer(requestInstance *operatorv1alpha1.OperandRequest, request reconcile.Request) error {
	klog.V(2).Infof("Deleting OperandRequest %s in the namespace %s", requestInstance.Name, requestInstance.Namespace)
	existingSub, err := r.olmClient.OperatorsV1alpha1().Subscriptions(metav1.NamespaceAll).List(metav1.ListOptions{
		LabelSelector: "operator.ibm.com/opreq-control",
	})
	if err != nil {
		return err
	}
	if len(existingSub.Items) == 0 {
		return nil
	}
	// Delete all the subscriptions that created by current request
	for _, req := range requestInstance.Spec.Requests {
		registryInstance, err := r.getRegistryInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			klog.Error("Failed to get OperandRegistry: ", err)
			return err
		}
		configInstance, err := r.getConfigInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			klog.Error("Failed to get OperandConfig: ", err)
			return err
		}
		for _, operand := range req.Operands {
			if err := r.deleteSubscription(operand.Name, requestInstance, registryInstance, configInstance, request); err != nil {
				klog.Error("Failed to delete subscriptions during the uninstall: ", err)
				klog.Error(err)
				return err
			}
		}
	}
	return nil
}

func getRegistryToRquestMapper(mgr manager.Manager) handler.ToRequestsFunc {
	return func(object handler.MapObject) []reconcile.Request {
		mgrClient := mgr.GetClient()
		requestList := &operatorv1alpha1.OperandRequestList{}
		opts := []client.ListOption{
			client.MatchingLabels(map[string]string{object.Meta.GetNamespace() + "." + object.Meta.GetName() + "/registry": "true"}),
		}

		_ = mgrClient.List(context.TODO(), requestList, opts...)

		requests := []reconcile.Request{}
		for _, request := range requestList.Items {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := reconcile.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}

func getConfigToRquestMapper(mgr manager.Manager) handler.ToRequestsFunc {
	return func(object handler.MapObject) []reconcile.Request {
		mgrClient := mgr.GetClient()
		requestList := &operatorv1alpha1.OperandRequestList{}
		opts := []client.ListOption{
			client.MatchingLabels(map[string]string{object.Meta.GetNamespace() + "." + object.Meta.GetName() + "/config": "true"}),
		}

		_ = mgrClient.List(context.TODO(), requestList, opts...)

		requests := []reconcile.Request{}
		for _, request := range requestList.Items {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := reconcile.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}
