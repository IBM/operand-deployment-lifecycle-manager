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

package namespacescope

import (
	"context"
	"time"

	gset "github.com/deckarep/golang-set"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nssv1 "github.com/IBM/ibm-namespace-scope-operator/api/v1"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

// Reconciler automagically updates NamespaceScope CR with the proporly namespace
type Reconciler struct {
	*deploy.ODLMOperator
}

// ReconcileOperandRequest reads that state of the cluster for OperandRequest object and update NamespaceScope CR based on the state read
func (r *Reconciler) ReconcileOperandRequest(req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
	// Creat context for the OperandBindInfo reconciler
	ctx := context.Background()

	exist, err := r.checkNamespaceScapeAPI()
	if err != nil {
		return ctrl.Result{}, err
	} else if !exist {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	nss, err := r.getNamespaceScopeCR(ctx)
	if err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	} else if nss == nil {
		klog.Warning("Not found NamespaceScope instance, ignore update it.")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// Fetch the OperandRequest instance
	requestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, requestInstance); err != nil {
		// Error reading the object - requeue the request.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	originalNss := nss.DeepCopy()

	var nsMems []string

	defer func() {
		if !util.StringSliceContentEqual(nsMems, nss.Spec.NamespaceMembers) {
			nss.Spec.NamespaceMembers = nsMems
			if err := r.Patch(ctx, nss, client.MergeFrom(originalNss)); err != nil {
				reconcileErr = errors.Wrapf(err, "failed to update NamespaceScope %s/%s", nss.Namespace, nss.Name)
			}
			klog.V(2).Infof("Updated NamespaceScope %s/%s", nss.Namespace, nss.Name)
		}
	}()

	// Remove finalizer when DeletionTimestamp none zero
	if !requestInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Check and remove namespaceMember from NamespaceScope CR
		nsMems, err = r.removeNamespaceMemberFromNamespaceScope(ctx, req)
		if err != nil {
			klog.Errorf("failed to remove NamespaceMember %s from NamespaceScope: %v", req.Namespace, err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	opreqNs, err := r.getOpreqNs(ctx)
	if err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	opregNs, err := r.getOpregNs(ctx)
	if err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	nsSet := gset.NewSet()

	for _, ns := range opreqNs {
		nsSet.Add(ns)
	}

	for _, ns := range opregNs {
		nsSet.Add(ns)
	}

	for ns := range nsSet.Iter() {
		nsMems = append(nsMems, ns.(string))
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) getOpreqNs(ctx context.Context) (nsMems []string, getOpreqNsErr error) {
	opreqList, err := r.ListOperandRequests(ctx, nil)
	if err != nil {
		getOpreqNsErr = errors.Wrap(err, "failed to list OperandRequest")
		return
	}

	nsSet := gset.NewSet()

	operatorNs := util.GetOperatorNamespace()
	if operatorNs != "" {
		nsSet.Add(operatorNs)
	}

	for _, opreq := range opreqList.Items {
		nsSet.Add(opreq.Namespace)
	}

	for ns := range nsSet.Iter() {
		nsMems = append(nsMems, ns.(string))
	}
	return
}

func (r *Reconciler) getOpregNs(ctx context.Context) (nsMems []string, getOpregNsErr error) {
	opregList, err := r.ListOperandRegistry(ctx, nil)
	if err != nil {
		getOpregNsErr = errors.Wrap(err, "failed to list OperandRegistry")
		return
	}

	nsSet := gset.NewSet()

	operatorNs := util.GetOperatorNamespace()
	if operatorNs != "" {
		nsSet.Add(operatorNs)
	}

	for _, opreq := range opregList.Items {
		for _, op := range opreq.Spec.Operators {
			if _, ok := opreq.Status.OperatorsStatus[op.Name]; ok {
				if op.InstallMode == operatorv1alpha1.InstallModeCluster {
					nsSet.Add(constant.ClusterOperatorNamespace)
				} else {
					nsSet.Add(op.Namespace)
				}
			}
		}
	}

	for ns := range nsSet.Iter() {
		nsMems = append(nsMems, ns.(string))
	}
	return
}

func (r *Reconciler) removeNamespaceMemberFromNamespaceScope(ctx context.Context, req ctrl.Request) (nsMems []string, err error) {
	klog.Infof("Deleting OperandRequest %s ...", req.NamespacedName.String())
	opreqList, err := r.ListOperandRequests(ctx, nil)
	if err != nil {
		err = errors.Wrap(err, "failed to list OperandRequest")
		return
	}

	nsSet := gset.NewSet()

	operatorNs := util.GetOperatorNamespace()
	if operatorNs != "" {
		nsSet.Add(operatorNs)
	}

	for _, opreq := range opreqList.Items {
		if opreq.Namespace == req.NamespacedName.Namespace && opreq.Name == req.NamespacedName.Name {
			continue
		}
		nsSet.Add(opreq.Namespace)
	}

	for ns := range nsSet.Iter() {
		nsMems = append(nsMems, ns.(string))
	}
	klog.Infof("namespace list %v ...", nsMems)

	return
}

func (r *Reconciler) checkNamespaceScapeAPI() (bool, error) {
	dc := discovery.NewDiscoveryClientForConfigOrDie(r.Config)
	if exist, err := util.ResourceExists(dc, "operator.ibm.com/v1", "NamespaceScope"); err != nil {
		err = errors.Wrap(err, "failed to check if the NamespaceScope api exist")
		klog.Error(err)
		return false, err
	} else if !exist {
		klog.Warning("Not found NamespaceScope API, ignore update it.")
		return false, nil
	}
	return true, nil
}

func (r *Reconciler) getNamespaceScopeCR(ctx context.Context) (*nssv1.NamespaceScope, error) {
	nsScope := &nssv1.NamespaceScope{}
	nsScopeKey := types.NamespacedName{Name: constant.NamespaceScopeCrName, Namespace: util.GetOperatorNamespace()}
	if err := r.Client.Get(ctx, nsScopeKey, nsScope); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Warningf("Not found NamespaceScope CR %s, ignore update it.", nsScopeKey.String())
			return nil, nil
		}
		return nil, err
	}
	return nsScope, nil
}

// SetupWithManager adds OperandBindInfo controller to the manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.OperandRequest{}).
		Complete(reconcile.Func(r.ReconcileOperandRequest))
	if err != nil {
		return err
	}

	return nil
}
