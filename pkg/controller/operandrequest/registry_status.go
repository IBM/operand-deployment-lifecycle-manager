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

	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileOperandRequest) updateRegistryStatus(cr *operatorv1alpha1.OperandRegistry, reconcileReq reconcile.Request, optName string, optPhase operatorv1alpha1.OperatorPhase) error {
	klog.V(3).Info("Updating OperandRegistry status")

	cr.SetOperatorStatus(optName, optPhase, reconcileReq)

	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileOperandRequest) deleteRegistryStatus(cr *operatorv1alpha1.OperandRegistry, reconcileReq reconcile.Request, optName string) error {
	klog.V(3).Info("Deleting OperandRegistry status")

	cr.CleanOperatorStatus(optName, reconcileReq)
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}
