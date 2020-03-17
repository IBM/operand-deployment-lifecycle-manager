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

	operatorv1alpha1 "github.com/IBM/operand-deployment-lifecycle-manager/pkg/apis/operator/v1alpha1"
)

func (r *ReconcileOperandRequest) UpdateMemberStatus(cr *operatorv1alpha1.OperandRequest) error {
	klog.V(3).Info("Updating OperandRequest member status")
	for _, req := range cr.Spec.Requests {
		registryInstance, err := r.getRegistryInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return err
		}
		configInstance, err := r.getConfigInstance(req.Registry, req.RegistryNamespace)
		if err != nil {
			return err
		}

		for _, ro := range req.Operands {
			operatorPhase := registryInstance.Status.OperatorsStatus[ro.Name].Phase
			operandPhase := getOperandPhase(configInstance.Status.ServiceStatus[ro.Name].CrStatus)
			cr.SetMemberStatus(ro.Name, operatorPhase, operandPhase)
		}
	}
	cr.UpdateClusterPhase()
	if err := r.client.Status().Update(context.TODO(), cr); err != nil {
		return err
	}
	return nil
}

func getOperandPhase(sp map[string]operatorv1alpha1.ServicePhase) operatorv1alpha1.ServicePhase {
	klog.V(3).Info("Get OperandPhase for OperandRequest member status")
	operandStatusStat := struct {
		readyNum   int
		runningNum int
		failedNum  int
	}{
		readyNum:   0,
		runningNum: 0,
		failedNum:  0,
	}
	for _, v := range sp {
		switch v {
		case operatorv1alpha1.ServiceReady:
			operandStatusStat.readyNum++
		case operatorv1alpha1.ServiceRunning:
			operandStatusStat.runningNum++
		case operatorv1alpha1.ServiceFailed:
			operandStatusStat.failedNum++
		default:
		}
	}
	operandPhase := operatorv1alpha1.ServiceNone
	if operandStatusStat.failedNum > 0 {
		operandPhase = operatorv1alpha1.ServiceFailed
	} else if operandStatusStat.readyNum > 0 {
		operandPhase = operatorv1alpha1.ServiceReady
	} else if operandStatusStat.runningNum > 0 {
		operandPhase = operatorv1alpha1.ServiceRunning

	}
	return operandPhase
}
