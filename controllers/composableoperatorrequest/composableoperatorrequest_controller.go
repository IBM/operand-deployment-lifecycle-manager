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

package composableoperatorrequest

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/api/v1alpha1"
	deploy "github.com/IBM/operand-deployment-lifecycle-manager/controllers/operator"
)

// Reconciler reconciles a ComposableOperatorRequest object
type Reconciler struct {
	*deploy.ODLMOperator
}

// Reconcile
func (r *Reconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reconcileErr error) {

	// Creat context for the ComposableOperatorRequest reconciler
	ctx := context.Background()

	// Fetch the ComposableOperatorRequest instance
	requestInstance := &operatorv1alpha1.ComposableOperatorRequest{}
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

	// Initialize the status for OperandRequest instance
	if !requestInstance.InitRequestStatus() {
		requestInstance.Status.Phase = operatorv1alpha1.ComposableInstalling
		return ctrl.Result{Requeue: true}, nil
	}

	klog.V(1).Infof("Reconciling ComposableOperatorRequest: %s", req.NamespacedName)

	opreq, fn := r.buildOperandRequest(requestInstance)

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, opreq, fn)
	if err != nil {
		requestInstance.Status.Phase = operatorv1alpha1.ComposableFailed
		return ctrl.Result{}, err
	} else if result == controllerutil.OperationResultNone {
		return ctrl.Result{Requeue: true}, nil
	}

	requestInstance.Status.Phase = operatorv1alpha1.ComposableRunning
	klog.V(1).Infof("ComposableOperatorRequest: %s has been reconciled: %s", req.NamespacedName, result)
	return ctrl.Result{}, nil
}

func (r *Reconciler) buildOperandRequest(instance *operatorv1alpha1.ComposableOperatorRequest) (*operatorv1alpha1.OperandRequest, controllerutil.MutateFn) {
	opName := instance.Name
	opNs := instance.Namespace
	opreq := &operatorv1alpha1.OperandRequest{}
	opreq.SetName(opName)
	opreq.SetNamespace(opNs)

	fn := func() error {
		opreq.Spec = operatorv1alpha1.OperandRequestSpec{
			Requests: []operatorv1alpha1.Request{
				{
					Description:       "It is a request created from ComposableOperatorRequest " + opNs + "/" + opName,
					Registry:          opName,
					RegistryNamespace: opNs,
				},
			},
		}

		for _, op := range instance.Spec.ComposableComponents {
			if op.Enabled {
				opreq.Spec.Requests[0].Operands = append(opreq.Spec.Requests[0].Operands, operatorv1alpha1.Operand{Name: op.OperatorName})
			}
		}

		// Set the ComposableOperatorRequest as the controller of the OperandRequest
		if err := controllerutil.SetControllerReference(instance, opreq, r.Scheme); err != nil {
			return errors.Wrapf(err, "failed to set ComposableOperatorRequest %s/%s as the owner of OperatorRequest", instance.Namespace, instance.Name)
		}

		return nil
	}

	return opreq, fn
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.ComposableOperatorRequest{}).
		Complete(r)
}
