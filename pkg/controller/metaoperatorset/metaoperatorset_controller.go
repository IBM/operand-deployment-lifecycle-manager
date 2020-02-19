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

package metaoperatorset

import (
	"context"
	"strings"
	"time"

	olmv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1"
	olmv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	olmclient "github.com/operator-framework/operator-lifecycle-manager/pkg/api/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/meta-operator/pkg/apis/operator/v1alpha1"
)

var log = logf.Log.WithName("controller_metaoperatorset")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MetaOperatorSet Controller and adds it to the Manager. The Manager will set fields on the Controller
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
	return &ReconcileMetaOperatorSet{
		client:    mgr.GetClient(),
		recorder:  mgr.GetEventRecorderFor("serviceset"),
		scheme:    mgr.GetScheme(),
		olmClient: olmClientset}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("serviceset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MetaOperatorSet
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.MetaOperatorSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MetaOperator
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.MetaOperatorCatalog{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MetaOperatorConfig
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.MetaOperatorConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner MetaOperatorSet
	err = c.Watch(&source.Kind{Type: &olmv1alpha1.Subscription{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.MetaOperatorSet{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &olmv1.OperatorGroup{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.MetaOperatorSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMetaOperatorSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMetaOperatorSet{}

// ReconcileMetaOperatorSet reconciles a MetaOperatorSet object
type ReconcileMetaOperatorSet struct {
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

// Reconcile reads that state of the cluster for a MetaOperatorSet object and makes changes based on the state read
// and what is in the MetaOperatorSet.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMetaOperatorSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MetaOperatorSet")

	// Fetch the MetaOperatorCatalog instance
	moc := &operatorv1alpha1.MetaOperatorCatalog{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: request.Namespace, Name: "common-service"}, moc); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Initialize MetaOperatorCatalog status
	if err := r.initOperatorStatus(moc); err != nil {
		return reconcile.Result{}, err
	}

	// Fetch the MetaOperatorConfig instance
	csc := &operatorv1alpha1.MetaOperatorConfig{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: request.Namespace, Name: "common-service"}, csc); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Initialize MetaOperatorConfig status
	if err := r.initServiceStatus(csc); err != nil {
		return reconcile.Result{}, err
	}

	// Fetch the MetaOperatorSet instance
	setInstance := &operatorv1alpha1.MetaOperatorSet{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, setInstance); err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Add finalizer
	if setInstance.GetFinalizers() == nil {
		if err := r.addFinalizer(setInstance); err != nil {
			return reconcile.Result{}, err
		}
	}

	// Fetch all subscription definition
	opts, err := r.fetchOperators(moc, setInstance)
	if opts == nil {
		if err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	err = r.reconcileMetaOperator(opts, setInstance, moc)

	if err != nil {
		return reconcile.Result{}, err
	}

	// Fetch MetaOperatorConfig instance
	serviceConfigs, err := r.fetchConfigs(request, setInstance)
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

	// Reconcile the MetaOperatorConfig
	merr := r.reconcileMetaOperatorConfig(serviceConfigs, csc)

	if len(merr.errors) != 0 {
		return reconcile.Result{}, merr
	}

	// Check unready number of subscription
	if err := r.updateMemberStatus(setInstance); err != nil {
		return reconcile.Result{}, err
	}
	if setInstance.Status.Members.Unready != nil {
		if err := r.updatePhaseStatus(setInstance, operatorv1alpha1.ClusterPhaseCreating); err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("Waiting for all the operators ready ......")
		return reconcile.Result{Requeue: true}, nil
	}
	if err := r.updatePhaseStatus(setInstance, operatorv1alpha1.ClusterPhaseRunning); err != nil {
		return reconcile.Result{}, err
	}

	// Remove finalizer
	if !setInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Update finalizer to allow delete CR
		setInstance.SetFinalizers(nil)
		err := r.client.Update(context.TODO(), setInstance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileMetaOperatorSet) waitForInstallPlan(moc *operatorv1alpha1.MetaOperatorCatalog) error {
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

func (r *ReconcileMetaOperatorSet) fetchSets(currentCr *operatorv1alpha1.MetaOperatorSet) (map[string]operatorv1alpha1.SetService, error) {
	crs := &operatorv1alpha1.MetaOperatorSetList{}
	if err := r.client.List(context.TODO(), crs); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	// If current CR is deleted, mark all the operators as absent state
	isCurrentCrToBeDeleted := currentCr.GetDeletionTimestamp() != nil
	sets := make(map[string]operatorv1alpha1.SetService)
	for _, cr := range crs.Items {
		for _, s := range cr.Spec.Services {
			if isCurrentCrToBeDeleted && cr.GetUID() == currentCr.GetUID() {
				s.State = Absent
			}
			sets = addSet(sets, s)
		}
	}
	return sets, nil
}

func addSet(sets map[string]operatorv1alpha1.SetService, set operatorv1alpha1.SetService) map[string]operatorv1alpha1.SetService {
	if _, ok := sets[set.Name]; ok {
		if set.State == Present {
			sets[set.Name] = set
		}
		return sets
	}
	sets[set.Name] = set
	return sets
}

func (r *ReconcileMetaOperatorSet) addFinalizer(cr *operatorv1alpha1.MetaOperatorSet) error {
	if len(cr.GetFinalizers()) < 1 && cr.GetDeletionTimestamp() == nil {
		cr.SetFinalizers([]string{"finalizer.set.ibm.com"})
		// Update CR
		err := r.client.Update(context.TODO(), cr)
		if err != nil {
			return err
		}
	}
	return nil
}
