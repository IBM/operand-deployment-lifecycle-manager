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
	"fmt"
	"strings"
	"time"

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

var log = logf.Log.WithName("controller_operandrequest")

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
		log.Error(err, "Initialize the OLM client failed.")
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
	return "Commn services installation errors : " + strings.Join(mer.errors, ":")
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
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling OperandRequest")

	// Fetch the OperandRegistry instance
	moc, err := r.listRegistry(request.Namespace)

	if moc == nil {
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	// Initialize OperandRegistry status
	if err := r.initOperatorStatus(moc); err != nil {
		return reconcile.Result{}, err
	}

	// Fetch the OperandConfig instance
	csc, err := r.listConfig(request.Namespace)

	if csc == nil {
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	// Initialize OperandConfig status
	if err := r.initServiceStatus(csc); err != nil {
		return reconcile.Result{}, err
	}

	// Fetch the OperandRequest instance
	operandRequestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, operandRequestInstance); err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer
	if operandRequestInstance.GetFinalizers() == nil {
		if err := r.addFinalizer(operandRequestInstance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Fetch all subscription definition
	opts, err := r.fetchOperators(moc, operandRequestInstance)
	if opts == nil {
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	if err = r.reconcileOperator(opts, operandRequestInstance, moc); err != nil {
		return reconcile.Result{}, err
	}

	// Fetch OperandConfig instance
	serviceConfigs, err := r.fetchConfigs(request, operandRequestInstance)
	if serviceConfigs == nil {
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// Fetch Subscriptions and check the status of install plan
	err = r.waitForInstallPlan(moc)
	if err != nil {
		if err.Error() == "timed out waiting for the condition" {
			return reconcile.Result{Requeue: true}, nil
		}
		return reconcile.Result{}, err
	}

	// Reconcile the Operand
	merr := r.reconcileOperand(serviceConfigs, csc)

	if len(merr.errors) != 0 {
		return reconcile.Result{}, merr
	}

	if err := r.updateMemberStatus(operandRequestInstance); err != nil {
		return reconcile.Result{}, err
	}

	// Remove finalizer
	if !operandRequestInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Update finalizer to allow delete CR
		operandRequestInstance.SetFinalizers(nil)
		err := r.client.Update(context.TODO(), operandRequestInstance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Check if all csv deploy successed
	if operandRequestInstance.Status.Phase != operatorv1alpha1.ClusterPhaseRunning {
		reqLogger.Info("Waiting for all the operands deploy successed")
		return reconcile.Result{RequeueAfter: 5}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileOperandRequest) waitForInstallPlan(moc *operatorv1alpha1.OperandRegistry) error {
	reqLogger := log.WithValues()
	reqLogger.Info("Waiting for subscriptions to be ready ...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()
	subs := make(map[string]string)
	err := wait.PollImmediateUntil(time.Second*20, func() (bool, error) {
		foundSub, err := r.olmClient.OperatorsV1alpha1().Subscriptions("").List(metav1.ListOptions{
			LabelSelector: "operator.ibm.com/mos-control",
		})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		ready := true
		for _, sub := range foundSub.Items {
			if sub.Status.Install == nil {
				subs[sub.ObjectMeta.Name] = "Install Plan is not ready"
				ready = false
				continue
			}

			ip, err := r.olmClient.OperatorsV1alpha1().InstallPlans(sub.Namespace).Get(sub.Status.InstallPlanRef.Name, metav1.GetOptions{})

			if err != nil {
				err := r.updateOperatorStatus(moc, sub.ObjectMeta.Name, operatorv1alpha1.OperatorFailed)
				return false, err
			}

			if ip.Status.Phase != olmv1alpha1.InstallPlanPhaseComplete {
				subs[sub.ObjectMeta.Name] = "Cluster Service Version is not ready"
				ready = false
				continue
			}

			err = r.updateOperatorStatus(moc, sub.ObjectMeta.Name, operatorv1alpha1.OperatorRunning)
			if err != nil {
				return false, err
			}
			subs[sub.ObjectMeta.Name] = "Ready"
		}

		return ready, nil
	}, ctx.Done())
	if err != nil {
		for sub, state := range subs {
			reqLogger.Info("Subscription " + sub + " state: " + state)
		}
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) fetchRequests(currentCr *operatorv1alpha1.OperandRequest) (map[string]operatorv1alpha1.RequestService, error) {
	crs := &operatorv1alpha1.OperandRequestList{}
	if err := r.client.List(context.TODO(), crs); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	// If current CR is deleted, mark all the operators as absent state
	isCurrentCrToBeDeleted := currentCr.GetDeletionTimestamp() != nil
	requests := make(map[string]operatorv1alpha1.RequestService)
	for _, cr := range crs.Items {
		for _, s := range cr.Spec.Services {
			if isCurrentCrToBeDeleted && cr.GetUID() == currentCr.GetUID() {
				s.State = Absent
			}
			requests = addRequest(requests, s)
		}
	}
	return requests, nil
}

func addRequest(requests map[string]operatorv1alpha1.RequestService, request operatorv1alpha1.RequestService) map[string]operatorv1alpha1.RequestService {
	if _, ok := requests[request.Name]; ok {
		if request.State == Present {
			requests[request.Name] = request
		}
		return requests
	}
	requests[request.Name] = request
	return requests
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

func (r *ReconcileOperandRequest) listConfig(namespace string) (*operatorv1alpha1.OperandConfig, error) {
	reqLogger := log.WithValues("Request.Namespace", namespace)
	reqLogger.Info("Fetch OperandConfig instance")

	// Fetch the OperandConfig instance
	cscList := &operatorv1alpha1.OperandConfigList{}
	if err := r.client.List(context.TODO(), cscList, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}

	if len(cscList.Items) == 0 {
		return nil, nil
	}

	if len(cscList.Items) > 1 {
		reqLogger.Error(fmt.Errorf("multiple OperandConfig in one namespace"),
			"There are multiple OperandConfig custom resource in "+
				namespace+
				". Choose the first one "+
				cscList.Items[0].Name+
				". You need to leave one and delete the others")
	}
	return &cscList.Items[0], nil
}

func (r *ReconcileOperandRequest) listRegistry(namespace string) (*operatorv1alpha1.OperandRegistry, error) {
	reqLogger := log.WithValues("Request.Namespace", namespace)
	reqLogger.Info("Fetch OperandRegistry instance")
	// Fetch the OperandRegistry instance
	mocList := &operatorv1alpha1.OperandRegistryList{}
	if err := r.client.List(context.TODO(), mocList, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}

	if len(mocList.Items) == 0 {
		return nil, nil
	}

	if len(mocList.Items) > 1 {
		reqLogger.Error(fmt.Errorf("multiple OperandRegistry in one namespace"),
			"There are multiple OperandRegistry custom resource in "+
				namespace+
				". Choose the first one "+
				mocList.Items[0].Name+
				". You need to leave one and delete the others")
	}
	return &mocList.Items[0], nil
}
