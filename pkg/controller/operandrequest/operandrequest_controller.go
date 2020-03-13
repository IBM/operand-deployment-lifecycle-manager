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
	"strings"
	"time"

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
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

	// Watch for changes to primary resource OperandRegistry
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource OperandConfig
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.OperandConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner OperandRequest
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.Subscription{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.OperandRequest{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &olmv1.OperatorGroup{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.OperandRequest{},
	})
	if err != nil {
		return err
	}

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

// Multiple error slice
type multiErr struct {
	errors []string
}

func (mer *multiErr) Error() string {
	return "Operand reconcile error list : " + strings.Join(mer.errors, " # ")
}

func (mer *multiErr) Add(err error) {
	if mer.errors == nil {
		mer.errors = []string{}
	}
	mer.errors = append(mer.errors, err.Error())
}

// Reconcile reads that state of the cluster for a OperandRequest object and makes changes based on the state read
// and what is in the OperandRequest.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileOperandRequest) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	klog.V(1).Info("Reconciling OperandRequest: ", request)

	// Fetch the OperandRequest instance
	requestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, requestInstance); err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Set default for OperandRequest instance
	requestInstance.SetDefaultsRequest()
	err := r.client.Update(context.TODO(), requestInstance)
	if err != nil {
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
		err := r.checkFinalizer(requestInstance,request)
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

	// Fetch Subscriptions and check the status of install plan
	err = r.waitForInstallPlan(requestInstance, request)
	if err != nil {
		if err.Error() == "timed out waiting for the condition" {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, err
	}

	// Reconcile the Operand
	merr := r.reconcileOperand(requestInstance)

	if len(merr.errors) != 0 {
		return reconcile.Result{}, merr
	}

	if err := r.updateMemberStatus(requestInstance); err != nil {
		return reconcile.Result{}, err
	}

	// Check if all csv deploy successed
	if requestInstance.Status.Phase != operatorv1alpha1.ClusterPhaseRunning {
		klog.V(2).Info("Waiting for all the operands deploy successed")
		return reconcile.Result{RequeueAfter: 5}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileOperandRequest) waitForInstallPlan(requestInstance *operatorv1alpha1.OperandRequest, reconcileReq reconcile.Request) error {
	klog.V(2).Info("Waiting for subscriptions to be ready ...")

	subs := make(map[string]string)
	err := wait.PollImmediate(time.Second*20, time.Minute*10, func() (bool, error) {
		ready := true
		for _, req := range requestInstance.Spec.Requests {
			registryInstance, err := r.getRegistryInstance(req.Registry, req.RegistryNamespace)
			if err != nil {
				return false, err
			}
			for _, operand := range req.Operands {
				// Check the requested Operand if exist in specific OperandRegistry
				opt := r.getOperatorFromRegistryInstance(operand.Name, registryInstance)
				if opt != nil {
					// Check subscription if exist
					found, err := r.olmClient.OperatorsV1alpha1().Subscriptions(opt.Namespace).Get(opt.Name, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					// Subscription existing and managed by OperandRequest controller
					if _, ok := found.Labels["operator.ibm.com/opreq-control"]; ok {
						if found.Status.Install == nil {
							subs[found.ObjectMeta.Name] = "Install Plan is not ready"
							ready = false
							continue
						}
						ip, err := r.olmClient.OperatorsV1alpha1().InstallPlans(found.Namespace).Get(found.Status.InstallPlanRef.Name, metav1.GetOptions{})

						if err != nil {
							err := r.updateRegistryStatus(registryInstance, reconcileReq, found.ObjectMeta.Name, operatorv1alpha1.OperatorFailed)
							return false, err
						}

						if ip.Status.Phase != olmv1alpha1.InstallPlanPhaseComplete {
							subs[found.ObjectMeta.Name] = "Cluster Service Version is not ready"
							ready = false
							continue
						}

						err = r.updateRegistryStatus(registryInstance, reconcileReq, found.ObjectMeta.Name, operatorv1alpha1.OperatorRunning)
						if err != nil {
							return false, err
						}
						subs[found.ObjectMeta.Name] = "Ready"
					} else {
						// Subscription existing and not managed by OperandRequest controller
						klog.V(3).Info("Subscription has created by other user, ignore update/delete it. ", "Subscription.Namespace: ", found.Namespace, "Subscription.Name: ", found.Name)
					}
				}
			}
		}
		return ready, nil
	})
	for sub, state := range subs {
		klog.V(2).Info("Subscription: " + sub + ", state: " + state)
	}
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) addFinalizer(cr *operatorv1alpha1.OperandRequest) error {
	if len(cr.GetFinalizers()) < 1 && cr.GetDeletionTimestamp() == nil {
		cr.SetFinalizers([]string{"finalizer.request.ibm.com"})
		// Update CR
		err := r.client.Update(context.TODO(), cr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcileOperandRequest) checkFinalizer(requestInstance *operatorv1alpha1.OperandRequest,request reconcile.Request) error {
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
			return err
		}
		configInstance, err := r.getConfigInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return err
		}
		for _, operand := range req.Operands {
			if err := r.deleteSubscription(operand.Name, requestInstance, registryInstance, configInstance, request); err != nil {
				return err
			}
		}
	}
	return nil
}