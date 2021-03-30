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

package controllers

import (
	"context"
	"time"

	gset "github.com/deckarep/golang-set"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	nssv1 "github.com/IBM/ibm-namespace-scope-operator/api/v1"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	fetch "github.com/IBM/operand-deployment-lifecycle-manager/controllers/common"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/constant"
	"github.com/IBM/operand-deployment-lifecycle-manager/controllers/util"
)

// NamespaceScopeReconciler automagically updates NamespaceScope CR with the proporly namespace
type NamespaceScopeReconciler struct {
	*rest.Config
	Client   client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// ReconcileOperandRequest reads that state of the cluster for OperandRequest object and update NamespaceScope CR based on the state read
func (r *NamespaceScopeReconciler) ReconcileOperandRequest(req ctrl.Request) (_ ctrl.Result, reconcileErr error) {
	// Creat context for the namespacescope reconciler
	ctx := context.Background()

	exist, err := r.checkNamespaceScopeAPI()
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

	var nsMems []string

	originalNss := nss.DeepCopy()

	defer func() {
		nsMems, reconcileErr = r.updateNamespaceMemberFromNamespaceScope()
		if reconcileErr != nil {
			klog.Error(reconcileErr)
			return
		}
		if !util.StringSliceContentEqual(nsMems, nss.Spec.NamespaceMembers) {
			nss.Spec.NamespaceMembers = nsMems
			if err := r.Client.Patch(ctx, nss, client.MergeFrom(originalNss)); err != nil {
				reconcileErr = errors.Wrapf(err, "failed to update NamespaceScope %s/%s", nss.Namespace, nss.Name)
				klog.Error(reconcileErr)
				return
			}
			klog.V(2).Infof("Updated NamespaceScope %s/%s", nss.Namespace, nss.Name)
		}
	}()

	// Fetch the OperandRequest instance
	requestInstance := &operatorv1alpha1.OperandRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, requestInstance); err != nil {
		return
	}

	if !requestInstance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Wait OperandRequest is deleted
		err := wait.PollImmediate(time.Second*3, time.Minute*5, func() (bool, error) {
			err := r.Client.Get(ctx, req.NamespacedName, requestInstance)
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		})
		if err != nil {
			reconcileErr = err
		}
		return
	}
	return
}

func (r *NamespaceScopeReconciler) getOpreqNs() (nsMems []string, getOpreqNsErr error) {
	opreqList, err := fetch.FetchAllOperandRequests(r.Client, nil)
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

func (r *NamespaceScopeReconciler) getOpregNs() (nsMems []string, getOpregNsErr error) {
	opregList, err := fetch.FetchALLOperandRegistry(r.Client, nil)
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

func (r *NamespaceScopeReconciler) updateNamespaceMemberFromNamespaceScope() (nsMems []string, err error) {
	opreqNs, err := r.getOpreqNs()
	if err != nil {
		klog.Error(err)
		return
	}

	opregNs, err := r.getOpregNs()
	if err != nil {
		klog.Error(err)
		return
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

	return
}

func (r *NamespaceScopeReconciler) checkNamespaceScopeAPI() (bool, error) {
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

func (r *NamespaceScopeReconciler) getNamespaceScopeCR(ctx context.Context) (*nssv1.NamespaceScope, error) {
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

// SetupWithManager adds namespacescope controller to the manager.
func (r *NamespaceScopeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.OperandRequest{}).
		Complete(reconcile.Func(r.ReconcileOperandRequest))
	if err != nil {
		return err
	}

	return nil
}
