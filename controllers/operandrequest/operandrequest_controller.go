//
// Copyright 2021 IBM Corporation
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
	"k8s.io/client-go/discovery"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	nssv1 "github.com/IBM/ibm-namespace-scope-operator/api/v1"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
	util "github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

// Reconciler reconciles a OperandRequest object
type Reconciler struct {
	*deploy.ODLMOperator
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
func (r *Reconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reconcileErr error) {

	// Creat context for the OperandBindInfo reconciler
	ctx := context.Background()

	// Fetch the OperandRequest instance
	requestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, requestInstance); err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	originalInstance := requestInstance.DeepCopy()

	// Always attempt to patch the status after each reconciliation.
	defer func() {
		if reflect.DeepEqual(originalInstance.Status, requestInstance.Status) {
			return
		}
		if err := r.Client.Status().Patch(ctx, requestInstance, client.MergeFrom(originalInstance)); err != nil {
			reconcileErr = utilerrors.NewAggregate([]error{reconcileErr, fmt.Errorf("error while patching OperandRequest.Status: %v", err)})
		}
	}()

	// Add namespace member into NamespaceScope and check if has the update permission
	hasPermission, err := r.addPermission(ctx, req)
	if err != nil {
		klog.Errorf("failed to add namespace member into NamespaceScope: %v", err)
		return ctrl.Result{}, err
	}
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

	// Remove finalizer when DeletionTimestamp none zero
	if !requestInstance.ObjectMeta.DeletionTimestamp.IsZero() {

		// Check and clean up the subscriptions
		err := r.checkFinalizer(ctx, requestInstance)
		if err != nil {
			klog.Errorf("failed to clean up the subscriptions for OperandRequest %s: %v", req.NamespacedName.String(), err)
			return ctrl.Result{}, err
		}

		// Check and remove namespaceMember from NamespaceScope CR
		if err := r.RemoveNamespaceMemberFromNamespaceScope(ctx, req.NamespacedName); err != nil {
			klog.Errorf("failed to remove NamespaceMember %s from NamespaceScope: %v", req.Namespace, err)
			return ctrl.Result{}, err
		}

		// Update finalizer to allow delete CR
		if requestInstance.RemoveFinalizer() {
			err = r.Update(ctx, requestInstance)
			if err != nil {
				klog.Errorf("failed to remove finalizer for OperandRequest %s: %v", req.NamespacedName.String(), err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
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

	klog.V(1).Infof("Finished reconciling OperandRequest: %s", req.NamespacedName)
	return ctrl.Result{RequeueAfter: constant.DefaultSyncPeriod}, nil
}

func (r *Reconciler) addPermission(ctx context.Context, req ctrl.Request) (bool, error) {
	if err := r.AddNamespaceMemberIntoNamespaceScope(ctx, req.NamespacedName); err != nil {
		return false, errors.Wrapf(err, "failed to add Namespace %s to NamespaceScope", req.Namespace)
	}
	// Check update permission
	if !r.checkUpdateAuth(ctx, req.Namespace, "operator.ibm.com", "operandrequests") {
		return false, nil
	}
	if !r.checkUpdateAuth(ctx, req.Namespace, "operator.ibm.com", "operandrequests/status") {
		return false, nil
	}
	return true, nil
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

func (r *Reconciler) AddNamespaceMemberIntoNamespaceScope(ctx context.Context, namespacedName types.NamespacedName) error {
	return r.UpdateNamespaceScope(ctx, namespacedName, false)
}

func (r *Reconciler) RemoveNamespaceMemberFromNamespaceScope(ctx context.Context, namespacedName types.NamespacedName) error {
	return r.UpdateNamespaceScope(ctx, namespacedName, true)
}

func (r *Reconciler) UpdateNamespaceScope(ctx context.Context, namespacedName types.NamespacedName, delete bool) error {
	dc := discovery.NewDiscoveryClientForConfigOrDie(r.Config)
	if exist, err := util.ResourceExists(dc, "operator.ibm.com/v1", "NamespaceScope"); err != nil {
		return errors.Wrap(err, "failed to check if the NamespaceScope api exist")
	} else if !exist {
		klog.V(1).Info("Not found NamespaceScope instance, ignore update it.")
		return nil
	}
	nsScope := &nssv1.NamespaceScope{}
	nsScopeKey := types.NamespacedName{Name: constant.NamespaceScopeCrName, Namespace: util.GetOperatorNamespace()}
	if err := r.Client.Get(ctx, nsScopeKey, nsScope); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(2).Infof("Not found NamespaceScope CR %s, ignore update it.", nsScopeKey.String())
			return nil
		}
		return err
	}
	var nsMems []string

	opreqList, err := r.ListOperandRequests(ctx, nil)
	if err != nil {
		return errors.Wrap(err, "failed to list OperandRequest")
	}

	nsSet := gset.NewSet()

	operatorNs := util.GetOperatorNamespace()
	if operatorNs != "" {
		nsSet.Add(operatorNs)
	}

	for _, opreq := range opreqList.Items {
		if delete && opreq.Namespace == namespacedName.Namespace && opreq.Name == namespacedName.Name {
			continue
		}
		nsSet.Add(opreq.Namespace)
	}

	for ns := range nsSet.Iter() {
		nsMems = append(nsMems, ns.(string))
	}

	if !util.StringSliceContentEqual(nsMems, nsScope.Spec.NamespaceMembers) {
		nsScope.Spec.NamespaceMembers = nsMems
		if err := r.Update(ctx, nsScope); err != nil {
			return errors.Wrapf(err, "failed to update NamespaceScope %s", nsScopeKey.String())
		}
		klog.V(1).Infof("Updated NamespaceScope %s", nsScopeKey.String())
	}
	return nil
}

func (r *Reconciler) addFinalizer(ctx context.Context, cr *operatorv1alpha1.OperandRequest) (bool, error) {
	if cr.GetDeletionTimestamp() == nil {
		added := cr.EnsureFinalizer()
		if added {
			// Update CR
			err := r.Update(ctx, cr)
			if err != nil {
				return false, errors.Wrapf(err, "failed to update the OperandRequest %s/%s", cr.Namespace, cr.Name)
			}
			return false, nil
		}
	}
	return true, nil
}

func (r *Reconciler) checkFinalizer(ctx context.Context, requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(2).Infof("Deleting OperandRequest %s in the namespace %s", requestInstance.Name, requestInstance.Namespace)
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
	if err := r.absentOperatorsAndOperands(ctx, requestInstance); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) getRegistryToRequestMapper() handler.ToRequestsFunc {
	ctx := context.Background()
	return func(object handler.MapObject) []ctrl.Request {
		requestList, _ := r.ListOperandRequestsByRegistry(ctx, types.NamespacedName{Namespace: object.Meta.GetNamespace(), Name: object.Meta.GetName()})

		requests := []ctrl.Request{}
		for _, request := range requestList {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := ctrl.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}

func (r *Reconciler) getConfigToRequestMapper() handler.ToRequestsFunc {
	ctx := context.Background()
	return func(object handler.MapObject) []ctrl.Request {
		requestList, _ := r.ListOperandRequestsByConfig(ctx, types.NamespacedName{Namespace: object.Meta.GetNamespace(), Name: object.Meta.GetName()})

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
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.OperandRequest{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandRegistry{}}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: r.getRegistryToRequestMapper(),
		}, builder.WithPredicates(predicate.Funcs{
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
		Watches(&source.Kind{Type: &operatorv1alpha1.OperandConfig{}}, &handler.EnqueueRequestsFromMapFunc{
			ToRequests: r.getConfigToRequestMapper(),
		}, builder.WithPredicates(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				// Evaluates to false if the object has been confirmed deleted.
				return !e.DeleteStateUnknown
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldObject := e.ObjectOld.(*operatorv1alpha1.OperandConfig)
				newObject := e.ObjectNew.(*operatorv1alpha1.OperandConfig)
				return !reflect.DeepEqual(oldObject.Spec, newObject.Spec)
			},
		})).Complete(r)
}
