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

	gset "github.com/deckarep/golang-set"
	olmv1 "github.com/operator-framework/api/pkg/operators/v1"
	olmv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	nssv1 "github.com/IBM/ibm-namespace-scope-operator/api/v1"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/controllers/common"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	util "github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

// Reconciler reconciles a OperandRequest object
type Reconciler struct {
	client.Client
	Config   *rest.Config
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
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
func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	// Fetch the OperandRequest instance
	requestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.Get(context.TODO(), req.NamespacedName, requestInstance); err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Add namespace member into NamespaceScope and check if has the update permission
	hasPermission, err := r.addPermission(req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !hasPermission {
		klog.Warningf("No permission to update OperandRequest")
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	klog.V(1).Infof("Reconciling OperandRequest: %s", req.NamespacedName)
	// Update labels for the request
	if requestInstance.UpdateLabels() {
		if err := r.Update(context.TODO(), requestInstance); err != nil {
			klog.Errorf("failed to update the labels for OperandRequest %s: %v", req.NamespacedName.String(), err)
			return ctrl.Result{}, err
		}
	}

	// Initialize the status for OperandRequest instance
	if !requestInstance.InitRequestStatus() {
		if err := r.updateOperandRequestStatus(requestInstance); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.addFinalizer(requestInstance); err != nil {
		klog.Errorf("failed to add finalizer for OperandRequest %s: %v", req.NamespacedName.String(), err)
		return ctrl.Result{}, err
	}

	// Remove finalizer when DeletionTimestamp none zero
	if !requestInstance.ObjectMeta.DeletionTimestamp.IsZero() {

		// Check and clean up the subscriptions
		err := r.checkFinalizer(requestInstance)
		if err != nil {
			klog.Errorf("failed to clean up the subscriptions for OperandRequest %s: %v", req.NamespacedName.String(), err)
			return ctrl.Result{}, err
		}

		// Check and remove namespaceMember from NamespaceScope CR
		if err := r.RemoveNamespaceMemberFromNamespaceScope(req.NamespacedName); err != nil {
			klog.Errorf("failed to remove NamespaceMember %s from NamespaceScope: %v", req.Namespace, err)
			return ctrl.Result{}, err
		}

		// Update finalizer to allow delete CR
		removed := requestInstance.RemoveFinalizer()
		if removed {
			err = r.Update(context.TODO(), requestInstance)
			if err != nil {
				klog.Errorf("failed to remove finalizer for OperandRequest %s: %v", req.NamespacedName.String(), err)
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if err := r.reconcileOperator(req.NamespacedName); err != nil {
		klog.Errorf("failed to reconcile Operators for OperandRequest %s: %v", req.NamespacedName.String(), err)
		return ctrl.Result{}, err
	}

	// Reconcile the Operand
	merr := r.reconcileOperand(req.NamespacedName)

	if len(merr.Errors) != 0 {
		klog.Errorf("failed to reconcile Operands for OperandRequest %s: %v", req.NamespacedName.String(), merr)
		return ctrl.Result{}, merr
	}

	// Check if all csv deploy succeed
	if requestInstance.Status.Phase != operatorv1alpha1.ClusterPhaseRunning {
		klog.V(2).Info("Waiting for all operators and operands to be deployed successfully ...")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	klog.V(1).Infof("Finished reconciling OperandRequest: %s", req.NamespacedName)
	return ctrl.Result{RequeueAfter: 30 * time.Minute}, nil
}

func (r *Reconciler) addPermission(req ctrl.Request) (bool, error) {
	if err := r.AddNamespaceMemberIntoNamespaceScope(req.NamespacedName); err != nil {
		klog.Errorf("failed to add NamespaceMember %s to NamespaceScope: %v", req.Namespace, err)
		return false, err
	}
	// Check update permission
	if !r.checkUpdateAuth(req.Namespace, "operator.ibm.com", "operandrequests") {
		return false, nil
	}
	if !r.checkUpdateAuth(req.Namespace, "operator.ibm.com", "operandrequests/status") {
		return false, nil
	}
	return true, nil
}

// Check if operator has permission to update OperandRequest
func (r *Reconciler) checkUpdateAuth(namespace, group, resource string) bool {
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

	if err := r.Create(context.TODO(), sar); err != nil {
		klog.Errorf("Failed to check operator update permission: %v", err)
		return false
	}

	klog.V(3).Infof("Operator update permission in namesapce %s, Allowed: %t, Denied: %t, Reason: %s", namespace, sar.Status.Allowed, sar.Status.Denied, sar.Status.Reason)
	return sar.Status.Allowed
}

func (r *Reconciler) AddNamespaceMemberIntoNamespaceScope(namespacedName types.NamespacedName) error {
	return r.UpdateNamespaceScope(namespacedName, false)
}

func (r *Reconciler) RemoveNamespaceMemberFromNamespaceScope(namespacedName types.NamespacedName) error {
	return r.UpdateNamespaceScope(namespacedName, true)
}

func (r *Reconciler) UpdateNamespaceScope(namespacedName types.NamespacedName, delete bool) error {
	dc := discovery.NewDiscoveryClientForConfigOrDie(r.Config)
	if exist, err := util.ResourceExists(dc, "operator.ibm.com/v1", "NamespaceScope"); err != nil {
		klog.Errorf("check resource NamespaceScope exist failed: %v", err)
		return err
	} else if !exist {
		klog.V(1).Info("Not found NamespaceScope instance, ignore update it.")
		return nil
	}
	nsScope := &nssv1.NamespaceScope{}
	nsScopeKey := types.NamespacedName{Name: constant.NamespaceScopeCrName, Namespace: util.GetOperatorNamespace()}
	if err := r.Get(context.TODO(), nsScopeKey, nsScope); err != nil {
		if errors.IsNotFound(err) {
			klog.V(2).Infof("Not found NamespaceScope CR %s, ignore update it.", nsScopeKey.String())
			return nil
		}
		return err
	}
	var nsMems []string

	opreqList, err := fetch.FetchAllOperandRequests(r.Client, nil)

	if err != nil {
		klog.Errorf("failed to list OperandRequest: %v", err)
		return err
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
		if err := r.Update(context.TODO(), nsScope); err != nil {
			klog.Errorf("failed to update NamespaceScope %s: %v", nsScopeKey.String(), err)
			return err
		}
		klog.V(1).Infof("Updated NamespaceScope %s", nsScopeKey.String())
	}
	return nil
}

func (r *Reconciler) addFinalizer(cr *operatorv1alpha1.OperandRequest) error {
	if cr.GetDeletionTimestamp() == nil {
		added := cr.EnsureFinalizer()
		if added {
			// Update CR
			err := r.Update(context.TODO(), cr)
			if err != nil {
				klog.Errorf("failed to update the OperandRequest %s in the namespace %s: %v", cr.Name, cr.Namespace, err)
				return err
			}
		}
	}
	return nil
}

func (r *Reconciler) checkFinalizer(requestInstance *operatorv1alpha1.OperandRequest) error {
	klog.V(2).Infof("Deleting OperandRequest %s in the namespace %s", requestInstance.Name, requestInstance.Namespace)
	existingSub := &olmv1alpha1.SubscriptionList{}

	opts := []client.ListOption{
		client.MatchingLabels(map[string]string{constant.OpreqLabel: "true"}),
	}

	if err := r.List(context.TODO(), existingSub, opts...); err != nil {
		return err
	}
	if len(existingSub.Items) == 0 {
		return nil
	}
	// Delete all the subscriptions that created by current request
	if err := r.absentOperatorsAndOperands(requestInstance); err != nil {
		return err
	}
	return nil
}

func getRegistryToRequestMapper(mgr manager.Manager) handler.ToRequestsFunc {
	return func(object handler.MapObject) []ctrl.Request {
		mgrClient := mgr.GetClient()
		requestList := &operatorv1alpha1.OperandRequestList{}
		opts := []client.ListOption{
			client.MatchingLabels(map[string]string{object.Meta.GetNamespace() + "." + object.Meta.GetName() + "/registry": "true"}),
		}

		_ = mgrClient.List(context.TODO(), requestList, opts...)

		requests := []ctrl.Request{}
		for _, request := range requestList.Items {
			namespaceName := types.NamespacedName{Name: request.Name, Namespace: request.Namespace}
			req := ctrl.Request{NamespacedName: namespaceName}
			requests = append(requests, req)
		}
		return requests
	}
}

func getConfigToRequestMapper(mgr manager.Manager) handler.ToRequestsFunc {
	return func(object handler.MapObject) []ctrl.Request {
		mgrClient := mgr.GetClient()
		requestList := &operatorv1alpha1.OperandRequestList{}
		opts := []client.ListOption{
			client.MatchingLabels(map[string]string{object.Meta.GetNamespace() + "." + object.Meta.GetName() + "/config": "true"}),
		}

		_ = mgrClient.List(context.TODO(), requestList, opts...)

		requests := []ctrl.Request{}
		for _, request := range requestList.Items {
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
			ToRequests: getRegistryToRequestMapper(mgr),
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
			ToRequests: getConfigToRequestMapper(mgr),
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

func (r *Reconciler) updateOperandRequestStatus(newRequestInstance *operatorv1alpha1.OperandRequest) error {
	err := wait.PollImmediate(time.Millisecond*250, time.Second*5, func() (bool, error) {
		existingRequestInstance, err := fetch.FetchOperandRequest(r.Client, types.NamespacedName{Name: newRequestInstance.Name, Namespace: newRequestInstance.Namespace})
		if err != nil {
			klog.Errorf("failed to fetch the existing OperandRequest: %v", err)
			return false, err
		}

		existingStatus := existingRequestInstance.Status.DeepCopy()
		newStatus := newRequestInstance.Status.DeepCopy()
		if reflect.DeepEqual(existingStatus, newStatus) {
			return true, nil
		}
		existingRequestInstance.Status = *newStatus
		if err := r.Status().Update(context.TODO(), existingRequestInstance); err != nil {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		klog.Errorf("failed to update OperandRequest %s/%s status: %v", newRequestInstance.Namespace, newRequestInstance.Name, err)
		return err
	}
	return nil
}
