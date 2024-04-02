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

package operandrequest

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	gset "github.com/deckarep/golang-set"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/pkg/errors"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
)

// Reconciler reconciles a OperandRequest object
type Reconciler struct {
	*deploy.ODLMOperator
	StepSize int
	Mutex    sync.Mutex
}
type clusterObjects struct {
	namespace     *corev1.Namespace
	operatorGroup *olmv1.OperatorGroup
	subscription  *olmv1alpha1.Subscription
}

//+kubebuilder:rbac:groups=operator.ibm.com,resources=certmanagers;auditloggings,verbs=get;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=catalogsources,verbs=get

//+kubebuilder:rbac:groups=*,namespace="placeholder",resources=*,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=operator.ibm.com,namespace="placeholder",resources=operandrequests;operandrequests/status;operandrequests/finalizers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",namespace="placeholder",resources=configmaps;secrets;services;namespaces,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=route.openshift.io,namespace="placeholder",resources=routes,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=operators.coreos.com,namespace="placeholder",resources=operatorgroups;installplans,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=k8s.keycloak.org,namespace="placeholder",resources=keycloaks;keycloakrealmimports,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=packages.operators.coreos.com,namespace="placeholder",resources=packagemanifests,verbs=get;list;patch;update;watch
//+kubebuilder:rbac:groups=operators.coreos.com,namespace="placeholder",resources=clusterserviceversions;subscriptions,verbs=create;delete;get;list;patch;update;watch

// Reconcile reads that state of the cluster for a OperandRequest object and makes changes based on the state read
// and what is in the OperandRequest.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
	// Fetch the OperandRequest instance
	requestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, requestInstance); err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	originalInstance := requestInstance.DeepCopy()

	// Always attempt to patch the status after each reconciliation.
	defer func() {
		// get the latest instance from the server and check if the status has changed
		existingInstance := &operatorv1alpha1.OperandRequest{}
		if err := r.Client.Get(ctx, req.NamespacedName, existingInstance); err != nil && !apierrors.IsNotFound(err) {
			// Error reading the latest object - requeue the request.
			reconcileErr = utilerrors.NewAggregate([]error{reconcileErr, fmt.Errorf("error while get latest OperandRequest.Status from server: %v", err)})
		}

		if reflect.DeepEqual(existingInstance.Status, requestInstance.Status) {
			return
		}
		if err := r.Client.Status().Patch(ctx, requestInstance, client.MergeFrom(existingInstance)); err != nil && !apierrors.IsNotFound(err) {
			reconcileErr = utilerrors.NewAggregate([]error{reconcileErr, fmt.Errorf("error while patching OperandRequest.Status: %v", err)})
		}
	}()

	// Remove finalizer when DeletionTimestamp none zero
	if !requestInstance.ObjectMeta.DeletionTimestamp.IsZero() {

		// Check and clean up the subscriptions
		err := r.checkFinalizer(ctx, requestInstance)
		if err != nil {
			klog.Errorf("failed to clean up the subscriptions for OperandRequest %s: %v", req.NamespacedName.String(), err)
			return ctrl.Result{}, err
		}

		originalReq := requestInstance.DeepCopy()
		// Update finalizer to allow delete CR
		if requestInstance.RemoveFinalizer() {
			err = r.Patch(ctx, requestInstance, client.MergeFrom(originalReq))
			if err != nil {
				klog.Errorf("failed to remove finalizer for OperandRequest %s: %v", req.NamespacedName.String(), err)
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if operator has the update permission to update OperandRequest
	hasPermission := r.checkPermission(ctx, req)
	if !hasPermission {
		klog.Warningf("No permission to update OperandRequest")
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	klog.V(1).Infof("Reconciling OperandRequest: %s", req.NamespacedName)
	// Update labels for the request
	if requestInstance.UpdateLabels() {
		if err := r.Patch(ctx, requestInstance, client.MergeFrom(originalInstance)); err != nil {
			klog.Errorf("failed to update the labels for OperandRequest %s: %v", req.NamespacedName.String(), err)
			return ctrl.Result{}, err
		}
	}

	// Initialize the status for OperandRequest instance
	if !requestInstance.InitRequestStatus() {
		return ctrl.Result{Requeue: true}, nil
	}

	// Add finalizer to the instance
	if isAdded, err := r.addFinalizer(ctx, requestInstance); err != nil {
		klog.Errorf("failed to add finalizer for OperandRequest %s: %v", req.NamespacedName.String(), err)
		return ctrl.Result{}, err
	} else if !isAdded {
		return ctrl.Result{Requeue: true}, err
	}

	// Reconcile Operators
	if err := r.reconcileOperator(ctx, requestInstance); err != nil {
		klog.Errorf("failed to reconcile Operators for OperandRequest %s: %v", req.NamespacedName.String(), err)
		return ctrl.Result{}, err
	}

	// Reconcile Operands
	if merr := r.reconcileOperand(ctx, requestInstance); len(merr.Errors) != 0 {
		klog.Errorf("failed to reconcile Operands for OperandRequest %s: %v", req.NamespacedName.String(), merr)
		return ctrl.Result{}, merr
	}

	// Check if all csv deploy succeed
	if requestInstance.Status.Phase != operatorv1alpha1.ClusterPhaseRunning {
		klog.V(2).Info("Waiting for all operators and operands to be deployed successfully ...")
		return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
	}

	//check if status.services is present (if a relevant service was requested), requeue again is im/iam is not ready yet
	if requestInstance.CheckServiceStatus() {
		return ctrl.Result{RequeueAfter: constant.DefaultRequeueDuration}, nil
	}

	klog.V(1).Infof("Finished reconciling OperandRequest: %s", req.NamespacedName)
	return ctrl.Result{RequeueAfter: constant.DefaultSyncPeriod}, nil
}

func (r *Reconciler) checkPermission(ctx context.Context, req ctrl.Request) bool {
	// Check update permission
	if !r.checkUpdateAuth(ctx, req.Namespace, "operator.ibm.com", "operandrequests") {
		return false
	}
	if !r.checkUpdateAuth(ctx, req.Namespace, "operator.ibm.com", "operandrequests/status") {
		return false
	}
	return true
}

// Check if operator has permission to update OperandRequest
func (r *Reconciler) checkUpdateAuth(ctx context.Context, namespace, group, resource string) bool {
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      "update",
				Group:     group,
				Resource:  resource,
			},
		},
	}

	if err := r.Create(ctx, sar); err != nil {
		klog.Errorf("Failed to check operator update permission: %v", err)
		return false
	}

	klog.V(3).Infof("Operator update permission in namespace %s, Allowed: %t, Denied: %t, Reason: %s", namespace, sar.Status.Allowed, sar.Status.Denied, sar.Status.Reason)
	return sar.Status.Allowed
}

func (r *Reconciler) addFinalizer(ctx context.Context, cr *operatorv1alpha1.OperandRequest) (bool, error) {
	if cr.GetDeletionTimestamp() == nil {
		originalReq := cr.DeepCopy()
		added := cr.EnsureFinalizer()
		if added {
			// Add finalizer to OperandRequest instance
			err := r.Patch(ctx, cr, client.MergeFrom(originalReq))
			if err != nil {
				return false, errors.Wrapf(err, "failed to update the OperandRequest %s/%s", cr.Namespace, cr.Name)
			}
			return false, nil
		}
	}
	return true, nil
}

func (r *Reconciler) checkFinalizer(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(1).Infof("Deleting OperandRequest %s in the namespace %s", requestInstance.Name, requestInstance.Namespace)
	remainingOperands := gset.NewSet()
	for _, m := range requestInstance.Status.Members {
		remainingOperands.Add(m.Name)
	}
	existingSub := &olmv1alpha1.SubscriptionList{}

	opts := []client.ListOption{
		client.MatchingLabels(map[string]string{constant.OpreqLabel: "true"}),
	}

	if err := r.Client.List(ctx, existingSub, opts...); err != nil {
		return err
	}
	if len(existingSub.Items) == 0 {
		return nil
	}
	// Delete all the subscriptions that created by current request
	if err := r.absentOperatorsAndOperands(ctx, requestInstance, &remainingOperands); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) getRegistryToRequestMapper() handler.MapFunc {
	ctx := context.Background()
	return func(object client.Object) []ctrl.Request {
		requestList, _ := r.ListOperandRequestsByRegistry(ctx, types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()})

		requests := []ctrl.Request{}
		for _, request := range requestList {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := ctrl.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}

func (r *Reconciler) getSubToRequestMapper() handler.MapFunc {
	return func(object client.Object) []ctrl.Request {
		reg, _ := regexp.Compile(`^(.*)\.(.*)\.(.*)\/request`)
		annotations := object.GetAnnotations()
		var reqName, reqNamespace string
		for annotation := range annotations {
			if reg.MatchString(annotation) {
				annotationSlices := strings.Split(annotation, ".")
				reqNamespace = annotationSlices[0]
				reqName = annotationSlices[1]
			}
		}
		if reqNamespace == "" || reqName == "" {
			return []ctrl.Request{}
		}
		return []ctrl.Request{
			{NamespacedName: types.NamespacedName{
				Name:      reqName,
				Namespace: reqNamespace,
			}},
		}
	}
}

func (r *Reconciler) getConfigToRequestMapper() handler.MapFunc {
	ctx := context.Background()
	return func(object client.Object) []ctrl.Request {
		requestList, _ := r.ListOperandRequestsByConfig(ctx, types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()})

		requests := []ctrl.Request{}
		for _, request := range requestList {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := ctrl.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}

func (r *Reconciler) getReferenceToRequestMapper() handler.MapFunc {
	ctx := context.Background()
	return func(object client.Object) []ctrl.Request {
		labels := object.GetLabels()
		if labels == nil {
			return []ctrl.Request{}
		}
		odlmReference, ok := labels[constant.ODLMReferenceLabel]
		if !ok {
			return []ctrl.Request{}
		}
		odlmReferenceSlices := strings.Split(odlmReference, ".")
		if len(odlmReferenceSlices) != 3 {
			return []ctrl.Request{}
		}

		var requestList []operatorv1alpha1.OperandRequest
		if odlmReferenceSlices[0] == "OperandConfig" {
			requestList, _ = r.ListOperandRequestsByConfig(ctx, types.NamespacedName{Namespace: odlmReferenceSlices[1], Name: odlmReferenceSlices[2]})
		} else if odlmReferenceSlices[0] == "OperandRegistry" {
			requestList, _ = r.ListOperandRequestsByRegistry(ctx, types.NamespacedName{Namespace: odlmReferenceSlices[1], Name: odlmReferenceSlices[2]})
		} else {
			return []ctrl.Request{}
		}

		requests := []ctrl.Request{}
		for _, request := range requestList {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := ctrl.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}

// SetupWithManager adds OperandRequest controller to the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	options := controller.Options{
		MaxConcurrentReconciles: r.MaxConcurrentReconciles, // Set the desired value for max concurrent reconciles.
	}
	ReferencePredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			labels := e.Object.GetLabels()
			for labelKey, labelValue := range labels {
				if labelKey == constant.ODLMWatchedLabel {
					return labelValue == "true"
				}
			}
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			labels := e.ObjectNew.GetLabels()
			for labelKey, labelValue := range labels {
				if labelKey == constant.ODLMWatchedLabel {
					return labelValue == "true"
				}
			}
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return true
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&operatorv1alpha1.OperandRequest{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &olmv1alpha1.Subscription{}}, handler.EnqueueRequestsFromMapFunc(r.getSubToRequestMapper()), builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*olmv1alpha1.Subscription)
				newObject := e.ObjectNew.(*olmv1alpha1.Subscription)
				if oldObject.Labels != nil && oldObject.Labels[constant.OpreqLabel] == "true" {
					return (oldObject.Status.InstalledCSV != "" && newObject.Status.InstalledCSV != "" && oldObject.Status.InstalledCSV != newObject.Status.InstalledCSV)
				}
				return false
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return false
			},
		})).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, handler.EnqueueRequestsFromMapFunc(r.getRegistryToRequestMapper()), builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*operatorv1alpha1.OperandRegistry)
				newObject := e.ObjectNew.(*operatorv1alpha1.OperandRegistry)
				return !reflect.DeepEqual(oldObject.Spec, newObject.Spec)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return !e.DeleteStateUnknown
			},
		})).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandConfig{}}, handler.EnqueueRequestsFromMapFunc(r.getConfigToRequestMapper()), builder.WithPredicates(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return !e.DeleteStateUnknown
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*operatorv1alpha1.OperandConfig)
				newObject := e.ObjectNew.(*operatorv1alpha1.OperandConfig)
				return !reflect.DeepEqual(oldObject.Spec, newObject.Spec)
			},
		})).
		Watches(&source.Kind{Type: &corev1.ConfigMap{}}, handler.EnqueueRequestsFromMapFunc(r.getReferenceToRequestMapper()), builder.WithPredicates(ReferencePredicates)).
		Watches(&source.Kind{Type: &corev1.Secret{}}, handler.EnqueueRequestsFromMapFunc(r.getReferenceToRequestMapper()), builder.WithPredicates(ReferencePredicates)).
		Complete(r)
}
